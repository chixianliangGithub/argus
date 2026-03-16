package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTestDB() {
	// Use in-memory SQLite for testing
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	model.DB = db
	model.Migrate()
}

func TestRunbookFlow(t *testing.T) {
	setupTestDB()
	gin.SetMode(gin.TestMode)

	h := NewRunbookHandler()
	r := gin.Default()
	r.POST("/runbooks", h.CreateRunbook)
	r.POST("/runbooks/match", h.MatchRunbook)
	r.POST("/runbooks/:id/execute", h.ExecuteRunbook)
	r.POST("/runbooks/executions/:id/approve", h.ApproveExecution)
	r.GET("/runbooks/executions/:id", h.GetExecution)

	// 1. Create Runbook
	steps := []model.RunbookStep{
		{Type: "action", Content: "Restart Service"},
		{Type: "notify", Content: "Slack Channel"},
	}
	stepsBytes, _ := json.Marshal(steps)

	runbook := model.Runbook{
		Name:        "High CPU Runbook",
		Description: "Handles high CPU usage alerts",
		TriggerType: "keyword",
		TriggerVal:  "cpu usage high",
		Steps:       stepsBytes,
	}

	body, _ := json.Marshal(runbook)
	req, _ := http.NewRequest("POST", "/runbooks", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var createdRunbook model.Runbook
	json.Unmarshal(w.Body.Bytes(), &createdRunbook)
	assert.Equal(t, "High CPU Runbook", createdRunbook.Name)

	// 2. Match Runbook
	matchReq := map[string]string{"alert_content": "The cpu usage is very high on server-1"}
	body, _ = json.Marshal(matchReq)
	req, _ = http.NewRequest("POST", "/runbooks/match", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var matchResp struct {
		Matches []model.Runbook `json:"matches"`
	}
	json.Unmarshal(w.Body.Bytes(), &matchResp)
	assert.NotEmpty(t, matchResp.Matches)
	assert.Equal(t, createdRunbook.ID, matchResp.Matches[0].ID)

	// 3. Execute Runbook
	execReq := map[string]interface{}{
		"alert_id": 123,
		"context":  map[string]string{"hostname": "server-1"},
	}
	body, _ = json.Marshal(execReq)
	req, _ = http.NewRequest("POST", "/runbooks/"+useStr(createdRunbook.ID)+"/execute", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var execution model.RunbookExecution
	json.Unmarshal(w.Body.Bytes(), &execution)
	assert.Equal(t, "analyzing", execution.Status)

	// Wait for async processing (Analyzing -> Waiting Approval)
	deadline := time.Now().Add(2 * time.Second)
	for {
		req, _ = http.NewRequest("GET", "/runbooks/executions/"+useStr(execution.ID), nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		json.Unmarshal(w.Body.Bytes(), &execution)
		if execution.Status == "waiting_approval" {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.Equal(t, "waiting_approval", execution.Status)

	// 4. Approve Execution
	approveReq := map[string]interface{}{
		"approved": true,
		"feedback": "Go ahead",
	}
	body, _ = json.Marshal(approveReq)
	req, _ = http.NewRequest("POST", "/runbooks/executions/"+useStr(execution.ID)+"/approve", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for execution to complete
	deadline = time.Now().Add(8 * time.Second)
	for {
		req, _ = http.NewRequest("GET", "/runbooks/executions/"+useStr(execution.ID), nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		json.Unmarshal(w.Body.Bytes(), &execution)
		if execution.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.Equal(t, "completed", execution.Status)
	assert.Equal(t, 2, execution.CurrentStep)
}

func useStr(id uint) string {
	// Helper to convert uint to string
	b, _ := json.Marshal(id)
	return string(b)
}
