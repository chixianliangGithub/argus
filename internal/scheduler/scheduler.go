package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Scheduler struct {
	cron      *cron.Cron
	db        *gorm.DB
	rdb       *redis.Client
	executor  *Executor
	isLeader  bool
	leaderKey string
	mu        sync.Mutex
	stopCh    chan struct{}
	jobs      map[uint]cron.EntryID
	jobSpecs  map[uint]string
}

const (
	alertTaskStreamKey = "argus:alert:tasks:stream"
	alertTaskGroupName = "argus:alert:scheduler"
	alertTaskDLQKey    = "argus:alert:tasks:dlq"
)

func NewScheduler(db *gorm.DB, rdb *redis.Client, bus eventbus.EventBus) *Scheduler {
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		db:        db,
		rdb:       rdb,
		executor:  NewExecutor(db, rdb, bus),
		leaderKey: "argus:scheduler:leader",
		stopCh:    make(chan struct{}),
		jobs:      make(map[uint]cron.EntryID),
		jobSpecs:  make(map[uint]string),
	}
}

func (s *Scheduler) Start() {
	go s.leaderElectionLoop()
	go s.executorLoop()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.cron.Stop()
}

func (s *Scheduler) leaderElectionLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Sync rules periodically
	syncTicker := time.NewTicker(1 * time.Minute)
	defer syncTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tryBecomeLeader()
		case <-syncTicker.C:
			if s.isLeader {
				s.syncRules()
			}
		}
	}
}

func (s *Scheduler) tryBecomeLeader() {
	ctx := context.Background()
	// Try to acquire lock
	success, err := s.rdb.SetNX(ctx, s.leaderKey, "leader", 15*time.Second).Result()
	if err != nil {
		logger.Log.Error("Failed to acquire leader lock", zap.Error(err))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if success {
		if !s.isLeader {
			logger.Log.Info("I am the leader now")
			s.isLeader = true
			s.cron.Start()
			// Immediate sync when becoming leader
			// Release lock to call syncRules which acquires it?
			// syncRules acquires lock. So we must unlock before calling it.
			// But we are holding lock.
			// Let's refactor syncRules to NOT acquire lock, but assume caller holds it?
			// Or just call s.syncRulesInternal()
			s.syncRulesInternal()
		}
		// Refresh lock
		s.rdb.Expire(ctx, s.leaderKey, 15*time.Second)
	} else {
		if s.isLeader {
			// Try to renew
			res, err := s.rdb.Expire(ctx, s.leaderKey, 15*time.Second).Result()
			if err != nil || !res {
				logger.Log.Warn("Failed to renew leadership", zap.Error(err))
				s.stepDownInternal()
			}
		}
	}
}

func (s *Scheduler) stepDown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stepDownInternal()
}

func (s *Scheduler) stepDownInternal() {
	if s.isLeader {
		logger.Log.Info("Stepping down from leadership")
		s.isLeader = false
		s.cron.Stop()
		// Clear jobs
		s.cron = cron.New(cron.WithSeconds())
		s.jobs = make(map[uint]cron.EntryID)
	}
}

func (s *Scheduler) syncRules() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncRulesInternal()
}

