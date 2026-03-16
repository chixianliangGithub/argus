package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AIPromptHandler struct{}

func (h *AIPromptHandler) List(c *gin.Context) {
	scene := strings.TrimSpace(c.Query("scene"))
	q := model.DB.Model(&model.AIPromptVersion{}).Order("created_at desc")
	if scene != "" {
		q = q.Where("scene = ?", scene)
	}
	var rows []model.AIPromptVersion
	if err := q.Limit(500).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *AIPromptHandler) Create(c *gin.Context) {
	var req struct {
		Scene       string `json:"scene" binding:"required"`
		Version     string `json:"version" binding:"required"`
		Content     string `json:"content" binding:"required"`
		IsActive    bool   `json:"is_active"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Scene = strings.TrimSpace(req.Scene)
	req.Version = strings.TrimSpace(req.Version)
	if req.Scene == "" || req.Version == "" || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene, version, content are required"})
		return
	}

	rec := model.AIPromptVersion{
		Scene:       req.Scene,
		Version:     req.Version,
		Content:     req.Content,
		IsActive:    req.IsActive,
		Description: strings.TrimSpace(req.Description),
	}
	tx := model.DB.Begin()
	if err := tx.Create(&rec).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rec.IsActive {
		if err := tx.Model(&model.AIPromptVersion{}).Where("scene = ? AND id <> ?", rec.Scene, rec.ID).Update("is_active", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	tx.Commit()
	c.JSON(http.StatusCreated, rec)
}

func (h *AIPromptHandler) Activate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var rec model.AIPromptVersion
	if err := model.DB.First(&rec, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	tx := model.DB.Begin()
	if err := tx.Model(&model.AIPromptVersion{}).Where("scene = ?", rec.Scene).Update("is_active", false).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Model(&model.AIPromptVersion{}).Where("id = ?", rec.ID).Update("is_active", true).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tx.Commit()
	rec.IsActive = true
	c.JSON(http.StatusOK, rec)
}
