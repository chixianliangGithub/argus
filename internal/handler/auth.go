package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/config"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/argus-monitoring/argus/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{}

func (h *AuthHandler) Register(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	user := model.User{Username: input.Username, Password: string(hashedPassword)}
	if err := user.Create(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User created successfully"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		TOTPCode string `json:"totp_code"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := model.GetUserByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.Status == "disabled" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
		return
	}

	// Create JWT
	secret := strings.TrimSpace(config.AppConfig.Security.JWTSecret)
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not configured"})
		return
	}

	// 启用 2FA 的账号必须提供 Google Authenticator 验证码
	globalTwoFAEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	if globalTwoFAEnabled {
		keyMaterial, ok := twoFAKeyMaterial()
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "twofa_key_not_configured"})
			return
		}
		if user.TwoFAEnabled {
			if strings.TrimSpace(input.TOTPCode) == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "2fa_required"})
				return
			}
			seed, err := security.DecryptStringFromStorage(user.TwoFASecretEnc, keyMaterial)
			if err != nil {
				_ = model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
					"two_fa_enabled":    false,
					"two_fa_secret_enc": "",
				}).Error
				c.JSON(http.StatusUnauthorized, gin.H{"error": "2fa_setup_required"})
				return
			}
			if !security.VerifyTOTPCode(seed, input.TOTPCode) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_2fa_code"})
				return
			}
		} else {
			seed := ""
			if strings.TrimSpace(user.TwoFASecretEnc) != "" {
				if v, err := security.DecryptStringFromStorage(user.TwoFASecretEnc, keyMaterial); err == nil && strings.TrimSpace(v) != "" {
					seed = strings.TrimSpace(v)
				}
			}
			if seed == "" {
				newSeed, err := security.GenerateTOTPSecret()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
					return
				}
				enc, err := security.EncryptStringForStorage(newSeed, keyMaterial)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt secret"})
					return
				}
				if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
					"two_fa_enabled":    false,
					"two_fa_secret_enc": enc,
				}).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save 2fa secret"})
					return
				}
				seed = newSeed
			}

			issuer := "ArgusMonitor"
			account := strings.TrimSpace(user.Username)
			label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
			otpauthURL := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
				label,
				url.QueryEscape(seed),
				url.QueryEscape(issuer),
			)
			png, err := qrcode.Encode(otpauthURL, qrcode.High, 320)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate qrcode"})
				return
			}

			setupToken, err := signJWT(secret, &twoFASetupClaims{
				UserID:  user.ID,
				Purpose: "2fa_setup",
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
				return
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"error":          "2fa_setup_required",
				"setup_token":    setupToken,
				"issuer":         issuer,
				"account":        account,
				"secret":         seed,
				"otpauth_url":    otpauthURL,
				"qr_png_base64":  base64.StdEncoding.EncodeToString(png),
				"global_enabled": true,
			})
			return
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}
