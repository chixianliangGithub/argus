package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/rag"
	"github.com/gin-gonic/gin"
)

type RunbookHandler struct {
	VectorDB rag.VectorDBClient
	AIClient ai.LLMClient
}

func NewRunbookHandler() *RunbookHandler {
	// Initialize with simple in-memory vector DB
	vdb := rag.NewSimpleVectorDB()

	// Initialize with mock AI client
	aiClient := ai.NewMockClient()

	h := &RunbookHandler{
		VectorDB: vdb,
		AIClient: aiClient,
	}

	// Rebuild index in background
	go h.RebuildIndex()

	return h
}

func (h *RunbookHandler) RebuildIndex() {
	var runbooks []model.Runbook
	if err := model.DB.Find(&runbooks).Error; err != nil {
		fmt.Printf("Failed to load runbooks for indexing: %v\n", err)
		return
	}

	var vectors [][]float32
	var metadata []map[string]interface{}

	for _, rb := range runbooks {
		// Create a text representation for embedding
		text := fmt.Sprintf("%s %s %s", rb.Name, rb.Description, rb.TriggerVal)
		vec := rag.GenerateMockEmbedding(text, 128) // 128 dim

		vectors = append(vectors, vec)
		metadata = append(metadata, map[string]interface{}{
			"id":   fmt.Sprintf("%d", rb.ID),
			"name": rb.Name,
		})
	}

	if len(vectors) > 0 {
		h.VectorDB.Insert(context.Background(), "runbooks", vectors, metadata)
	}
}

