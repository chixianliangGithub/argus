package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type IncidentHandler struct{}

func (h *IncidentHandler) List(c *gin.Context) {
	query := model.DB.Model(&model.Incident{}).Order("last_activity_at desc")

	if teamID := c.Query("team_id"); teamID != "" {
		if id, err := strconv.Atoi(teamID); err == nil && id > 0 {
			query = query.Where("team_id = ?", id)
		}
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if severity := strings.TrimSpace(c.Query("severity")); severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if service := strings.TrimSpace(c.Query("service")); service != "" {
		query = query.Joins("LEFT JOIN alarms ON alarms.id = incidents.root_alarm_id").Where("alarms.service = ?", service)
	}
	if app := strings.TrimSpace(c.Query("app")); app != "" {
		query = query.Joins("LEFT JOIN alarms ON alarms.id = incidents.root_alarm_id").Where("alarms.app = ?", app)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query = query.Where("title LIKE ? OR summary LIKE ? OR group_key LIKE ? OR dedup_key LIKE ?", like, like, like, like)
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

	var rows []model.Incident
	if err := query.Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
}

func (h *IncidentHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var inc model.Incident
	if err := model.DB.First(&inc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	var rels []model.IncidentAlarm
	_ = model.DB.Where("incident_id = ?", inc.ID).Find(&rels).Error
	alarmIDs := make([]uint, 0, len(rels))
	for _, r := range rels {
		alarmIDs = append(alarmIDs, r.AlarmID)
	}

	type alarmRow struct {
		model.Alarm
		RuleName string `json:"rule_name"`
		Level    string `json:"level"`
		TeamID   uint   `json:"team_id"`
	}

	var alarms []alarmRow
	if len(alarmIDs) > 0 {
		_ = model.DB.Table("alarms").
			Select("alarms.*, alert_rules.name as rule_name, alert_rules.level as level, alert_rules.team_id as team_id").
			Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
			Where("alarms.id IN ?", alarmIDs).
			Order("alarms.created_at desc").
			Find(&alarms).Error
	}

	var logs []model.AlertLog
	if len(alarmIDs) > 0 {
		_ = model.DB.Where("alarm_id IN ?", alarmIDs).Order("created_at desc").Limit(300).Find(&logs).Error
	}

	var rcas []model.RCAReport
	if len(alarmIDs) > 0 {
		_ = model.DB.Where("alarm_id IN ?", alarmIDs).Order("created_at desc").Limit(50).Find(&rcas).Error
	}

	var activities []model.IncidentActivity
	_ = model.DB.Where("incident_id = ?", inc.ID).Order("created_at desc").Limit(200).Find(&activities).Error

	dnCandidates, recommendedDataNameID := getIncidentDataNameCandidatesByAlarmIDs(alarmIDs)

	c.JSON(http.StatusOK, gin.H{
		"incident":                inc,
		"alarms":                  alarms,
		"logs":                    logs,
		"rca":                     rcas,
		"activity":                activities,
		"dataname_candidates":     dnCandidates,
		"recommended_dataname_id": recommendedDataNameID,
	})
}

func (h *IncidentHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var inc model.Incident
	if err := model.DB.First(&inc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	var req struct {
		Title          string `json:"title"`
		Summary        string `json:"summary"`
		Impact         string `json:"impact"`
		RootCause      string `json:"root_cause"`
		Classification string `json:"classification"`
		Mitigation     string `json:"mitigation"`
		Verification   string `json:"verification"`
		RollbackPlan   string `json:"rollback_plan"`
		FollowUps      string `json:"follow_ups"`
		Tags           string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Title) != "" {
		updates["title"] = strings.TrimSpace(req.Title)
	}
	if req.Summary != "" {
		updates["summary"] = strings.TrimSpace(req.Summary)
	}
	updates["impact"] = strings.TrimSpace(req.Impact)
	updates["root_cause"] = strings.TrimSpace(req.RootCause)
	updates["classification"] = strings.TrimSpace(req.Classification)
	updates["mitigation"] = strings.TrimSpace(req.Mitigation)
	updates["verification"] = strings.TrimSpace(req.Verification)
	updates["rollback_plan"] = strings.TrimSpace(req.RollbackPlan)
	updates["follow_ups"] = strings.TrimSpace(req.FollowUps)
	if req.Tags != "" {
		updates["tags"] = strings.TrimSpace(req.Tags)
	}
	updates["last_activity_at"] = time.Now()

	if err := model.DB.Model(&model.Incident{}).Where("id = ?", inc.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.addActivity(c, inc.ID, model.IncidentActivityComment, "updated incident fields")
	_ = model.DB.First(&inc, id).Error
	c.JSON(http.StatusOK, inc)
}

func (h *IncidentHandler) Ack(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var input struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&input)
	input.Note = strings.TrimSpace(input.Note)

	now := time.Now()
	updates := map[string]interface{}{
		"status":           model.IncidentStatusAcked,
		"acked_at":         &now,
		"last_activity_at": now,
	}
	if err := model.DB.Model(&model.Incident{}).Where("id = ? AND status <> ?", id, model.IncidentStatusResolved).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var inc model.Incident
	_ = model.DB.First(&inc, id).Error
	h.addActivity(c, inc.ID, model.IncidentActivityAcked, input.Note)
	c.JSON(http.StatusOK, inc)
}

func (h *IncidentHandler) Resolve(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var input struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&input)
	input.Note = strings.TrimSpace(input.Note)

	now := time.Now()
	updates := map[string]interface{}{
		"status":           model.IncidentStatusResolved,
		"resolved_at":      &now,
		"last_activity_at": now,
	}
	if err := model.DB.Model(&model.Incident{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var inc model.Incident
	_ = model.DB.First(&inc, id).Error
	h.addActivity(c, inc.ID, model.IncidentActivityResolved, input.Note)
	c.JSON(http.StatusOK, inc)
}

func (h *IncidentHandler) Comment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var input struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note is required"})
		return
	}

	var inc model.Incident
	if err := model.DB.First(&inc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	h.addActivity(c, inc.ID, model.IncidentActivityComment, input.Note)
	_ = model.DB.Model(&model.Incident{}).Where("id = ?", inc.ID).Update("last_activity_at", time.Now()).Error
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *IncidentHandler) addActivity(c *gin.Context, incidentID uint, typ model.IncidentActivityType, note string) {
	msg := strings.TrimSpace(note)
	var createdBy *uint
	if raw, ok := c.Get("user_id"); ok {
		if uid, err := parseUint(raw); err == nil && uid > 0 {
			createdBy = &uid
		}
	}
	_ = model.DB.Create(&model.IncidentActivity{
		IncidentID: incidentID,
		Type:       typ,
		Message:    msg,
		CreatedBy:  createdBy,
	}).Error
}

func (h *IncidentHandler) Activities(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var inc model.Incident
	if err := model.DB.Select("id").First(&inc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	query := model.DB.Model(&model.IncidentActivity{}).Where("incident_id = ?", inc.ID).Order("created_at desc")
	if typ := strings.TrimSpace(c.Query("type")); typ != "" {
		query = query.Where("type = ?", typ)
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

	var rows []model.IncidentActivity
	if err := query.Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
}

func (h *IncidentHandler) ExportMarkdown(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var inc model.Incident
	if err := model.DB.First(&inc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	var rels []model.IncidentAlarm
	_ = model.DB.Where("incident_id = ?", inc.ID).Find(&rels).Error
	alarmIDs := make([]uint, 0, len(rels))
	for _, r := range rels {
		alarmIDs = append(alarmIDs, r.AlarmID)
	}

	type alarmRow struct {
		model.Alarm
		RuleName string `json:"rule_name"`
		Level    string `json:"level"`
		TeamID   uint   `json:"team_id"`
	}
	var alarms []alarmRow
	if len(alarmIDs) > 0 {
		_ = model.DB.Table("alarms").
			Select("alarms.*, alert_rules.name as rule_name, alert_rules.level as level, alert_rules.team_id as team_id").
			Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
			Where("alarms.id IN ?", alarmIDs).
			Order("alarms.created_at desc").
			Find(&alarms).Error
	}

	var logs []model.AlertLog
	if len(alarmIDs) > 0 {
		_ = model.DB.Where("alarm_id IN ?", alarmIDs).Order("created_at desc").Limit(500).Find(&logs).Error
	}
	var activities []model.IncidentActivity
	_ = model.DB.Where("incident_id = ?", inc.ID).Order("created_at desc").Limit(300).Find(&activities).Error

	lines := []string{}
	title := strings.TrimSpace(inc.Title)
	if title == "" {
		title = fmt.Sprintf("Incident #%d", inc.ID)
	}
	lines = append(lines, "# "+title)
	lines = append(lines, "")
	lines = append(lines, "## 概要")
	lines = append(lines, fmt.Sprintf("- 状态: %s", inc.Status))
	lines = append(lines, fmt.Sprintf("- 级别: %s", inc.Severity))
	lines = append(lines, fmt.Sprintf("- 团队: %d", inc.TeamID))
	lines = append(lines, fmt.Sprintf("- 开始: %s", inc.OpenedAt.Format(time.RFC3339)))
	if inc.AckedAt != nil {
		lines = append(lines, fmt.Sprintf("- ACK: %s", inc.AckedAt.Format(time.RFC3339)))
	}
	if inc.ResolvedAt != nil {
		lines = append(lines, fmt.Sprintf("- 关闭: %s", inc.ResolvedAt.Format(time.RFC3339)))
	}
	lines = append(lines, "")
	if strings.TrimSpace(inc.Summary) != "" {
		lines = append(lines, "### 现象与摘要")
		lines = append(lines, strings.TrimSpace(inc.Summary))
		lines = append(lines, "")
	}

	lines = append(lines, "## 复盘草稿")
	if strings.TrimSpace(inc.Impact) != "" {
		lines = append(lines, "### 影响面")
		lines = append(lines, strings.TrimSpace(inc.Impact))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.RootCause) != "" {
		lines = append(lines, "### 根因")
		lines = append(lines, strings.TrimSpace(inc.RootCause))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.Classification) != "" {
		lines = append(lines, "### 根因分类")
		lines = append(lines, strings.TrimSpace(inc.Classification))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.Mitigation) != "" {
		lines = append(lines, "### 处置步骤")
		lines = append(lines, strings.TrimSpace(inc.Mitigation))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.Verification) != "" {
		lines = append(lines, "### 验证")
		lines = append(lines, strings.TrimSpace(inc.Verification))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.RollbackPlan) != "" {
		lines = append(lines, "### 回滚")
		lines = append(lines, strings.TrimSpace(inc.RollbackPlan))
		lines = append(lines, "")
	}
	if strings.TrimSpace(inc.FollowUps) != "" {
		lines = append(lines, "### Follow-ups")
		lines = append(lines, strings.TrimSpace(inc.FollowUps))
		lines = append(lines, "")
	}

	lines = append(lines, "## 关联告警")
	if len(alarms) == 0 {
		lines = append(lines, "- (无)")
	} else {
		for _, a := range alarms {
			lines = append(lines, fmt.Sprintf("- alarm_id=%d status=%s level=%s rule=%s service=%s app=%s", a.ID, a.Status, a.Level, a.RuleName, a.Service, a.App))
		}
	}
	lines = append(lines, "")

	lines = append(lines, "## 时间线")
	for _, a := range activities {
		lines = append(lines, fmt.Sprintf("- %s [%s] %s", a.CreatedAt.Format(time.RFC3339), a.Type, strings.TrimSpace(a.Message)))
	}
	for _, l := range logs {
		lines = append(lines, fmt.Sprintf("- %s [alarm:%d %s] %s", l.CreatedAt.Format(time.RFC3339), l.AlarmID, l.Status, strings.TrimSpace(l.Message)))
	}
	lines = append(lines, "")

	body := strings.Join(lines, "\n")
	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=incident-%d.md", inc.ID))
	c.String(http.StatusOK, body)
}

func (h *IncidentHandler) Merge(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req struct {
		SourceIDs []uint `json:"source_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.SourceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_ids is required"})
		return
	}

	tx := model.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var target model.Incident
	if err := tx.First(&target, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}

	now := time.Now()
	for _, sid := range req.SourceIDs {
		if sid == 0 || sid == uint(id) {
			continue
		}
		var src model.Incident
		if err := tx.First(&src, sid).Error; err != nil {
			continue
		}
		var rels []model.IncidentAlarm
		_ = tx.Where("incident_id = ?", src.ID).Find(&rels).Error
		for _, r := range rels {
			var exist model.IncidentAlarm
			if err := tx.Where("incident_id = ? AND alarm_id = ?", target.ID, r.AlarmID).First(&exist).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					_ = tx.Create(&model.IncidentAlarm{
						IncidentID:   target.ID,
						AlarmID:      r.AlarmID,
						RelationType: "related",
					}).Error
				}
			}
		}
		_ = tx.Model(&model.Incident{}).Where("id = ?", src.ID).Updates(map[string]interface{}{
			"status":           model.IncidentStatusResolved,
			"resolved_at":      &now,
			"last_activity_at": now,
		}).Error
	}

	_ = tx.Model(&model.Incident{}).Where("id = ?", target.ID).Update("last_activity_at", now).Error
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var out model.Incident
	_ = model.DB.First(&out, id).Error
	c.JSON(http.StatusOK, out)
}
