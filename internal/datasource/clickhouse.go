package datasource

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/argus-monitoring/argus/internal/model"
)

type ClickHouseDataSource struct {
	conn driver.Conn
}

func NewClickHouseDataSource(ds *model.DataSource) (*ClickHouseDataSource, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{ds.URL},
		Auth: clickhouse.Auth{
			Database: ds.Database,
			Username: ds.Username,
			Password: ds.Password,
		},
	})
	if err != nil {
		return nil, err
	}
	return &ClickHouseDataSource{conn: conn}, nil
}

func (c *ClickHouseDataSource) Query(ctx context.Context, query string) (*Result, error) {
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	columns := rows.Columns()

	typeNames := make([]string, len(columns))
	rv := reflect.ValueOf(rows)
	m := rv.MethodByName("ColumnTypes")
	if m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
		out := m.Call(nil)[0]
		if out.IsValid() && out.Kind() == reflect.Slice {
			for i := 0; i < out.Len() && i < len(typeNames); i++ {
				ct := out.Index(i)
				if ct.Kind() == reflect.Pointer {
					ct = ct.Elem()
				}
				if !ct.IsValid() {
					continue
				}
				if ct.Kind() == reflect.Struct {
					f := ct.FieldByName("Type")
					if f.IsValid() && f.Kind() == reflect.String {
						typeNames[i] = f.String()
					}
				}
				if typeNames[i] == "" {
					mt := ct.MethodByName("DatabaseTypeName")
					if mt.IsValid() && mt.Type().NumIn() == 0 && mt.Type().NumOut() == 1 {
						v := mt.Call(nil)[0]
						if v.IsValid() && v.Kind() == reflect.String {
							typeNames[i] = v.String()
						}
					}
				}
			}
		}
	}

	unwrapOne := func(s, prefix string) (string, bool) {
		ss := strings.TrimSpace(s)
		p := strings.ToLower(prefix)
		l := strings.ToLower(ss)
		if strings.HasPrefix(l, p+"(") && strings.HasSuffix(l, ")") {
			return ss[len(prefix)+1 : len(ss)-1], true
		}
		return s, false
	}

	normalize := func(s string) string {
		ss := strings.TrimSpace(s)
		for {
			changed := false
			if inner, ok := unwrapOne(ss, "Nullable"); ok {
				ss = inner
				changed = true
			}
			if inner, ok := unwrapOne(ss, "LowCardinality"); ok {
				ss = inner
				changed = true
			}
			if !changed {
				break
			}
		}
		return strings.TrimSpace(ss)
	}

	isNullable := func(s string) bool {
		l := strings.ToLower(strings.TrimSpace(s))
		return strings.HasPrefix(l, "nullable(") && strings.HasSuffix(l, ")")
	}

	isArray := func(s string) (string, bool) {
		ss := normalize(s)
		l := strings.ToLower(ss)
		if strings.HasPrefix(l, "array(") && strings.HasSuffix(l, ")") {
			return ss[len("Array(") : len(ss)-1], true
		}
		return "", false
	}

	baseType := func(s string) string {
		ss := normalize(s)
		l := strings.ToLower(ss)
		return l
	}

	for rows.Next() {
		valuePtrs := make([]interface{}, len(columns))
		getters := make([]func() interface{}, len(columns))

		for i := range columns {
			tn := typeNames[i]
			nullable := isNullable(tn)
			if inner, ok := isArray(tn); ok {
				bt := baseType(inner)
				switch {
				case bt == "string" || strings.HasPrefix(bt, "fixedstring") || strings.HasPrefix(bt, "enum") || bt == "uuid" || bt == "ipv4" || bt == "ipv6":
					if nullable {
						var p *[]string
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v []string
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				case strings.HasPrefix(bt, "int") || bt == "int8" || bt == "int16" || bt == "int32" || bt == "int64":
					if nullable {
						var p *[]int64
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v []int64
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				case strings.HasPrefix(bt, "uint") || bt == "uint8" || bt == "uint16" || bt == "uint32" || bt == "uint64":
					switch bt {
					case "uint8":
						if nullable {
							var p *[]uint8
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []uint8
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					case "uint16":
						if nullable {
							var p *[]uint16
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []uint16
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					case "uint32":
						if nullable {
							var p *[]uint32
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []uint32
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					default:
						if nullable {
							var p *[]uint64
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []uint64
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					}
				case strings.HasPrefix(bt, "float") || bt == "float32" || bt == "float64":
					switch bt {
					case "float32":
						if nullable {
							var p *[]float32
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []float32
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					default:
						if nullable {
							var p *[]float64
							valuePtrs[i] = &p
							getters[i] = func() interface{} { return p }
						} else {
							var v []float64
							valuePtrs[i] = &v
							getters[i] = func() interface{} { return v }
						}
					}
				case bt == "date" || bt == "date32" || strings.HasPrefix(bt, "datetime"):
					if nullable {
						var p *[]time.Time
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v []time.Time
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				default:
					if nullable {
						var p *[]string
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v []string
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				}
				continue
			}

			bt := baseType(tn)
			switch {
			case bt == "string" || strings.HasPrefix(bt, "fixedstring") || strings.HasPrefix(bt, "enum") || bt == "uuid" || bt == "ipv4" || bt == "ipv6" || strings.HasPrefix(bt, "decimal"):
				if nullable {
					var p *string
					valuePtrs[i] = &p
					getters[i] = func() interface{} { return p }
				} else {
					var v string
					valuePtrs[i] = &v
					getters[i] = func() interface{} { return v }
				}
			case strings.HasPrefix(bt, "int") || bt == "int8" || bt == "int16" || bt == "int32" || bt == "int64":
				if nullable {
					var p *int64
					valuePtrs[i] = &p
					getters[i] = func() interface{} { return p }
				} else {
					var v int64
					valuePtrs[i] = &v
					getters[i] = func() interface{} { return v }
				}
			case strings.HasPrefix(bt, "uint") || bt == "uint8" || bt == "uint16" || bt == "uint32" || bt == "uint64":
				switch bt {
				case "uint8":
					if nullable {
						var p *uint8
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v uint8
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				case "uint16":
					if nullable {
						var p *uint16
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v uint16
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				case "uint32":
					if nullable {
						var p *uint32
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v uint32
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				default:
					if nullable {
						var p *uint64
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v uint64
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				}
			case strings.HasPrefix(bt, "float") || bt == "float32" || bt == "float64":
				switch bt {
				case "float32":
					if nullable {
						var p *float32
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v float32
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				default:
					if nullable {
						var p *float64
						valuePtrs[i] = &p
						getters[i] = func() interface{} { return p }
					} else {
						var v float64
						valuePtrs[i] = &v
						getters[i] = func() interface{} { return v }
					}
				}
			case bt == "bool":
				if nullable {
					var p *bool
					valuePtrs[i] = &p
					getters[i] = func() interface{} { return p }
				} else {
					var v bool
					valuePtrs[i] = &v
					getters[i] = func() interface{} { return v }
				}
			case bt == "date" || bt == "date32" || strings.HasPrefix(bt, "datetime"):
				if nullable {
					var p *time.Time
					valuePtrs[i] = &p
					getters[i] = func() interface{} { return p }
				} else {
					var v time.Time
					valuePtrs[i] = &v
					getters[i] = func() interface{} { return v }
				}
			default:
				if nullable {
					var p *string
					valuePtrs[i] = &p
					getters[i] = func() interface{} { return p }
				} else {
					var v string
					valuePtrs[i] = &v
					getters[i] = func() interface{} { return v }
				}
			}
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Create a map for the current row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			if getters[i] != nil {
				rowMap[col] = getters[i]()
			}
		}
		results = append(results, rowMap)
	}

	return &Result{Data: results}, nil
}

func (c *ClickHouseDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for ClickHouse")
}

func (c *ClickHouseDataSource) Type() string {
	return "clickhouse"
}
