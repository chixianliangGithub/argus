package datasource

import (
	"context"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type MockDataSource struct {
	DataSource *model.DataSource
}

func NewMockDataSource(ds *model.DataSource) (DataSource, error) {
	return &MockDataSource{DataSource: ds}, nil
}

func (d *MockDataSource) Query(ctx context.Context, query string) (*Result, error) {
	// Simulate query delay
	time.Sleep(100 * time.Millisecond)

	// Return mock data based on query
	return &Result{
		Data: []map[string]interface{}{
			{"timestamp": "2023-01-01T10:00:00Z", "value": 120},
			{"timestamp": "2023-01-01T10:05:00Z", "value": 132},
			{"timestamp": "2023-01-01T10:10:00Z", "value": 101},
			{"timestamp": "2023-01-01T10:15:00Z", "value": 134},
			{"timestamp": "2023-01-01T10:20:00Z", "value": 90},
		},
		Error: nil,
	}, nil
}

func (d *MockDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return d.Query(ctx, query)
}

func (d *MockDataSource) Type() string {
	return string(model.DataSourceTypeMock)
}
