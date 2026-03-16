package handler

import (
	"net/http"
	"strconv"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type RCAHandler struct{}

func (h *RCAHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var report model.RCAReport
	if err := model.DB.Preload("Alarm.AlertRule").First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RCA report not found"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *RCAHandler) List(c *gin.Context) {
	var reports []model.RCAReport
	// Optionally filter by alarm_id
	query := model.DB.Preload("Alarm.AlertRule")
	if alarmID := c.Query("alarm_id"); alarmID != "" {
		query = query.Where("alarm_id = ?", alarmID)
	}

	if err := query.Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reports)
}
