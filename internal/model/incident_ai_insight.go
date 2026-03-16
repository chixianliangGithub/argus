package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type IncidentAIInsight struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	IncidentID    uint            `gorm:"index;not null" json:"incident_id"`
	DataSourceID  uint            `gorm:"index;not null" json:"datasource_id"`
	Message       string          `gorm:"type:text" json:"message"`
	Query         string          `gorm:"type:text" json:"query"`
	Summary       string          `gorm:"type:text" json:"summary"`
	ChartConfig   json.RawMessage `gorm:"type:json" json:"chart_config"`
	ResultSnippet string          `gorm:"type:text" json:"result_snippet"`
	CreatedBy     *uint           `gorm:"index" json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}
