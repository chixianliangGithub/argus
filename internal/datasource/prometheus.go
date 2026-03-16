package datasource

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type PrometheusDataSource struct {
	client v1.API
}

func NewPrometheusDataSource(address string) (*PrometheusDataSource, error) {
	addr := strings.TrimSpace(address)
	if addr != "" && !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	client, err := api.NewClient(api.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	return &PrometheusDataSource{client: v1api}, nil
}

func (p *PrometheusDataSource) Type() string {
	return "prometheus"
}

func (p *PrometheusDataSource) Query(ctx context.Context, query string) (*Result, error) {
	// Simple query at current time
	val, _, err := p.client.Query(ctx, query, time.Now())
	if err != nil {
		return &Result{Error: err}, err
	}
	
	// Convert model.Value to interface{}
	// Depending on type (Vector, Matrix, Scalar, String)
	// For now just return raw
	return &Result{Data: val}, nil
}

func (p *PrometheusDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	r := v1.Range{
		Start: time.Unix(start, 0),
		End:   time.Unix(end, 0),
		Step:  time.Duration(step) * time.Second,
	}
	val, _, err := p.client.QueryRange(ctx, query, r)
	if err != nil {
		return &Result{Error: err}, err
	}
	
	return &Result{Data: val}, nil
}

// Ensure interface implementation
var _ DataSource = (*PrometheusDataSource)(nil)
