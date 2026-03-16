package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/config"
	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	LLMClient ai.LLMClient
}

func NewAIHandler(client ai.LLMClient) *AIHandler {
	return &AIHandler{LLMClient: client}
}

type TextToQueryRequest struct {
	DataSourceType string `json:"datasource_type" binding:"required"`
	QueryText      string `json:"query_text" binding:"required"`
}

type TextToQueryResponse struct {
	TraceID       string    `json:"trace_id"`
	PromptVersion string    `json:"prompt_version"`
	Model         string    `json:"model,omitempty"`
	Usage         *ai.Usage `json:"usage,omitempty"`
	Query         string    `json:"query"`
}

type InsightRequest struct {
	DataSourceID uint   `json:"datasource_id" binding:"required"`
	Message      string `json:"message" binding:"required"`
}

type InsightResponse struct {
	TraceID       string                 `json:"trace_id"`
	PromptVersion string                 `json:"prompt_version"`
	Model         string                 `json:"model,omitempty"`
	Usage         *ai.Usage              `json:"usage,omitempty"`
	Summary       string                 `json:"summary"`
	ChartConfig   map[string]interface{} `json:"chart_config"`
	Query         string                 `json:"query"`
}

func (h *AIHandler) Insight(c *gin.Context) {
	var req InsightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	traceID := newTraceID()
	c.Header("X-Trace-Id", traceID)

	var ds model.DataSource
	if err := model.DB.First(&ds, req.DataSourceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DataSource not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout(60*time.Second))
	defer cancel()
	createdBy := getUserID(c)
	teamID := resolveTeamID(createdBy)
	if err := enforceTeamQuota(teamID); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	query, summary, chartConfig, usage, modelName, _, promptVersion, promptHash, err := generateInsight(ctx, h.LLMClient, ds, req.Message)
	if err != nil {
		if isTimeoutErr(err) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "AI request timeout"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	writeAIAudit(traceID, createdBy, teamID, "/api/ai/insight", providerFromClient(h.LLMClient), modelName, promptVersion, promptHash, req.Message, summary, usage)
	c.JSON(http.StatusOK, InsightResponse{
		TraceID:       traceID,
		PromptVersion: promptVersion,
		Model:         modelName,
		Usage:         usage,
		Summary:       summary,
		ChartConfig:   chartConfig,
		Query:         query,
	})
}

func extractJSON(response string) string {
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func sqlLooksLikeDataQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	if !strings.HasPrefix(q, "select") && !strings.HasPrefix(q, "with") {
		return false
	}
	if strings.Contains(q, " from ") {
		return true
	}
	if strings.Contains(q, "\nfrom ") {
		return true
	}
	return false
}

func generateInsight(ctx context.Context, client ai.LLMClient, ds model.DataSource, message string) (string, string, map[string]interface{}, *ai.Usage, string, string, string, string, error) {
	queryScene := "insight_query/" + string(ds.Type)
	queryPrompt, queryPromptVersion, queryPromptHash := getPromptOrDefault(queryScene, defaultQueryTemplate(ds.Type), map[string]string{
		"query":            message,
		"datasource_type":  string(ds.Type),
		"data_source_type": string(ds.Type),
	})
	queryMessages := []ai.Message{
		{Role: "system", Content: "You are an expert in monitoring and observability query languages."},
		{Role: "user", Content: queryPrompt},
	}
	queryResp, err := client.ChatCompletion(ctx, queryMessages)
	if err != nil {
		return "", "", nil, nil, "", "", queryPromptVersion, queryPromptHash, fmt.Errorf("Failed to generate query: %w", err)
	}
	usage := sumUsage(nil, queryResp.Usage)
	modelName := strings.TrimSpace(queryResp.Model)
	query := extractQuery(queryResp.Content)

	promptVersion := queryPromptVersion
	promptHash := queryPromptHash

	if ds.Type == model.DataSourceTypeMySQL || ds.Type == model.DataSourceTypeClickHouse || ds.Type == model.DataSourceTypeSqlServer {
		if !sqlLooksLikeDataQuery(query) {
			analysisScene := "insight_sql_fallback"
			analysisPrompt, pv, ph := getPromptOrDefault(analysisScene, defaultSQLFallbackTemplate(), map[string]string{
				"question":         message,
				"query_text":       message,
				"datasource_type":  string(ds.Type),
				"data_source_type": string(ds.Type),
			})
			analysisMessages := []ai.Message{
				{Role: "system", Content: "You are a data analyst helper. Return only JSON."},
				{Role: "user", Content: analysisPrompt},
			}
			analysisResp, err := client.ChatCompletion(ctx, analysisMessages)
			if err != nil {
				return "", "", nil, usage, modelName, "", joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), fmt.Errorf("Failed to analyze data: %w", err)
			}
			usage = sumUsage(usage, analysisResp.Usage)
			if modelName == "" {
				modelName = strings.TrimSpace(analysisResp.Model)
			}
			jsonStr := extractJSON(analysisResp.Content)
			var llmResult struct {
				Summary     string                 `json:"summary"`
				ChartConfig map[string]interface{} `json:"chart_config"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
				return "", "", nil, usage, modelName, "", joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), fmt.Errorf("Failed to parse AI analysis")
			}
			return "", llmResult.Summary, llmResult.ChartConfig, usage, modelName, "", joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), nil
		}
	}

	dsInstance, err := datasource.NewDataSource(ds)
	if err != nil {
		return "", "", nil, usage, modelName, "", promptVersion, promptHash, fmt.Errorf("Failed to create datasource instance: %w", err)
	}
	result, err := dsInstance.Query(ctx, query)
	if err != nil {
		return "", "", nil, usage, modelName, "", promptVersion, promptHash, fmt.Errorf("Failed to execute query: %w", err)
	}

	snippet := truncateRunes(fmt.Sprintf("%v", result.Data), 20000)
	analysisScene := "insight_analyze"
	analysisPrompt, pv, ph := getPromptOrDefault(analysisScene, defaultAnalyzeTemplate(), map[string]string{
		"question":   message,
		"query_text": message,
		"query":      query,
		"data":       snippet,
		"result":     snippet,
	})
	analysisMessages := []ai.Message{
		{Role: "system", Content: "You are a data analyst helper. Return only JSON."},
		{Role: "user", Content: analysisPrompt},
	}
	analysisResp, err := client.ChatCompletion(ctx, analysisMessages)
	if err != nil {
		return "", "", nil, usage, modelName, snippet, joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), fmt.Errorf("Failed to analyze data: %w", err)
	}
	usage = sumUsage(usage, analysisResp.Usage)
	if modelName == "" {
		modelName = strings.TrimSpace(analysisResp.Model)
	}
	jsonStr := extractJSON(analysisResp.Content)
	var llmResult struct {
		Summary     string                 `json:"summary"`
		ChartConfig map[string]interface{} `json:"chart_config"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
		return "", "", nil, usage, modelName, snippet, joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), fmt.Errorf("Failed to parse AI analysis")
	}
	return query, llmResult.Summary, llmResult.ChartConfig, usage, modelName, snippet, joinPromptVersions(promptVersion, pv), joinPromptHashes(promptHash, ph), nil
}

func (h *AIHandler) TextToQuery(c *gin.Context) {
	var req TextToQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	traceID := newTraceID()
	c.Header("X-Trace-Id", traceID)

	createdBy := getUserID(c)
	teamID := resolveTeamID(createdBy)
	if err := enforceTeamQuota(teamID); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	// Construct the prompt based on datasource type
	scene := "text_to_query/" + strings.TrimSpace(req.DataSourceType)
	prompt, promptVersion, promptHash := getPromptOrDefault(scene, defaultQueryTemplate(model.DataSourceType(req.DataSourceType)), map[string]string{
		"query":            req.QueryText,
		"datasource_type":  req.DataSourceType,
		"data_source_type": req.DataSourceType,
	})

	// Call LLM
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout(30*time.Second))
	defer cancel()

	messages := []ai.Message{
		{Role: "system", Content: "You are an expert in monitoring and observability query languages."},
		{Role: "user", Content: prompt},
	}

	response, err := h.LLMClient.ChatCompletion(ctx, messages)
	if err != nil {
		if isTimeoutErr(err) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "AI request timeout"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate query: " + err.Error()})
		return
	}

	// Extract the query from the response (simple extraction for now)
	modelName := strings.TrimSpace(response.Model)
	query := extractQuery(response.Content)

	// Validate the query
	if err := validateQuery(req.DataSourceType, query); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":           "Generated query is invalid: " + err.Error(),
			"generated_query": query,
		})
		return
	}

	writeAIAudit(traceID, createdBy, teamID, "/api/ai/text-to-query", providerFromClient(h.LLMClient), modelName, promptVersion, promptHash, req.QueryText, query, response.Usage)
	c.JSON(http.StatusOK, TextToQueryResponse{TraceID: traceID, PromptVersion: promptVersion, Model: modelName, Usage: response.Usage, Query: query})
}

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func sumUsage(a, b *ai.Usage) *ai.Usage {
	if a == nil && b == nil {
		return nil
	}
	out := &ai.Usage{}
	if a != nil {
		out.PromptTokens += a.PromptTokens
		out.CompletionTokens += a.CompletionTokens
		out.TotalTokens += a.TotalTokens
	}
	if b != nil {
		out.PromptTokens += b.PromptTokens
		out.CompletionTokens += b.CompletionTokens
		out.TotalTokens += b.TotalTokens
	}
	return out
}

