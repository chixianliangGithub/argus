package model

import (
	"time"

	"gorm.io/gorm"
)

type DataSourceType string

const (
	DataSourceTypePrometheus    DataSourceType = "prometheus"
	DataSourceTypeElasticsearch DataSourceType = "elasticsearch"
	DataSourceTypeInfluxDB      DataSourceType = "influxdb"
	DataSourceTypeClickHouse    DataSourceType = "clickhouse"
	DataSourceTypeMySQL         DataSourceType = "mysql"
	DataSourceTypeSkyWalking    DataSourceType = "skywalking"
	DataSourceTypeIoTDB         DataSourceType = "iotdb"
	DataSourceTypeSqlServer     DataSourceType = "sqlserver"
	DataSourceTypeMock          DataSourceType = "mock"
)

type DataSource struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Type        DataSourceType `gorm:"size:50;not null" json:"type"`
	URL         string         `gorm:"size:255;not null" json:"url"`
	Username    string         `gorm:"size:100" json:"username"`
	Password    string         `gorm:"size:100" json:"password,omitempty"`
	Database    string         `gorm:"size:100" json:"database"` // Database name or index name
	Config      string         `gorm:"type:text" json:"config"`  // JSON string for extra config
	Description string         `gorm:"size:255" json:"description"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
