package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/ai"
	"github.com/argus-monitoring/argus/internal/config"
	"github.com/argus-monitoring/argus/internal/eventbus"
	"github.com/argus-monitoring/argus/internal/handler"
	"github.com/argus-monitoring/argus/internal/incident"
	"github.com/argus-monitoring/argus/internal/middleware"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/rca"
	"github.com/argus-monitoring/argus/internal/scheduler"
	"github.com/argus-monitoring/argus/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 1. Load Config
	config.LoadConfig()

	// 2. Init Logger
	logger.InitLogger()
	defer logger.Sync()

	// 3. Init DB
	// Note: Ensure MySQL is running locally or update config.yaml
	model.InitDB()
	model.Migrate()
	model.Seed()

	// 3.5. Init EventBus
	bus := eventbus.NewMemoryEventBus()
	defer bus.Close()

	var aiClient ai.LLMClient
	switch strings.ToLower(strings.TrimSpace(config.AppConfig.AI.Provider)) {
	case "", "mock":
		aiClient = ai.NewMockClient()
	case "openai":
		aiClient = ai.NewOpenAIClientWithConfig(
			config.AppConfig.AI.APIKey,
			config.AppConfig.AI.BaseURL,
			config.AppConfig.AI.Model,
			time.Duration(config.AppConfig.AI.TimeoutSeconds)*time.Second,
		)
	case "deepseek":
		aiClient = ai.NewDeepSeekClientWithConfig(
			config.AppConfig.AI.APIKey,
			config.AppConfig.AI.BaseURL,
			config.AppConfig.AI.Model,
			time.Duration(config.AppConfig.AI.TimeoutSeconds)*time.Second,
		)
	default:
		aiClient = ai.NewMockClient()
	}
	_ = incident.NewService(model.DB, bus)
	_ = rca.NewRCAService(model.DB, bus, aiClient)

	// 4. Init Scheduler
	sched := scheduler.NewScheduler(model.DB, model.RDB, bus)
	_, err := sched.AddJob("*/10 * * * * *", func() {
		logger.Log.Info("Scheduler tick: executing dummy job")
	})
	if err != nil {
		logger.Log.Error("Failed to add job", zap.Error(err))
	}
	sched.Start()
	defer sched.Stop()

	// 5. Init Gin
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dbOK := false
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				dbOK = sqlDB.PingContext(ctx) == nil
			}
		}

		redisOK := false
		streamLen := int64(0)
		pendingCount := int64(0)
		leaderTTLSeconds := int64(0)
		if model.RDB != nil {
			redisOK = model.RDB.Ping(ctx).Err() == nil
			if redisOK {
				streamLen, _ = model.RDB.XLen(ctx, "argus:alert:tasks:stream").Result()
				if p, err := model.RDB.XPending(ctx, "argus:alert:tasks:stream", "argus:alert:scheduler").Result(); err == nil {
					pendingCount = p.Count
				}
				if ttl, err := model.RDB.TTL(ctx, "argus:scheduler:leader").Result(); err == nil && ttl > 0 {
					leaderTTLSeconds = int64(ttl.Seconds())
				}
			}
		}

		ok := dbOK && redisOK
		c.JSON(http.StatusOK, gin.H{
			"ok":                 ok,
			"db_ok":              dbOK,
			"redis_ok":           redisOK,
			"stream_len":         streamLen,
			"pending_count":      pendingCount,
			"leader_ttl_seconds": leaderTTLSeconds,
			"now":                time.Now(),
		})
	})

	// Public API Routes
	publicApi := r.Group("/api")
	{
		authHandler := &handler.AuthHandler{}
		publicApi.POST("/register", authHandler.Register)
		publicApi.POST("/login", authHandler.Login)

		twoFAHandler := &handler.TwoFAHandler{}
		publicApi.POST("/2fa/setup/confirm", twoFAHandler.SetupConfirm)
	}

	// Protected API Routes
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	api.Use(middleware.AuditMiddleware())
	if config.AppConfig.Security.PrivacySanitizeResponse {
		api.Use(middleware.PrivacyMiddleware())
	}
	{
		// User Routes
		userHandler := &handler.UserHandler{}
		api.GET("/user/info", userHandler.GetInfo)
		api.GET("/users", middleware.RequireRole("admin"), userHandler.List)
		api.POST("/users", middleware.RequireRole("admin"), userHandler.Create)
		api.PUT("/users/:id", middleware.RequireRole("admin"), userHandler.Update)
		api.DELETE("/users/:id", middleware.RequireRole("admin"), userHandler.Delete)

		// 2FA Routes (Google Authenticator)
		twoFAHandler := &handler.TwoFAHandler{}
		api.GET("/2fa/status", twoFAHandler.Status)
		api.POST("/2fa/setup", twoFAHandler.Setup)
		api.POST("/2fa/enable", twoFAHandler.Enable)
		api.POST("/2fa/disable", twoFAHandler.Disable)

		// System Settings
		sysHandler := &handler.SystemSettingHandler{}
		api.GET("/system-settings/2fa", sysHandler.GetTwoFA)
		api.PUT("/system-settings/2fa", middleware.RequireRole("admin"), sysHandler.SetTwoFA)

		// Message Template Routes
		tplHandler := &handler.MessageTemplateHandler{}
		api.GET("/templates", tplHandler.List)
		api.POST("/templates", middleware.RequireRole("admin"), tplHandler.Create)
		api.PUT("/templates/:id", middleware.RequireRole("admin"), tplHandler.Update)
		api.DELETE("/templates/:id", middleware.RequireRole("admin"), tplHandler.Delete)

		// Notification Channel Routes
		chHandler := &handler.NotificationChannelHandler{}
		api.GET("/notification-channels", chHandler.List)
		api.POST("/notification-channels", middleware.RequireRole("admin"), chHandler.Create)
		api.PUT("/notification-channels/:id", middleware.RequireRole("admin"), chHandler.Update)
		api.DELETE("/notification-channels/:id", middleware.RequireRole("admin"), chHandler.Delete)
		api.POST("/notification-channels/:id/test", chHandler.Test)

		// Audit Log Routes
		auditHandler := &handler.AuditLogHandler{}
		api.GET("/audit-logs", middleware.RequireRole("admin"), auditHandler.List)

		api.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "You are authenticated!"})
		})

		// Datasource Routes
		dsHandler := &handler.DataSourceHandler{}
		api.POST("/datasources", dsHandler.Create)
		api.GET("/datasources/:id", dsHandler.Get)
		api.PUT("/datasources/:id", dsHandler.Update)
		api.DELETE("/datasources/:id", dsHandler.Delete)
		api.GET("/datasources", dsHandler.List)

		// DataName Routes
		dnHandler := &handler.DataNameHandler{}
		api.POST("/datanames", dnHandler.Create)
		api.GET("/datanames/:id", dnHandler.Get)
		api.PUT("/datanames/:id", dnHandler.Update)
		api.DELETE("/datanames/:id", dnHandler.Delete)
		api.GET("/datanames", dnHandler.List)

		// AlertRule Routes
		ruleHandler := &handler.AlertRuleHandler{}
		api.POST("/alert-rules", ruleHandler.Create)
		api.GET("/alert-rules/:id", ruleHandler.Get)
		api.PUT("/alert-rules/:id", ruleHandler.Update)
		api.DELETE("/alert-rules/:id", ruleHandler.Delete)
		api.GET("/alert-rules", ruleHandler.List)
		api.PUT("/alert-rules/batch-enable", middleware.RequireRole("admin"), ruleHandler.BatchUpdateEnabled)
		api.GET("/alert-rules/export", ruleHandler.ExportYAML)
		api.POST("/alert-rules/import", ruleHandler.ImportYAML)
		api.POST("/alert-rules/preview", ruleHandler.Preview)
		api.POST("/alert-rules/:id/trigger", ruleHandler.Trigger)
		api.GET("/alert-rules/:id/runtime", ruleHandler.Runtime)

		// Alarm Routes
		alarmHandler := &handler.AlarmHandler{}
		api.GET("/alarms/:id", alarmHandler.Get)
		api.GET("/alarms", alarmHandler.List)
		api.PUT("/alarms/:id", alarmHandler.Update)
		api.POST("/alarms/:id/claim", alarmHandler.Claim)
		api.POST("/alarms/:id/unclaim", alarmHandler.Unclaim)
		api.POST("/alarms/:id/handle", alarmHandler.Handle)
		api.POST("/alarms/batch-claim", alarmHandler.BatchClaim)
		api.POST("/alarms/batch-unclaim", alarmHandler.BatchUnclaim)
		api.POST("/alarms/batch-handle", alarmHandler.BatchHandle)
		api.POST("/alarms/batch-resolve", alarmHandler.BatchResolve)
		api.POST("/alarms/batch-silence", alarmHandler.BatchSilence)

		// Incident Routes
		incidentHandler := &handler.IncidentHandler{}
		api.GET("/incidents", incidentHandler.List)
		api.GET("/incidents/:id", incidentHandler.Get)
		api.PUT("/incidents/:id", incidentHandler.Update)
		api.POST("/incidents/:id/ack", incidentHandler.Ack)
		api.POST("/incidents/:id/resolve", incidentHandler.Resolve)
		api.POST("/incidents/:id/merge", incidentHandler.Merge)
		api.POST("/incidents/:id/comment", incidentHandler.Comment)
		api.GET("/incidents/:id/activities", incidentHandler.Activities)
		api.GET("/incidents/:id/export", incidentHandler.ExportMarkdown)
		incidentAIInsightHandler := handler.NewIncidentAIInsightHandler(aiClient)
		api.POST("/incidents/:id/ai-insights", incidentAIInsightHandler.Create)
		api.GET("/incidents/:id/ai-insights", incidentAIInsightHandler.List)

		// Service Routes
		serviceHandler := &handler.ServiceHandler{}
		api.GET("/services/overview", serviceHandler.Overview)
		api.GET("/services/:service/summary", serviceHandler.Summary)

		// AlertLog Routes
		logHandler := &handler.AlertLogHandler{}
		api.GET("/alert-logs/:id", logHandler.Get)
		api.GET("/alert-logs", logHandler.List)

		// Alert Eval Record Routes
		evalHandler := &handler.AlertEvalRecordHandler{}
		api.GET("/alert-eval-records", evalHandler.List)

		// Query Routes
		queryHandler := &handler.QueryHandler{}
		api.POST("/query", queryHandler.Query)

		// RCA Routes
		rcaHandler := &handler.RCAHandler{}
		api.GET("/rca-reports", rcaHandler.List)
		api.GET("/rca-reports/:id", rcaHandler.Get)

		// AI Routes
		aiHandler := handler.NewAIHandler(aiClient)
		api.POST("/ai/text-to-query", aiHandler.TextToQuery)
		api.POST("/ai/insight", aiHandler.Insight)
		aiPromptHandler := &handler.AIPromptHandler{}
		api.GET("/ai/prompts", middleware.RequireRole("admin"), aiPromptHandler.List)
		api.POST("/ai/prompts", middleware.RequireRole("admin"), aiPromptHandler.Create)
		api.POST("/ai/prompts/:id/activate", middleware.RequireRole("admin"), aiPromptHandler.Activate)
		aiQuotaHandler := &handler.AIQuotaHandler{}
		api.GET("/ai/quotas/:team_id", middleware.RequireRole("admin"), aiQuotaHandler.Get)
		api.PUT("/ai/quotas/:team_id", middleware.RequireRole("admin"), aiQuotaHandler.Upsert)

		// Silence Rules
		silenceHandler := &handler.SilenceHandler{}
		api.GET("/silences", silenceHandler.List)
		api.POST("/silences", silenceHandler.Create)
		api.PUT("/silences/:id", silenceHandler.Update)
		api.DELETE("/silences/:id", silenceHandler.Delete)

		// Inhibit Rules
		inhibitRuleHandler := &handler.InhibitRuleHandler{}
		api.GET("/inhibit-rules", inhibitRuleHandler.List)
		api.GET("/inhibit-rules/:id", inhibitRuleHandler.Get)
		api.POST("/inhibit-rules", inhibitRuleHandler.Create)
		api.PUT("/inhibit-rules/:id", inhibitRuleHandler.Update)
		api.DELETE("/inhibit-rules/:id", inhibitRuleHandler.Delete)

		// Escalation Policies
		escHandler := &handler.EscalationHandler{}
		api.GET("/escalations", escHandler.List)
		api.GET("/escalations/:id", escHandler.Get)
		api.POST("/escalations", escHandler.Create)
		api.PUT("/escalations/:id", escHandler.Update)
		api.DELETE("/escalations/:id", escHandler.Delete)

		// Routing Rules
		routingRuleHandler := &handler.RoutingRuleHandler{}
		api.GET("/routing-rules", routingRuleHandler.List)
		api.GET("/routing-rules/:id", routingRuleHandler.Get)
		api.POST("/routing-rules", routingRuleHandler.Create)
		api.PUT("/routing-rules/:id", routingRuleHandler.Update)
		api.DELETE("/routing-rules/:id", routingRuleHandler.Delete)
		api.POST("/routing-rules/preview", routingRuleHandler.Preview)
		api.GET("/routing-rules/matcher-options", routingRuleHandler.MatcherOptions)

		// Team Routes
		teamHandler := &handler.TeamHandler{}
		api.GET("/teams", teamHandler.List)
		api.POST("/teams", teamHandler.Create)
		api.GET("/teams/:id", teamHandler.Get)
		api.PUT("/teams/:id", teamHandler.Update)
		api.DELETE("/teams/:id", teamHandler.Delete)
		api.POST("/teams/:id/members", teamHandler.AddMember)
		api.DELETE("/teams/:id/members/:user_id", teamHandler.RemoveMember)

		// Runbook Routes
		runbookHandler := handler.NewRunbookHandler()
		api.POST("/runbooks", runbookHandler.CreateRunbook)
		api.GET("/runbooks", runbookHandler.ListRunbooks)
		api.POST("/runbooks/match", runbookHandler.MatchRunbook)
		api.POST("/runbooks/:id/execute", runbookHandler.ExecuteRunbook)
		api.POST("/runbooks/executions/:id/approve", runbookHandler.ApproveExecution)
		api.GET("/runbooks/executions/:id", runbookHandler.GetExecution)
		api.GET("/runbooks/executions", runbookHandler.ListExecutions)
	}

	// 6. Run Server
	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	logger.Log.Info("Starting server", zap.String("addr", addr))
	if err := r.Run(addr); err != nil {
		logger.Log.Fatal("Server failed to start", zap.Error(err))
	}
}
