package model

import (
	"time"
)

type AIAudit struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	TraceID          string    `gorm:"size:64;index;not null" json:"trace_id"`
	UserID           *uint     `gorm:"index" json:"user_id"`
	TeamID           *uint     `gorm:"index" json:"team_id"`
	Path             string    `gorm:"size:200" json:"path"`
	Provider         string    `gorm:"size:50" json:"provider"`
	Model            string    `gorm:"size:100" json:"model"`
	PromptVersion    string    `gorm:"size:100" json:"prompt_version"`
	PromptHash       string    `gorm:"size:64" json:"prompt_hash"`
	InputExcerpt     string    `gorm:"type:text" json:"input_excerpt"`
	OutputExcerpt    string    `gorm:"type:text" json:"output_excerpt"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}
