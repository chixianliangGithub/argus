package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient defines the interface for interacting with LLM providers.
type LLMClient interface {
	ChatCompletion(ctx context.Context, messages []Message) (Result, error)
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Result struct {
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
}

// MockClient is a mock implementation of LLMClient.
type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) ChatCompletion(ctx context.Context, messages []Message) (Result, error) {
	// Simple logic to return mock queries based on input prompt
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1].Content
		if strings.Contains(lastMsg, "PromQL") {
			return Result{Content: "rate(http_requests_total[5m])", Model: "mock"}, nil
		}
		if strings.Contains(lastMsg, "Elasticsearch") {
			return Result{Content: `{"query": {"match_all": {}}}`, Model: "mock"}, nil
		}
		if strings.Contains(lastMsg, "MySQL") || strings.Contains(lastMsg, "ClickHouse") || strings.Contains(lastMsg, "SQL Server") || strings.Contains(lastMsg, "SQL query") || strings.Contains(lastMsg, "Only return SQL") {
			return Result{Content: "SELECT 1 AS value", Model: "mock"}, nil
		}
		if strings.Contains(lastMsg, "Analyze the following data result") {
			return Result{Content: `{
  "summary": "The error rate has increased by 15% in the last hour compared to the previous period.",
  "chart_config": {
    "title": { "text": "Error Rate Trend" },
    "xAxis": { "type": "category", "data": ["10:00", "10:05", "10:10", "10:15", "10:20"] },
    "yAxis": { "type": "value" },
    "series": [{ "data": [120, 132, 101, 134, 90], "type": "line" }]
  }
}`, Model: "mock"}, nil
		}
	}
	return Result{Content: "mock_query", Model: "mock"}, nil
}

// OpenAIClient is a structure for OpenAI integration.
type OpenAIClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

func NewOpenAIClient(apiKey string) *OpenAIClient {
	return NewOpenAIClientWithConfig(apiKey, "https://api.openai.com/v1", "gpt-4o-mini", 30*time.Second)
}

func NewOpenAIClientWithConfig(apiKey, baseURL, model string, timeout time.Duration) *OpenAIClient {
	bu := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if bu == "" {
		bu = "https://api.openai.com/v1"
	}
	m := strings.TrimSpace(model)
	if m == "" {
		m = "gpt-4o-mini"
	}
	if timeout < 0 {
		timeout = 30 * time.Second
	}
	return &OpenAIClient{
		APIKey:  apiKey,
		BaseURL: bu,
		Model:   m,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenAIClient) ChatCompletion(ctx context.Context, messages []Message) (Result, error) {
	return chatCompletionOpenAICompat(ctx, c.HTTPClient, c.BaseURL, c.APIKey, c.Model, messages)
}

// DeepSeekClient is a structure for DeepSeek integration.
type DeepSeekClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

func NewDeepSeekClient(apiKey string) *DeepSeekClient {
	return NewDeepSeekClientWithConfig(apiKey, "https://api.deepseek.com/v1", "deepseek-chat", 30*time.Second)
}

func NewDeepSeekClientWithConfig(apiKey, baseURL, model string, timeout time.Duration) *DeepSeekClient {
	bu := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if bu == "" {
		bu = "https://api.deepseek.com/v1"
	}
	m := strings.TrimSpace(model)
	if m == "" {
		m = "deepseek-chat"
	}
	if timeout < 0 {
		timeout = 30 * time.Second
	}
	return &DeepSeekClient{
		APIKey:  apiKey,
		BaseURL: bu,
		Model:   m,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *DeepSeekClient) ChatCompletion(ctx context.Context, messages []Message) (Result, error) {
	return chatCompletionOpenAICompat(ctx, c.HTTPClient, c.BaseURL, c.APIKey, c.Model, messages)
}

type openAICompatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type openAICompatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func chatCompletionOpenAICompat(ctx context.Context, hc *http.Client, baseURL, apiKey, model string, messages []Message) (Result, error) {
	if strings.TrimSpace(apiKey) == "" {
		return Result{}, errors.New("ai api_key is empty")
	}
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	url := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/chat/completions"

	body := openAICompatRequest{
		Model:       strings.TrimSpace(model),
		Messages:    messages,
		Temperature: 0.2,
	}
	if body.Model == "" {
		body.Model = "gpt-4o-mini"
	}
	b, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := hc.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("ai request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out openAICompatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Result{}, err
	}
	if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
		return Result{}, errors.New(strings.TrimSpace(out.Error.Message))
	}
	if len(out.Choices) == 0 {
		return Result{}, errors.New("ai response has no choices")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return Result{}, errors.New("ai response content is empty")
	}
	return Result{Content: content, Model: strings.TrimSpace(out.Model), Usage: out.Usage}, nil
}
