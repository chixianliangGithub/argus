package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
	prommodel "github.com/prometheus/common/model"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type AlertRuleHandler struct{}

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

func schedulerIsWithinEffectiveTime(raw string, now time.Time) bool {
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

func (h *AlertRuleHandler) Runtime(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var rule model.AlertRule
	if err := model.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	now := time.Now()
	spec := rule.CronExpression
	if strings.TrimSpace(spec) == "" {
		d := rule.Duration
		if d <= 0 {
			d = 60
		}
		spec = fmt.Sprintf("@every %ds", d)
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, perr := parser.Parse(spec)
	nextTimes := []time.Time{}
	if perr == nil {
		t := schedule.Next(now)
		limit := 0
		for len(nextTimes) < 5 && limit < 200 {
			if schedulerIsWithinEffectiveTime(rule.EffectiveTime, t) {
				nextTimes = append(nextTimes, t)
			}
			t = schedule.Next(t)
			limit++
		}
	}

	effectiveNow := schedulerIsWithinEffectiveTime(rule.EffectiveTime, now)

	var lastLog model.AlertLog
	lastLogAt := (*time.Time)(nil)
	if err := model.DB.Where("alert_rule_id = ?", id).Order("created_at desc").First(&lastLog).Error; err == nil {
		lastLogAt = &lastLog.CreatedAt
	}

	var alarms []model.Alarm
	_ = model.DB.Where("alert_rule_id = ? AND status = ?", id, model.AlarmStatusFiring).Find(&alarms).Error

	type alarmRuntime struct {
		ID                   uint       `json:"id"`
		Fingerprint          string     `json:"fingerprint"`
		Status               string     `json:"status"`
		StartsAt             time.Time  `json:"starts_at"`
		EndsAt               *time.Time `json:"ends_at"`
		SilenceTTLSeconds    int64      `json:"silence_ttl_seconds"`
		EscalationTTLSeconds int64      `json:"escalation_ttl_seconds"`
	}
	var alarmStates []alarmRuntime
	leaderTTLSeconds := int64(0)
	queueLen := int64(0)
	if model.RDB != nil {
		ttl, _ := model.RDB.TTL(context.Background(), "argus:scheduler:leader").Result()
		if ttl > 0 {
			leaderTTLSeconds = int64(ttl.Seconds())
		}
		ql, _ := model.RDB.XLen(context.Background(), "argus:alert:tasks:stream").Result()
		queueLen = ql
		for _, a := range alarms {
			silTTL := int64(0)
			escTTL := int64(0)
			if a.Fingerprint != "" {
				ttl1, _ := model.RDB.TTL(context.Background(), "alert:silence:"+a.Fingerprint).Result()
				if ttl1 > 0 {
					silTTL = int64(ttl1.Seconds())
				}
				ttl2, _ := model.RDB.TTL(context.Background(), fmt.Sprintf("alert:escalation:%s:%d", a.Fingerprint, rule.EscalationPolicyID)).Result()
				if ttl2 > 0 {
					escTTL = int64(ttl2.Seconds())
				}
			}
			alarmStates = append(alarmStates, alarmRuntime{
				ID:                   a.ID,
				Fingerprint:          a.Fingerprint,
				Status:               string(a.Status),
				StartsAt:             a.StartsAt,
				EndsAt:               a.EndsAt,
				SilenceTTLSeconds:    silTTL,
				EscalationTTLSeconds: escTTL,
			})
		}
	} else {
		for _, a := range alarms {
			alarmStates = append(alarmStates, alarmRuntime{
				ID:          a.ID,
				Fingerprint: a.Fingerprint,
				Status:      string(a.Status),
				StartsAt:    a.StartsAt,
				EndsAt:      a.EndsAt,
			})
		}
	}

	out := gin.H{
		"now":                  now,
		"is_enabled":           rule.IsEnabled,
		"cron_expression":      spec,
		"effective_now":        effectiveNow,
		"next_times":           nextTimes,
		"last_log_at":          lastLogAt,
		"leader_ttl_seconds":   leaderTTLSeconds,
		"queue_len":            queueLen,
		"escalation_enabled":   rule.EscalationEnabled,
		"escalation_policy_id": rule.EscalationPolicyID,
		"escalation_after":     rule.EscalationAfter,
		"alarms":               alarmStates,
	}

	if rule.SilenceRuleID != 0 {
		var s model.SilenceRule
		if err := model.DB.Where("id = ? AND team_id = ?", rule.SilenceRuleID, rule.TeamID).First(&s).Error; err == nil {
			out["silence_rule"] = gin.H{
				"id":        s.ID,
				"name":      s.Name,
				"team_id":   s.TeamID,
				"starts_at": s.StartsAt,
				"ends_at":   s.EndsAt,
				"matchers":  s.Matchers,
				"comment":   s.Comment,
			}
			out["silence_rule_active_now"] = !s.StartsAt.IsZero() && !s.EndsAt.IsZero() && !now.Before(s.StartsAt) && !now.After(s.EndsAt)
		} else {
			out["silence_rule_error"] = err.Error()
		}
	}

	if perr != nil {
		out["cron_error"] = perr.Error()
	}

	c.JSON(http.StatusOK, out)
}

func (h *AlertRuleHandler) Preview(c *gin.Context) {
	var req struct {
		DataNameID        uint   `json:"data_name_id"`
		Query             string `json:"query"`
		EvalWindowMinutes int    `json:"eval_window_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dn model.DataName
	if err := model.DB.Preload("DataSource").First(&dn, req.DataNameID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DataName not found"})
		return
	}

	ds, err := datasource.NewDataSource(dn.DataSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create datasource: " + err.Error()})
		return
	}

	q := req.Query
	if req.EvalWindowMinutes > 0 && strings.TrimSpace(dn.TimeField) != "" {
		t := dn.DataSource.Type
		mins := req.EvalWindowMinutes
		if mins < 0 {
			mins = 0
		}
		if mins > 0 && (t == "mysql" || t == "sqlserver" || t == "clickhouse") {
			expr := ""
			if t == "clickhouse" {
				expr = fmt.Sprintf("now() - INTERVAL %d MINUTE", mins)
			} else {
				expr = fmt.Sprintf("NOW() - INTERVAL %d MINUTE", mins)
			}
			q = datasource.InjectSQLTimeFilter(q, dn.TimeField, expr)
		}
	}

	result, err := ds.Query(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query execution failed: " + err.Error()})
		return
	}

	data := result.Data
	switch v := data.(type) {
	case prommodel.Vector:
		rows := make([]map[string]interface{}, 0, len(v))
		for _, s := range v {
			row := map[string]interface{}{
				"metric":    s.Metric.String(),
				"value":     float64(s.Value),
				"timestamp": int64(s.Timestamp),
			}
			for k, vv := range s.Metric {
				row[string(k)] = string(vv)
			}
			rows = append(rows, row)
			if len(rows) >= 50 {
				break
			}
		}
		data = rows
	case prommodel.Matrix:
		rows := make([]map[string]interface{}, 0, len(v))
		for _, stream := range v {
			row := map[string]interface{}{
				"metric": stream.Metric.String(),
			}
			if len(stream.Values) > 0 {
				last := stream.Values[len(stream.Values)-1]
				row["value"] = float64(last.Value)
				row["timestamp"] = int64(last.Timestamp)
			}
			for k, vv := range stream.Metric {
				row[string(k)] = string(vv)
			}
			rows = append(rows, row)
			if len(rows) >= 50 {
				break
			}
		}
		data = rows
	case prommodel.Scalar:
		data = []map[string]interface{}{
			{
				"metric":    "scalar",
				"value":     float64(v.Value),
				"timestamp": int64(v.Timestamp),
			},
		}
	}
	if rows, ok := data.([]map[string]interface{}); ok {
		limit := 10
		if dn.DataSource.Type == "mysql" || dn.DataSource.Type == "sqlserver" || dn.DataSource.Type == "clickhouse" {
			limit = 1
		}
		if limit > 0 && len(rows) > limit {
			data = rows[:limit]
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *AlertRuleHandler) Trigger(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req struct {
		Force bool `json:"force"`
	}
	_ = c.ShouldBindJSON(&req)

	if model.RDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis not configured"})
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"rule_id": uint(id),
		"force":   req.Force,
	})
	if _, err := model.RDB.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "argus:alert:tasks:stream",
		Values: map[string]interface{}{"payload": string(payload)},
	}).Result(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Triggered"})
}

func (h *AlertRuleHandler) Create(c *gin.Context) {
	var rule model.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.Team = model.Team{}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule name is required"})
		return
	}

	var existing model.AlertRule
	if err := model.DB.Unscoped().Where("name = ?", rule.Name).First(&existing).Error; err == nil {
		if existing.DeletedAt.Valid {
			_ = model.DB.Unscoped().Delete(&existing).Error
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "Alert rule name already exists"})
			return
		}
	}

	if err := model.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *AlertRuleHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var rule model.AlertRule
	if err := model.DB.Preload("DataName.DataSource").Preload("Team").First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

func (h *AlertRuleHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var rule model.AlertRule
	if err := model.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.Team = model.Team{}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule name is required"})
		return
	}

	var other model.AlertRule
	if err := model.DB.Unscoped().Where("name = ? AND id <> ?", rule.Name, rule.ID).First(&other).Error; err == nil {
		if other.DeletedAt.Valid {
			_ = model.DB.Unscoped().Delete(&other).Error
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "Alert rule name already exists"})
			return
		}
	}

	if err := model.DB.Save(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

func (h *AlertRuleHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := model.DB.Delete(&model.AlertRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}

func (h *AlertRuleHandler) List(c *gin.Context) {
	var rules []model.AlertRule
	query := model.DB.Preload("DataName.DataSource").Preload("Team")

	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if teamID := c.Query("team_id"); teamID != "" {
		if id, err := strconv.Atoi(teamID); err == nil && id > 0 {
			query = query.Where("team_id = ?", id)
		}
	}
	if level := c.Query("level"); level != "" {
		query = query.Where("level = ?", level)
	}
	if enabled := c.Query("is_enabled"); enabled != "" {
		if enabled == "true" || enabled == "1" {
			query = query.Where("is_enabled = ?", true)
		} else if enabled == "false" || enabled == "0" {
			query = query.Where("is_enabled = ?", false)
		}
	}

	if err := query.Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rules)
}

func (h *AlertRuleHandler) BatchUpdateEnabled(c *gin.Context) {
	var req struct {
		IDs       []uint `json:"ids"`
		IsEnabled bool   `json:"is_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required"})
		return
	}

	if err := model.DB.Model(&model.AlertRule{}).Where("id IN ?", req.IDs).Update("is_enabled", req.IsEnabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

type AlertRuleYAML struct {
	Name               string  `yaml:"name"`
	DataName           string  `yaml:"data_name"`  // Name of the dataname
	DataSource         string  `yaml:"datasource"` // Name of the datasource (optional/derived)
	Query              string  `yaml:"query"`
	Duration           int     `yaml:"duration"`
	Level              string  `yaml:"level"`
	Condition          string  `yaml:"condition"`
	Threshold          float64 `yaml:"threshold"`
	Algorithm          string  `yaml:"algorithm"`
	AlgoParams         string  `yaml:"algo_params"`
	ValueField         string  `yaml:"value_field"`
	ValueMode          string  `yaml:"value_mode"`
	RowPick            string  `yaml:"row_pick"`
	NotifyOnResolve    bool    `yaml:"notify_on_resolve"`
	SilenceRuleID      uint    `yaml:"silence_rule_id"`
	EscalationEnabled  bool    `yaml:"escalation_enabled"`
	EscalationPolicyID uint    `yaml:"escalation_policy_id"`
	SilencePeriod      int     `yaml:"silence_period"`
	GroupBy            string  `yaml:"group_by"`
	EvalWindow         string  `yaml:"eval_window"`
	EvalConsecutive    int     `yaml:"eval_consecutive"`
	EffectiveTime      string  `yaml:"effective_time"`
	Team               string  `yaml:"team"` // Name of the team
	IsEnabled          bool    `yaml:"is_enabled"`
	Description        string  `yaml:"description"`
	CronExpression     string  `yaml:"cron_expression"`
	NotificationConfig string  `yaml:"notification_config"`
}

func (h *AlertRuleHandler) ExportYAML(c *gin.Context) {
	var rules []model.AlertRule
	if err := model.DB.Preload("DataSource").Preload("Team").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var yamlRules []AlertRuleYAML
	for _, rule := range rules {
		yamlRules = append(yamlRules, AlertRuleYAML{
			Name:               rule.Name,
			DataName:           rule.DataName.Name,
			DataSource:         rule.DataName.DataSource.Name,
			Query:              rule.Query,
			Duration:           rule.Duration,
			Level:              rule.Level,
			Condition:          rule.Condition,
			Threshold:          rule.Threshold,
			Algorithm:          rule.Algorithm,
			AlgoParams:         rule.AlgoParams,
			ValueField:         rule.ValueField,
			ValueMode:          rule.ValueMode,
			RowPick:            rule.RowPick,
			NotifyOnResolve:    rule.NotifyOnResolve,
			SilenceRuleID:      rule.SilenceRuleID,
			EscalationEnabled:  rule.EscalationEnabled,
			EscalationPolicyID: rule.EscalationPolicyID,
			SilencePeriod:      rule.SilencePeriod,
			GroupBy:            rule.GroupBy,
			EvalWindow:         rule.EvalWindow,
			EvalConsecutive:    rule.EvalConsecutive,
			EffectiveTime:      rule.EffectiveTime,
			Team:               rule.Team.Name,
			IsEnabled:          rule.IsEnabled,
			Description:        rule.Description,
			CronExpression:     rule.CronExpression,
			NotificationConfig: rule.NotificationConfig,
		})
	}

	data, err := yaml.Marshal(yamlRules)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal YAML"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=alert_rules.yaml")
	c.Data(http.StatusOK, "application/x-yaml", data)
}

func (h *AlertRuleHandler) ImportYAML(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer f.Close()

	var yamlRules []AlertRuleYAML
	if err := yaml.NewDecoder(f).Decode(&yamlRules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode YAML"})
		return
	}

	for _, yr := range yamlRules {
		// Resolve DataName
		var dn model.DataName
		if err := model.DB.Where("name = ?", yr.DataName).First(&dn).Error; err != nil {
			// Try to find by DataSource name for backward compatibility if DataName not present
			// This is tricky, maybe skip or require DataName
			continue
		}

		// Resolve Team
		var team model.Team
		if err := model.DB.Where("name = ?", yr.Team).First(&team).Error; err != nil {
			// Skip or handle error
			continue
		}

		rule := model.AlertRule{
			Name:               yr.Name,
			DataNameID:         dn.ID,
			Query:              yr.Query,
			Duration:           yr.Duration,
			Level:              yr.Level,
			Condition:          yr.Condition,
			Threshold:          yr.Threshold,
			Algorithm:          yr.Algorithm,
			AlgoParams:         yr.AlgoParams,
			ValueField:         yr.ValueField,
			ValueMode:          yr.ValueMode,
			RowPick:            yr.RowPick,
			NotifyOnResolve:    yr.NotifyOnResolve,
			SilenceRuleID:      yr.SilenceRuleID,
			EscalationEnabled:  yr.EscalationEnabled,
			EscalationPolicyID: yr.EscalationPolicyID,
			SilencePeriod:      yr.SilencePeriod,
			GroupBy:            yr.GroupBy,
			EvalWindow:         yr.EvalWindow,
			EvalConsecutive:    yr.EvalConsecutive,
			EffectiveTime:      yr.EffectiveTime,
			TeamID:             team.ID,
			IsEnabled:          yr.IsEnabled,
			Description:        yr.Description,
			CronExpression:     yr.CronExpression,
			NotificationConfig: yr.NotificationConfig,
		}

		// Upsert based on Name
		var existingRule model.AlertRule
		if err := model.DB.Where("name = ?", rule.Name).First(&existingRule).Error; err == nil {
			rule.ID = existingRule.ID
			model.DB.Save(&rule)
		} else {
			model.DB.Create(&rule)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert rules imported successfully"})
}
