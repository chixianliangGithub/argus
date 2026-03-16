package handler

import (
	"net/http"
	"strconv"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AIQuotaHandler struct{}

func (h *AIQuotaHandler) Get(c *gin.Context) {
	teamID, err := strconv.Atoi(c.Param("team_id"))
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team_id"})
		return
	}
	var q model.AIQuota
	if err := model.DB.Where("team_id = ?", teamID).First(&q).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"team_id": teamID, "enabled": false, "monthly_token_limit": 0})
		return
	}
	c.JSON(http.StatusOK, q)
}

func (h *AIQuotaHandler) Upsert(c *gin.Context) {
	teamID, err := strconv.Atoi(c.Param("team_id"))
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team_id"})
		return
	}
	var req struct {
		Enabled           bool `json:"enabled"`
		MonthlyTokenLimit int  `json:"monthly_token_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.MonthlyTokenLimit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monthly_token_limit must be >= 0"})
		return
	}
	var q model.AIQuota
	if err := model.DB.Where("team_id = ?", teamID).First(&q).Error; err != nil {
		q = model.AIQuota{TeamID: uint(teamID), Enabled: req.Enabled, MonthlyTokenLimit: req.MonthlyTokenLimit}
		if err := model.DB.Create(&q).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, q)
		return
	}
	q.Enabled = req.Enabled
	q.MonthlyTokenLimit = req.MonthlyTokenLimit
	if err := model.DB.Save(&q).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, q)
}
