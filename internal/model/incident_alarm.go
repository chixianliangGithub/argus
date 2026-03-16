package model

import (
	"time"

	"gorm.io/gorm"
)

type IncidentAlarm struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	IncidentID   uint           `gorm:"index;not null;uniqueIndex:idx_incident_alarm" json:"incident_id"`
	AlarmID      uint           `gorm:"index;not null;uniqueIndex:idx_incident_alarm" json:"alarm_id"`
	RelationType string         `gorm:"size:30" json:"relation_type"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
