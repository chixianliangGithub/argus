package model

import (
	"time"

	"gorm.io/gorm"
)

type AIPromptVersion struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Scene       string         `gorm:"size:120;index;not null" json:"scene"`
	Version     string         `gorm:"size:100;index;not null" json:"version"`
	Content     string         `gorm:"type:text" json:"content"`
	IsActive    bool           `gorm:"index;default:false" json:"is_active"`
	Description string         `gorm:"size:255" json:"description"`
	CreatedAt   time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
