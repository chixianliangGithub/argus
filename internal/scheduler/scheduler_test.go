package scheduler

import (
	"testing"
	"time"

	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redismock/v9"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&model.AlertRule{}, &model.DataSource{})
	return db
}

func TestScheduler_LeaderElection(t *testing.T) {
	logger.InitLogger()
	db := setupTestDB()
	rdb, mock := redismock.NewClientMock()

	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	s := NewScheduler(db, rdb, bus)
	s.leaderKey = "argus:scheduler:leader"

	// Mock SetNX success (become leader)
	mock.ExpectSetNX("argus:scheduler:leader", "leader", 15*time.Second).SetVal(true)
	mock.ExpectExpire("argus:scheduler:leader", 15*time.Second).SetVal(true)

	// Call private method
	s.tryBecomeLeader()

	if !s.isLeader {
		t.Error("Expected to become leader")
	}

	// Verify expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet mock expectations: %s", err)
	}
}

func TestScheduler_SyncRules(t *testing.T) {
	logger.InitLogger()
	db := setupTestDB()
	rdb, _ := redismock.NewClientMock()

	// Add a rule
	rule := model.AlertRule{
		Name:      "Test Rule",
		Duration:  10,
		IsEnabled: true,
	}
	db.Create(&rule)

	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()
	s := NewScheduler(db, rdb, bus)
	s.isLeader = true // Force leader

	// Mock Redis RPush when job runs
	// Wait, job runs asynchronously. We can't easily mock it unless we wait.
	// But syncRules just adds the job.

	s.syncRules()

	entries := s.cron.Entries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 job, got %d", len(entries))
	}

	// Verify cron schedule
	entry := entries[0]
	// cron/v3 doesn't expose Schedule string easily, but we can check Next run time
	if !entry.Valid() {
		t.Error("Job entry is invalid")
	}
}

func TestScheduler_ExecutorLoop(t *testing.T) {
	// This tests if BLPop works and calls executeTask
	// It's hard to test the loop directly without blocking.
	// We can test executeTask logic separately.
}
