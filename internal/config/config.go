package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	AI       AIConfig       `mapstructure:"ai"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AIConfig struct {
	Provider       string `mapstructure:"provider"`
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type SecurityConfig struct {
	JWTSecret               string `mapstructure:"jwt_secret"`
	TwoFASecretKey          string `mapstructure:"two_fa_secret_key"`
	PrivacySanitizeResponse bool   `mapstructure:"privacy_sanitize_response"`
}

var AppConfig *Config
var JWTSecretEphemeral bool

func LoadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("ARGUS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	if strings.TrimSpace(AppConfig.Security.JWTSecret) == "" {
		if strings.EqualFold(strings.TrimSpace(AppConfig.Server.Mode), "release") {
			log.Fatalf("security.jwt_secret is required in release mode")
		}
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		AppConfig.Security.JWTSecret = hex.EncodeToString(buf)
		JWTSecretEphemeral = true
		log.Printf("Warning: security.jwt_secret is empty, using ephemeral secret in non-release mode")
	}
}
