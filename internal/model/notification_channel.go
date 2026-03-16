package model

import (
	"time"

	"gorm.io/gorm"
)

type NotificationChannel struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`   // dingtalk, feishu, wechat, slack, email, webhook
	Config    string         `json:"config"` // JSON string
	TeamID    *uint          `json:"team_id"`
	Team      *Team          `gorm:"foreignKey:TeamID" json:"team"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
