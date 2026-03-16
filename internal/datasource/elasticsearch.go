package datasource

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	prommodel "github.com/prometheus/common/model"
)

type ElasticsearchDataSource struct {
	client *elasticsearch.Client
	index  string
}

func NewElasticsearchDataSource(ds *model.DataSource) (*ElasticsearchDataSource, error) {
	type tlsConfig struct {
		Enabled            bool   `json:"enabled"`
		InsecureSkipVerify bool   `json:"insecure_skip_verify"`
		CAPem              string `json:"ca_pem"`
	}
	type esConfig struct {
		Version     string            `json:"version"`
		Index       string            `json:"index"`
		TLS         tlsConfig         `json:"tls"`
		APIKey      string            `json:"api_key"`
		BearerToken string            `json:"bearer_token"`
		Headers     map[string]string `json:"headers"`
		TimeoutSec  int               `json:"timeout_sec"`
	}

	var extra esConfig
	if strings.TrimSpace(ds.Config) != "" {
		_ = json.Unmarshal([]byte(ds.Config), &extra)
	}

	addr := strings.TrimSpace(ds.URL)
	if addr != "" && !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		if extra.TLS.Enabled {
			addr = "https://" + addr
		} else {
			addr = "http://" + addr
		}
	}

	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	if extra.TimeoutSec > 0 {
		baseTransport.ResponseHeaderTimeout = time.Duration(extra.TimeoutSec) * time.Second
	}
	if extra.TLS.Enabled || strings.HasPrefix(addr, "https://") {
		tlsCfg := &tls.Config{InsecureSkipVerify: extra.TLS.InsecureSkipVerify}
		if strings.TrimSpace(extra.TLS.CAPem) != "" {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM([]byte(extra.TLS.CAPem)) {
				tlsCfg.RootCAs = pool
			}
		}
		baseTransport.TLSClientConfig = tlsCfg
	}

	compatHeaders := map[string]string{}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(extra.Version)), "7") {
		compatHeaders["Accept"] = "application/vnd.elasticsearch+json;compatible-with=7"
		compatHeaders["Content-Type"] = "application/vnd.elasticsearch+json;compatible-with=7"
	}
	for k, v := range extra.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		compatHeaders[k] = v
	}

	authHeader := ""
	if strings.TrimSpace(extra.APIKey) != "" {
		authHeader = "ApiKey " + strings.TrimSpace(extra.APIKey)
	} else if strings.TrimSpace(extra.BearerToken) != "" {
		authHeader = "Bearer " + strings.TrimSpace(extra.BearerToken)
	}

	transport := roundTripperWithHeaders{base: baseTransport, headers: compatHeaders, authHeader: authHeader}
	cfg := elasticsearch.Config{
		Addresses: []string{addr},
		Username:  ds.Username,
		Password:  ds.Password,
		Transport: transport,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	idx := strings.TrimSpace(extra.Index)
	if idx == "" {
		idx = strings.TrimSpace(ds.Database)
	}
	return &ElasticsearchDataSource{client: client, index: idx}, nil
}

func (e *ElasticsearchDataSource) Query(ctx context.Context, query string) (*Result, error) {
	var buf bytes.Buffer
	buf.WriteString(query)
	searchOpts := []func(*esapi.SearchRequest){
		e.client.Search.WithContext(ctx),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
	}
	if strings.TrimSpace(e.index) != "" {
		searchOpts = append(searchOpts, e.client.Search.WithIndex(e.index))
	}
	res, err := e.client.Search(searchOpts...)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("error parsing the response body: %s", err)
		}
		// Print the response status and error information.
		return nil, fmt.Errorf("[%s] %s: %s",
			res.Status(),
			e["error"].(map[string]interface{})["type"],
			e["error"].(map[string]interface{})["reason"],
		)
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	metricMode := false
	if m, ok := parseJSONObject(query); ok {
		if _, ok := m["aggs"]; ok {
			metricMode = true
		}
		if _, ok := m["aggregations"]; ok {
			metricMode = true
		}
		if v, ok := m["size"].(float64); ok && v == 0 {
			metricMode = true
		}
	}

	if metricMode {
		row := map[string]interface{}{}
		if total, ok := extractTotalHits(r); ok {
			row["__count__"] = total
		}
		if aggs, ok := r["aggregations"].(map[string]interface{}); ok {
			flattenAggValues("", aggs, row)
		}
		return &Result{Data: []map[string]interface{}{row}}, nil
	}

	hitsObj, ok := r["hits"].(map[string]interface{})
	if ok {
		hitsArr, ok := hitsObj["hits"].([]interface{})
		if ok {
			var rows []map[string]interface{}
			for _, item := range hitsArr {
				hit, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				row := map[string]interface{}{}
				if src, ok := hit["_source"].(map[string]interface{}); ok {
					for k, v := range src {
						row[k] = v
					}
				}
				if id, ok := hit["_id"]; ok {
					row["_id"] = id
				}
				if idx, ok := hit["_index"]; ok {
					row["_index"] = idx
				}
				rows = append(rows, row)
			}
			return &Result{Data: rows}, nil
		}
	}

	return &Result{Data: r}, nil
}

