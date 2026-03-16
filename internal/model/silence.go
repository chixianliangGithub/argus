package model

import (
	"time"

	"gorm.io/gorm"
)

type SilenceRule struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100" json:"name"`
	TeamID    uint           `json:"team_id"`
	Matchers  string         `gorm:"type:text;not null" json:"matchers"` // JSON: [{"name": "host", "value": "server-1", "isRegex": false}]
	StartsAt  time.Time      `json:"starts_at"`
	EndsAt    time.Time      `json:"ends_at"`
	CreatedBy string         `gorm:"size:50" json:"created_by"`
	Comment   string         `gorm:"size:255" json:"comment"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
}
