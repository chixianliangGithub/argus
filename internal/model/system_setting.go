package model

import (
	"strings"
	"time"
)

type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:200" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	SettingKeySecurityTwoFAEnabled = "security.two_fa_enabled"
)

func GetSetting(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || DB == nil {
		return "", false
	}
	var s SystemSetting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		return "", false
	}
	return s.Value, true
}

func GetBoolSetting(key string, def bool) bool {
	v, ok := GetSetting(key)
	if !ok {
		return def
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "1" || v == "true" || v == "yes" || v == "on" {
		return true
	}
	if v == "0" || v == "false" || v == "no" || v == "off" {
		return false
	}
	return def
}

func SetSetting(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" || DB == nil {
		return nil
	}
	s := SystemSetting{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}
	return DB.Save(&s).Error
}
