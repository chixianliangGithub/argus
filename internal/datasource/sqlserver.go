package datasource

import (
	"context"
	"fmt"

	"github.com/argus-monitoring/argus/internal/model"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

type SqlServerDataSource struct {
	db *gorm.DB
}

func NewSqlServerDataSource(ds *model.DataSource) (*SqlServerDataSource, error) {
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s?database=%s",
		ds.Username, ds.Password, ds.URL, ds.Database)
	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &SqlServerDataSource{db: db}, nil
}

func (s *SqlServerDataSource) Query(ctx context.Context, query string) (*Result, error) {
	var results []map[string]interface{}
	tx := s.db.WithContext(ctx).Raw(query).Scan(&results)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &Result{Data: results}, nil
}

func (s *SqlServerDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for SqlServer")
}

func (s *SqlServerDataSource) Type() string {
	return "sqlserver"
}
