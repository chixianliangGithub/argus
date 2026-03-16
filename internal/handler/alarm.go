package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AlarmHandler struct{}

type batchIDsReq struct {
	IDs []uint `json:"ids"`
}

type batchResultResp struct {
	SucceededIDs []uint `json:"succeeded_ids"`
	FailedIDs    []uint `json:"failed_ids"`
}

func (h *AlarmHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.Preload("AlertRule").First(&alarm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}

	c.JSON(http.StatusOK, alarm)
}

func (h *AlarmHandler) List(c *gin.Context) {
	type alarmRow struct {
		model.Alarm
		RuleName      string `json:"rule_name"`
		Level         string `json:"level"`
		ClaimedByName string `json:"claimed_by_name"`
		HandledByName string `json:"handled_by_name"`
	}
	var rows []alarmRow

	query := model.DB.Table("alarms").
		Select("alarms.*, alert_rules.name as rule_name, alert_rules.level as level, u_claim.username as claimed_by_name, u_handle.username as handled_by_name").
		Joins("LEFT JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Joins("LEFT JOIN users as u_claim ON u_claim.id = alarms.claimed_by").
		Joins("LEFT JOIN users as u_handle ON u_handle.id = alarms.handled_by").
		Order("alarms.last_seen_at desc, alarms.updated_at desc")
	if status := c.Query("status"); status != "" {
		query = query.Where("alarms.status = ?", status)
	}
	if claimed := c.Query("claimed"); claimed != "" {
		if claimed == "1" || claimed == "true" {
			query = query.Where("alarms.claimed_by IS NOT NULL")
		} else if claimed == "0" || claimed == "false" {
			query = query.Where("alarms.claimed_by IS NULL")
		}
	}
	if handled := c.Query("handled"); handled != "" {
		if handled == "1" || handled == "true" {
			query = query.Where("alarms.handled_by IS NOT NULL")
		} else if handled == "0" || handled == "false" {
			query = query.Where("alarms.handled_by IS NULL")
		}
	}
	if service := strings.TrimSpace(c.Query("service")); service != "" {
		query = query.Where("alarms.service = ?", service)
	}
	if app := strings.TrimSpace(c.Query("app")); app != "" {
		query = query.Where("alarms.app = ?", app)
	}
	if claimedBy := strings.TrimSpace(c.Query("claimed_by")); claimedBy != "" {
		if id, err := strconv.Atoi(claimedBy); err == nil && id > 0 {
			query = query.Where("alarms.claimed_by = ?", id)
		}
	}
	if claimedByName := strings.TrimSpace(c.Query("claimed_by_name")); claimedByName != "" {
		query = query.Where("u_claim.username LIKE ?", "%"+claimedByName+"%")
	}
	if startFrom, ok := parseTimeParam(c.Query("start_from")); ok {
		query = query.Where("alarms.starts_at >= ?", startFrom)
	}
	if startTo, ok := parseTimeParam(c.Query("start_to")); ok {
		query = query.Where("alarms.starts_at <= ?", startTo)
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

func parseTimeParam(v string) (time.Time, bool) {
	s := strings.TrimSpace(v)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (h *AlarmHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.First(&alarm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}

	var input struct {
		Status model.AlarmStatus `json:"status"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	alarm.Status = input.Status
	if err := model.DB.Save(&alarm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, alarm)
}

func (h *AlarmHandler) Claim(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.First(&alarm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}
	if alarm.ClaimedBy != nil && *alarm.ClaimedBy != userID {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Already claimed by user %d", *alarm.ClaimedBy)})
		return
	}

	now := time.Now()
	res := model.DB.Model(&model.Alarm{}).
		Where("id = ? AND (claimed_by IS NULL OR claimed_by = ?)", alarm.ID, userID).
		Updates(map[string]interface{}{"claimed_by": userID, "claimed_at": now})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already claimed"})
		return
	}

	_ = model.DB.First(&alarm, id).Error
	_ = model.DB.Create(&model.AlertLog{
		AlarmID:     alarm.ID,
		AlertRuleID: alarm.AlertRuleID,
		Status:      "claimed",
		Message:     fmt.Sprintf("claimed by user %d", userID),
		TriggeredAt: time.Now(),
	}).Error
	c.JSON(http.StatusOK, alarm)
}

func (h *AlarmHandler) Unclaim(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.First(&alarm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}
	if alarm.ClaimedBy != nil && *alarm.ClaimedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	if alarm.ClaimedBy == nil {
		c.JSON(http.StatusOK, alarm)
		return
	}

	res := model.DB.Model(&model.Alarm{}).
		Where("id = ? AND claimed_by = ?", alarm.ID, userID).
		Updates(map[string]interface{}{"claimed_by": nil, "claimed_at": nil})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	_ = model.DB.First(&alarm, id).Error
	_ = model.DB.Create(&model.AlertLog{
		AlarmID:     alarm.ID,
		AlertRuleID: alarm.AlertRuleID,
		Status:      "unclaimed",
		Message:     fmt.Sprintf("unclaimed by user %d", userID),
		TriggeredAt: time.Now(),
	}).Error
	c.JSON(http.StatusOK, alarm)
}

func (h *AlarmHandler) Handle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input struct {
		Result string `json:"result"`
		Note   string `json:"note"`
	}
	_ = c.ShouldBindJSON(&input)
	input.Result = strings.TrimSpace(input.Result)
	input.Note = strings.TrimSpace(input.Note)
	if input.Result == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "result is required"})
		return
	}

	var alarm model.Alarm
	if err := model.DB.First(&alarm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alarm not found"})
		return
	}
	if alarm.ClaimedBy != nil && *alarm.ClaimedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	if alarm.HandledBy != nil && *alarm.HandledBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"handled_by":    userID,
		"handled_at":    now,
		"handle_result": input.Result,
		"handle_note":   input.Note,
	}
	if alarm.ClaimedBy == nil {
		updates["claimed_by"] = userID
		updates["claimed_at"] = now
	}
	if err := model.DB.Model(&model.Alarm{}).Where("id = ?", alarm.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = model.DB.First(&alarm, id).Error
	_ = model.DB.Create(&model.AlertLog{
		AlarmID:     alarm.ID,
		AlertRuleID: alarm.AlertRuleID,
		Status:      "handled",
		Message:     fmt.Sprintf("handled by user %d, result=%s", userID, input.Result),
		TriggeredAt: now,
	}).Error
	c.JSON(http.StatusOK, alarm)
}

func (h *AlarmHandler) BatchClaim(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req batchIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusOK, batchResultResp{SucceededIDs: []uint{}, FailedIDs: []uint{}})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}

	type row struct {
		ID          uint  `json:"id"`
		AlertRuleID uint  `json:"alert_rule_id"`
		ClaimedBy   *uint `json:"claimed_by"`
	}
	var rows []row
	if err := model.DB.Model(&model.Alarm{}).Select("id, alert_rule_id, claimed_by").Where("id IN ?", req.IDs).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	claimableIDs := make([]uint, 0, len(rows))
	alreadyIDs := make([]uint, 0, len(rows))
	failedIDs := make([]uint, 0, len(rows))
	rowByID := map[uint]row{}
	for _, r := range rows {
		rowByID[r.ID] = r
		if r.ClaimedBy == nil {
			claimableIDs = append(claimableIDs, r.ID)
		} else if *r.ClaimedBy == userID {
			alreadyIDs = append(alreadyIDs, r.ID)
		} else {
			failedIDs = append(failedIDs, r.ID)
		}
	}
	for _, id := range req.IDs {
		if _, ok := rowByID[id]; !ok {
			failedIDs = append(failedIDs, id)
		}
	}

	if len(claimableIDs) > 0 {
		if err := model.DB.Model(&model.Alarm{}).
			Where("id IN ? AND claimed_by IS NULL", claimableIDs).
			Updates(map[string]interface{}{"claimed_by": userID, "claimed_at": now}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		logs := make([]model.AlertLog, 0, len(claimableIDs))
		for _, id := range claimableIDs {
			r := rowByID[id]
			logs = append(logs, model.AlertLog{
				AlarmID:     id,
				AlertRuleID: r.AlertRuleID,
				Status:      "claimed",
				Message:     fmt.Sprintf("claimed by user %d", userID),
				TriggeredAt: now,
			})
		}
		if len(logs) > 0 {
			_ = model.DB.Create(&logs).Error
		}
	}

	succeeded := append(alreadyIDs, claimableIDs...)
	c.JSON(http.StatusOK, batchResultResp{SucceededIDs: succeeded, FailedIDs: failedIDs})
}

func (h *AlarmHandler) BatchUnclaim(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req batchIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusOK, batchResultResp{SucceededIDs: []uint{}, FailedIDs: []uint{}})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}

	type row struct {
		ID          uint  `json:"id"`
		AlertRuleID uint  `json:"alert_rule_id"`
		ClaimedBy   *uint `json:"claimed_by"`
	}
	var rows []row
	if err := model.DB.Model(&model.Alarm{}).Select("id, alert_rule_id, claimed_by").Where("id IN ?", req.IDs).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	unclaimIDs := make([]uint, 0, len(rows))
	succeededIDs := make([]uint, 0, len(rows))
	failedIDs := make([]uint, 0, len(rows))
	rowByID := map[uint]row{}
	for _, r := range rows {
		rowByID[r.ID] = r
		if r.ClaimedBy == nil {
			succeededIDs = append(succeededIDs, r.ID)
		} else if *r.ClaimedBy == userID {
			unclaimIDs = append(unclaimIDs, r.ID)
		} else {
			failedIDs = append(failedIDs, r.ID)
		}
	}
	for _, id := range req.IDs {
		if _, ok := rowByID[id]; !ok {
			failedIDs = append(failedIDs, id)
		}
	}

	if len(unclaimIDs) > 0 {
		if err := model.DB.Model(&model.Alarm{}).
			Where("id IN ? AND claimed_by = ?", unclaimIDs, userID).
			Updates(map[string]interface{}{"claimed_by": nil, "claimed_at": nil}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logs := make([]model.AlertLog, 0, len(unclaimIDs))
		for _, id := range unclaimIDs {
			r := rowByID[id]
			logs = append(logs, model.AlertLog{
				AlarmID:     id,
				AlertRuleID: r.AlertRuleID,
				Status:      "unclaimed",
				Message:     fmt.Sprintf("unclaimed by user %d", userID),
				TriggeredAt: now,
			})
		}
		if len(logs) > 0 {
			_ = model.DB.Create(&logs).Error
		}
		succeededIDs = append(succeededIDs, unclaimIDs...)
	}

	c.JSON(http.StatusOK, batchResultResp{SucceededIDs: succeededIDs, FailedIDs: failedIDs})
}

func (h *AlarmHandler) BatchHandle(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		IDs    []uint `json:"ids"`
		Result string `json:"result"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Result = strings.TrimSpace(req.Result)
	req.Note = strings.TrimSpace(req.Note)
	if req.Result == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "result is required"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusOK, batchResultResp{SucceededIDs: []uint{}, FailedIDs: []uint{}})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}

	type row struct {
		ID          uint  `json:"id"`
		AlertRuleID uint  `json:"alert_rule_id"`
		ClaimedBy   *uint `json:"claimed_by"`
		HandledBy   *uint `json:"handled_by"`
		Status      string
	}
	var rows []row
	if err := model.DB.Model(&model.Alarm{}).Select("id, alert_rule_id, claimed_by, handled_by, status").Where("id IN ?", req.IDs).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	allowedIDs := make([]uint, 0, len(rows))
	toClaimIDs := make([]uint, 0, len(rows))
	failedIDs := make([]uint, 0, len(rows))
	rowByID := map[uint]row{}
	for _, r := range rows {
		rowByID[r.ID] = r
		if r.Status != string(model.AlarmStatusFiring) {
			failedIDs = append(failedIDs, r.ID)
			continue
		}
		if r.ClaimedBy != nil && *r.ClaimedBy != userID {
			failedIDs = append(failedIDs, r.ID)
			continue
		}
		if r.HandledBy != nil && *r.HandledBy != userID {
			failedIDs = append(failedIDs, r.ID)
			continue
		}
		allowedIDs = append(allowedIDs, r.ID)
		if r.ClaimedBy == nil {
			toClaimIDs = append(toClaimIDs, r.ID)
		}
	}
	for _, id := range req.IDs {
		if _, ok := rowByID[id]; !ok {
			failedIDs = append(failedIDs, id)
		}
	}

	if len(allowedIDs) > 0 {
		if len(toClaimIDs) > 0 {
			_ = model.DB.Model(&model.Alarm{}).
				Where("id IN ? AND claimed_by IS NULL", toClaimIDs).
				Updates(map[string]interface{}{"claimed_by": userID, "claimed_at": now}).Error
		}
		if err := model.DB.Model(&model.Alarm{}).
			Where("id IN ?", allowedIDs).
			Updates(map[string]interface{}{
				"handled_by":    userID,
				"handled_at":    now,
				"handle_result": req.Result,
				"handle_note":   req.Note,
			}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logs := make([]model.AlertLog, 0, len(allowedIDs))
		for _, id := range allowedIDs {
			r := rowByID[id]
			logs = append(logs, model.AlertLog{
				AlarmID:     id,
				AlertRuleID: r.AlertRuleID,
				Status:      "handled",
				Message:     fmt.Sprintf("handled by user %d, result=%s", userID, req.Result),
				TriggeredAt: now,
			})
		}
		if len(logs) > 0 {
			_ = model.DB.Create(&logs).Error
		}
	}

	c.JSON(http.StatusOK, batchResultResp{SucceededIDs: allowedIDs, FailedIDs: failedIDs})
}

func (h *AlarmHandler) BatchResolve(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req batchIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusOK, batchResultResp{SucceededIDs: []uint{}, FailedIDs: []uint{}})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}

	type row struct {
		ID          uint              `json:"id"`
		AlertRuleID uint              `json:"alert_rule_id"`
		Status      model.AlarmStatus `json:"status"`
	}
	var rows []row
	if err := model.DB.Model(&model.Alarm{}).Select("id, alert_rule_id, status").Where("id IN ?", req.IDs).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rowByID := map[uint]row{}
	for _, r := range rows {
		rowByID[r.ID] = r
	}

	now := time.Now()
	succeededIDs := make([]uint, 0, len(rows))
	failedIDs := make([]uint, 0, len(req.IDs))
	resolveIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		if r.Status == model.AlarmStatusResolved {
			succeededIDs = append(succeededIDs, r.ID)
			continue
		}
		if r.Status != model.AlarmStatusFiring {
			failedIDs = append(failedIDs, r.ID)
			continue
		}
		resolveIDs = append(resolveIDs, r.ID)
	}
	for _, id := range req.IDs {
		if _, ok := rowByID[id]; !ok {
			failedIDs = append(failedIDs, id)
		}
	}

	if len(resolveIDs) > 0 {
		if err := model.DB.Model(&model.Alarm{}).Where("id IN ? AND status = ?", resolveIDs, model.AlarmStatusFiring).
			Updates(map[string]interface{}{"status": model.AlarmStatusResolved, "ends_at": now}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logs := make([]model.AlertLog, 0, len(resolveIDs))
		for _, id := range resolveIDs {
			r := rowByID[id]
			logs = append(logs, model.AlertLog{
				AlarmID:     id,
				AlertRuleID: r.AlertRuleID,
				Status:      "resolved",
				Message:     fmt.Sprintf("manual resolved by user %d", userID),
				TriggeredAt: now,
			})
		}
		if len(logs) > 0 {
			_ = model.DB.Create(&logs).Error
		}
		succeededIDs = append(succeededIDs, resolveIDs...)
	}

	c.JSON(http.StatusOK, batchResultResp{SucceededIDs: succeededIDs, FailedIDs: failedIDs})
}

func (h *AlarmHandler) BatchSilence(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := parseUint(raw)
	if err != nil || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		IDs          []uint `json:"ids"`
		DurationMins int    `json:"duration_mins"`
		Comment      string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"created": []model.SilenceRule{}})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}
	if req.DurationMins <= 0 || req.DurationMins > 7*24*60 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid duration_mins"})
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)

	type row struct {
		ID          uint   `json:"id"`
		Fingerprint string `json:"fingerprint"`
		TeamID      uint   `json:"team_id"`
	}
	var rows []row
	if err := model.DB.Table("alarms").
		Select("alarms.id, alarms.fingerprint, alert_rules.team_id as team_id").
		Joins("JOIN alert_rules ON alert_rules.id = alarms.alert_rule_id").
		Where("alarms.id IN ?", req.IDs).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rowByID := map[uint]row{}
	for _, r := range rows {
		rowByID[r.ID] = r
	}

	failedIDs := make([]uint, 0, len(req.IDs))
	for _, id := range req.IDs {
		if _, ok := rowByID[id]; !ok {
			failedIDs = append(failedIDs, id)
		}
	}
	if len(failedIDs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "some ids not found", "failed_ids": failedIDs})
		return
	}

	fpByTeam := map[uint][]string{}
	for _, r := range rows {
		if r.TeamID == 0 || strings.TrimSpace(r.Fingerprint) == "" {
			continue
		}
		fpByTeam[r.TeamID] = append(fpByTeam[r.TeamID], r.Fingerprint)
	}
	if len(fpByTeam) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid alarms to silence"})
		return
	}

	now := time.Now()
	endsAt := now.Add(time.Duration(req.DurationMins) * time.Minute)
	created := make([]model.SilenceRule, 0, len(fpByTeam))
	for teamID, fps := range fpByTeam {
		if len(fps) == 0 {
			continue
		}
		re := "^(" + strings.Join(fps, "|") + ")$"
		matchers, _ := json.Marshal([]model.Matcher{
			{Name: "fingerprint", Value: re, IsRegex: true},
		})
		s := model.SilenceRule{
			Name:      fmt.Sprintf("Batch silence (%d alarms)", len(fps)),
			TeamID:    teamID,
			Matchers:  string(matchers),
			StartsAt:  now,
			EndsAt:    endsAt,
			CreatedBy: fmt.Sprintf("%d", userID),
			Comment:   req.Comment,
		}
		if err := model.DB.Create(&s).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created = append(created, s)
	}

	c.JSON(http.StatusOK, gin.H{"created": created})
}

func parseUint(v interface{}) (uint, error) {
	switch x := v.(type) {
	case uint:
		return x, nil
	case uint64:
		return uint(x), nil
	case int:
		if x < 0 {
			return 0, fmt.Errorf("invalid")
		}
		return uint(x), nil
	case int64:
		if x < 0 {
			return 0, fmt.Errorf("invalid")
		}
		return uint(x), nil
	case float64:
		if x < 0 {
			return 0, fmt.Errorf("invalid")
		}
		return uint(x), nil
	case string:
		n, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint(n), nil
	default:
		return 0, fmt.Errorf("invalid")
	}
}
