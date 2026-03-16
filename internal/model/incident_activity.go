package model

import (
	"time"

	"gorm.io/gorm"
)

type IncidentActivityType string

const (
	IncidentActivityAcked     IncidentActivityType = "acked"
	IncidentActivityResolved  IncidentActivityType = "resolved"
	IncidentActivityComment   IncidentActivityType = "comment"
	IncidentActivityAIInsight IncidentActivityType = "ai_insight"
	IncidentActivityRunbook   IncidentActivityType = "runbook"
)

type IncidentActivity struct {
	ID         uint                 `gorm:"primaryKey" json:"id"`
	IncidentID uint                 `gorm:"index;not null" json:"incident_id"`
	Type       IncidentActivityType `gorm:"size:30;index;not null" json:"type"`
	Message    string               `gorm:"type:text" json:"message"`
	RefType    string               `gorm:"size:50;index" json:"ref_type"`
	RefID      *uint                `gorm:"index" json:"ref_id"`
	Meta       string               `gorm:"type:text" json:"meta"`
	CreatedBy  *uint                `gorm:"index" json:"created_by"`
	CreatedAt  time.Time            `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
	DeletedAt  gorm.DeletedAt       `gorm:"index" json:"-"`
}
