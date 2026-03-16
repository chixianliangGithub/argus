package model

import (
	"time"

	"gorm.io/gorm"
)

type AlertRule struct {
	ID            uint     `gorm:"primaryKey" json:"id"`
	Name          string   `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DataNameID    uint     `json:"data_name_id"`
	DataName      DataName `gorm:"foreignKey:DataNameID" json:"-"`
	Query         string   `gorm:"type:text;not null" json:"query"` // Query to execute
	Duration      int      `gorm:"default:60" json:"duration"`      // Duration in seconds
	Level         string   `gorm:"size:20;default:'warning'" json:"level"`
	Condition     string   `gorm:"size:20;not null" json:"condition"` // >, <, =
	Threshold     float64  `json:"threshold"`
	Algorithm     string   `gorm:"size:20;default:'static'" json:"algorithm"` // static, 3sigma, mad
	AlgoParams    string   `gorm:"type:text" json:"algo_params"`              // JSON parameters for algorithm
	SilencePeriod int      `gorm:"default:5" json:"silence_period"`           // Silence period in minutes
	SilenceRuleID uint     `json:"silence_rule_id"`
	GroupBy       string   `gorm:"size:255" json:"group_by"` // Comma-separated labels for grouping

	// Enhanced Configuration
	MessageTemplateID  uint   `json:"message_template_id"`
	NotificationProps  string `gorm:"type:text" json:"notification_props"` // JSON: { "channel_ids": [1, 2] }
	Annotations        string `gorm:"type:text" json:"annotations"`        // JSON: { "summary": "...", "runbook": "..." }
	EvalWindow         string `gorm:"size:20;default:'0'" json:"eval_window"`
	EvalConsecutive    int    `gorm:"default:1" json:"eval_consecutive"`
	EffectiveTime      string `gorm:"type:text" json:"effective_time"` // JSON: { "days": [1,2,3,4,5], "start": "09:00", "end": "18:00" }
	ValueField         string `gorm:"size:100" json:"value_field"`
	ValueMode          string `gorm:"size:20;default:'auto'" json:"value_mode"`
	RowPick            string `gorm:"size:20;default:'first'" json:"row_pick"`
	NotifyOnResolve    bool   `gorm:"default:true" json:"notify_on_resolve"`
	EscalationEnabled  bool   `gorm:"default:false" json:"escalation_enabled"`
	EscalationPolicyID uint   `json:"escalation_policy_id"`
	EscalationAfter    int    `gorm:"default:1" json:"escalation_after"`

	// Advanced Scheduling & Notification
	CronExpression     string `gorm:"size:100" json:"cron_expression"`
	NotificationConfig string `gorm:"type:text" json:"notification_config"` // JSON: { "channels": ["dingtalk"], "receivers": ["user:1"], "webhooks": [...] }
	CallbackURL        string `gorm:"size:500" json:"callback_url"`

	TeamID      uint           `json:"team_id"`
	Team        Team           `gorm:"foreignKey:TeamID" json:"team"`
	IsEnabled   bool           `gorm:"default:true" json:"is_enabled"`
	Description string         `gorm:"size:255" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
