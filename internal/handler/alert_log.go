package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AlertLogHandler struct{}

func (h *AlertLogHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var log model.AlertLog
	if err := model.DB.Preload("Alarm").First(&log, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

func (h *AlertLogHandler) List(c *gin.Context) {
	type alertLogRow struct {
		model.AlertLog
		RuleName string `json:"rule_name"`
		Level    string `json:"level"`
	}
	var rows []alertLogRow

	query := model.DB.Table("alert_logs").
		Select("alert_logs.*, alert_rules.name as rule_name, alert_rules.level as level").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alert_logs.alert_rule_id").
		Order("alert_logs.created_at desc")

	parseTime := func(s string) (time.Time, bool) {
		ss := strings.TrimSpace(s)
		if ss == "" {
			return time.Time{}, false
		}
		if n, err := strconv.ParseInt(ss, 10, 64); err == nil {
			if n > 1_000_000_000_000 {
				return time.UnixMilli(n), true
			}
			if n > 0 {
				return time.Unix(n, 0), true
			}
		}
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05.000",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05.000",
		}
		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, ss, time.Local); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}

	if ruleID := c.Query("alert_rule_id"); ruleID != "" {
		if id, err := strconv.Atoi(ruleID); err == nil {
			query = query.Where("alert_rule_id = ?", id)
		}
	}
	if alarmID := c.Query("alarm_id"); alarmID != "" {
		if id, err := strconv.Atoi(alarmID); err == nil {
			query = query.Where("alarm_id = ?", id)
		}
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if start, ok := parseTime(c.Query("start_time")); ok {
		query = query.Where("alert_logs.created_at >= ?", start)
	}
	if end, ok := parseTime(c.Query("end_time")); ok {
		query = query.Where("alert_logs.created_at <= ?", end)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pageSize := 20
	if ps := c.Query("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 200 {
			pageSize = n
		}
	}
	current := 1
	if cur := c.Query("current"); cur != "" {
		if n, err := strconv.Atoi(cur); err == nil && n > 0 {
			current = n
		}
	}
	offset := (current - 1) * pageSize

	if err := query.Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  rows,
		"total": total,
	})
}
