package model

import (
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func Migrate() {
	err := DB.AutoMigrate(
		&User{},
		&SystemSetting{},
		&DataSource{},
		&Team{},
		&DataName{},
		&AlertRule{},
		&Alarm{},
		&AlertLog{},
		&AuditLog{},
		&NotificationChannel{},
		&MessageTemplate{},
		&RCAReport{},
		&Runbook{},
		&RunbookExecution{},
		&SilenceRule{},
		&EscalationPolicy{},
		&RoutingRule{},
		&InhibitRule{},
		&AlertEvalRecord{},
		&Incident{},
		&IncidentAlarm{},
		&IncidentActivity{},
		&IncidentAIInsight{},
		&AIAudit{},
		&AIPromptVersion{},
	)
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	log.Println("Database migration completed successfully")
}

func Seed() {
	var count int64
	DB.Model(&User{}).Count(&count)
	if count == 0 {
		username := strings.TrimSpace(os.Getenv("ARGUS_INIT_ADMIN_USERNAME"))
		password := strings.TrimSpace(os.Getenv("ARGUS_INIT_ADMIN_PASSWORD"))
		if username == "" || password == "" {
			log.Printf("No users found. Set ARGUS_INIT_ADMIN_USERNAME and ARGUS_INIT_ADMIN_PASSWORD to bootstrap an admin user.")
			return
		}
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		admin := User{
			Username: username,
			Password: string(hashedPassword),
			Role:     "admin",
			Status:   "active",
		}
		if err := DB.Create(&admin).Error; err != nil {
			log.Printf("Failed to create admin user: %v", err)
		} else {
			log.Printf("Bootstrapped admin user: %s", username)
		}
	}
}
