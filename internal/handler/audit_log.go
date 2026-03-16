package handler

import (
	"net/http"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AuditLogHandler struct{}

func (h *AuditLogHandler) List(c *gin.Context) {
	var logs []model.AuditLog
	query := model.DB.Model(&model.AuditLog{}).Order("created_at desc")

	if username := c.Query("username"); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource LIKE ?", "%"+resource+"%")
	}

	// TODO: Pagination
	if err := query.Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}
