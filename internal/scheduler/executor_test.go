package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/notification"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/glebarez/sqlite"
	prommodel "github.com/prometheus/common/model"
	"gorm.io/gorm"
)

type MockDataSource struct{}

func (m *MockDataSource) Query(ctx context.Context, query string) (*datasource.Result, error) {
	// Return a vector result that triggers the alert
	return &datasource.Result{
		Data: prommodel.Vector{
			{
				Metric:    prommodel.Metric{"__name__": "test_metric"},
				Value:     100.0,
				Timestamp: prommodel.Time(time.Now().UnixMilli()),
			},
		},
	}, nil
}

func (m *MockDataSource) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*datasource.Result, error) {
	// Return a matrix with consistent values (e.g., 50) so that 100 is an anomaly
	var values []prommodel.SamplePair
	// Just return a few points
	for t := start; t <= end; t += step {
		values = append(values, prommodel.SamplePair{
			Timestamp: prommodel.Time(t * 1000),
			Value:     50.0,
		})
	}

	// Ensure at least enough points for stddev
	if len(values) < 5 {
		for i := 0; i < 5; i++ {
			values = append(values, prommodel.SamplePair{
				Timestamp: prommodel.Time((start + int64(i)*step) * 1000),
				Value:     50.0,
			})
		}
	}

	return &datasource.Result{
		Data: prommodel.Matrix{
			{
				Metric: prommodel.Metric{"__name__": "test_metric"},
				Values: values,
			},
		},
	}, nil
}

func (m *MockDataSource) Type() string {
	return "mock"
}

// MockSender implements AlarmSender
type MockSender struct {
	Called bool
}

func (s *MockSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	s.Called = true
	return nil
}

type MockSenderSignal struct {
	Done chan bool
}

func (s *MockSenderSignal) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	s.Done <- true
	return nil
}

func TestExecutor_ExecuteRule_Notification(t *testing.T) {
	// 0. Init Logger
	logger.InitLogger()

	// 1. Setup DB
	db, err := gorm.Open(sqlite.Open("file:memdb_notif?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	db.AutoMigrate(&model.AlertRule{}, &model.DataSource{}, &model.DataName{}, &model.Alarm{}, &model.AlertLog{}, &model.NotificationChannel{}, &model.Team{})

	// 2. Setup Data
	team := model.Team{Name: "Test Team"}
	db.Create(&team)

	channel := model.NotificationChannel{
		Name:   "Test Channel",
		Type:   "mock",
		Config: `{"webhook_url": "http://localhost"}`,
		TeamID: &team.ID,
	}
	db.Create(&channel)

	dsModel := model.DataSource{
		Name: "Test DS",
		Type: "prometheus",
	}
	db.Create(&dsModel)

	dn := model.DataName{
		Name:         "test_dataname_notif",
		DataSourceID: dsModel.ID,
	}
	db.Create(&dn)

	rule := model.AlertRule{
		Name:       "Test Rule",
		DataNameID: dn.ID,
		Query:      "up",
		Condition:  ">",
		Threshold:  50.0,
		TeamID:     team.ID,
		IsEnabled:  true,
	}
	db.Create(&rule)

	// 3. Register Mock Sender
	// Create a channel to signal when sender is called
	done := make(chan bool, 1)

	notification.RegisterSender("mock", func() notification.AlarmSender {
		return &MockSenderSignal{Done: done}
	})

	// 4. Setup Executor
	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	executor := NewExecutor(db, nil, bus)
	executor.DSFactory = func(ds model.DataSource) (datasource.DataSource, error) {
		return &MockDataSource{}, nil
	}

	// 5. Execute Rule
	executor.ExecuteRule(rule.ID)

	// 6. Verify
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Expected notification sender to be called")
	}

	// Verify Alarm Created
	var alarm model.Alarm
	if err := db.First(&alarm).Error; err != nil {
		t.Error("Expected alarm to be created")
	}
	if alarm.Status != model.AlarmStatusFiring {
		t.Errorf("Expected alarm status firing, got %s", alarm.Status)
	}
}

func TestExecutor_ExecuteRule_AIOps_3Sigma(t *testing.T) {
	// 0. Init Logger
	logger.InitLogger()

	// 1. Setup DB
	db, err := gorm.Open(sqlite.Open("file:memdb_aiops?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	db.AutoMigrate(&model.AlertRule{}, &model.DataSource{}, &model.DataName{}, &model.Alarm{}, &model.AlertLog{}, &model.NotificationChannel{}, &model.Team{})

	// 2. Setup Data
	team := model.Team{Name: "Test Team AIOps"}
	db.Create(&team)

	channel := model.NotificationChannel{
		Name:   "Test Channel AIOps",
		Type:   "mock_aiops",
		Config: `{"webhook_url": "http://localhost"}`,
		TeamID: &team.ID,
	}
	db.Create(&channel)

	dsModel := model.DataSource{
		Name: "Test DS AIOps",
		Type: "prometheus",
	}
	db.Create(&dsModel)

	dn := model.DataName{
		Name:         "test_dataname_aiops",
		DataSourceID: dsModel.ID,
	}
	db.Create(&dn)

	// Rule with 3sigma
	rule := model.AlertRule{
		Name:       "Test Rule 3Sigma",
		DataNameID: dn.ID,
		Query:      "up",
		Algorithm:  "3sigma",
		AlgoParams: `{"n": 3, "window": "1h"}`,
		TeamID:     team.ID,
		IsEnabled:  true,
	}
	db.Create(&rule)

	// 3. Register Mock Sender
	done := make(chan bool, 1)
	notification.RegisterSender("mock_aiops", func() notification.AlarmSender {
		return &MockSenderSignal{Done: done}
	})

	// 4. Setup Executor
	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	executor := NewExecutor(db, nil, bus)
	executor.DSFactory = func(ds model.DataSource) (datasource.DataSource, error) {
		return &MockDataSource{}, nil
	}

	// 5. Execute Rule
	// MockDS returns current=100, history=50 (flat). Mean=50, Std=0. 100 is anomaly.
	executor.ExecuteRule(rule.ID)

	// 6. Verify
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Expected notification sender to be called for 3sigma anomaly")
	}

	var alarm model.Alarm
	if err := db.Where("alert_rule_id = ?", rule.ID).First(&alarm).Error; err != nil {
		t.Error("Expected alarm to be created")
	}
}
