package datasource

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/prometheus/common/model"
)

// GetVectorValues extracts vector values from result data.
// Returns a map of metric name/labels to value.
func GetVectorValues(res *Result) (map[string]float64, error) {
	if res == nil || res.Data == nil {
		return nil, fmt.Errorf("result is nil")
	}

	// Handle Prometheus types
	switch v := res.Data.(type) {
	case model.Vector:
		result := make(map[string]float64)
		for _, sample := range v {
			result[sample.Metric.String()] = float64(sample.Value)
		}
		return result, nil
	case model.Scalar:
		return map[string]float64{"scalar": float64(v.Value)}, nil
	case model.Matrix:
		result := make(map[string]float64)
		for _, stream := range v {
			if len(stream.Values) == 0 {
				continue
			}
			last := stream.Values[len(stream.Values)-1]
			result[stream.Metric.String()] = float64(last.Value)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported result data type: %T", res.Data)
	}
}

// GetMatrixValues extracts matrix values from result data.
// Returns a map of metric name/labels to slice of values.
func GetMatrixValues(res *Result) (map[string][]float64, error) {
	if res == nil || res.Data == nil {
		return nil, fmt.Errorf("result is nil")
	}

	switch v := res.Data.(type) {
	case model.Matrix:
		result := make(map[string][]float64)
		for _, stream := range v {
			var values []float64
			for _, pair := range stream.Values {
				values = append(values, float64(pair.Value))
			}
			result[stream.Metric.String()] = values
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported result data type for matrix: %T", res.Data)
	}
}

// GetSQLFirstNumeric extracts a numeric value from SQL-like row results:
// []map[string]interface{} (e.g. MySQL/ClickHouse).
// It returns (value, fieldName, ok, error).
func GetSQLFirstNumeric(res *Result, preferredField string) (float64, string, bool, error) {
	if res == nil || res.Data == nil {
		return 0, "", false, fmt.Errorf("result is nil")
	}

	rows, ok := res.Data.([]map[string]interface{})
	if !ok {
		return 0, "", false, fmt.Errorf("unsupported result data type: %T", res.Data)
	}
	if len(rows) == 0 {
		return 0, "", false, nil
	}

	row := rows[0]
	if preferredField != "" {
		if v, exists := row[preferredField]; exists {
			if fv, ok := toFloat64(v); ok {
				return fv, preferredField, true, nil
			}
			return 0, preferredField, false, fmt.Errorf("preferred field not numeric: %s", preferredField)
		}
		return 0, preferredField, false, fmt.Errorf("preferred field not found: %s", preferredField)
	}

	if v, exists := row["value"]; exists {
		if fv, ok := toFloat64(v); ok {
			return fv, "value", true, nil
		}
	}

	for k, v := range row {
		if fv, ok := toFloat64(v); ok {
			return fv, k, true, nil
		}
	}

	return 0, "", false, nil
}

func GetSQLNumericFromRow(row map[string]interface{}, preferredField string) (float64, string, bool) {
	if row == nil {
		return 0, "", false
	}

	if preferredField != "" {
		if v, exists := row[preferredField]; exists {
			if fv, ok := toFloat64(v); ok {
				return fv, preferredField, true
			}
			return 0, preferredField, false
		}
		return 0, preferredField, false
	}

	if v, exists := row["value"]; exists {
		if fv, ok := toFloat64(v); ok {
			return fv, "value", true
		}
	}

	for k, v := range row {
		if fv, ok := toFloat64(v); ok {
			return fv, k, true
		}
	}

	return 0, "", false
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case json.Number:
		fv, err := t.Float64()
		if err != nil {
			return 0, false
		}
		return fv, true
	case string:
		fv, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return fv, true
	default:
		return 0, false
	}
}