func (e *ElasticsearchDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	var buf bytes.Buffer
	buf.WriteString(query)
	searchOpts := []func(*esapi.SearchRequest){
		e.client.Search.WithContext(ctx),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(false),
	}
	if strings.TrimSpace(e.index) != "" {
		searchOpts = append(searchOpts, e.client.Search.WithIndex(e.index))
	}
	res, err := e.client.Search(searchOpts...)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var ee map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&ee); err != nil {
			return nil, fmt.Errorf("error parsing the response body: %s", err)
		}
		return nil, fmt.Errorf("[%s] %s: %s",
			res.Status(),
			ee["error"].(map[string]interface{})["type"],
			ee["error"].(map[string]interface{})["reason"],
		)
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	aggs, ok := r["aggregations"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing aggregations in response")
	}
	buckets, metricLabel, err := extractFirstDateHistogram(aggs)
	if err != nil {
		return nil, err
	}

	stream := &prommodel.SampleStream{
		Metric: prommodel.Metric{
			"__name__": prommodel.LabelValue("elasticsearch"),
			"series":   prommodel.LabelValue(metricLabel),
		},
	}
	for _, b := range buckets {
		ts, ok := extractBucketTime(b)
		if !ok {
			continue
		}
		val, ok := extractBucketValue(b)
		if !ok {
			continue
		}
		stream.Values = append(stream.Values, prommodel.SamplePair{
			Timestamp: prommodel.Time(ts),
			Value:     prommodel.SampleValue(val),
		})
	}
	matrix := prommodel.Matrix{stream}
	_ = start
	_ = end
	_ = step
	return &Result{Data: matrix}, nil
}

func (e *ElasticsearchDataSource) Type() string {
	return "elasticsearch"
}

type roundTripperWithHeaders struct {
	base       http.RoundTripper
	headers    map[string]string
	authHeader string
}

func (r roundTripperWithHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	rr := req.Clone(req.Context())
	if rr.Header == nil {
		rr.Header = make(http.Header)
	}
	for k, v := range r.headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		if rr.Header.Get(k) == "" {
			rr.Header.Set(k, v)
		}
	}
	if strings.TrimSpace(r.authHeader) != "" && rr.Header.Get("Authorization") == "" {
		rr.Header.Set("Authorization", r.authHeader)
	}
	return r.base.RoundTrip(rr)
}

func parseJSONObject(s string) (map[string]interface{}, bool) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return nil, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ss), &m); err != nil {
		return nil, false
	}
	return m, true
}

func extractTotalHits(resp map[string]interface{}) (float64, bool) {
	hitsObj, ok := resp["hits"].(map[string]interface{})
	if !ok {
		return 0, false
	}
	totalObj, ok := hitsObj["total"].(map[string]interface{})
	if ok {
		if v, ok := totalObj["value"].(float64); ok {
			return v, true
		}
	}
	if v, ok := hitsObj["total"].(float64); ok {
		return v, true
	}
	return 0, false
}

func flattenAggValues(prefix string, v interface{}, out map[string]interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		if vv, ok := t["value"].(float64); ok {
			key := strings.Trim(prefix, ".")
			if key != "" {
				out[key] = vv
			}
			return
		}
		if vv, ok := t["doc_count"].(float64); ok && t["buckets"] == nil {
			key := strings.Trim(prefix, ".")
			if key != "" {
				out[key] = vv
			}
			return
		}
		for k, vv := range t {
			if k == "buckets" {
				continue
			}
			pp := k
			if prefix != "" {
				pp = prefix + "." + k
			}
			flattenAggValues(pp, vv, out)
		}
	}
}

func extractFirstDateHistogram(aggs map[string]interface{}) ([]map[string]interface{}, string, error) {
	for name, raw := range aggs {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if b, ok := obj["buckets"].([]interface{}); ok {
			var buckets []map[string]interface{}
			for _, it := range b {
				if bb, ok := it.(map[string]interface{}); ok {
					buckets = append(buckets, bb)
				}
			}
			if len(buckets) > 0 {
				return buckets, name, nil
			}
		}
		for _, k := range []string{"aggs", "aggregations"} {
			if sub, ok := obj[k].(map[string]interface{}); ok {
				if buckets, label, err := extractFirstDateHistogram(sub); err == nil {
					return buckets, label, nil
				}
			}
		}
	}
	return nil, "", fmt.Errorf("no date_histogram buckets found")
}

func extractBucketTime(bucket map[string]interface{}) (int64, bool) {
	if v, ok := bucket["key"].(float64); ok {
		return int64(v), true
	}
	if v, ok := bucket["key"].(int64); ok {
		return v, true
	}
	if v, ok := bucket["key"].(json.Number); ok {
		if n, err := v.Int64(); err == nil {
			return n, true
		}
	}
	return 0, false
}

func extractBucketValue(bucket map[string]interface{}) (float64, bool) {
	for k, v := range bucket {
		if k == "key" || k == "key_as_string" || k == "doc_count" {
			continue
		}
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if vv, ok := obj["value"].(float64); ok {
			return vv, true
		}
	}
	if dc, ok := bucket["doc_count"].(float64); ok {
		return dc, true
	}
	if dc, ok := bucket["doc_count"].(int64); ok {
		return float64(dc), true
	}
	return 0, false
}
