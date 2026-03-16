package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type RoutingRuleHandler struct{}

func (h *RoutingRuleHandler) List(c *gin.Context) {
	var rules []model.RoutingRule
	q := model.DB.Model(&model.RoutingRule{})
	if teamID := strings.TrimSpace(c.Query("team_id")); teamID != "" {
		if tid, err := strconv.Atoi(teamID); err == nil && tid > 0 {
			q = q.Where("team_id = ?", tid)
		}
	}
	if err := q.Order("priority desc, created_at desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *RoutingRuleHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var rule model.RoutingRule
	if err := model.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *RoutingRuleHandler) Create(c *gin.Context) {
	var rule model.RoutingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.TeamID == 0 || rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id and name are required"})
		return
	}
	if strings.TrimSpace(rule.Matchers) == "" {
		rule.Matchers = "[]"
	}
	if err := model.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *RoutingRuleHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var existing model.RoutingRule
	if err := model.DB.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	var input model.RoutingRule
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.TeamID != 0 && input.TeamID != existing.TeamID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id cannot be changed"})
		return
	}

	existing.Name = strings.TrimSpace(input.Name)
	if existing.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	existing.IsEnabled = input.IsEnabled
	existing.Priority = input.Priority
	if strings.TrimSpace(input.Matchers) != "" {
		existing.Matchers = input.Matchers
	}
	existing.NotificationProps = input.NotificationProps
	existing.NotificationConfig = input.NotificationConfig

	if err := model.DB.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *RoutingRuleHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := model.DB.Delete(&model.RoutingRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (h *RoutingRuleHandler) Preview(c *gin.Context) {
	var req struct {
		AlarmID uint `json:"alarm_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.AlarmID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alarm_id is required"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.First(&alarm, req.AlarmID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}
	var rule model.AlertRule
	if err := model.DB.Select("id, name, team_id, level, notification_props, notification_config").First(&rule, alarm.AlertRuleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	// 路由匹配使用的 labels 集合：
	// 1) 先从 alarm.labels（评估时根据 group_by keys 生成）反序列化得到基础 labels；
	// 2) 再补充 rule_name/level/status/service/app 这些“便捷字段”；
	// 3) rr.matchers 的多个条件是且关系（AND），任一条件不满足即不命中；matchers 为空视为命中全部。
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

	var baseProps struct {
		ChannelIDs []uint `json:"channel_ids"`
	}
	if strings.TrimSpace(rule.NotificationProps) != "" {
		_ = json.Unmarshal([]byte(rule.NotificationProps), &baseProps)
	}
	baseChannelIDs := baseProps.ChannelIDs
	notificationConfigRaw := rule.NotificationConfig

	var routingRules []model.RoutingRule
	_ = model.DB.Where("team_id = ? AND is_enabled = ?", rule.TeamID, true).
		Order("priority desc, created_at desc").
		Find(&routingRules).Error

	matchRule := func(rr model.RoutingRule) bool {
		// AND 关系匹配：
		// - 每条 matcher 要求 labels[name] 存在；
		// - isRegex=true 时用正则匹配，否则用等值匹配；
		// - 任一条件失败则整体失败；所有条件通过才命中。
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

	var matched *model.RoutingRule
	var routingChannelIDs []uint
	routingOverrideChannels := true
	for i := range routingRules {
		rr := routingRules[i]
		if !matchRule(rr) {
			continue
		}
		matched = &rr
		if strings.TrimSpace(rr.NotificationProps) != "" {
			var rp struct {
				ChannelIDs []uint `json:"channel_ids"`
				Override   *bool  `json:"override"`
			}
			if err := json.Unmarshal([]byte(rr.NotificationProps), &rp); err == nil && len(rp.ChannelIDs) > 0 {
				routingChannelIDs = rp.ChannelIDs
			}
			// 与实际发送逻辑保持一致：override=true(默认) 覆盖；override=false 叠加（去重合并）。
			if rp.Override != nil {
				routingOverrideChannels = *rp.Override
			}
		}
		if strings.TrimSpace(rr.NotificationConfig) != "" {
			notificationConfigRaw = rr.NotificationConfig
		}
		break
	}

	finalChannelIDs := baseChannelIDs
	if len(routingChannelIDs) > 0 {
		if routingOverrideChannels {
			finalChannelIDs = routingChannelIDs
		} else {
			seen := map[uint]struct{}{}
			merged := make([]uint, 0, len(baseChannelIDs)+len(routingChannelIDs))
			for _, id := range baseChannelIDs {
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
			finalChannelIDs = merged
		}
	}

	var channels []model.NotificationChannel
	if len(finalChannelIDs) > 0 {
		_ = model.DB.Where("id IN ?", finalChannelIDs).Find(&channels).Error
	} else {
		_ = model.DB.Where("team_id = ?", rule.TeamID).Find(&channels).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"alarm_id": req.AlarmID,
		"alert_rule": gin.H{
			"id":      rule.ID,
			"name":    rule.Name,
			"level":   rule.Level,
			"team_id": rule.TeamID,
		},
		"matched_routing_rule": matched,
		"base_channel_ids":     baseChannelIDs,
		"routing_channel_ids":  routingChannelIDs,
		"routing_override":     routingOverrideChannels,
		"final_channel_ids":    finalChannelIDs,
		"final_channels":       channels,
		"notification_config":  notificationConfigRaw,
		"labels":               labels,
	})
}

func (h *RoutingRuleHandler) MatcherOptions(c *gin.Context) {
	// 给前端提供“字段/值”的可选项提示（用于减少靠猜）：
	// - fields：内置字段（rule_name/level/status/service/app） + 最近告警 labels 里出现过的 key
	// - levels：从 alert_rules.level 去重得到（兜底返回 critical/warning/info）
	// - statuses：枚举 firing/resolved
	// - services/apps：从 alarms.service / alarms.app 去重采样（用于下拉提示）
	var teamID uint
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			teamID = uint(n)
		}
	}

	type levelRow struct {
		Level string `json:"level"`
	}
	lq := model.DB.Model(&model.AlertRule{}).Select("DISTINCT level as level").Where("level <> ''")
	if teamID > 0 {
		lq = lq.Where("team_id = ?", teamID)
	}
	var levelRows []levelRow
	_ = lq.Find(&levelRows).Error
	levels := make([]string, 0, len(levelRows))
	for _, r := range levelRows {
		v := strings.TrimSpace(r.Level)
		if v != "" {
			levels = append(levels, v)
		}
	}
	if len(levels) == 0 {
		levels = []string{"critical", "warning", "info"}
	}
	sort.Strings(levels)

	statuses := []string{string(model.AlarmStatusFiring), string(model.AlarmStatusResolved)}

	type strRow struct {
		V string `json:"v"`
	}
	svcQ := model.DB.Table("alarms").
		Select("DISTINCT alarms.service as v").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.service <> ''")
	appQ := model.DB.Table("alarms").
		Select("DISTINCT alarms.app as v").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.app <> ''")
	if teamID > 0 {
		svcQ = svcQ.Where("alert_rules.team_id = ?", teamID)
		appQ = appQ.Where("alert_rules.team_id = ?", teamID)
	}
	svcQ = svcQ.Limit(200)
	appQ = appQ.Limit(200)
	var svcRows []strRow
	var appRows []strRow
	_ = svcQ.Find(&svcRows).Error
	_ = appQ.Find(&appRows).Error
	services := make([]string, 0, len(svcRows))
	apps := make([]string, 0, len(appRows))
	for _, r := range svcRows {
		v := strings.TrimSpace(r.V)
		if v != "" {
			services = append(services, v)
		}
	}
	for _, r := range appRows {
		v := strings.TrimSpace(r.V)
		if v != "" {
			apps = append(apps, v)
		}
	}
	sort.Strings(services)
	sort.Strings(apps)

	type labelRow struct {
		Labels string `json:"labels"`
	}
	labelsQ := model.DB.Table("alarms").
		Select("alarms.labels as labels").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Order("alarms.updated_at desc").
		Limit(500)
	if teamID > 0 {
		labelsQ = labelsQ.Where("alert_rules.team_id = ?", teamID)
	}
	var labelRows []labelRow
	_ = labelsQ.Find(&labelRows).Error
	keySet := map[string]struct{}{}
	for _, r := range labelRows {
		raw := strings.TrimSpace(r.Labels)
		if raw == "" {
			continue
		}
		m := map[string]string{}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		for k := range m {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			keySet[k] = struct{}{}
		}
	}
	labelKeys := make([]string, 0, len(keySet))
	for k := range keySet {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	fields := []string{"rule_name", "level", "status", "service", "app", "alert_rule_id", "team_id", "fingerprint"}
	fields = append(fields, labelKeys...)

	c.JSON(http.StatusOK, gin.H{
		"team_id":    teamID,
		"fields":     fields,
		"levels":     levels,
		"statuses":   statuses,
		"services":   services,
		"apps":       apps,
		"label_keys": labelKeys,
	})
}
