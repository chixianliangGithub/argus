package model

import (
	"context"
	"log"

	"github.com/argus-monitoring/argus/internal/config"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var RDB *redis.Client

func InitDB() {
	var err error
	dsn := config.AppConfig.Database.DSN
	driver := config.AppConfig.Database.Driver

	switch driver {
	case "mysql":
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "sqlite":
		DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		log.Fatalf("unsupported database driver: %s", driver)
	}

	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.Redis.Addr,
		Password: config.AppConfig.Redis.Password,
		DB:       config.AppConfig.Redis.DB,
	})

	// Ping Redis
	// In dev mode, we might want to skip redis if not available, or just log a warning
	_, err = RDB.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	}
}
