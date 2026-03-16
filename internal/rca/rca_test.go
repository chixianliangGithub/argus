package rca

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Mock DataSource
type MockDataSource struct{}

func (m *MockDataSource) Type() string { return "mock" }
func (m *MockDataSource) Query(ctx context.Context, query string) (*datasource.Result, error) {
	return &datasource.Result{Data: map[string]interface{}{"foo": "bar"}}, nil
}
func (m *MockDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*datasource.Result, error) {
	return &datasource.Result{Data: map[string]interface{}{"foo": "bar"}}, nil
}

func TestRCAService_HandleAlertFiring(t *testing.T) {
	logger.InitLogger()
	// 1. Setup DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.AlertRule{}, &model.Alarm{}, &model.RCAReport{}, &model.DataSource{}, &model.DataName{})

	// 2. Setup Bus and AI
	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	aiClient := ai.NewMockClient()

	// 3. Setup Service
	service := NewRCAService(db, bus, aiClient)
	// Mock DSFactory
	service.DSFactory = func(model.DataSource) (datasource.DataSource, error) {
		return &MockDataSource{}, nil
	}

	// 4. Create Data
	ds := model.DataSource{
		Name: "test-ds",
		Type: "prometheus",
		URL:  "http://localhost:9090",
	}
	db.Create(&ds)

	dn := model.DataName{
		Name:         "test-dn",
		DataSourceID: ds.ID,
	}
	db.Create(&dn)

	rule := model.AlertRule{
		Name:       "High CPU",
		DataNameID: dn.ID,
		Query:      "up",
	}
	db.Create(&rule)

	alarm := model.Alarm{
		AlertRuleID: rule.ID,
		Status:      model.AlarmStatusFiring,
		Content:     "CPU > 90%",
		StartsAt:    time.Now(),
		Fingerprint: "test-fingerprint",
	}
	db.Create(&alarm)

	// 5. Trigger Event
	// We manually call the handler to avoid async timing issues in test,
	// or we can publish and wait.
	// Let's call manually for simplicity, or publish to test subscription.

	// Publish
	err = bus.Publish(context.Background(), "alert.firing", alarm)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// 6. Verify Report
	var report model.RCAReport
	err = db.Where("alarm_id = ?", alarm.ID).First(&report).Error
	assert.NoError(t, err)
	assert.Equal(t, alarm.ID, report.AlarmID)
	assert.NotEmpty(t, report.RootCause)
	assert.NotEmpty(t, report.Evidence)

	var evidence map[string]interface{}
	err = json.Unmarshal([]byte(report.Evidence), &evidence)
	assert.NoError(t, err)
	assert.Contains(t, evidence, "logs")
	assert.Contains(t, evidence, "commits")
	assert.Contains(t, evidence, "metrics")
}