func getUserID(c *gin.Context) *uint {
	if c == nil {
		return nil
	}
	raw, ok := c.Get("user_id")
	if !ok {
		return nil
	}
	id, err := parseUint(raw)
	if err != nil || id == 0 {
		return nil
	}
	return &id
}

func providerFromClient(c ai.LLMClient) string {
	switch c.(type) {
	case *ai.MockClient:
		return "mock"
	case *ai.OpenAIClient:
		return "openai"
	case *ai.DeepSeekClient:
		return "deepseek"
	default:
		return ""
	}
}

func writeAIAudit(traceID string, userID, teamID *uint, path, provider, modelName, promptVersion, promptHash, input, output string, usage *ai.Usage) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return
	}
	inEx := truncateRunes(strings.TrimSpace(input), 800)
	outEx := truncateRunes(strings.TrimSpace(output), 800)
	rec := model.AIAudit{
		TraceID:       traceID,
		UserID:        userID,
		TeamID:        teamID,
		Path:          strings.TrimSpace(path),
		Provider:      strings.TrimSpace(provider),
		Model:         strings.TrimSpace(modelName),
		PromptVersion: strings.TrimSpace(promptVersion),
		PromptHash:    strings.TrimSpace(promptHash),
		InputExcerpt:  inEx,
		OutputExcerpt: outEx,
	}
	if usage != nil {
		rec.PromptTokens = usage.PromptTokens
		rec.CompletionTokens = usage.CompletionTokens
		rec.TotalTokens = usage.TotalTokens
	}
	_ = model.DB.Create(&rec).Error
}

