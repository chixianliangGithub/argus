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
	"github.com/go-redis/redismock/v9"
	prommodel "github.com/prometheus/common/model"
	"gorm.io/gorm"
)

// MockDataSourceGov for Governance testing
type MockDataSourceGov struct {
	Data []prommodel.SamplePair
}

func (m *MockDataSourceGov) Query(ctx context.Context, query string) (*datasource.Result, error) {
	// Return multiple metrics to test grouping
	return &datasource.Result{
		Data: prommodel.Vector{
			{
				Metric:    prommodel.Metric{"__name__": "http_requests", "job": "api", "instance": "server1"},
				Value:     100.0,
				Timestamp: prommodel.Time(time.Now().UnixMilli()),
			},
			{
				Metric:    prommodel.Metric{"__name__": "http_requests", "job": "api", "instance": "server2"},
				Value:     120.0,
				Timestamp: prommodel.Time(time.Now().UnixMilli()),
			},
			{
				Metric:    prommodel.Metric{"__name__": "http_requests", "job": "db", "instance": "server3"},
				Value:     200.0,
				Timestamp: prommodel.Time(time.Now().UnixMilli()),
			},
		},
	}, nil
}

func (m *MockDataSourceGov) QueryRange(ctx context.Context, query string, start, end int64, step int64) (*datasource.Result, error) {
	return nil, nil
}

func (m *MockDataSourceGov) Type() string {
	return "mock_gov"
}

func TestExecutor_GroupingAndDeduplication(t *testing.T) {
	logger.InitLogger()

	// 1. Setup DB
	db, err := gorm.Open(sqlite.Open("file:memdb_gov?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	db.AutoMigrate(&model.AlertRule{}, &model.DataSource{}, &model.DataName{}, &model.Alarm{}, &model.AlertLog{}, &model.NotificationChannel{}, &model.Team{})

	// 2. Setup Redis Mock
	rdb, mock := redismock.NewClientMock()

	// 3. Setup Data
	team := model.Team{Name: "Test Team Gov"}
	db.Create(&team)

	channel := model.NotificationChannel{
		Name:   "Test Channel Gov",
		Type:   "mock_gov",
		Config: `{"webhook_url": "http://localhost"}`,
		TeamID: &team.ID,
	}
	db.Create(&channel)

	dsModel := model.DataSource{
		Name: "Test DS Gov",
		Type: "prometheus",
	}
	db.Create(&dsModel)

	dn := model.DataName{
		Name:         "test_dataname_gov",
		DataSourceID: dsModel.ID,
	}
	db.Create(&dn)

	// Rule 1: Group by "job"
	rule := model.AlertRule{
		Name:          "Group By Job",
		DataNameID:    dn.ID,
		Query:         "http_requests",
		Condition:     ">",
		Threshold:     50.0,
		TeamID:        team.ID,
		IsEnabled:     true,
		GroupBy:       "job",
		SilencePeriod: 10,
	}
	db.Create(&rule)

	// 4. Register Mock Sender
	callCount := 0
	notification.RegisterSender("mock_gov", func() notification.AlarmSender {
		return &MockSenderFunc{
			Func: func(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
				callCount++
				return nil
			},
		}
	})

	// 5. Setup Executor
	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	executor := NewExecutor(db, rdb, bus)
	executor.DSFactory = func(ds model.DataSource) (datasource.DataSource, error) {
		return &MockDataSourceGov{}, nil
	}

	// 6. Execute Rule - First Run
	// Expectation:
	// - 3 triggered metrics.
	// - Grouped by "job": "api" (server1, server2) and "db" (server3).
	// - 2 Alarms created.
	// - 2 Notifications sent.
	// - 2 Redis keys set (silence).

	// Redis expectations
	// job=api (first alphabetically)
	mock.ExpectExists("alert:silence:" + generateFingerprint(rule.ID, "job=api")).SetVal(0)
	mock.ExpectSet("alert:silence:"+generateFingerprint(rule.ID, "job=api"), "1", 10*time.Minute).SetVal("OK")

	// job=db (second alphabetically)
	mock.ExpectExists("alert:silence:" + generateFingerprint(rule.ID, "job=db")).SetVal(0)
	mock.ExpectSet("alert:silence:"+generateFingerprint(rule.ID, "job=db"), "1", 10*time.Minute).SetVal("OK")

	executor.ExecuteRule(rule.ID)

	// Wait for async notifications
	time.Sleep(100 * time.Millisecond)

	if callCount != 2 {
		t.Errorf("Expected 2 notifications (one per group), got %d", callCount)
	}

	var alarms []model.Alarm
	db.Find(&alarms)
	if len(alarms) != 2 {
		t.Errorf("Expected 2 alarms, got %d", len(alarms))
	}

	// 7. Execute Rule - Second Run (Silenced)
	// Expectation:
	// - Same triggers.
	// - Redis keys exist.
	// - No new notifications.
	// - Alarms updated (content same, so maybe no update, but definitely no notification).

	callCount = 0 // Reset

	mock.ExpectExists("alert:silence:" + generateFingerprint(rule.ID, "job=api")).SetVal(1)
	mock.ExpectExists("alert:silence:" + generateFingerprint(rule.ID, "job=db")).SetVal(1)

	// Note: If silenced, we don't Set silence again in current implementation unless we want to extend it?
	// The current implementation calls setSilence only if !isSilenced (for existing alarm) or if !isSilenced (for new alarm).
	// So we expect NO Set calls.

	executor.ExecuteRule(rule.ID)

	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Errorf("Expected 0 notifications (silenced), got %d", callCount)
	}

	// Verify expectation
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

type MockSenderFunc struct {
	Func func(ctx context.Context, alarm *model.Alarm, config map[string]string) error
}

func (s *MockSenderFunc) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	return s.Func(ctx, alarm, config)
}
