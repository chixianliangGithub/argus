package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type InhibitRuleHandler struct{}

func (h *InhibitRuleHandler) List(c *gin.Context) {
	var rules []model.InhibitRule
	q := model.DB.Model(&model.InhibitRule{})
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

func (h *InhibitRuleHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var rule model.InhibitRule
	if err := model.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *InhibitRuleHandler) Create(c *gin.Context) {
	var rule model.InhibitRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.Name = strings.TrimSpace(rule.Name)
	rule.EqualLabels = strings.TrimSpace(rule.EqualLabels)
	rule.Description = strings.TrimSpace(rule.Description)
	if rule.TeamID == 0 || rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id and name are required"})
		return
	}
	if strings.TrimSpace(rule.Source) == "" {
		rule.Source = "[]"
	}
	if strings.TrimSpace(rule.Target) == "" {
		rule.Target = "[]"
	}
	if err := model.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *InhibitRuleHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var existing model.InhibitRule
	if err := model.DB.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	var input model.InhibitRule
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
	if strings.TrimSpace(input.Source) != "" {
		existing.Source = input.Source
	}
	if strings.TrimSpace(input.Target) != "" {
		existing.Target = input.Target
	}
	existing.EqualLabels = strings.TrimSpace(input.EqualLabels)
	existing.Description = strings.TrimSpace(input.Description)
	if err := model.DB.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *InhibitRuleHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := model.DB.Delete(&model.InhibitRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}
