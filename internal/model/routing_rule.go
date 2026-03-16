package model

import (
	"time"

	"gorm.io/gorm"
)

type RoutingRule struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Name               string         `gorm:"size:100;not null" json:"name"`
	TeamID             uint           `gorm:"index;not null" json:"team_id"`
	IsEnabled          bool           `gorm:"default:true" json:"is_enabled"`
	Priority           int            `gorm:"default:0" json:"priority"`
	Matchers           string         `gorm:"type:text;not null" json:"matchers"`
	NotificationProps  string         `gorm:"type:text" json:"notification_props"`
	NotificationConfig string         `gorm:"type:text" json:"notification_config"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}
