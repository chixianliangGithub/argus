package scheduler

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/notification"
	"github.com/argus-monitoring/argus/pkg/aiops"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Executor struct {
	DB        *gorm.DB
	RDB       *redis.Client
	Bus       eventbus.EventBus
	DSFactory func(model.DataSource) (datasource.DataSource, error)
}

func sqlWindowDebugEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ARGUS_DEBUG_SQL_WINDOW")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func truncateForLog(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func getRowValueCaseInsensitive(row map[string]interface{}, key string) (interface{}, bool) {
	if row == nil {
		return nil, false
	}
	if v, ok := row[key]; ok {
		return v, true
	}
	for k, v := range row {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

func parseAnyTime(v interface{}) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return time.Time{}, false
		}
		return t, true
	case *time.Time:
		if t == nil || t.IsZero() {
			return time.Time{}, false
		}
		return *t, true
	case int64:
		if t > 1_000_000_000_000 {
			return time.UnixMilli(t), true
		}
		if t > 0 {
			return time.Unix(t, 0), true
		}
		return time.Time{}, false
	case int:
		tt := int64(t)
		if tt > 1_000_000_000_000 {
			return time.UnixMilli(tt), true
		}
		if tt > 0 {
			return time.Unix(tt, 0), true
		}
		return time.Time{}, false
	case float64:
		tt := int64(t)
		if tt > 1_000_000_000_000 {
			return time.UnixMilli(tt), true
		}
		if tt > 0 {
			return time.Unix(tt, 0), true
		}
		return time.Time{}, false
	case json.Number:
		if tt, err := t.Int64(); err == nil {
			if tt > 1_000_000_000_000 {
				return time.UnixMilli(tt), true
			}
			if tt > 0 {
				return time.Unix(tt, 0), true
			}
		}
		return time.Time{}, false
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}, false
		}
		if tt, err := time.Parse(time.RFC3339, s); err == nil {
			return tt, true
		}
		if tt, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
			return tt, true
		}
		return time.Time{}, false
	case []byte:
		return parseAnyTime(string(t))
	default:
		return time.Time{}, false
	}
}

func NewExecutor(db *gorm.DB, rdb *redis.Client, bus eventbus.EventBus) *Executor {
	return &Executor{
		DB:        db,
		RDB:       rdb,
		Bus:       bus,
		DSFactory: datasource.NewDataSource,
	}
}

type AlgoConfig struct {
	Window                string  `json:"window"`
	Offset                string  `json:"offset"`
	ChangePercent         float64 `json:"change_percent"`
	Direction             string  `json:"direction"`
	N                     float64 `json:"n"`
	K                     float64 `json:"k"`
	SensitivityK          float64 `json:"sensitivity_k"`
	Seasonality           string  `json:"seasonality"`
	TrainingDays          int     `json:"training_days"`
	LookbackWindowMinutes int     `json:"lookback_window_minutes"`
	MinPoints             int     `json:"min_points"`
	Method                string  `json:"method"`
}

type Trigger struct {
	Metric       string
	Value        float64
	Threshold    float64
	ConditionStr string
	Labels       map[string]string
	Payload      string
	TriggerAt    time.Time
}

func (e *Executor) ExecuteRule(ruleID uint) {
	e.ExecuteRuleWithForce(ruleID, false)
}

type effectiveTimeConfig struct {
	Mode    string                `json:"mode"`
	Windows []effectiveTimeWindow `json:"windows"`
}

