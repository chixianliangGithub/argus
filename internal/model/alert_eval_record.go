package model

import (
	"time"

	"gorm.io/gorm"
)

type AlertEvalStatus string

const (
	AlertEvalStatusRunning                   AlertEvalStatus = "running"
	AlertEvalStatusSuccess                   AlertEvalStatus = "success"
	AlertEvalStatusNoTrigger                 AlertEvalStatus = "no_trigger"
	AlertEvalStatusNoData                    AlertEvalStatus = "no_data"
	AlertEvalStatusError                     AlertEvalStatus = "error"
	AlertEvalStatusSkippedDisabled           AlertEvalStatus = "skipped_disabled"
	AlertEvalStatusSkippedOutOfEffectiveTime AlertEvalStatus = "skipped_out_of_effective_time"
)

type AlertEvalRecord struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	AlertRuleID  uint            `gorm:"index;not null" json:"alert_rule_id"`
	Forced       bool            `gorm:"default:false" json:"forced"`
	Status       AlertEvalStatus `gorm:"size:50;index;not null" json:"status"`
	ErrorMessage string          `gorm:"type:text" json:"error_message"`
	SeriesCount  int             `json:"series_count"`
	TriggerCount int             `json:"trigger_count"`
	EvalAt       *time.Time      `json:"eval_at"`
	StartedAt    time.Time       `gorm:"index" json:"started_at"`
	FinishedAt   *time.Time      `gorm:"index" json:"finished_at"`
	DurationMs   int64           `json:"duration_ms"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`
}
