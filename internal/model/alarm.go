package model

import (
	"time"

	"gorm.io/gorm"
)

type AlarmStatus string

const (
	AlarmStatusFiring   AlarmStatus = "firing"
	AlarmStatusResolved AlarmStatus = "resolved"
)

type Alarm struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	AlertRuleID  uint           `json:"alert_rule_id"`
	AlertRule    AlertRule      `gorm:"foreignKey:AlertRuleID" json:"-"`
	Fingerprint  string         `gorm:"size:255;index" json:"fingerprint"` // Unique identifier
	Status       AlarmStatus    `gorm:"size:20;not null" json:"status"`
	Content      string         `gorm:"type:text" json:"content"`
	Payload      string         `gorm:"type:text" json:"payload"`
	Labels       string         `gorm:"type:text" json:"labels"`
	Service      string         `gorm:"size:255;index" json:"service"`
	App          string         `gorm:"size:255;index" json:"app"`
	StartsAt     time.Time      `json:"starts_at"`
	LastSeenAt   time.Time      `json:"last_seen_at"`
	EndsAt       *time.Time     `json:"ends_at"`
	FireCount    int            `gorm:"default:0" json:"fire_count"`
	ClaimedBy    *uint          `json:"claimed_by"`
	ClaimedAt    *time.Time     `json:"claimed_at"`
	HandledBy    *uint          `json:"handled_by"`
	HandledAt    *time.Time     `json:"handled_at"`
	HandleResult string         `gorm:"size:50" json:"handle_result"`
	HandleNote   string         `gorm:"type:text" json:"handle_note"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