type effectiveTimeWindow struct {
	Days      []int  `json:"days"`
	Start     string `json:"start"`
	End       string `json:"end"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

func (e *Executor) ExecuteRuleWithForce(ruleID uint, force bool) {
	startedAt := time.Now()
	status := model.AlertEvalStatusRunning
	errorMessage := ""
	seriesCount := 0
	triggerCount := 0
	var evalAt *time.Time

	rec := model.AlertEvalRecord{
		AlertRuleID: ruleID,
		Forced:      force,
		Status:      model.AlertEvalStatusRunning,
		StartedAt:   startedAt,
	}
	if e.DB != nil {
		_ = e.DB.Create(&rec).Error
	}
	defer func() {
		if e.DB == nil || rec.ID == 0 {
			return
		}
		finishedAt := time.Now()
		if status == model.AlertEvalStatusRunning {
			status = model.AlertEvalStatusSuccess
		}
		updates := map[string]interface{}{
			"status":        status,
			"error_message": errorMessage,
			"series_count":  seriesCount,
			"trigger_count": triggerCount,
			"finished_at":   &finishedAt,
			"duration_ms":   finishedAt.Sub(startedAt).Milliseconds(),
		}
		if evalAt != nil {
			updates["eval_at"] = evalAt
		}
		_ = e.DB.Model(&model.AlertEvalRecord{}).Where("id = ?", rec.ID).Updates(updates).Error
	}()

	var rule model.AlertRule
	// Preload DataName and DataSource to get connection info
	if err := e.DB.Preload("DataName.DataSource").First(&rule, ruleID).Error; err != nil {
		logger.Log.Error("Failed to fetch rule", zap.Uint("rule_id", ruleID), zap.Error(err))
		status = model.AlertEvalStatusError
		errorMessage = err.Error()
		return
	}

	if !rule.IsEnabled {
		status = model.AlertEvalStatusSkippedDisabled
		return
	}

	if !force && !isWithinEffectiveTime(rule.EffectiveTime, time.Now()) {
		status = model.AlertEvalStatusSkippedOutOfEffectiveTime
		return
	}

	// Create DataSource instance
	ds, err := e.DSFactory(rule.DataName.DataSource)
	if err != nil {
		logger.Log.Error("Failed to create datasource", zap.Uint("rule_id", ruleID), zap.Error(err))
		status = model.AlertEvalStatusError
		errorMessage = err.Error()
		return
	}

	// Query Data (Current Value)
	queryCtx, queryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer queryCancel()

	query := rule.Query
	origQuery := query
	if rule.DataName.DataSource.Type == "mysql" || rule.DataName.DataSource.Type == "sqlserver" || rule.DataName.DataSource.Type == "clickhouse" {
		if rule.EvalWindow != "" && rule.EvalWindow != "0" {
			if strings.TrimSpace(rule.DataName.TimeField) == "" {
				if sqlWindowDebugEnabled() {
					logger.Log.Info(
						"SQL window skipped (time_field empty)",
						zap.Uint("rule_id", ruleID),
						zap.String("datasource_type", string(rule.DataName.DataSource.Type)),
						zap.String("eval_window", rule.EvalWindow),
						zap.String("query", truncateForLog(query, 4000)),
					)
				}
			} else if wd, err := time.ParseDuration(rule.EvalWindow); err == nil && wd > 0 {
				mins := int(wd.Minutes())
				if mins > 0 {
					if rule.DataName.DataSource.Type == "clickhouse" {
						expr := fmt.Sprintf("now() - INTERVAL %d MINUTE", mins)
						query = datasource.InjectSQLTimeFilter(query, rule.DataName.TimeField, expr)
					} else if rule.DataName.DataSource.Type == "sqlserver" {
						expr := fmt.Sprintf("DATEADD(MINUTE, -%d, GETDATE())", mins)
						query = datasource.InjectSQLTimeFilter(query, rule.DataName.TimeField, expr)
					} else {
						expr := fmt.Sprintf("NOW() - INTERVAL %d MINUTE", mins)
						query = datasource.InjectSQLTimeFilter(query, rule.DataName.TimeField, expr)
					}
				}
			}
		}
	}
	if sqlWindowDebugEnabled() && origQuery != query {
		logger.Log.Info(
			"SQL window injected",
			zap.Uint("rule_id", ruleID),
			zap.String("datasource_type", string(rule.DataName.DataSource.Type)),
			zap.String("time_field", rule.DataName.TimeField),
			zap.String("eval_window", rule.EvalWindow),
			zap.String("query_original", truncateForLog(origQuery, 4000)),
			zap.String("query_injected", truncateForLog(query, 4000)),
		)
	}

	result, err := ds.Query(queryCtx, query)
	if err != nil {
		logger.Log.Warn("Query failed (check datasource connection)", zap.Uint("rule_id", ruleID), zap.Error(err))
		status = model.AlertEvalStatusError
		errorMessage = err.Error()
		return
	}

	// Parse Result
	var sqlPayload string
	var sqlTriggerAt time.Time
	sqlRowLabels := map[string]string{}
	values, err := datasource.GetVectorValues(result)
	if err != nil {
		rows, ok := result.Data.([]map[string]interface{})
		if !ok {
			logger.Log.Warn("Failed to parse query result", zap.Uint("rule_id", ruleID), zap.Error(err))
			status = model.AlertEvalStatusError
			errorMessage = err.Error()
			return
		}

		if len(rows) == 0 {
			status = model.AlertEvalStatusNoData
			evalAt = func() *time.Time { t := time.Now(); return &t }()
			e.processTriggers(rule, nil, force)
			return
		}

		rowPick := rule.RowPick
		if rowPick == "" {
			if strings.TrimSpace(rule.DataName.TimeField) != "" {
				rowPick = "last"
			} else {
				rowPick = "first"
			}
		}

		sampleRow := rows[0]
		var sampleRowTime time.Time
		minRowTime := time.Time{}
		maxRowTime := time.Time{}
		foundTimeField := false
		timeParseFail := 0
		if tf := strings.TrimSpace(rule.DataName.TimeField); tf != "" {
			var minRow map[string]interface{}
			var maxRow map[string]interface{}
			for _, r := range rows {
				v, ok := getRowValueCaseInsensitive(r, tf)
				if !ok {
					continue
				}
				foundTimeField = true
				tt, ok := parseAnyTime(v)
				if !ok {
					timeParseFail++
					continue
				}
				if minRowTime.IsZero() || tt.Before(minRowTime) {
					minRowTime = tt
					minRow = r
				}
				if maxRowTime.IsZero() || tt.After(maxRowTime) {
					maxRowTime = tt
					maxRow = r
				}
			}
			if rowPick == "last" && maxRow != nil {
				sampleRow = maxRow
			} else if rowPick == "first" && minRow != nil {
				sampleRow = minRow
			} else if rowPick == "last" {
				sampleRow = rows[len(rows)-1]
			}
			if v, ok := getRowValueCaseInsensitive(sampleRow, tf); ok {
				if tt, ok := parseAnyTime(v); ok {
					sampleRowTime = tt
				}
			}
		} else if rowPick == "last" {
			sampleRow = rows[len(rows)-1]
		}
		sqlRowLabels = extractSQLRowLabels(sampleRow, rule.GroupBy)

		valueMode := rule.ValueMode
		if valueMode == "" {
			valueMode = "auto"
		}

		sqlTriggerAt = sampleRowTime
		if sqlWindowDebugEnabled() && strings.TrimSpace(rule.DataName.TimeField) != "" {
			logger.Log.Info(
				"SQL window eval",
				zap.Uint("rule_id", ruleID),
				zap.String("datasource_type", string(rule.DataName.DataSource.Type)),
				zap.String("time_field", rule.DataName.TimeField),
				zap.Bool("time_field_found", foundTimeField),
				zap.Int("time_parse_fail", timeParseFail),
				zap.String("row_pick", rowPick),
				zap.String("value_mode", valueMode),
				zap.Int("rows", len(rows)),
				zap.Time("min_row_time", minRowTime),
				zap.Time("max_row_time", maxRowTime),
				zap.Time("sample_row_time", sampleRowTime),
			)
		}

		var v float64
		var field string
		okNum := false

		if valueMode == "row_count" {
			if len(rows) > 0 {
				if vv, _, ok := datasource.GetSQLNumericFromRow(rows[0], "__count__"); ok {
					v = vv
				} else {
					v = float64(len(rows))
				}
			} else {
				v = 0
			}
			field = "__count__"
			okNum = true
		} else if valueMode == "field" {
			v, field, okNum = datasource.GetSQLNumericFromRow(sampleRow, rule.ValueField)
		} else {
			v, field, okNum = datasource.GetSQLNumericFromRow(sampleRow, rule.ValueField)
			if !okNum {
				v, field, okNum = datasource.GetSQLNumericFromRow(sampleRow, "")
			}
		}

		if !okNum {
			logger.Log.Warn("Failed to parse query result", zap.Uint("rule_id", ruleID), zap.Error(err))
			status = model.AlertEvalStatusError
			errorMessage = err.Error()
			return
		}

		metricName := "sql"
		if field == "__count__" {
			metricName = "sql{mode=\"row_count\"}"
		} else if field != "" {
			metricName = fmt.Sprintf("sql{field=%q}", field)
		}
		values = map[string]float64{metricName: v}

		limit := rows
		if len(limit) > 10 {
			limit = rows[:10]
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"row":         sampleRow,
			"rows":        limit,
			"count":       len(rows),
			"value":       v,
			"value_field": field,
			"value_mode":  valueMode,
			"row_pick":    rowPick,
			"trigger_at":  sqlTriggerAt,
		})
		sqlPayload = string(payload)
	}
	seriesCount = len(values)
	if seriesCount == 0 {
		status = model.AlertEvalStatusNoData
		evalAt = func() *time.Time { t := time.Now(); return &t }()
		e.processTriggers(rule, nil, force)
		return
	}

	// Apply eval window (avg over recent window) when possible
	if rule.EvalWindow != "" && rule.EvalWindow != "0" {
		if wd, err := time.ParseDuration(rule.EvalWindow); err == nil && wd > 0 {
			step := int64(60)
			if wd.Hours() > 24 {
				step = 300
			}
			endTs := time.Now().Unix()
			startTs := endTs - int64(wd.Seconds())
			rangeCtx, rangeCancel := context.WithTimeout(context.Background(), 45*time.Second)
			avgMap, err := queryWindowAvg(rangeCtx, ds, rule.Query, startTs, endTs, step)
			rangeCancel()
			if err == nil && len(avgMap) > 0 {
				values = avgMap
			}
		}
	}

	// Prepare history if needed
	var history map[string][]float64
	historyStep := int64(0)
	if rule.Algorithm == "3sigma" || rule.Algorithm == "mad" || rule.Algorithm == "baseline_auto" || rule.Algorithm == "baseline_sigma" || rule.Algorithm == "baseline_mad" {
		var config AlgoConfig
		_ = json.Unmarshal([]byte(rule.AlgoParams), &config)

		windowDuration := 3 * time.Hour
		if rule.Algorithm == "baseline_auto" || rule.Algorithm == "baseline_sigma" || rule.Algorithm == "baseline_mad" {
			seasonality := strings.ToLower(strings.TrimSpace(config.Seasonality))
			if seasonality == "daily" || seasonality == "weekly" {
				days := config.TrainingDays
				if days <= 0 {
					if seasonality == "weekly" {
						days = 14
					} else {
						days = 7
					}
				}
				windowDuration = time.Duration(days) * 24 * time.Hour
			}
		}
		if strings.TrimSpace(config.Window) != "" {
			if wd, err := time.ParseDuration(config.Window); err == nil && wd > 0 {
				windowDuration = wd
			}
		} else if config.LookbackWindowMinutes > 0 {
			windowDuration = time.Duration(config.LookbackWindowMinutes) * time.Minute
		}

		step := int64(60)
		if windowDuration.Hours() > 24 {
			step = 300
		}
		if rule.Algorithm == "baseline_auto" || rule.Algorithm == "baseline_sigma" || rule.Algorithm == "baseline_mad" {
			seasonality := strings.ToLower(strings.TrimSpace(config.Seasonality))
			if seasonality == "daily" || seasonality == "weekly" {
				step = 300
			}
		}
		historyStep = step
		end := time.Now().Unix() - step
		start := end - int64(windowDuration.Seconds())

		rangeTimeout := 45 * time.Second
		if windowDuration.Hours() > 24 {
			rangeTimeout = 90 * time.Second
		}
		rangeCtx, rangeCancel := context.WithTimeout(context.Background(), rangeTimeout)
		histResult, err := ds.QueryRange(rangeCtx, rule.Query, start, end, step)
		rangeCancel()
		if err != nil {
			logger.Log.Warn("QueryRange failed", zap.Uint("rule_id", ruleID), zap.Error(err))
		} else {
			history, err = datasource.GetMatrixValues(histResult)
			if err != nil {
				logger.Log.Warn("Failed to parse history result", zap.Uint("rule_id", ruleID), zap.Error(err))
			}
		}
	}

	// Collect Triggers
	var triggers []Trigger

	for metric, value := range values {
		triggered := false
		var thresholdVal float64
		var conditionStr string

		if rule.Algorithm == "baseline_auto" || rule.Algorithm == "baseline_sigma" || rule.Algorithm == "baseline_mad" {
			histData, ok := history[metric]
			if !ok || len(histData) < 2 {
				continue
			}
			var config AlgoConfig
			_ = json.Unmarshal([]byte(rule.AlgoParams), &config)
			k := config.SensitivityK
			if k == 0 {
				if config.N != 0 {
					k = config.N
				} else if config.K != 0 {
					k = config.K
				} else {
					k = 3
				}
			}

			method := strings.ToLower(strings.TrimSpace(config.Method))
			seasonality := strings.ToLower(strings.TrimSpace(config.Seasonality))
			if method == "" || method == "auto" {
				if seasonality == "daily" || seasonality == "weekly" {
					method = "hw"
				} else {
					method = chooseBaselineMethod(histData)
				}
			}
			if rule.Algorithm == "baseline_sigma" {
				method = "sigma"
			} else if rule.Algorithm == "baseline_mad" {
				method = "mad"
			}

			var isAnomaly bool
			var lower, upper float64
			pred := 0.0
			sigma := 0.0
			if method == "hw" {
				stepSeconds := historyStep
				if stepSeconds <= 0 {
					stepSeconds = 60
				}
				seasonLen := 0
				if seasonality == "daily" {
					seasonLen = int((24 * 60 * 60) / stepSeconds)
				} else if seasonality == "weekly" {
					seasonLen = int((7 * 24 * 60 * 60) / stepSeconds)
				}
				p, s, ok := aiops.HoltWintersAdditiveForecast(histData, seasonLen, 0.2, 0.01, 0.2)
				if ok && s > 0 {
					pred = p
					sigma = s
					lower = pred - k*sigma
					upper = pred + k*sigma
					isAnomaly = value < lower || value > upper
				} else {
					method = "sigma"
				}
			}
			if method == "mad" {
				isAnomaly, lower, upper = aiops.IsAnomalyMAD(value, histData, k)
			} else if method == "sigma" {
				isAnomaly, lower, upper = aiops.IsAnomaly3Sigma(value, histData, k)
			}
			if isAnomaly {
				triggered = true
				if value > upper {
					thresholdVal = upper
					if method == "hw" {
						conditionStr = fmt.Sprintf("> 基线上界[%.2f, %.2f] (k=%.1f, %s, pred=%.2f, σ=%.2f)", lower, upper, k, method, pred, sigma)
					} else {
						conditionStr = fmt.Sprintf("> 基线上界[%.2f, %.2f] (k=%.1f, %s)", lower, upper, k, method)
					}
				} else {
					thresholdVal = lower
					if method == "hw" {
						conditionStr = fmt.Sprintf("< 基线下界[%.2f, %.2f] (k=%.1f, %s, pred=%.2f, σ=%.2f)", lower, upper, k, method, pred, sigma)
					} else {
						conditionStr = fmt.Sprintf("< 基线下界[%.2f, %.2f] (k=%.1f, %s)", lower, upper, k, method)
					}
				}
			}
		} else if rule.Algorithm == "3sigma" {
			histData, ok := history[metric]
			if !ok || len(histData) < 2 {
				continue
			}
			var config AlgoConfig
			_ = json.Unmarshal([]byte(rule.AlgoParams), &config)
			n := config.N
			if n == 0 {
				n = 3
			}

			isAnomaly, lower, upper := aiops.IsAnomaly3Sigma(value, histData, n)
			if isAnomaly {
				triggered = true
				if value > upper {
					thresholdVal = upper
					conditionStr = fmt.Sprintf("> (3-sigma upper, n=%.1f)", n)
				} else {
					thresholdVal = lower
					conditionStr = fmt.Sprintf("< (3-sigma lower, n=%.1f)", n)
				}
			}
		} else if rule.Algorithm == "mad" {
			histData, ok := history[metric]
			if !ok || len(histData) == 0 {
				continue
			}
			var config AlgoConfig
			_ = json.Unmarshal([]byte(rule.AlgoParams), &config)
			k := config.K
			if k == 0 {
				k = 3
			}

			isAnomaly, lower, upper := aiops.IsAnomalyMAD(value, histData, k)
			if isAnomaly {
				triggered = true
				if value > upper {
					thresholdVal = upper
					conditionStr = fmt.Sprintf("> (MAD upper, k=%.1f)", k)
				} else {
					thresholdVal = lower
					conditionStr = fmt.Sprintf("< (MAD lower, k=%.1f)", k)
				}
			}
		} else if rule.Algorithm == "mom" || rule.Algorithm == "yoy" {
			var config AlgoConfig
			_ = json.Unmarshal([]byte(rule.AlgoParams), &config)
			windowDuration, err := time.ParseDuration(config.Window)
			if err != nil || windowDuration <= 0 {
				windowDuration = time.Hour
			}

			var offsetDuration time.Duration
			if rule.Algorithm == "mom" {
				offsetDuration = windowDuration
				if config.Offset != "" {
					if d, err := time.ParseDuration(config.Offset); err == nil && d > 0 {
						offsetDuration = d
					}
				}
			} else {
				offsetDuration = 365 * 24 * time.Hour
				if config.Offset != "" {
					if d, err := time.ParseDuration(config.Offset); err == nil && d > 0 {
						offsetDuration = d
					}
				}
			}

			change := config.ChangePercent
			if change == 0 {
				change = 20
			}
			direction := config.Direction
			if direction == "" {
				direction = "both"
			}

			step := int64(60)
			if windowDuration.Hours() > 24 {
				step = 300
			}

			endTs := time.Now().Unix()
			curStart := endTs - int64(windowDuration.Seconds())
			prevEnd := endTs - int64(offsetDuration.Seconds())
			prevStart := prevEnd - int64(windowDuration.Seconds())

			rangeCtx, rangeCancel := context.WithTimeout(context.Background(), 45*time.Second)
			curAvg, err := queryWindowAvg(rangeCtx, ds, rule.Query, curStart, endTs, step)
			if err != nil {
				rangeCancel()
				continue
			}
			prevAvg, err := queryWindowAvg(rangeCtx, ds, rule.Query, prevStart, prevEnd, step)
			rangeCancel()
			if err != nil {
				continue
			}

			cur, ok1 := curAvg[metric]
			prev, ok2 := prevAvg[metric]
			if !ok1 {
				cur = value
			}
			if !ok2 || prev == 0 {
				continue
			}
			pct := (cur - prev) / prev * 100

			switch direction {
			case "increase":
				triggered = pct >= change
			case "decrease":
				triggered = pct <= -change
			default:
				if pct < 0 {
					pct = -pct
				}
				triggered = pct >= change
			}

			thresholdVal = prev
			if rule.Algorithm == "mom" {
				conditionStr = fmt.Sprintf("环比变化 >= %.1f%%", change)
			} else {
				conditionStr = fmt.Sprintf("同比变化 >= %.1f%%", change)
			}
		} else {
			// Static
			thresholdVal = rule.Threshold
			conditionStr = rule.Condition
			switch rule.Condition {
			case ">":
				triggered = value > rule.Threshold
			case "<":
				triggered = value < rule.Threshold
			case "=":
				triggered = value == rule.Threshold
			case ">=":
				triggered = value >= rule.Threshold
			case "<=":
				triggered = value <= rule.Threshold
			}
		}

		if triggered {
			triggerAt := sqlTriggerAt
			if triggerAt.IsZero() {
				triggerAt = time.Now()
			}
			triggers = append(triggers, Trigger{
				Metric:       formatMetricForDisplay(rule.Query, metric),
				Value:        value,
				Threshold:    thresholdVal,
				ConditionStr: conditionStr,
				Labels:       mergeLabels(parseLabels(metric), sqlRowLabels),
				Payload:      mergeBaselinePayload(sqlPayload, rule, metric, value, thresholdVal, conditionStr, history, historyStep),
				TriggerAt:    triggerAt,
			})
		}
	}

	if !sqlTriggerAt.IsZero() {
		t := sqlTriggerAt
		evalAt = &t
	} else {
		t := time.Now()
		evalAt = &t
	}
	triggerCount = len(triggers)
	if triggerCount == 0 {
		status = model.AlertEvalStatusNoTrigger
	} else {
		status = model.AlertEvalStatusSuccess
	}

	// Group and Process Triggers
	e.processTriggers(rule, triggers, force)
}

func queryWindowAvg(ctx context.Context, ds datasource.DataSource, query string, start, end int64, step int64) (map[string]float64, error) {
	res, err := ds.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, err
	}
	values, err := datasource.GetMatrixValues(res)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64)
	for k, series := range values {
		if len(series) == 0 {
			continue
		}
		var sum float64
		for _, v := range series {
			sum += v
		}
		out[k] = sum / float64(len(series))
	}
	return out, nil
}

func isWithinEffectiveTime(raw string, now time.Time) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	var cfg effectiveTimeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return true
	}
	if cfg.Mode == "" || cfg.Mode == "none" || len(cfg.Windows) == 0 {
		return true
	}

	for _, w := range cfg.Windows {
		if cfg.Mode == "weekly" {
			wd := int(now.Weekday())
			if wd == 0 {
				wd = 7
			}
			if len(w.Days) > 0 {
				found := false
				for _, d := range w.Days {
					if d == wd {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}

		if cfg.Mode == "monthly" {
			day := now.Day()
			if len(w.Days) > 0 {
				found := false
				for _, d := range w.Days {
					if d == day {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}

		if cfg.Mode == "range" {
			if w.StartDate == "" || w.EndDate == "" {
				continue
			}
			sd, err1 := time.Parse("2006-01-02", w.StartDate)
			ed, err2 := time.Parse("2006-01-02", w.EndDate)
			if err1 != nil || err2 != nil {
				continue
			}
			ed = ed.Add(24*time.Hour - time.Nanosecond)
			if now.Before(sd) || now.After(ed) {
				continue
			}
		}

		if w.Start != "" && w.End != "" {
			st, err1 := time.Parse("15:04", w.Start)
			et, err2 := time.Parse("15:04", w.End)
			if err1 != nil || err2 != nil {
				return true
			}
			startMin := st.Hour()*60 + st.Minute()
			endMin := et.Hour()*60 + et.Minute()
			nowMin := now.Hour()*60 + now.Minute()
			if startMin <= endMin {
				if nowMin < startMin || nowMin > endMin {
					continue
				}
			} else {
				if nowMin > endMin && nowMin < startMin {
					continue
				}
			}
		}

		return true
	}

	return false
}

func parseLabels(metric string) map[string]string {
	labels := make(map[string]string)
	// Simple parser for metric{label="value",...}
	start := strings.Index(metric, "{")
	end := strings.LastIndex(metric, "}")
	if start == -1 || end == -1 || start >= end {
		return labels
	}
	content := metric[start+1 : end]
	pairs := strings.Split(content, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
			labels[key] = value
		}
	}
	return labels
}

func mergeLabels(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func chooseBaselineMethod(history []float64) string {
	if len(history) < 10 {
		return "sigma"
	}
	median := aiops.CalculateMedian(history)
	mad := aiops.CalculateMAD(history)
	if mad <= 0 {
		return "sigma"
	}
	maxDev := 0.0
	for _, v := range history {
		d := v - median
		if d < 0 {
			d = -d
		}
		if d > maxDev {
			maxDev = d
		}
	}
	if maxDev > 10*mad {
		return "mad"
	}
	return "sigma"
}

var simpleSubExprRE = regexp.MustCompile(`^\s*\(?\s*([0-9]+(?:\.[0-9]+)?)\s*-\s*([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(?:\{[^}]*\})?\s*\)?\s*$`)

func formatMetricForDisplay(query string, seriesMetric string) string {
	sm := strings.TrimSpace(seriesMetric)
	if sm == "sql" {
		return "result"
	}
	if strings.HasPrefix(sm, "sql{") {
		return "result" + strings.TrimPrefix(sm, "sql")
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return seriesMetric
	}
	for strings.HasPrefix(q, "(") && strings.HasSuffix(q, ")") {
		nq := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(q, "("), ")"))
		if nq == q || nq == "" {
			break
		}
		q = nq
	}
	sub := simpleSubExprRE.FindStringSubmatch(q)
	if len(sub) == 3 {
		constant := strings.TrimSpace(sub[1])
		baseMetric := strings.TrimSpace(sub[2])
		if baseMetric != "" {
			if strings.HasPrefix(seriesMetric, baseMetric+"{") || seriesMetric == baseMetric {
				return fmt.Sprintf("%s - %s", constant, seriesMetric)
			}
		}
	}
	return seriesMetric
}

func mergeBaselinePayload(sqlPayload string, rule model.AlertRule, metric string, value float64, threshold float64, conditionStr string, history map[string][]float64, historyStep int64) string {
	if rule.Algorithm != "baseline_auto" && rule.Algorithm != "baseline_sigma" && rule.Algorithm != "baseline_mad" {
		return sqlPayload
	}
	histData, ok := history[metric]
	if !ok || len(histData) < 2 {
		return sqlPayload
	}
	var config AlgoConfig
	_ = json.Unmarshal([]byte(rule.AlgoParams), &config)
	k := config.SensitivityK
	if k == 0 {
		if config.N != 0 {
			k = config.N
		} else if config.K != 0 {
			k = config.K
		} else {
			k = 3
		}
	}
	method := strings.ToLower(strings.TrimSpace(config.Method))
	seasonality := strings.ToLower(strings.TrimSpace(config.Seasonality))
	if method == "" || method == "auto" {
		if seasonality == "daily" || seasonality == "weekly" {
			method = "hw"
		} else {
			method = chooseBaselineMethod(histData)
		}
	}
	if rule.Algorithm == "baseline_sigma" {
		method = "sigma"
	} else if rule.Algorithm == "baseline_mad" {
		method = "mad"
	}

	pred := 0.0
	sigma := 0.0
	var lower, upper float64
	if method == "hw" {
		stepSeconds := historyStep
		if stepSeconds <= 0 {
			stepSeconds = 60
		}
		seasonLen := 0
		if seasonality == "daily" {
			seasonLen = int((24 * 60 * 60) / stepSeconds)
		} else if seasonality == "weekly" {
			seasonLen = int((7 * 24 * 60 * 60) / stepSeconds)
		}
		p, s, ok := aiops.HoltWintersAdditiveForecast(histData, seasonLen, 0.2, 0.01, 0.2)
		if ok && s > 0 {
			pred = p
			sigma = s
			lower = pred - k*sigma
			upper = pred + k*sigma
		} else {
			method = "sigma"
		}
	}
	if method == "mad" {
		_, lower, upper = aiops.IsAnomalyMAD(value, histData, k)
	} else if method == "sigma" {
		_, lower, upper = aiops.IsAnomaly3Sigma(value, histData, k)
	}

	mean := aiops.CalculateMean(histData)
	std := aiops.CalculateStdDev(histData)
	median := aiops.CalculateMedian(histData)
	mad := aiops.CalculateMAD(histData)

	baseline := map[string]interface{}{
		"method":        method,
		"k":             k,
		"seasonality":   seasonality,
		"lower":         lower,
		"upper":         upper,
		"mean":          mean,
		"std":           std,
		"median":        median,
		"mad":           mad,
		"pred":          pred,
		"sigma":         sigma,
		"value":         value,
		"threshold":     threshold,
		"condition_str": conditionStr,
	}

	out := map[string]interface{}{}
	if strings.TrimSpace(sqlPayload) != "" {
		_ = json.Unmarshal([]byte(sqlPayload), &out)
	}
	out["baseline"] = baseline
	b, err := json.Marshal(out)
	if err != nil {
		return sqlPayload
	}
	return string(b)
}

func stringifyLabelValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case []byte:
		return strings.TrimSpace(string(x))
	case json.Number:
		return strings.TrimSpace(x.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(x, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(x), 'f', -1, 64))
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case time.Time:
		return x.Format(time.RFC3339)
	case *time.Time:
		if x == nil {
			return ""
		}
		return x.Format(time.RFC3339)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func pickRowString(row map[string]interface{}, keys []string) string {
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if v, ok := row[k]; ok {
			s := stringifyLabelValue(v)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func extractSQLRowLabels(row map[string]interface{}, groupBy string) map[string]string {
	out := map[string]string{}
	if len(row) == 0 {
		return out
	}

	keys := []string{}
	if strings.TrimSpace(groupBy) != "" {
		keys = strings.Split(groupBy, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
	}
	for _, k := range keys {
		if k == "" {
			continue
		}
		if v, ok := row[k]; ok {
			if s := stringifyLabelValue(v); s != "" {
				out[k] = s
			}
		}
	}

	service := pickRowString(row, []string{"service", "service_name", "svc", "svc_name"})
	if service != "" {
		out["service"] = service
	}
	app := pickRowString(row, []string{"app", "app_name", "application"})
	if app != "" {
		out["app"] = app
	}
	cluster := pickRowString(row, []string{"cluster", "cluster_name"})
	if cluster != "" {
		out["cluster"] = cluster
	}
	env := pickRowString(row, []string{"env", "environment"})
	if env != "" {
		out["env"] = env
	}
	namespace := pickRowString(row, []string{"namespace", "ns"})
	if namespace != "" {
		out["namespace"] = namespace
	}
	job := pickRowString(row, []string{"job"})
	if job != "" {
		out["job"] = job
	}
	host := pickRowString(row, []string{"host", "hostname"})
	if host != "" {
		out["host"] = host
	}
	instance := pickRowString(row, []string{"instance"})
	if instance != "" {
		out["instance"] = instance
	}
	pod := pickRowString(row, []string{"pod"})
	if pod != "" {
		out["pod"] = pod
	}
	node := pickRowString(row, []string{"node"})
	if node != "" {
		out["node"] = node
	}

	return out
}

func extractServiceApp(labelsJSON string) (string, string) {
	s := strings.TrimSpace(labelsJSON)
	if s == "" {
		return "", ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return "", ""
	}
	getStr := func(key string) string {
		v, ok := m[key]
		if !ok || v == nil {
			return ""
		}
		if ss, ok := v.(string); ok {
			return strings.TrimSpace(ss)
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	service := getStr("service")
	if service == "" {
		service = getStr("svc")
	}
	app := getStr("app")
	if app == "" {
		app = getStr("application")
	}
	if app == "" {
		app = getStr("pp")
	}
	if app == "" {
		app = getStr("project")
	}
	if service == "" {
		service = app
	}
	return service, app
}

func buildGroupLabels(triggers []Trigger, groupByKeys []string) map[string]string {
	out := map[string]string{}
	if len(triggers) == 0 {
		return out
	}
	first := triggers[0].Labels
	if len(first) == 0 {
		return out
	}
	getSame := func(k string) (string, bool) {
		v0, ok := first[k]
		if !ok || strings.TrimSpace(v0) == "" {
			return "", false
		}
		for i := 1; i < len(triggers); i++ {
			v, ok := triggers[i].Labels[k]
			if !ok || v != v0 {
				return "", false
			}
		}
		return v0, true
	}

	for _, k := range groupByKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if v, ok := getSame(k); ok {
			out[k] = v
		}
	}

	preferred := []string{"env", "cluster", "namespace", "service", "app", "job", "host", "instance", "pod", "node"}
	for _, k := range preferred {
		if _, exists := out[k]; exists {
			continue
		}
		if v, ok := getSame(k); ok {
			out[k] = v
		}
	}

	return out
}

func (e *Executor) processTriggers(rule model.AlertRule, triggers []Trigger, force bool) {
	// 1. Group triggers
	groups := make(map[string][]Trigger)
	groupByKeys := []string{}
	if rule.GroupBy != "" {
		groupByKeys = strings.Split(rule.GroupBy, ",")
		for i := range groupByKeys {
			groupByKeys[i] = strings.TrimSpace(groupByKeys[i])
		}
	}
	if len(groupByKeys) == 0 && strings.EqualFold(strings.TrimSpace(string(rule.DataName.DataSource.Type)), "prometheus") {
		hasKey := func(k string) bool {
			for _, t := range triggers {
				if v, ok := t.Labels[k]; ok && strings.TrimSpace(v) != "" {
					return true
				}
			}
			return false
		}
		auto := []string{}
		if hasKey("service") {
			auto = append(auto, "service")
		}
		if hasKey("app") {
			auto = append(auto, "app")
		}
		if len(auto) == 0 {
			if hasKey("job") {
				auto = append(auto, "job")
			}
			if hasKey("instance") {
				auto = append(auto, "instance")
			}
		}
		if len(auto) > 0 {
			groupByKeys = auto
		}
	}

	for _, t := range triggers {
		groupKey := ""
		if len(groupByKeys) > 0 {
			var keys []string
			for _, k := range groupByKeys {
				if v, ok := t.Labels[k]; ok {
					keys = append(keys, fmt.Sprintf("%s=%s", k, v))
				}
			}
			sort.Strings(keys)
			groupKey = strings.Join(keys, ",")
			if groupKey == "" {
				groupKey = "default"
			}
		} else {
			groupKey = "default" // All in one group
		}
		groups[groupKey] = append(groups[groupKey], t)
	}

	// 2. Fetch active alarms for this rule
	var activeAlarms []model.Alarm
	if err := e.DB.Where("alert_rule_id = ? AND status = ?", rule.ID, model.AlarmStatusFiring).Find(&activeAlarms).Error; err != nil {
		logger.Log.Error("Failed to fetch active alarms", zap.Error(err))
		return
	}

	activeAlarmMap := make(map[string]*model.Alarm)
	for i := range activeAlarms {
		activeAlarmMap[activeAlarms[i].Fingerprint] = &activeAlarms[i]
	}

	processedFingerprints := make(map[string]bool)

	// Sort group keys for deterministic execution
	var groupKeys []string
	for k := range groups {
		groupKeys = append(groupKeys, k)
	}
	sort.Strings(groupKeys)

	// 3. Process each group
	for _, groupKey := range groupKeys {
		groupTriggers := groups[groupKey]
		fingerprint := generateFingerprint(rule.ID, groupKey)
		processedFingerprints[fingerprint] = true

		groupLabels := buildGroupLabels(groupTriggers, groupByKeys)

		// Aggregate content
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Rule: %s\n", rule.Name))
		if groupKey != "default" {
			sb.WriteString(fmt.Sprintf("Group: %s\n", groupKey))
		}
		for _, t := range groupTriggers {
			sb.WriteString(fmt.Sprintf("- Metric: %s, Value: %.2f %s %.2f\n", t.Metric, t.Value, t.ConditionStr, t.Threshold))
		}
		content := sb.String()

		// Check Deduplication (Silence)
		// Use labels from the first trigger for silence matching
		// In a real system, we might want to check against the specific grouping labels
		silenceLabels := groupLabels

		isSilenced := false
		if !force {
			isSilenced = e.checkSilence(rule, groupKey, silenceLabels)
		}
		silenceKey := fmt.Sprintf("alert:silence:%s", fingerprint)

		groupPayload := ""
		groupTriggerAt := time.Time{}
		groupSeenAt := time.Now()
		groupLabelsJSON := ""
		if len(groupLabels) > 0 {
			if b, err := json.Marshal(groupLabels); err == nil {
				groupLabelsJSON = string(b)
			}
		}
		if len(groupTriggers) > 0 {
			groupPayload = groupTriggers[0].Payload
			for _, t := range groupTriggers {
				if t.TriggerAt.IsZero() {
					continue
				}
				if groupTriggerAt.IsZero() || t.TriggerAt.After(groupTriggerAt) {
					groupTriggerAt = t.TriggerAt
				}
			}
			if !groupTriggerAt.IsZero() {
				groupSeenAt = groupTriggerAt
			}
		}

		alarm, exists := activeAlarmMap[fingerprint]
		if exists {
			// Update existing alarm
			if alarm.Content != content || alarm.Payload != groupPayload || alarm.Labels != groupLabelsJSON {
				alarm.Content = content
				alarm.Payload = groupPayload
				alarm.Labels = groupLabelsJSON
				e.DB.Save(alarm)
			}
			if !groupSeenAt.IsZero() && (alarm.LastSeenAt.IsZero() || groupSeenAt.After(alarm.LastSeenAt)) {
				alarm.LastSeenAt = groupSeenAt
				e.DB.Save(alarm)
			}
			if !isSilenced {
				logEntry := model.AlertLog{
					AlarmID:     alarm.ID,
					AlertRuleID: rule.ID,
					Status:      "firing",
					Message:     fmt.Sprintf("Level: %s\n%s", rule.Level, content),
					TriggeredAt: func() time.Time {
						if !groupTriggerAt.IsZero() {
							return groupTriggerAt
						}
						return time.Now()
					}(),
				}
				e.DB.Create(&logEntry)

				updates := map[string]interface{}{
					"fire_count":   gorm.Expr("fire_count + 1"),
					"last_seen_at": groupSeenAt,
				}
				if alarm.Service == "" || alarm.App == "" {
					svc, app := extractServiceApp(groupLabelsJSON)
					if alarm.Service == "" && svc != "" {
						updates["service"] = svc
						alarm.Service = svc
					}
					if alarm.App == "" && app != "" {
						updates["app"] = app
						alarm.App = app
					}
				}
				_ = e.DB.Model(&model.Alarm{}).
					Where("id = ?", alarm.ID).
					Updates(updates).Error
				alarm.FireCount = alarm.FireCount + 1
				alarm.LastSeenAt = groupSeenAt
				if rule.EscalationEnabled && rule.EscalationPolicyID > 0 {
					after := rule.EscalationAfter
					if after <= 0 {
						after = 1
					}
					if alarm.FireCount >= after {
						e.startEscalation(rule, *alarm)
					}
				}
				e.sendNotification(rule, *alarm)
				e.fireCallback(rule, *alarm, "firing")
				if !force {
					e.setSilence(silenceKey, rule.SilencePeriod)
				}
				if e.Bus != nil {
					e.Bus.Publish(context.Background(), "alert.firing", *alarm)
				}
			}
		} else {
			// Create new alarm
			if rule.EvalConsecutive > 1 && e.RDB != nil && !force {
				key := fmt.Sprintf("alert:consecutive:%s", fingerprint)
				cnt, _ := e.RDB.Incr(context.Background(), key).Result()
				e.RDB.Expire(context.Background(), key, 24*time.Hour)
				if int(cnt) < rule.EvalConsecutive {
					continue
				}
			}
			startAt := time.Now()
			if !groupTriggerAt.IsZero() {
				startAt = groupTriggerAt
			}
			svc, app := extractServiceApp(groupLabelsJSON)
			newAlarm := model.Alarm{
				AlertRuleID: rule.ID,
				Status:      model.AlarmStatusFiring,
				Content:     content,
				Payload:     groupPayload,
				Labels:      groupLabelsJSON,
				Service:     svc,
				App:         app,
				StartsAt:    startAt,
				LastSeenAt:  groupSeenAt,
				FireCount:   1,
				Fingerprint: fingerprint,
			}
			if err := e.DB.Create(&newAlarm).Error; err != nil {
				logger.Log.Error("Failed to create alarm", zap.Error(err))
				continue
			}

			if rule.EscalationEnabled && rule.EscalationPolicyID > 0 {
				after := rule.EscalationAfter
				if after <= 0 {
					after = 1
				}
				if newAlarm.FireCount >= after {
					e.startEscalation(rule, newAlarm)
				}
			}

			// Create Alert Log
			logEntry := model.AlertLog{
				AlarmID:     newAlarm.ID,
				AlertRuleID: rule.ID,
				Status:      "firing",
				Message:     fmt.Sprintf("Level: %s\n%s", rule.Level, content),
				TriggeredAt: func() time.Time {
					if !groupTriggerAt.IsZero() {
						return groupTriggerAt
					}
					return time.Now()
				}(),
			}
			e.DB.Create(&logEntry)

			if !isSilenced {
				e.sendNotification(rule, newAlarm)
				e.fireCallback(rule, newAlarm, "firing")
				if !force {
					e.setSilence(silenceKey, rule.SilencePeriod)
				}
				if e.Bus != nil {
					e.Bus.Publish(context.Background(), "alert.firing", newAlarm)
				}
			}
		}
	}

	// 4. Resolve alarms that are no longer triggering
	for fingerprint, alarm := range activeAlarmMap {
		if !processedFingerprints[fingerprint] {
			if e.RDB != nil {
				e.RDB.Del(context.Background(), fmt.Sprintf("alert:consecutive:%s", fingerprint))
			}
			// Resolve
			now := time.Now()
			alarm.Status = model.AlarmStatusResolved
			alarm.EndsAt = &now
			e.enrichResolvedEvalPayload(rule, alarm)
			e.DB.Save(alarm)

			if rule.NotifyOnResolve {
				e.sendNotification(rule, *alarm)
			}
			e.fireCallback(rule, *alarm, "resolved")

			if e.Bus != nil {
				e.Bus.Publish(context.Background(), "alert.resolved", *alarm)
			}

			// Log resolution
			logEntry := model.AlertLog{
				AlarmID:     alarm.ID,
				AlertRuleID: rule.ID,
				Status:      "resolved",
				Message:     fmt.Sprintf("Level: %s\nResolved", rule.Level),
				TriggeredAt: now,
			}
			e.DB.Create(&logEntry)
		}
	}
}

func generateFingerprint(ruleID uint, groupKey string) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%d:%s", ruleID, groupKey)))
	return hex.EncodeToString(hash[:])
}

func (e *Executor) enrichResolvedEvalPayload(rule model.AlertRule, alarm *model.Alarm) {
	if e == nil || alarm == nil {
		return
	}
	dsType := rule.DataName.DataSource.Type
	if dsType != "mysql" && dsType != "sqlserver" && dsType != "clickhouse" {
		return
	}
	if strings.TrimSpace(rule.EvalWindow) == "" || rule.EvalWindow == "0" {
		return
	}
	timeField := strings.TrimSpace(rule.DataName.TimeField)
	if timeField == "" {
		return
	}
	wd, err := time.ParseDuration(rule.EvalWindow)
	if err != nil || wd <= 0 {
		return
	}
	mins := int(wd.Minutes())
	if mins <= 0 {
		return
	}

	ds, err := e.DSFactory(rule.DataName.DataSource)
	if err != nil {
		return
	}

	rawQuery := rule.Query
	injected := rawQuery
	if dsType == "clickhouse" {
		injected = datasource.InjectSQLTimeFilter(injected, timeField, fmt.Sprintf("now() - INTERVAL %d MINUTE", mins))
	} else if dsType == "sqlserver" {
		injected = datasource.InjectSQLTimeFilter(injected, timeField, fmt.Sprintf("DATEADD(MINUTE, -%d, GETDATE())", mins))
	} else {
		injected = datasource.InjectSQLTimeFilter(injected, timeField, fmt.Sprintf("NOW() - INTERVAL %d MINUTE", mins))
	}

	query := injected
	if strings.TrimSpace(rule.GroupBy) == "" {
		if dsType == "clickhouse" {
			query = fmt.Sprintf("SELECT count() AS __count__, max(%s) AS __max_time__ FROM (%s) AS _argus_sub", timeField, injected)
		} else {
			query = fmt.Sprintf("SELECT COUNT(*) AS __count__, MAX(%s) AS __max_time__ FROM (%s) AS _argus_sub", timeField, injected)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := ds.Query(ctx, query)
	if err != nil && query != injected {
		res, err = ds.Query(ctx, injected)
	}
	if err != nil || res == nil || res.Data == nil {
		return
	}
	rows, ok := res.Data.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return
	}

	currentCount := 0
	currentLastTime := time.Time{}
	if v, _, ok := datasource.GetSQLNumericFromRow(rows[0], "__count__"); ok {
		currentCount = int(v)
		if tv, ok := getRowValueCaseInsensitive(rows[0], "__max_time__"); ok {
			if tt, ok := parseAnyTime(tv); ok {
				currentLastTime = tt
			}
		}
	} else {
		currentCount = len(rows)
		for _, r := range rows {
			if tv, ok := getRowValueCaseInsensitive(r, timeField); ok {
				if tt, ok := parseAnyTime(tv); ok && tt.After(currentLastTime) {
					currentLastTime = tt
				}
			}
		}
	}

	m := map[string]interface{}{}
	if strings.TrimSpace(alarm.Payload) != "" {
		_ = json.Unmarshal([]byte(alarm.Payload), &m)
	}
	m["current_eval_at"] = time.Now()
	m["current_count"] = currentCount
	m["current_value"] = float64(currentCount)
	m["current_value_field"] = "__count__"
	if !currentLastTime.IsZero() {
		m["current_trigger_at"] = currentLastTime
	}
	if b, err := json.Marshal(m); err == nil {
		alarm.Payload = string(b)
	}
}

func (e *Executor) isInhibited(rule model.AlertRule, alarm model.Alarm) (bool, uint, string, uint, string) {
	if e == nil || e.DB == nil {
		return false, 0, "", 0, ""
	}
	if rule.TeamID == 0 {
		return false, 0, "", 0, ""
	}

	var inhibitRules []model.InhibitRule
	if err := e.DB.Where("team_id = ? AND is_enabled = ?", rule.TeamID, true).
		Order("priority desc, created_at desc").
		Find(&inhibitRules).Error; err != nil || len(inhibitRules) == 0 {
		return false, 0, "", 0, ""
	}

	targetLabels := map[string]string{}
	if strings.TrimSpace(alarm.Labels) != "" {
		_ = json.Unmarshal([]byte(alarm.Labels), &targetLabels)
	}
	targetLabels["rule_name"] = rule.Name
	targetLabels["level"] = rule.Level
	targetLabels["status"] = string(alarm.Status)
	if strings.TrimSpace(alarm.Service) != "" {
		targetLabels["service"] = alarm.Service
	}
	if strings.TrimSpace(alarm.App) != "" {
		targetLabels["app"] = alarm.App
	}

	matchersOK := func(raw string, labels map[string]string) bool {
		s := strings.TrimSpace(raw)
		if s == "" || s == "[]" {
			return true
		}
		var mms []model.Matcher
		if err := json.Unmarshal([]byte(s), &mms); err != nil {
			return false
		}
		for _, mm := range mms {
			name := strings.TrimSpace(mm.Name)
			if name == "" {
				return false
			}
			val, ok := labels[name]
			if !ok {
				return false
			}
			if mm.IsRegex {
				re, err := regexp.Compile(mm.Value)
				if err != nil || !re.MatchString(val) {
					return false
				}
			} else {
				if val != mm.Value {
					return false
				}
			}
		}
		return true
	}

	type sourceRow struct {
		ID        uint
		Labels    string
		Service   string
		App       string
		RuleName  string
		RuleLevel string
	}
	var sources []sourceRow
	_ = e.DB.Table("alarms").
		Select("alarms.id, alarms.labels, alarms.service, alarms.app, alert_rules.name as rule_name, alert_rules.level as rule_level").
		Joins("JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.status = ? AND alert_rules.team_id = ?", model.AlarmStatusFiring, rule.TeamID).
		Where("alarms.id <> ?", alarm.ID).
		Order("alarms.last_seen_at desc").
		Limit(2000).
		Find(&sources).Error

	for _, ir := range inhibitRules {
		if !matchersOK(ir.Target, targetLabels) {
			continue
		}
		eq := []string{}
		for _, p := range strings.Split(ir.EqualLabels, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				eq = append(eq, p)
			}
		}
		for _, src := range sources {
			sourceLabels := map[string]string{}
			if strings.TrimSpace(src.Labels) != "" {
				_ = json.Unmarshal([]byte(src.Labels), &sourceLabels)
			}
			sourceLabels["rule_name"] = src.RuleName
			sourceLabels["level"] = src.RuleLevel
			sourceLabels["status"] = string(model.AlarmStatusFiring)
			if strings.TrimSpace(src.Service) != "" {
				sourceLabels["service"] = src.Service
			}
			if strings.TrimSpace(src.App) != "" {
				sourceLabels["app"] = src.App
			}
			if !matchersOK(ir.Source, sourceLabels) {
				continue
			}
			okEq := true
			for _, k := range eq {
				tv, okT := targetLabels[k]
				sv, okS := sourceLabels[k]
				if !okT || !okS || tv != sv {
					okEq = false
					break
				}
			}
			if okEq {
				return true, ir.ID, ir.Name, src.ID, src.RuleName
			}
		}
	}

	return false, 0, "", 0, ""
}

func (e *Executor) setSilence(key string, minutes int) {
	if e.RDB == nil {
		return
	}
	if minutes <= 0 {
		return
	}
	e.RDB.Set(context.Background(), key, "1", time.Duration(minutes)*time.Minute)
}

func (e *Executor) checkSilence(rule model.AlertRule, groupKey string, labels map[string]string) bool {
	allLabels := make(map[string]string, len(labels)+5)
	for k, v := range labels {
		allLabels[k] = v
	}
	allLabels["alert_rule_id"] = strconv.FormatUint(uint64(rule.ID), 10)
	allLabels["team_id"] = strconv.FormatUint(uint64(rule.TeamID), 10)
	allLabels["level"] = rule.Level
	allLabels["rule_name"] = rule.Name
	allLabels["fingerprint"] = generateFingerprint(rule.ID, groupKey)

	// 1. Check Redis-based simple silence (cooldown)
	if rule.SilencePeriod > 0 {
		fingerprint := generateFingerprint(rule.ID, groupKey)
		redisKey := fmt.Sprintf("alert:silence:%s", fingerprint)
		if e.RDB != nil {
			exists, _ := e.RDB.Exists(context.Background(), redisKey).Result()
			if exists > 0 {
				return true
			}
		}
	}

	// 2. Check rule-linked advanced Silence Rule
	if rule.SilenceRuleID == 0 {
		return false
	}

	var silences []model.SilenceRule
	now := time.Now()
	if err := e.DB.Where("id = ? AND team_id = ? AND starts_at <= ? AND ends_at >= ?", rule.SilenceRuleID, rule.TeamID, now, now).Find(&silences).Error; err != nil {
		logger.Log.Error("Failed to fetch silence rules", zap.Error(err))
		return false
	}

	for _, s := range silences {
		var matchers []model.Matcher
		if err := json.Unmarshal([]byte(s.Matchers), &matchers); err != nil {
			continue
		}

		matched := true
		if len(matchers) == 0 {
			return true
		}
		for _, mm := range matchers {
			val, ok := allLabels[mm.Name]
			if !ok {
				matched = false
				break
			}
			if mm.IsRegex {
				re, err := regexp.Compile(mm.Value)
				if err != nil || !re.MatchString(val) {
					matched = false
					break
				}
			} else {
				if val != mm.Value {
					matched = false
					break
				}
			}
		}
		if matched {
			return true
		}
	}

	return false
}

func (e *Executor) sendNotification(rule model.AlertRule, alarm model.Alarm) {
	var channels []model.NotificationChannel

	// 1. Determine Notification Channels
	// Check rule-specific props first
	var props struct {
		ChannelIDs     []uint `json:"channel_ids"`
		IsEscalation   bool   `json:"escalation"`
		EscalationStep int    `json:"escalation_step"`
	}
	notificationConfigRaw := rule.NotificationConfig
	var routingChannelIDs []uint
	routingOverrideChannels := true

	var routingRules []model.RoutingRule
	if e.DB != nil && rule.TeamID > 0 {
		_ = e.DB.Where("team_id = ? AND is_enabled = ?", rule.TeamID, true).
			Order("priority desc, created_at desc").
			Find(&routingRules).Error
	}
	if len(routingRules) > 0 {
		// 路由匹配时会把 alarm.labels 与一些“便捷字段”合并成 labels 字典：
		// - alarm.labels：评估时根据 group_by keys 生成（只有组内一致的 label 才会被写入）
		// - rule_name / level：来自告警规则
		// - status：来自 alarm.status（firing/resolved）
		// - service / app：来自 alarm.service/app（从 labels 中抽取并落库）
		labels := map[string]string{}
		if strings.TrimSpace(alarm.Labels) != "" {
			_ = json.Unmarshal([]byte(alarm.Labels), &labels)
		}
		labels["rule_name"] = rule.Name
		labels["level"] = rule.Level
		labels["status"] = string(alarm.Status)
		if strings.TrimSpace(alarm.Service) != "" {
			labels["service"] = alarm.Service
		}
		if strings.TrimSpace(alarm.App) != "" {
			labels["app"] = alarm.App
		}

		matchRule := func(rr model.RoutingRule) bool {
			// 多个 matcher 条件之间为且关系（AND）：
			// - 任一条件不满足，则该 rr 不命中；
			// - matchers 为空表示命中全部；
			// - 字段不存在（labels 里无该 key）表示不命中；
			// - isRegex=true 时使用正则，否则使用等值匹配。
			if !rr.IsEnabled {
				return false
			}
			raw := strings.TrimSpace(rr.Matchers)
			if raw == "" || raw == "[]" {
				return true
			}
			var mms []model.Matcher
			if err := json.Unmarshal([]byte(raw), &mms); err != nil {
				return false
			}
			for _, mm := range mms {
				name := strings.TrimSpace(mm.Name)
				if name == "" {
					return false
				}
				val, ok := labels[name]
				if !ok {
					return false
				}
				if mm.IsRegex {
					re, err := regexp.Compile(mm.Value)
					if err != nil || !re.MatchString(val) {
						return false
					}
				} else {
					if val != mm.Value {
						return false
					}
				}
			}
			return true
		}

		for _, rr := range routingRules {
			if !matchRule(rr) {
				continue
			}
			if strings.TrimSpace(rr.NotificationProps) != "" {
				var rp struct {
					ChannelIDs []uint `json:"channel_ids"`
					Override   *bool  `json:"override"`
				}
				if err := json.Unmarshal([]byte(rr.NotificationProps), &rp); err == nil && len(rp.ChannelIDs) > 0 {
					routingChannelIDs = rp.ChannelIDs
				}
				// 路由“通道覆盖开关”：
				// - override=true(默认)：命中路由后，仅使用路由配置的通道（覆盖规则的基础通道）
				// - override=false：命中路由后，同时发送到“规则基础通道 + 路由通道”（去重叠加）
				if rp.Override != nil {
					routingOverrideChannels = *rp.Override
				}
			}
			if strings.TrimSpace(rr.NotificationConfig) != "" {
				notificationConfigRaw = rr.NotificationConfig
			}
			break
		}
	}

	if alarm.Status == model.AlarmStatusFiring {
		inhibited, inhibitRuleID, inhibitRuleName, sourceAlarmID, sourceRuleName := e.isInhibited(rule, alarm)
		if inhibited {
			inhibitRuleName = strings.TrimSpace(inhibitRuleName)
			sourceRuleName = strings.TrimSpace(sourceRuleName)
			rn := "-"
			if inhibitRuleName != "" {
				rn = inhibitRuleName
			}
			srn := "-"
			if sourceRuleName != "" {
				srn = sourceRuleName
			}
			_ = e.DB.Create(&model.AlertLog{
				AlarmID:     alarm.ID,
				AlertRuleID: alarm.AlertRuleID,
				Status:      "inhibited",
				Message:     fmt.Sprintf("inhibited by rule=%s(inhibit_rule_id=%d) source_alarm_id=%d source_rule=%s", rn, inhibitRuleID, sourceAlarmID, srn),
				TriggeredAt: time.Now(),
			}).Error
			return
		}
	}

	useRuleChannels := false
	explicitRuleProps := false
	if strings.TrimSpace(rule.NotificationProps) != "" {
		explicitRuleProps = true
		if err := json.Unmarshal([]byte(rule.NotificationProps), &props); err != nil {
			logger.Log.Warn("Failed to parse notification props", zap.Uint("rule_id", rule.ID), zap.Error(err))
			return
		}
	}
	if len(routingChannelIDs) > 0 {
		if routingOverrideChannels {
			props.ChannelIDs = routingChannelIDs
		} else {
			// 叠加模式：合并规则基础通道与路由通道（去重）
			seen := map[uint]struct{}{}
			merged := make([]uint, 0, len(props.ChannelIDs)+len(routingChannelIDs))
			for _, id := range props.ChannelIDs {
				if id == 0 {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				merged = append(merged, id)
			}
			for _, id := range routingChannelIDs {
				if id == 0 {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				merged = append(merged, id)
			}
			props.ChannelIDs = merged
		}
	}
	if explicitRuleProps && len(props.ChannelIDs) == 0 && len(routingChannelIDs) == 0 {
		return
	}
	if len(props.ChannelIDs) > 0 {
		useRuleChannels = true
		e.DB.Where("id IN ?", props.ChannelIDs).Find(&channels)
	}
	isEscalation := props.IsEscalation

	// Fallback to Team channels
	if !useRuleChannels {
		if err := e.DB.Where("team_id = ?", rule.TeamID).Find(&channels).Error; err != nil {
			logger.Log.Error("Failed to fetch notification channels", zap.Error(err))
			return
		}
	}

	// 2. Determine Message Template
	var tpl model.MessageTemplate
	if rule.MessageTemplateID > 0 {
		e.DB.First(&tpl, rule.MessageTemplateID)
	}
	// Fallback to default if not found or not set
	if tpl.ID == 0 {
		e.DB.Where("is_default = ?", true).First(&tpl)
	}

	// 3. Render Template
	sendAt := time.Now()
	content := fmt.Sprintf("[Argus Alarm]\nRule: %s\nStatus: %s\nStartsAt: %s\nLastSeenAt: %s\nSendAt: %s\n\n%s",
		rule.Name,
		alarm.Status,
		alarm.StartsAt.Format(time.RFC3339),
		alarm.LastSeenAt.Format(time.RFC3339),
		sendAt.Format(time.RFC3339),
		alarm.Content,
	)
	format := "text"
	type alarmPayload struct {
		Row            map[string]interface{}   `json:"row"`
		Rows           []map[string]interface{} `json:"rows"`
		Count          int                      `json:"count"`
		Value          float64                  `json:"value"`
		ValueField     string                   `json:"value_field"`
		TriggerAt      time.Time                `json:"trigger_at"`
		CurrentEvalAt  time.Time                `json:"current_eval_at"`
		CurrentCount   *int                     `json:"current_count"`
		CurrentValue   *float64                 `json:"current_value"`
		CurrentField   string                   `json:"current_value_field"`
		CurrentTrigger time.Time                `json:"current_trigger_at"`
	}
	var ap alarmPayload
	if alarm.Payload != "" {
		_ = json.Unmarshal([]byte(alarm.Payload), &ap)
	}

	timeWindow := rule.EvalWindow
	if timeWindow == "" || timeWindow == "0" {
		timeWindow = "-"
	}
	alertSilence := rule.SilencePeriod
	eventAt := alarm.StartsAt
	if !ap.TriggerAt.IsZero() {
		eventAt = ap.TriggerAt
	}

	legacyTplReplacer := strings.NewReplacer(
		"${ALERT_SILENCE}", "{{ .AlertSilence }}",
		"${TIME_WINDOW}", "{{ .TimeWindow }}",
		"${NUMBER}", "{{ .Number }}",
		"${TRIGGER_AT}", "{{ .TriggerAt }}",
		"${SEND_AT}", "{{ .SendAt }}",
		"${ALERT_LEVEL}", "{{ .AlertLevel }}",
	)
	if tpl.ID > 0 {
		if tpl.Format != "" {
			format = tpl.Format
		}
		tplContent := legacyTplReplacer.Replace(tpl.Content)
		funcs := template.FuncMap{
			"truncate": func(v interface{}, n int) string {
				if n <= 0 {
					return ""
				}
				if v == nil {
					return ""
				}
				s := fmt.Sprint(v)
				r := []rune(s)
				if len(r) <= n {
					return s
				}
				if n >= 3 {
					return string(r[:n-3]) + "..."
				}
				return string(r[:n])
			},
			"default": func(def interface{}, v interface{}) string {
				if v == nil {
					return fmt.Sprint(def)
				}
				s := strings.TrimSpace(fmt.Sprint(v))
				if s == "" {
					return fmt.Sprint(def)
				}
				return s
			},
		}
		t, err := template.New("msg").Funcs(funcs).Parse(tplContent)
		if err == nil {
			var buf bytes.Buffer
			data := map[string]interface{}{
				"Rule":         rule,
				"Alarm":        alarm,
				"Content":      alarm.Content,
				"TriggerAt":    alarm.StartsAt,
				"StartsAt":     alarm.StartsAt,
				"LastSeenAt":   alarm.LastSeenAt,
				"ResolvedAt":   alarm.EndsAt,
				"EventAt":      eventAt,
				"SendAt":       sendAt,
				"Row":          ap.Row,
				"Rows":         ap.Rows,
				"Count":        ap.Count,
				"Value":        ap.Value,
				"ValueField":   ap.ValueField,
				"AlertLevel":   rule.Level,
				"AlertSilence": alertSilence,
				"TimeWindow":   timeWindow,
				"Number":       ap.Count,
			}
			if err := t.Execute(&buf, data); err == nil {
				content = buf.String()
			} else {
				logger.Log.Error("Failed to render template", zap.Error(err))
			}
		}
	}

	levelText := rule.Level
	if rule.Level == "critical" {
		levelText = "严重"
	} else if rule.Level == "warning" {
		levelText = "警告"
	} else if rule.Level == "info" {
		levelText = "信息"
	}

	statusText := string(alarm.Status)
	if alarm.Status == model.AlarmStatusFiring {
		statusText = "触发"
	} else if alarm.Status == model.AlarmStatusResolved {
		statusText = "恢复"
	}

	metaLines := []string{
		fmt.Sprintf("状态: %s", statusText),
		fmt.Sprintf("级别: %s", levelText),
		fmt.Sprintf("触发时间: %s", alarm.StartsAt.Format(time.RFC3339)),
		fmt.Sprintf("最近命中时间: %s", alarm.LastSeenAt.Format(time.RFC3339)),
	}
	if alarm.Status == model.AlarmStatusResolved && alarm.EndsAt != nil && !alarm.EndsAt.IsZero() {
		metaLines = append(metaLines, fmt.Sprintf("恢复时间: %s", alarm.EndsAt.Format(time.RFC3339)))
		if ap.CurrentCount != nil || ap.CurrentValue != nil {
			if ap.CurrentCount != nil {
				metaLines = append(metaLines, fmt.Sprintf("本次评估命中数: %d", *ap.CurrentCount))
			}
			if ap.CurrentValue != nil {
				field := strings.TrimSpace(ap.CurrentField)
				if field == "" {
					field = "__count__"
				}
				metaLines = append(metaLines, fmt.Sprintf("本次评估值: %.4f (%s)", *ap.CurrentValue, field))
			}
			if !ap.CurrentTrigger.IsZero() {
				metaLines = append(metaLines, fmt.Sprintf("本次评估最新时间: %s", ap.CurrentTrigger.Format(time.RFC3339)))
			}
		}
		metaLines = append(metaLines, "说明: 正文的 Metric/Value 为上次命中快照；以上为本次评估结果")
	}
	metaLines = append(metaLines,
		fmt.Sprintf("发送时间: %s", sendAt.Format(time.RFC3339)),
	)
	if format == "markdown" {
		content = content + "\n\n---\n" + strings.Join(metaLines, "\n")
	} else {
		content = content + "\n\n" + strings.Join(metaLines, "\n")
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if format == "markdown" {
		content = strings.ReplaceAll(content, "\n", "  \n")
	}

	type notificationConfig struct {
		Channels  []string            `json:"channels"`
		Receivers []uint              `json:"receivers"`
		Webhooks  map[string][]string `json:"webhooks"`
	}

	var cfg notificationConfig
	if strings.TrimSpace(notificationConfigRaw) != "" {
		err := json.Unmarshal([]byte(notificationConfigRaw), &cfg)
		if err != nil {
			var legacy struct {
				Webhooks []string `json:"webhooks"`
			}
			if err2 := json.Unmarshal([]byte(notificationConfigRaw), &legacy); err2 == nil && len(legacy.Webhooks) > 0 {
				cfg.Webhooks = map[string][]string{"webhook": legacy.Webhooks}
			}
		}
	}

	var atMobiles []string
	var toEmails []string
	if len(cfg.Receivers) > 0 {
		var users []model.User
		e.DB.Model(&model.User{}).Select("phone,email").Where("id IN ?", cfg.Receivers).Find(&users)
		for _, u := range users {
			if strings.TrimSpace(u.Phone) != "" {
				atMobiles = append(atMobiles, strings.TrimSpace(u.Phone))
			}
			if strings.TrimSpace(u.Email) != "" {
				toEmails = append(toEmails, strings.TrimSpace(u.Email))
			}
		}
	}
	atMobilesStr := strings.Join(atMobiles, ",")
	toEmailsStr := strings.Join(toEmails, ",")

	if !useRuleChannels {
		hasWebhook := len(cfg.Webhooks) > 0
		if hasWebhook {
			for _, urls := range cfg.Webhooks {
				if len(urls) > 0 {
					hasWebhook = true
					break
				}
			}
		}

		if hasWebhook {
			for channelType, urls := range cfg.Webhooks {
				if len(urls) == 0 {
					continue
				}
				sender, err := notification.GetSender(channelType)
				if err != nil {
					logger.Log.Warn("Unknown channel type", zap.String("type", channelType))
					continue
				}

				for _, url := range urls {
					if strings.TrimSpace(url) == "" {
						continue
					}
					conf := map[string]string{}
					conf["template"] = content
					conf["format"] = format
					conf["alert_level"] = rule.Level
					conf["level"] = rule.Level
					conf["alert_status"] = string(alarm.Status)
					conf["rule_name"] = rule.Name
					levelText := rule.Level
					if rule.Level == "critical" {
						levelText = "严重"
					} else if rule.Level == "warning" {
						levelText = "警告"
					} else if rule.Level == "info" {
						levelText = "信息"
					}
					title := fmt.Sprintf("【%s】%s", levelText, rule.Name)
					if alarm.Status == model.AlarmStatusResolved {
						title = fmt.Sprintf("【恢复】%s", rule.Name)
					}
					if isEscalation && alarm.Status == model.AlarmStatusFiring {
						title = "【升级】" + title
					}
					conf["title"] = title
					if channelType == "dingtalk" && atMobilesStr != "" {
						conf["at_mobiles"] = atMobilesStr
					}

					go func(s notification.AlarmSender, c map[string]string) {
						if err := s.Send(context.Background(), &alarm, c); err != nil {
							logger.Log.Error("Failed to send notification", zap.Error(err))
							if e.DB != nil {
								_ = e.DB.Create(&model.AlertLog{
									AlarmID:     alarm.ID,
									AlertRuleID: alarm.AlertRuleID,
									Status:      "notify_failed",
									Message:     fmt.Sprintf("channel_type=%s channel_id=0 err=%v", channelType, err),
									TriggeredAt: time.Now(),
								}).Error
							}
						}
					}(sender, conf)
				}
			}
			return
		}
	}

	for _, channel := range channels {
		sender, err := notification.GetSender(channel.Type)
		if err != nil {
			logger.Log.Warn("Unknown channel type", zap.String("type", channel.Type))
			continue
		}

		var config map[string]string
		if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
			logger.Log.Error("Failed to parse channel config", zap.Uint("channel_id", channel.ID), zap.Error(err))
			continue
		}

		// Inject rendered content
		config["template"] = content
		config["format"] = format
		config["alert_level"] = rule.Level
		config["alert_status"] = string(alarm.Status)
		config["rule_name"] = rule.Name
		levelText := rule.Level
		if rule.Level == "critical" {
			levelText = "严重"
		} else if rule.Level == "warning" {
			levelText = "警告"
		} else if rule.Level == "info" {
			levelText = "信息"
		}
		title := fmt.Sprintf("【%s】%s", levelText, rule.Name)
		if alarm.Status == model.AlarmStatusResolved {
			title = fmt.Sprintf("【恢复】%s", rule.Name)
		}
		if isEscalation && alarm.Status == model.AlarmStatusFiring {
			title = "【升级】" + title
		}
		config["title"] = title
		config["level"] = rule.Level
		if channel.Type == "dingtalk" && atMobilesStr != "" {
			config["at_mobiles"] = atMobilesStr
		}
		if channel.Type == "email" {
			if strings.TrimSpace(toEmailsStr) == "" {
				continue
			}
			config["to"] = toEmailsStr
		}

		go func(s notification.AlarmSender, c map[string]string) {
			if err := s.Send(context.Background(), &alarm, c); err != nil {
				logger.Log.Error("Failed to send notification", zap.Error(err))
				if e.DB != nil {
					_ = e.DB.Create(&model.AlertLog{
						AlarmID:     alarm.ID,
						AlertRuleID: alarm.AlertRuleID,
						Status:      "notify_failed",
						Message:     fmt.Sprintf("channel_type=%s channel_id=%d err=%v", channel.Type, channel.ID, err),
						TriggeredAt: time.Now(),
					}).Error
				}
			}
		}(sender, config)
	}
}

func (e *Executor) startEscalation(rule model.AlertRule, alarm model.Alarm) {
	var policy model.EscalationPolicy
	if err := e.DB.First(&policy, rule.EscalationPolicyID).Error; err != nil {
		logger.Log.Error("Failed to fetch escalation policy", zap.Uint("rule_id", rule.ID), zap.Error(err))
		return
	}
	if policy.TeamID != 0 && policy.TeamID != rule.TeamID {
		return
	}

	var steps []model.EscalationStep
	if err := json.Unmarshal([]byte(policy.Steps), &steps); err != nil {
		logger.Log.Error("Failed to parse escalation steps", zap.Uint("rule_id", rule.ID), zap.Error(err))
		return
	}
	if len(steps) == 0 {
		return
	}

	repeats := policy.RepeatTimes
	if repeats < 0 {
		repeats = 0
	}
	totalMinutes := 0
	for i := 0; i <= repeats; i++ {
		for _, s := range steps {
			if s.WaitMinutes > 0 {
				totalMinutes += s.WaitMinutes
			}
		}
	}
	if totalMinutes <= 0 {
		totalMinutes = 60
	}
	ttl := time.Duration(totalMinutes+10) * time.Minute

	if alarm.Fingerprint == "" {
		return
	}
	if e.RDB != nil {
		key := fmt.Sprintf("alert:escalation:%s:%d", alarm.Fingerprint, policy.ID)
		ok, _ := e.RDB.SetNX(context.Background(), key, "1", ttl).Result()
		if !ok {
			return
		}
	}

	go func(fingerprint string) {
		for i := 0; i <= repeats; i++ {
			for si, s := range steps {
				if s.WaitMinutes > 0 {
					time.Sleep(time.Duration(s.WaitMinutes) * time.Minute)
				}
				var cur model.Alarm
				if err := e.DB.Where("fingerprint = ? AND status = ?", fingerprint, model.AlarmStatusFiring).First(&cur).Error; err != nil {
					return
				}

				if len(s.ChannelIDs) == 0 {
					continue
				}
				props, _ := json.Marshal(map[string]interface{}{"channel_ids": s.ChannelIDs, "escalation": true})
				r2 := rule
				r2.NotificationProps = string(props)
				e.sendNotification(r2, cur)
				e.fireCallback(rule, cur, "escalation")
				e.DB.Create(&model.AlertLog{
					AlarmID:     cur.ID,
					AlertRuleID: rule.ID,
					Status:      "escalation",
					Message:     fmt.Sprintf("Escalation round=%d step=%d channels=%v", i+1, si+1, s.ChannelIDs),
					TriggeredAt: time.Now(),
				})
			}
		}
	}(alarm.Fingerprint)
}

func (e *Executor) fireCallback(rule model.AlertRule, alarm model.Alarm, event string) {
	url := strings.TrimSpace(rule.CallbackURL)
	if url == "" {
		return
	}
	payload := map[string]interface{}{
		"event":     event,
		"sent_at":   time.Now().Format(time.RFC3339),
		"rule_id":   rule.ID,
		"rule_name": rule.Name,
		"level":     rule.Level,
		"alarm":     alarm,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
}
