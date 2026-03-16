package model

import (
	"time"

	"gorm.io/gorm"
)

type AuditLog struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Username   string         `gorm:"size:50" json:"username"`
	Action     string         `gorm:"size:50;not null" json:"action"`   // CREATE, UPDATE, DELETE
	Resource   string         `gorm:"size:50;not null" json:"resource"` // User, DataSource, AlertRule
	ResourceID string         `gorm:"size:50" json:"resource_id"`
	ClientIP   string         `gorm:"size:50" json:"client_ip"`
	Details    string         `gorm:"type:text" json:"details"` // JSON diff or description
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
