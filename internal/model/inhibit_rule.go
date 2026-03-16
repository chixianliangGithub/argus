package model

import (
	"time"

	"gorm.io/gorm"
)

type InhibitRule struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	TeamID      uint           `gorm:"index;not null" json:"team_id"`
	IsEnabled   bool           `gorm:"default:true" json:"is_enabled"`
	Priority    int            `gorm:"default:0" json:"priority"`
	Source      string         `gorm:"type:text;not null" json:"source"` // JSON: []Matcher
	Target      string         `gorm:"type:text;not null" json:"target"` // JSON: []Matcher
	EqualLabels string         `gorm:"size:255" json:"equal_labels"`     // comma-separated label names
	Description string         `gorm:"size:255" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
