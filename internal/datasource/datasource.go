package datasource

import (
	"context"
)

type Result struct {
	Data  interface{}
	Error error
}

type DataSource interface {
	Query(ctx context.Context, query string) (*Result, error)
	QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error)
	Type() string
}