func resolveTeamID(userID *uint) *uint {
	if userID == nil || *userID == 0 {
		return nil
	}
	var tid uint
	if err := model.DB.Table("team_users").Select("team_id").Where("user_id = ?", *userID).Order("team_id asc").Limit(1).Scan(&tid).Error; err != nil {
		return nil
	}
	if tid == 0 {
		return nil
	}
	return &tid
}

func enforceTeamQuota(teamID *uint) error {
	if teamID == nil || *teamID == 0 {
		return nil
	}
	var q model.AIQuota
	if err := model.DB.Where("team_id = ? AND enabled = ?", *teamID, true).First(&q).Error; err != nil {
		return nil
	}
	if q.MonthlyTokenLimit <= 0 {
		return fmt.Errorf("AI quota exceeded")
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var used int64
	_ = model.DB.Model(&model.AIAudit{}).
		Select("COALESCE(SUM(total_tokens),0)").
		Where("team_id = ? AND created_at >= ?", *teamID, start).
		Scan(&used).Error
	if used >= int64(q.MonthlyTokenLimit) {
		return fmt.Errorf("AI quota exceeded")
	}
	return nil
}

func getPromptOrDefault(scene, fallback string, vars map[string]string) (string, string, string) {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		p := renderTemplate(fallback, vars)
		return p, "builtin", hashStrings(fallback)
	}
	var pv model.AIPromptVersion
	if err := model.DB.Where("scene = ? AND is_active = ?", scene, true).Order("created_at desc").First(&pv).Error; err == nil && strings.TrimSpace(pv.Content) != "" {
		content := strings.TrimSpace(pv.Content)
		p := renderTemplate(content, vars)
		v := strings.TrimSpace(pv.Version)
		if v == "" {
			v = "custom"
		}
		return p, scene + "@" + v, hashStrings(scene, v, content)
	}
	p := renderTemplate(fallback, vars)
	return p, scene + "@builtin", hashStrings(scene, fallback)
}