// CreateRunbook creates a new runbook and indexes it
func (h *RunbookHandler) CreateRunbook(c *gin.Context) {
	var runbook model.Runbook
	if err := c.ShouldBindJSON(&runbook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Create(&runbook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update index
	text := fmt.Sprintf("%s %s %s", runbook.Name, runbook.Description, runbook.TriggerVal)
	vec := rag.GenerateMockEmbedding(text, 128)
	h.VectorDB.Insert(context.Background(), "runbooks", [][]float32{vec}, []map[string]interface{}{
		{
			"id":   fmt.Sprintf("%d", runbook.ID),
			"name": runbook.Name,
		},
	})

	c.JSON(http.StatusCreated, runbook)
}

// ListRunbooks returns all runbooks
func (h *RunbookHandler) ListRunbooks(c *gin.Context) {
	var runbooks []model.Runbook
	query := model.DB.Model(&model.Runbook{})

	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if err := query.Find(&runbooks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runbooks)
}

// MatchRunbook finds runbooks matching the alert content
func (h *RunbookHandler) MatchRunbook(c *gin.Context) {
	var req struct {
		AlertContent string `json:"alert_content"`
		Query        string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	content := strings.TrimSpace(req.AlertContent)
	if content == "" {
		content = strings.TrimSpace(req.Query)
	}
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alert_content is required"})
		return
	}

	// Generate embedding for alert
	vec := rag.GenerateMockEmbedding(content, 128)

	// Search
	results, err := h.VectorDB.Search(context.Background(), "runbooks", vec, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch full runbook details
	var matchedRunbooks []model.Runbook
	for _, res := range results {
		id, _ := strconv.Atoi(res.ID)
		var rb model.Runbook
		if err := model.DB.First(&rb, id).Error; err == nil {
			matchedRunbooks = append(matchedRunbooks, rb)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"matches": matchedRunbooks,
		"scores":  results,
	})
}

// ExecuteRunbook starts a runbook execution
func (h *RunbookHandler) ExecuteRunbook(c *gin.Context) {
	runbookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Runbook ID"})
		return
	}

	var req struct {
		AlertID uint                   `json:"alert_id"`
		Context map[string]interface{} `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Context is optional
	}

	var runbook model.Runbook
	if err := model.DB.First(&runbook, runbookID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runbook not found"})
		return
	}

	// Create execution record
	contextBytes, _ := json.Marshal(req.Context)
	var incidentID *uint
	if req.Context != nil {
		if raw, ok := req.Context["incident_id"]; ok {
			if id, err := parseUint(raw); err == nil && id > 0 {
				incidentID = &id
			}
		}
	}
	execution := model.RunbookExecution{
		RunbookID:   uint(runbookID),
		AlertID:     req.AlertID,
		IncidentID:  incidentID,
		Status:      "analyzing", // Start with analyzing
		CurrentStep: 0,
		Context:     contextBytes,
		Logs:        []byte("[]"),
	}

	if err := model.DB.Create(&execution).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if incidentID != nil {
		h.addIncidentActivity(c, *incidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook started: %s execution_id=%d status=%s", runbook.Name, execution.ID, execution.Status), "runbook_execution", &execution.ID, map[string]interface{}{
			"runbook_id":       runbook.ID,
			"runbook_name":     runbook.Name,
			"execution_id":     execution.ID,
			"execution_status": execution.Status,
		})
	}

	// Trigger async execution flow
	go h.processExecution(&execution, &runbook)

	c.JSON(http.StatusCreated, execution)
}

// ApproveExecution continues execution after user approval
func (h *RunbookHandler) ApproveExecution(c *gin.Context) {
	executionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Execution ID"})
		return
	}

	var req struct {
		Approved bool   `json:"approved"`
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var execution model.RunbookExecution
	if err := model.DB.Preload("Runbook").First(&execution, executionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Execution not found"})
		return
	}

	if execution.Status != "waiting_approval" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Execution is not waiting for approval"})
		return
	}

	if req.Approved {
		execution.Status = "executing"
		// Add feedback to logs
		h.appendLog(&execution, "User Approved: "+req.Feedback)
		model.DB.Save(&execution)
		if execution.IncidentID != nil {
			h.addIncidentActivity(c, *execution.IncidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook approved: %s execution_id=%d status=%s", execution.Runbook.Name, execution.ID, execution.Status), "runbook_execution", &execution.ID, map[string]interface{}{
				"runbook_id":       execution.Runbook.ID,
				"runbook_name":     execution.Runbook.Name,
				"execution_id":     execution.ID,
				"execution_status": execution.Status,
			})
		}

		// Continue execution
		go h.processExecution(&execution, &execution.Runbook)
	} else {
		execution.Status = "cancelled"
		h.appendLog(&execution, "User Rejected: "+req.Feedback)
		model.DB.Save(&execution)
		if execution.IncidentID != nil {
			h.addIncidentActivity(c, *execution.IncidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook cancelled: %s execution_id=%d status=%s", execution.Runbook.Name, execution.ID, execution.Status), "runbook_execution", &execution.ID, map[string]interface{}{
				"runbook_id":       execution.Runbook.ID,
				"runbook_name":     execution.Runbook.Name,
				"execution_id":     execution.ID,
				"execution_status": execution.Status,
			})
		}
	}

	c.JSON(http.StatusOK, execution)
}

// ListExecutions returns all executions
func (h *RunbookHandler) ListExecutions(c *gin.Context) {
	var executions []model.RunbookExecution
	query := model.DB.Preload("Runbook").Order("created_at desc")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if rbID := c.Query("runbook_id"); rbID != "" {
		query = query.Where("runbook_id = ?", rbID)
	}

	if err := query.Limit(100).Find(&executions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, executions)
}

// GetExecution returns execution status
func (h *RunbookHandler) GetExecution(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var execution model.RunbookExecution
	if err := model.DB.Preload("Runbook").First(&execution, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Execution not found"})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// Internal processing logic
func (h *RunbookHandler) processExecution(exec *model.RunbookExecution, rb *model.Runbook) {
	// Parse steps
	var steps []model.RunbookStep
	if err := json.Unmarshal(rb.Steps, &steps); err != nil {
		h.failExecution(exec, "Failed to parse runbook steps: "+err.Error())
		return
	}

	// If we are just starting or resuming
	startStep := exec.CurrentStep

	// Check if we are in "analyzing" phase (implicit first step or part of flow)
	if exec.Status == "analyzing" {
		h.appendLog(exec, "Analyzing alert context with AI...")
		// Call Mock AI
		resp, _ := h.AIClient.ChatCompletion(context.Background(), []ai.Message{
			{Role: "user", Content: "Analyze this alert context: " + string(exec.Context)},
		})
		h.appendLog(exec, "AI Analysis: "+resp.Content)

		// Move to waiting approval if configured, or just next step
		// For this task: "Analyze -> Ask User -> Execute"
		exec.Status = "waiting_approval"
		model.DB.Save(exec)
		if exec.IncidentID != nil {
			h.addIncidentActivity(nil, *exec.IncidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook waiting approval: %s execution_id=%d status=%s", rb.Name, exec.ID, exec.Status), "runbook_execution", &exec.ID, map[string]interface{}{
				"runbook_id":       rb.ID,
				"runbook_name":     rb.Name,
				"execution_id":     exec.ID,
				"execution_status": exec.Status,
			})
		}
		return
	}

	if exec.Status == "executing" {
		for i := startStep; i < len(steps); i++ {
			step := steps[i]
			h.appendLog(exec, fmt.Sprintf("Executing step %d: %s (%s)", i+1, step.Type, step.Content))

			// Simulate execution delay
			time.Sleep(1 * time.Second)

			// Update progress
			exec.CurrentStep = i + 1
			model.DB.Save(exec)
		}

		exec.Status = "completed"
		h.appendLog(exec, "Runbook execution completed successfully.")
		model.DB.Save(exec)
		if exec.IncidentID != nil {
			h.addIncidentActivity(nil, *exec.IncidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook completed: %s execution_id=%d status=%s", rb.Name, exec.ID, exec.Status), "runbook_execution", &exec.ID, map[string]interface{}{
				"runbook_id":       rb.ID,
				"runbook_name":     rb.Name,
				"execution_id":     exec.ID,
				"execution_status": exec.Status,
			})
		}
	}
}

func (h *RunbookHandler) failExecution(exec *model.RunbookExecution, reason string) {
	exec.Status = "failed"
	h.appendLog(exec, "Error: "+reason)
	model.DB.Save(exec)
	if exec.IncidentID != nil {
		h.addIncidentActivity(nil, *exec.IncidentID, model.IncidentActivityRunbook, fmt.Sprintf("runbook failed: execution_id=%d reason=%s", exec.ID, strings.TrimSpace(reason)), "runbook_execution", &exec.ID, map[string]interface{}{
			"execution_id":     exec.ID,
			"execution_status": exec.Status,
		})
	}
}

func (h *RunbookHandler) appendLog(exec *model.RunbookExecution, message string) {
	var logs []string
	json.Unmarshal(exec.Logs, &logs)
	logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), message))
	logBytes, _ := json.Marshal(logs)
	exec.Logs = logBytes
}

func (h *RunbookHandler) addIncidentActivity(c *gin.Context, incidentID uint, typ model.IncidentActivityType, message string, refType string, refID *uint, meta map[string]interface{}) {
	msg := strings.TrimSpace(message)
	var createdBy *uint
	if c != nil {
		if raw, ok := c.Get("user_id"); ok {
			if uid, err := parseUint(raw); err == nil && uid > 0 {
				createdBy = &uid
			}
		}
	}
	metaStr := ""
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			metaStr = string(b)
		}
	}
	_ = model.DB.Create(&model.IncidentActivity{
		IncidentID: incidentID,
		Type:       typ,
		Message:    msg,
		RefType:    strings.TrimSpace(refType),
		RefID:      refID,
		Meta:       metaStr,
		CreatedBy:  createdBy,
	}).Error
	_ = model.DB.Model(&model.Incident{}).Where("id = ?", incidentID).Update("last_activity_at", time.Now()).Error
}
