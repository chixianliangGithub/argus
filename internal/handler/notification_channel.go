package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/notification"
	"github.com/gin-gonic/gin"
)

type NotificationChannelHandler struct{}

type channelUsage struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	Rules []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"rules"`
}

type channelWithUsage struct {
	model.NotificationChannel
	Usage channelUsage `json:"usage"`
}

func (h *NotificationChannelHandler) Create(c *gin.Context) {
	var ch model.NotificationChannel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Create(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ch)
}

func (h *NotificationChannelHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var ch model.NotificationChannel
	if err := model.DB.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification channel not found"})
		return
	}

	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Save(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ch)
}

func (h *NotificationChannelHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := model.DB.Delete(&model.NotificationChannel{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notification channel deleted"})
}

func (h *NotificationChannelHandler) List(c *gin.Context) {
	var channels []model.NotificationChannel
	query := model.DB.Model(&model.NotificationChannel{}).Preload("Team")

	if teamID := c.Query("team_id"); teamID != "" {
		if id, err := strconv.Atoi(teamID); err == nil {
			query = query.Where("team_id = ? OR team_id IS NULL", id)
		}
	}
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if err := query.Order("id desc").Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	usageByID := make(map[uint]*channelUsage)
	for _, ch := range channels {
		usageByID[ch.ID] = &channelUsage{ID: ch.ID, Name: ch.Name}
	}

	var rules []model.AlertRule
	if err := model.DB.Model(&model.AlertRule{}).Select("id,name,notification_props").Find(&rules).Error; err == nil {
		for _, r := range rules {
			var props struct {
				ChannelIDs []uint `json:"channel_ids"`
			}
			if r.NotificationProps == "" {
				continue
			}
			if err := json.Unmarshal([]byte(r.NotificationProps), &props); err != nil {
				continue
			}
			for _, cid := range props.ChannelIDs {
				u := usageByID[cid]
				if u == nil {
					continue
				}
				u.Count++
				u.Rules = append(u.Rules, struct {
					ID   uint   `json:"id"`
					Name string `json:"name"`
				}{ID: r.ID, Name: r.Name})
			}
		}
	}

	out := make([]channelWithUsage, 0, len(channels))
	for _, ch := range channels {
		u := usageByID[ch.ID]
		if u == nil {
			u = &channelUsage{ID: ch.ID, Name: ch.Name}
		}
		out = append(out, channelWithUsage{
			NotificationChannel: ch,
			Usage:               *u,
		})
	}

	c.JSON(http.StatusOK, out)
}

func (h *NotificationChannelHandler) Test(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var ch model.NotificationChannel
	if err := model.DB.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification channel not found"})
		return
	}

	var req struct {
		Content string `json:"content"`
		Format  string `json:"format"`
		To      string `json:"to"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Content == "" {
		req.Content = "Argus test notification"
	}
	if req.Format == "" {
		req.Format = "text"
	}

	sender, err := notification.GetSender(ch.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var config map[string]string
	if err := json.Unmarshal([]byte(ch.Config), &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel config JSON"})
		return
	}
	config["template"] = req.Content
	config["format"] = req.Format
	if req.To != "" {
		config["to"] = req.To
	}

	now := time.Now()
	alarm := model.Alarm{
		Status:   model.AlarmStatusFiring,
		Content:  req.Content,
		StartsAt: now,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := sender.Send(ctx, &alarm, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}
