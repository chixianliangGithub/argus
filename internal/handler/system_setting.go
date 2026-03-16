package handler

import (
	"net/http"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type SystemSettingHandler struct{}

func (h *SystemSettingHandler) GetTwoFA(c *gin.Context) {
	enabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (h *SystemSettingHandler) SetTwoFA(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v := "false"
	if req.Enabled {
		v = "true"
	}
	if err := model.SetSetting(model.SettingKeySecurityTwoFAEnabled, v); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}
