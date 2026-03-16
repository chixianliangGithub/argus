package model

import (
	"time"

	"gorm.io/gorm"
)

type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "open"
	IncidentStatusAcked    IncidentStatus = "acked"
	IncidentStatusResolved IncidentStatus = "resolved"
)

type IncidentSeverity string

const (
	IncidentSeverityCritical IncidentSeverity = "critical"
	IncidentSeverityWarning  IncidentSeverity = "warning"
	IncidentSeverityInfo     IncidentSeverity = "info"
)

type Incident struct {
	ID             uint             `gorm:"primaryKey" json:"id"`
	TeamID         uint             `gorm:"index;not null" json:"team_id"`
	Title          string           `gorm:"size:255" json:"title"`
	Summary        string           `gorm:"type:text" json:"summary"`
	Status         IncidentStatus   `gorm:"size:30;index;not null" json:"status"`
	Severity       IncidentSeverity `gorm:"size:30;index" json:"severity"`
	DedupKey       string           `gorm:"size:255;index;not null" json:"dedup_key"`
	GroupKey       string           `gorm:"size:255;index" json:"group_key"`
	Tags           string           `gorm:"type:text" json:"tags"`
	RootAlarmID    *uint            `gorm:"index" json:"root_alarm_id"`
	Impact         string           `gorm:"type:text" json:"impact"`
	RootCause      string           `gorm:"type:text" json:"root_cause"`
	Classification string           `gorm:"size:100" json:"classification"`
	Mitigation     string           `gorm:"type:text" json:"mitigation"`
	Verification   string           `gorm:"type:text" json:"verification"`
	RollbackPlan   string           `gorm:"type:text" json:"rollback_plan"`
	FollowUps      string           `gorm:"type:text" json:"follow_ups"`
	OpenedAt       time.Time        `gorm:"index" json:"opened_at"`
	AckedAt        *time.Time       `gorm:"index" json:"acked_at"`
	ResolvedAt     *time.Time       `gorm:"index" json:"resolved_at"`
	LastActivityAt time.Time        `gorm:"index" json:"last_activity_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"-"`
}
