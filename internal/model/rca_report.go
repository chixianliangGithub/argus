package model

import (
	"time"

	"gorm.io/gorm"
)

type RCAReport struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	AlarmID   uint           `gorm:"index" json:"alarm_id"`
	Alarm     Alarm          `gorm:"foreignKey:AlarmID" json:"-"`
	RootCause string         `gorm:"type:text" json:"root_cause"`
	Evidence  string         `gorm:"type:text" json:"evidence"` // JSON string of gathered data
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
