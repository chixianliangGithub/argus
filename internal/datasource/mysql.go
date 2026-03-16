package datasource

import (
	"context"
	"fmt"

	"github.com/argus-monitoring/argus/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MySQLDataSource struct {
	db *gorm.DB
}

func NewMySQLDataSource(ds *model.DataSource) (*MySQLDataSource, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		ds.Username, ds.Password, ds.URL, ds.Database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &MySQLDataSource{db: db}, nil
}

func (m *MySQLDataSource) Query(ctx context.Context, query string) (*Result, error) {
	var results []map[string]interface{}
	tx := m.db.WithContext(ctx).Raw(query).Scan(&results)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &Result{Data: results}, nil
}

func (m *MySQLDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for MySQL")
}

func (m *MySQLDataSource) Type() string {
	return "mysql"
}