func renderTemplate(tpl string, vars map[string]string) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func defaultQueryTemplate(dsType model.DataSourceType) string {
	switch dsType {
	case model.DataSourceTypePrometheus:
		return "Convert the following natural language query to PromQL. Only return the PromQL query, no markdown, no explanation.\n\nQuery: {{query}}"
	case model.DataSourceTypeElasticsearch:
		return "Convert the following natural language query to Elasticsearch Query DSL (JSON). Only return the JSON, no markdown, no explanation.\n\nQuery: {{query}}"
	case model.DataSourceTypeMySQL:
		return "Convert the following natural language query to a MySQL SQL query.\nConstraints:\n- MUST query real tables and MUST include a FROM clause\n- MUST NOT return hardcoded diagnosis text (e.g. SELECT '...')\n- Prefer returning aggregated counts/time series suitable for charting\nOnly return SQL, no markdown, no explanation.\n\nQuery: {{query}}"
	case model.DataSourceTypeClickHouse:
		return "Convert the following natural language query to a ClickHouse SQL query.\nConstraints:\n- MUST query real tables and MUST include a FROM clause\n- MUST NOT return hardcoded diagnosis text (e.g. SELECT '...')\n- Prefer returning aggregated counts/time series suitable for charting\nOnly return SQL, no markdown, no explanation.\n\nQuery: {{query}}"
	case model.DataSourceTypeSqlServer:
		return "Convert the following natural language query to a SQL Server T-SQL query.\nConstraints:\n- MUST query real tables and MUST include a FROM clause\n- MUST NOT return hardcoded diagnosis text (e.g. SELECT '...')\n- Prefer returning aggregated counts/time series suitable for charting\nOnly return SQL, no markdown, no explanation.\n\nQuery: {{query}}"
	default:
		return "Convert the following natural language query to a query suitable for {{datasource_type}}. Only return the query code.\n\nQuery: {{query}}"
	}
}

func defaultSQLFallbackTemplate() string {
	return `You are helping troubleshoot an incident.
The system could not generate a safe executable SQL query (missing FROM or looks like a constant-result query).

User Question: "{{question}}"
DataSource Type: "{{datasource_type}}"

Please provide:
1. A brief summary of the likely issue and next steps (max 3 sentences).
2. A ECharts JSON configuration. If no data is available, return an empty object for chart_config.

Return ONLY a valid JSON object with the following structure:
{
  "summary": "...",
  "chart_config": { }
}`
}

func defaultAnalyzeTemplate() string {
	return `Analyze the following data result from a monitoring query.
User Question: "{{question}}"
Query Executed: "{{query}}"
Data Result: {{data}}

Please provide:
1. A brief summary of the findings (max 2 sentences).
2. A ECharts JSON configuration to visualize this data.

Return ONLY a valid JSON object with the following structure:
{
  "summary": "...",
  "chart_config": { ... }
}`
}

func joinPromptVersions(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if strings.Contains(a, b) {
		return a
	}
	return a + ";" + b
}

func joinPromptHashes(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return hashStrings(a, b)
}

func hashStrings(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func aiTimeout(min time.Duration) time.Duration {
	cfg := 0
	if config.AppConfig != nil {
		cfg = config.AppConfig.AI.TimeoutSeconds
	}
	if cfg <= 0 {
		return min
	}
	d := time.Duration(cfg)*time.Second + 10*time.Second
	if d < min {
		return min
	}
	return d
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded") || strings.Contains(s, "client.timeout exceeded") {
		return true
	}
	return false
}

func extractQuery(response string) string {
	// Remove markdown code blocks if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimPrefix(response, "promql")
	response = strings.TrimPrefix(response, "json")
	response = strings.TrimSuffix(response, "```")
	return strings.TrimSpace(response)
}

func validateQuery(dsType, query string) error {
	if query == "" {
		return fmt.Errorf("query is empty")
	}
	// TODO: Implement more specific validation logic
	return nil
}
