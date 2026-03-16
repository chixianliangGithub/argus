package datasource

import (
	"fmt"

	"github.com/argus-monitoring/argus/internal/model"
)

func NewDataSource(ds model.DataSource) (DataSource, error) {
	switch ds.Type {
	case model.DataSourceTypePrometheus:
		return NewPrometheusDataSource(ds.URL)
	case model.DataSourceTypeElasticsearch:
		return NewElasticsearchDataSource(&ds)
	case model.DataSourceTypeInfluxDB:
		return NewInfluxDBDataSource(&ds)
	case model.DataSourceTypeClickHouse:
		return NewClickHouseDataSource(&ds)
	case model.DataSourceTypeMySQL:
		return NewMySQLDataSource(&ds)
	case model.DataSourceTypeSkyWalking:
		return NewSkyWalkingDataSource(&ds)
	case model.DataSourceTypeSqlServer:
		return NewSqlServerDataSource(&ds)
	case model.DataSourceTypeIoTDB:
		return NewIoTDBDataSource(&ds)
	case model.DataSourceTypeMock:
		return NewMockDataSource(&ds)
	default:
		return nil, fmt.Errorf("unsupported datasource type: %s", ds.Type)
	}
}
