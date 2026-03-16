package model

import (
	"time"

	"gorm.io/gorm"
)

// DataName represents a logical grouping of data, often associated with a DataSource
type DataName struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DataSourceID uint           `json:"data_source_id"`
	DataSource   DataSource     `gorm:"foreignKey:DataSourceID" json:"data_source"`
	TimeField    string         `gorm:"size:100" json:"time_field"`
	Description  string         `gorm:"size:255" json:"description"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
