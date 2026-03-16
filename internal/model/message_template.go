package model

import (
	"time"

	"gorm.io/gorm"
)

type MessageTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Type      string         `gorm:"size:50;not null;default:'general'" json:"type"`
	Format    string         `gorm:"size:20;default:'text'" json:"format"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	IsDefault bool           `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
