package datasource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type SkyWalkingDataSource struct {
	url    string
	client *http.Client
}

func NewSkyWalkingDataSource(ds *model.DataSource) (*SkyWalkingDataSource, error) {
	return &SkyWalkingDataSource{
		url: ds.URL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (s *SkyWalkingDataSource) Query(ctx context.Context, query string) (*Result, error) {
	payload := map[string]string{
		"query": query,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.url+"/graphql", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skywalking query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if errors, ok := result["errors"]; ok {
		return nil, fmt.Errorf("skywalking graphql errors: %v", errors)
	}

	return &Result{Data: result["data"]}, nil
}

func (s *SkyWalkingDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*Result, error) {
	return nil, fmt.Errorf("QueryRange not supported for SkyWalking")
}

func (s *SkyWalkingDataSource) Type() string {
	return "skywalking"
}
