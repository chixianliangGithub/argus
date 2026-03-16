package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type IncidentAIInsightHandler struct {
	LLMClient ai.LLMClient
}

func NewIncidentAIInsightHandler(client ai.LLMClient) *IncidentAIInsightHandler {
	return &IncidentAIInsightHandler{LLMClient: client}
}

type incidentInsightCreateRequest struct {
	DataSourceID uint   `json:"datasource_id"`
	Message      string `json:"message" binding:"required"`
}

func (h *IncidentAIInsightHandler) Create(c *gin.Context) {
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

	var req incidentInsightCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	if req.DataSourceID == 0 {
		var rels []model.IncidentAlarm
		_ = model.DB.Where("incident_id = ?", inc.ID).Find(&rels).Error
		alarmIDs := make([]uint, 0, len(rels))
		for _, r := range rels {
			alarmIDs = append(alarmIDs, r.AlarmID)
		}
		cands, recommendedDataNameID := getIncidentDataNameCandidatesByAlarmIDs(alarmIDs)
		if recommendedDataNameID > 0 {
			for _, c := range cands {
				if c.ID == recommendedDataNameID && c.DataSourceID > 0 {
					req.DataSourceID = c.DataSourceID
					break
				}
			}
		}
	}
	if req.DataSourceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "datasource_id is required"})
		return
	}

	var ds model.DataSource
	if err := model.DB.First(&ds, req.DataSourceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DataSource not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	traceID := newTraceID()
	teamID := inc.TeamID
	if err := enforceTeamQuota(&teamID); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	query, summary, chartConfig, usage, modelName, snippet, promptVersion, promptHash, err := generateInsight(ctx, h.LLMClient, ds, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var createdBy *uint
	if raw, ok := c.Get("user_id"); ok {
		if uid, err := parseUint(raw); err == nil && uid > 0 {
			createdBy = &uid
		}
	}

	chartJSON, _ := json.Marshal(chartConfig)
	rec := model.IncidentAIInsight{
		IncidentID:    inc.ID,
		DataSourceID:  ds.ID,
		Message:       req.Message,
		Query:         query,
		Summary:       summary,
		ChartConfig:   chartJSON,
		ResultSnippet: snippet,
		CreatedBy:     createdBy,
	}
	if err := model.DB.Create(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metaStr := ""
	if b, err := json.Marshal(map[string]interface{}{
		"insight_id":    rec.ID,
		"datasource_id": rec.DataSourceID,
	}); err == nil {
		metaStr = string(b)
	}
	_ = model.DB.Create(&model.IncidentActivity{
		IncidentID: inc.ID,
		Type:       model.IncidentActivityAIInsight,
		Message:    summary,
		RefType:    "incident_ai_insight",
		RefID:      &rec.ID,
		Meta:       metaStr,
		CreatedBy:  createdBy,
	}).Error
	_ = model.DB.Model(&model.Incident{}).Where("id = ?", inc.ID).Update("last_activity_at", time.Now()).Error
	c.Header("X-Trace-Id", traceID)
	writeAIAudit(traceID, createdBy, &teamID, fmt.Sprintf("/api/incidents/%d/ai-insights", inc.ID), providerFromClient(h.LLMClient), modelName, promptVersion, promptHash, req.Message, summary, usage)

	c.JSON(http.StatusOK, rec)
}

func (h *IncidentAIInsightHandler) List(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
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

	query := model.DB.Model(&model.IncidentAIInsight{}).Where("incident_id = ?", id)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []model.IncidentAIInsight
	if err := query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
}
