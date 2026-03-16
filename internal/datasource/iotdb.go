package datasource

import (
	"context"
	"fmt"
	
	"github.com/apache/iotdb-client-go/client"
	"github.com/argus-monitoring/argus/internal/model"
)

type IoTDBDataSource struct {
	session client.Session
}

func NewIoTDBDataSource(ds *model.DataSource) (*IoTDBDataSource, error) {
	// host := ds.URL
	// port := "6667"

	// if strings.Contains(ds.URL, ":") {
	// 	parts := strings.Split(ds.URL, ":")
	// 	if len(parts) == 2 {
	// 		host = parts[0]
	// 		port = parts[1]
	// 	}
	// }

	// cfg := client.Config{
	// 	Host:     host,
	// 	Port:     port,
	// 	UserName: ds.Username,
	// 	Password: ds.Password,
	// }
	// session := client.NewSession(&cfg)
	// if err := session.Open(false, 0); err != nil {
	// 	return nil, err
	// }
	// return &IoTDBDataSource{session: session}, nil
	return nil, fmt.Errorf("iotdb not implemented")
}

func (i *IoTDBDataSource) Query(ctx context.Context, query string) (*Result, error) {
	// sessionDataSet, err := i.session.ExecuteStatement(query)
	// if err != nil {
	// 	return nil, err
	// }
	
	// var results []map[string]interface{}
	
	// for {
	// 	next, err := sessionDataSet.Next()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	if !next {
	// 		break
	// 	}

	// 	row := make(map[string]interface{})
	// 	for j := 0; j < sessionDataSet.GetColumnCount(); j++ {
	// 		colName, err := sessionDataSet.GetColumnName(j)
	// 		if err != nil {
	// 			continue
	// 		}
	// 		val := sessionDataSet.GetValue(colName)
	// 		row[colName] = val
	// 	}
	// 	results = append(results, row)
	// }

	// return &Result{Data: results}, nil
	return nil, fmt.Errorf("iotdb query not implemented")
}

func (i *IoTDBDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for IoTDB")
}

func (i *IoTDBDataSource) Type() string {
	return "iotdb"
}
