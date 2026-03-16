package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type SilenceHandler struct{}

func (h *SilenceHandler) Create(c *gin.Context) {
	var silence model.SilenceRule
	if err := c.ShouldBindJSON(&silence); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if silence.TeamID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id is required"})
		return
	}
	if silence.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if silence.Matchers == "" {
		silence.Matchers = "[]"
	}

	// Validate time
	if silence.StartsAt.IsZero() {
		silence.StartsAt = time.Now()
	}
	if silence.EndsAt.IsZero() || silence.EndsAt.Before(silence.StartsAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end time"})
		return
	}

	if err := model.DB.Create(&silence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, silence)
}

func (h *SilenceHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var silence model.SilenceRule
	if err := model.DB.First(&silence, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Silence rule not found"})
		return
	}

	var req model.SilenceRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TeamID != 0 && req.TeamID != silence.TeamID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id cannot be changed"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.Matchers == "" {
		req.Matchers = "[]"
	}

	startsAt := req.StartsAt
	if startsAt.IsZero() {
		startsAt = time.Now()
	}
	endsAt := req.EndsAt
	if endsAt.IsZero() || endsAt.Before(startsAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end time"})
		return
	}

	silence.Name = req.Name
	silence.Matchers = req.Matchers
	silence.StartsAt = startsAt
	silence.EndsAt = endsAt
	silence.Comment = req.Comment

	if err := model.DB.Save(&silence).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, silence)
}

func (h *SilenceHandler) List(c *gin.Context) {
	var silences []model.SilenceRule
	// Auto-expire check could be done here or via background job
	// For now just list all
	query := model.DB.Order("created_at desc")
	if teamID := c.Query("team_id"); teamID != "" {
		if id, err := strconv.Atoi(teamID); err == nil && id > 0 {
			query = query.Where("team_id = ?", id)
		}
	}
	if err := query.Find(&silences).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, silences)
}

func (h *SilenceHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := model.DB.Delete(&model.SilenceRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Silence rule deleted"})
}
