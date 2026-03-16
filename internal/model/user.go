package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Username   string `gorm:"uniqueIndex;size:50" json:"username"`
	Password   string `gorm:"size:100" json:"-"` // Hashed password
	Email      string `gorm:"size:100;uniqueIndex" json:"email"`
	Phone      string `gorm:"size:20" json:"phone"`
	Department string `gorm:"size:100" json:"department"`
	Status     string `gorm:"size:20;default:'active'" json:"status"` // active, disabled
	Role       string `gorm:"size:20;default:'user'" json:"role"`

	// Google Authenticator (TOTP) 二次验证：
	// - TwoFAEnabled：是否启用二次验证
	// - TwoFASecretEnc：加密后的 TOTP secret（base32 明文经过服务端密钥加密后落库）
	TwoFAEnabled   bool   `gorm:"default:false" json:"two_fa_enabled"`
	TwoFASecretEnc string `gorm:"type:text" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) Create() error {
	return DB.Create(u).Error
}

func GetUserByUsername(username string) (*User, error) {
	var user User
	err := DB.Where("username = ?", username).First(&user).Error
	return &user, err
}
