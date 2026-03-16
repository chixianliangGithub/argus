package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Runbook struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:255;not null" json:"name"`
	Description string          `gorm:"type:text" json:"description"`
	TriggerType string          `gorm:"size:50;not null" json:"trigger_type"` // e.g., "alert_rule", "keyword"
	TriggerVal  string          `gorm:"type:text" json:"trigger_val"`         // specific rule ID or keywords
	Steps       json.RawMessage `gorm:"type:json" json:"steps"`               // detailed steps configuration
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

type RunbookExecution struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	RunbookID   uint            `gorm:"not null" json:"runbook_id"`
	Runbook     Runbook         `json:"runbook"`
	AlertID     uint            `json:"alert_id"`                                // Optional, if triggered by an alert
	IncidentID  *uint           `gorm:"index" json:"incident_id"`                // Optional, if triggered by an incident
	Status      string          `gorm:"size:50;default:'pending'" json:"status"` // pending, analyzing, waiting_approval, executing, completed, failed
	CurrentStep int             `gorm:"default:0" json:"current_step"`
	Context     json.RawMessage `gorm:"type:json" json:"context"` // Snapshot of alert data or initial params
	Logs        json.RawMessage `gorm:"type:json" json:"logs"`    // Execution logs
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Helper methods for Runbook steps
type RunbookStep struct {
	Type    string                 `json:"type"`    // analyze, approval, action
	Content string                 `json:"content"` // prompt for AI, or script to run
	Params  map[string]interface{} `json:"params"`
}
