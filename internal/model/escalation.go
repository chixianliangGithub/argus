package model

import (
	"time"

	"gorm.io/gorm"
)

type EscalationPolicy struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Steps       string         `gorm:"type:text;not null" json:"steps"` // JSON: [{"wait_minutes": 10, "channel_ids": [1, 2]}]
	RepeatTimes int            `gorm:"default:0" json:"repeat_times"`   // Number of times to repeat the policy
	TeamID      uint           `json:"team_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type EscalationStep struct {
	WaitMinutes int    `json:"wait_minutes"`
	ChannelIDs  []uint `json:"channel_ids"`
}
