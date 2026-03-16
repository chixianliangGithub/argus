package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RCAService struct {
	DB        *gorm.DB
	AIClient  ai.LLMClient
	Bus       eventbus.EventBus
	DSFactory func(model.DataSource) (datasource.DataSource, error)
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func toJSONPreview(v interface{}, maxBytes int) map[string]interface{} {
	if maxBytes <= 0 {
		maxBytes = 200000
	}
	b, err := json.Marshal(v)
	if err != nil {
		s := truncateString(fmt.Sprint(v), maxBytes)
		return map[string]interface{}{
			"truncated": true,
			"bytes":     len(s),
			"preview":   s,
			"error":     err.Error(),
		}
	}
	if len(b) <= maxBytes {
		return map[string]interface{}{
			"truncated": false,
			"bytes":     len(b),
			"data":      json.RawMessage(b),
		}
	}
	return map[string]interface{}{
		"truncated":      true,
		"bytes":          len(b),
		"preview_bytes":  maxBytes,
		"preview_string": string(b[:maxBytes]),
	}
}

func NewRCAService(db *gorm.DB, bus eventbus.EventBus, aiClient ai.LLMClient) *RCAService {
	s := &RCAService{
		DB:        db,
		Bus:       bus,
		AIClient:  aiClient,
		DSFactory: datasource.NewDataSource,
	}
	s.subscribe()
	return s
}

func (s *RCAService) subscribe() {
	s.Bus.Subscribe("alert.firing", s.HandleAlertFiring)
}

func (s *RCAService) HandleAlertFiring(ctx context.Context, event eventbus.Event) error {
	alarmData, ok := event.Data.(model.Alarm)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Fetch full alarm with rule
	var alarm model.Alarm
	if err := s.DB.Preload("AlertRule").First(&alarm, alarmData.ID).Error; err != nil {
		logger.Log.Error("Failed to fetch alarm for RCA", zap.Uint("id", alarmData.ID), zap.Error(err))
		return err
	}

	// Check if RCA already exists for this alarm to avoid duplicate work
	var count int64
	s.DB.Model(&model.RCAReport{}).Where("alarm_id = ?", alarm.ID).Count(&count)
	if count > 0 {
		return nil
	}

	logger.Log.Info("Starting RCA for alarm", zap.Uint("alarm_id", alarm.ID))

	// 1. Gather Context
	logs := s.getMockLogs(alarm)
	commits := s.getMockCommits(alarm)
	metrics, err := s.getMetrics(ctx, alarm)
	if err != nil {
		logger.Log.Warn("Failed to gather metrics", zap.Error(err))
	}

	logsPreview := logs
	if len(logsPreview) > 50 {
		logsPreview = logsPreview[:50]
	}
	for i := range logsPreview {
		logsPreview[i] = truncateString(logsPreview[i], 500)
	}
	commitsPreview := commits
	if len(commitsPreview) > 50 {
		commitsPreview = commitsPreview[:50]
	}
	for i := range commitsPreview {
		commitsPreview[i] = truncateString(commitsPreview[i], 300)
	}
	metricsPreview := toJSONPreview(metrics, 200000)

	evidence := map[string]interface{}{
		"logs":    logsPreview,
		"commits": commitsPreview,
		"metrics": metricsPreview,
	}
	evidenceJSON, _ := json.Marshal(evidence)

	// 2. Construct Prompt
	prompt := s.constructPrompt(alarm, logsPreview, commitsPreview, metricsPreview)
	prompt = truncateString(prompt, 800000)

	// 3. Call AI
	aiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := s.AIClient.ChatCompletion(aiCtx, []ai.Message{
		{Role: "user", Content: prompt},
	})
	response := strings.TrimSpace(resp.Content)
	if err != nil {
		logger.Log.Warn("AI analysis failed", zap.Error(err))
		response = fmt.Sprintf("AI analysis failed: %s\n\nFallback Summary:\n- Check recent logs around %s\n- Review recent changes\n- Verify metric query manually\n", err.Error(), alarm.StartsAt.Format(time.RFC3339))
	}

	// 4. Save Report
	report := model.RCAReport{
		AlarmID:   alarm.ID,
		RootCause: response,
		Evidence:  string(evidenceJSON),
		CreatedAt: time.Now(),
	}
	if err := s.DB.Create(&report).Error; err != nil {
		logger.Log.Error("Failed to save RCA report", zap.Error(err))
		return err
	}

	logger.Log.Info("RCA report generated", zap.Uint("report_id", report.ID))
	return nil
}

func (s *RCAService) getMockLogs(alarm model.Alarm) []string {
	// Simulate fetching logs from Elasticsearch around the time of the alarm
	return []string{
		fmt.Sprintf("[%s] ERROR: Connection timeout to database", alarm.StartsAt.Format(time.RFC3339)),
		fmt.Sprintf("[%s] WARN: High latency detected", alarm.StartsAt.Add(-1*time.Minute).Format(time.RFC3339)),
		fmt.Sprintf("[%s] INFO: Request processed in 500ms", alarm.StartsAt.Add(-2*time.Minute).Format(time.RFC3339)),
	}
}

func (s *RCAService) getMockCommits(alarm model.Alarm) []string {
	// Simulate fetching recent git commits
	return []string{
		"feat: update database connection pool settings (sha: a1b2c3d)",
		"fix: resolve memory leak in worker (sha: e5f6g7h)",
		"chore: update dependencies (sha: i8j9k0l)",
	}
}

func (s *RCAService) getMetrics(ctx context.Context, alarm model.Alarm) (map[string]interface{}, error) {
	// Fetch the rule to get the query
	var rule model.AlertRule
	if err := s.DB.Preload("DataName.DataSource").First(&rule, alarm.AlertRuleID).Error; err != nil {
		return nil, err
	}

	ds, err := s.DSFactory(rule.DataName.DataSource)
	if err != nil {
		return nil, err
	}

	dsType := strings.ToLower(string(rule.DataName.DataSource.Type))
	if dsType == "mysql" || dsType == "sqlserver" || dsType == "clickhouse" {
		res, err := ds.Query(ctx, rule.Query)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"mode":   "instant",
			"query":  rule.Query,
			"values": toJSONPreview(res.Data, 200000),
		}, nil
	}

	end := alarm.StartsAt.Unix()
	start := end - 3600
	step := int64(60)
	result, err := ds.QueryRange(ctx, rule.Query, start, end, step)
	if err != nil {
		res, err2 := ds.Query(ctx, rule.Query)
		if err2 != nil {
			return nil, err
		}
		return map[string]interface{}{
			"mode":   "instant_fallback",
			"query":  rule.Query,
			"values": toJSONPreview(res.Data, 200000),
		}, nil
	}
	return map[string]interface{}{
		"mode":   "range",
		"query":  rule.Query,
		"values": toJSONPreview(result.Data, 200000),
	}, nil
}

func (s *RCAService) constructPrompt(alarm model.Alarm, logs []string, commits []string, metrics map[string]interface{}) string {
	alarmContent := truncateString(alarm.Content, 4000)
	prompt := fmt.Sprintf(`You are an SRE expert. Analyze the following alert and provide a Root Cause Analysis (RCA).

Alert Details:
- Rule: %s
- Status: %s
- Content: %s
- StartsAt: %s

Context:
1. Recent Logs:
%v

2. Recent Commits:
%v

3. Metric Data (Snapshot):
%v

Task:
- Identify the potential root cause.
- Suggest remediation steps.
- Provide a summary of the incident.
`, alarm.AlertRule.Name, alarm.Status, alarmContent, alarm.StartsAt, logs, commits, metrics)
	return prompt
}
