package datasource

import (
	"context"
	"fmt"

	"github.com/argus-monitoring/argus/internal/model"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type InfluxDBDataSource struct {
	client influxdb2.Client
	org    string
	bucket string
}

func NewInfluxDBDataSource(ds *model.DataSource) (*InfluxDBDataSource, error) {
	// For InfluxDB v2, Password is used as Token
	client := influxdb2.NewClient(ds.URL, ds.Password)
	return &InfluxDBDataSource{
		client: client,
		org:    ds.Username, // Assuming Username is used as Org
		bucket: ds.Database,
	}, nil
}

func (i *InfluxDBDataSource) Query(ctx context.Context, query string) (*Result, error) {
	queryAPI := i.client.QueryAPI(i.org)
	result, err := queryAPI.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	for result.Next() {
		records = append(records, result.Record().Values())
	}
	if result.Err() != nil {
		return nil, result.Err()
	}

	return &Result{Data: records}, nil
}

func (i *InfluxDBDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for InfluxDB")
}

func (i *InfluxDBDataSource) Type() string {
	return "influxdb"
}