func (s *Scheduler) syncRulesInternal() {
	logger.Log.Info("Syncing rules...")
	var rules []model.AlertRule
	if err := s.db.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		logger.Log.Error("Failed to load rules", zap.Error(err))
		return
	}

	activeIDs := make(map[uint]bool)

	for _, rule := range rules {
		activeIDs[rule.ID] = true

		var spec string
		if rule.CronExpression != "" {
			spec = rule.CronExpression
		} else {
			duration := rule.Duration
			if duration <= 0 {
				duration = 60
			}
			spec = fmt.Sprintf("@every %ds", duration)
		}

		// Check if spec changed or job not exists
		if oldSpec, exists := s.jobSpecs[rule.ID]; exists && oldSpec != spec {
			// Remove old job
			if entryID, ok := s.jobs[rule.ID]; ok {
				s.cron.Remove(entryID)
				delete(s.jobs, rule.ID)
			}
		}

		if _, exists := s.jobs[rule.ID]; !exists {
			r := rule // capture loop variable
			entryID, err := s.cron.AddFunc(spec, func() {
				task := AlertTask{RuleID: r.ID}
				data, _ := json.Marshal(task)
				if s.rdb != nil {
					_, _ = s.rdb.XAdd(context.Background(), &redis.XAddArgs{
						Stream: alertTaskStreamKey,
						Values: map[string]interface{}{"payload": string(data)},
					}).Result()
				}
			})
			if err == nil {
				s.jobs[rule.ID] = entryID
				s.jobSpecs[rule.ID] = spec
				logger.Log.Info("Scheduled rule", zap.Uint("id", rule.ID), zap.String("spec", spec))
			} else {
				logger.Log.Error("Failed to schedule rule", zap.Uint("rule_id", rule.ID), zap.Error(err))
			}
		}
	}

	// Remove deleted/disabled rules
	for id, entryID := range s.jobs {
		if !activeIDs[id] {
			s.cron.Remove(entryID)
			delete(s.jobs, id)
			delete(s.jobSpecs, id)
			logger.Log.Info("Removed rule", zap.Uint("id", id))
		}
	}
}

func (s *Scheduler) executorLoop() {
	if s.rdb == nil {
		return
	}
	consumer := strings.TrimSpace(os.Getenv("ARGUS_STREAM_CONSUMER"))
	if consumer == "" {
		hn, _ := os.Hostname()
		consumer = fmt.Sprintf("%s-%d", hn, os.Getpid())
	}
	_ = s.rdb.XGroupCreateMkStream(context.Background(), alertTaskStreamKey, alertTaskGroupName, "$").Err()
	lastClaim := time.Time{}
	for {
		select {
		case <-s.stopCh:
			return
		default:
			if lastClaim.IsZero() || time.Since(lastClaim) > 30*time.Second {
				lastClaim = time.Now()
				msgs, _, err := s.rdb.XAutoClaim(context.Background(), &redis.XAutoClaimArgs{
					Stream:   alertTaskStreamKey,
					Group:    alertTaskGroupName,
					Consumer: consumer,
					MinIdle:  60 * time.Second,
					Start:    "0-0",
					Count:    10,
				}).Result()
				if err == nil {
					for _, msg := range msgs {
						s.processStreamMessage(consumer, msg)
					}
				}
			}

			streams, err := s.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
				Group:    alertTaskGroupName,
				Consumer: consumer,
				Streams:  []string{alertTaskStreamKey, ">"},
				Count:    10,
				Block:    5 * time.Second,
			}).Result()
			if err != nil {
				continue
			}
			for _, st := range streams {
				for _, msg := range st.Messages {
					s.processStreamMessage(consumer, msg)
				}
			}
		}
	}
}

func (s *Scheduler) processStreamMessage(consumer string, msg redis.XMessage) {
	if s.rdb == nil {
		return
	}
	raw, _ := msg.Values["payload"]
	payload := fmt.Sprint(raw)
	var task AlertTask
	if err := json.Unmarshal([]byte(payload), &task); err != nil {
		_, _ = s.rdb.XAck(context.Background(), alertTaskStreamKey, alertTaskGroupName, msg.ID).Result()
		_, _ = s.rdb.XDel(context.Background(), alertTaskStreamKey, msg.ID).Result()
		return
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = s.rdb.XAdd(context.Background(), &redis.XAddArgs{
					Stream: alertTaskDLQKey,
					Values: map[string]interface{}{
						"payload":  payload,
						"error":    fmt.Sprint(r),
						"at":       time.Now().Format(time.RFC3339),
						"consumer": consumer,
					},
				}).Result()
			}
		}()
		s.executeTask(task)
	}()
	_, _ = s.rdb.XAck(context.Background(), alertTaskStreamKey, alertTaskGroupName, msg.ID).Result()
	_, _ = s.rdb.XDel(context.Background(), alertTaskStreamKey, msg.ID).Result()
}

func (s *Scheduler) executeTask(task AlertTask) {
	s.executor.ExecuteRuleWithForce(task.RuleID, task.Force)
}

// AddJob adds a local job (for testing or internal tasks)
func (s *Scheduler) AddJob(spec string, cmd func()) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, cmd)
}
