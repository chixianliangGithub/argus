package model

import (
	"time"

	"gorm.io/gorm"
)

type AlertLog struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	AlarmID     uint           `json:"alarm_id"`
	Alarm       Alarm          `gorm:"foreignKey:AlarmID" json:"-"`
	AlertRuleID uint           `json:"alert_rule_id"` // Snapshot of rule ID
	Status      string         `gorm:"size:20" json:"status"`
	Message     string         `gorm:"type:text" json:"message"`
	Value       float64        `json:"value"`
	TriggeredAt time.Time      `json:"triggered_at"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
