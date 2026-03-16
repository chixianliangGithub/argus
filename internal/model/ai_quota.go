package model

import (
	"time"

	"gorm.io/gorm"
)

type AIQuota struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	TeamID            uint           `gorm:"uniqueIndex;not null" json:"team_id"`
	Enabled           bool           `gorm:"default:false" json:"enabled"`
	MonthlyTokenLimit int            `json:"monthly_token_limit"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}
