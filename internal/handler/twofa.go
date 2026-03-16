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

type TwoFAHandler struct{}

func twoFAKeyMaterial() (string, bool) {
	k := strings.TrimSpace(config.AppConfig.Security.TwoFASecretKey)
	if k != "" {
		return k, true
	}
	k = strings.TrimSpace(config.AppConfig.Security.JWTSecret)
	if k == "" {
		return "", false
	}
	if config.JWTSecretEphemeral {
		return "", false
	}
	return k, true
}

type twoFASetupClaims struct {
	UserID  uint   `json:"user_id"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

func signJWT(secret string, claims jwt.Claims) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

func parseTwoFASetupToken(tokenString, secret string) (*twoFASetupClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenString, &twoFASetupClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || tok == nil {
		return nil, err
	}
	if c, ok := tok.Claims.(*twoFASetupClaims); ok && tok.Valid {
		return c, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func (h *TwoFAHandler) Status(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	uid, err := parseUint(raw)
	if err != nil || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var user model.User
	if err := model.DB.Select("id, two_fa_enabled").First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	globalEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	c.JSON(http.StatusOK, gin.H{"enabled": user.TwoFAEnabled, "global_enabled": globalEnabled})
}

func (h *TwoFAHandler) SetupConfirm(c *gin.Context) {
	var req struct {
		SetupToken string `json:"setup_token" binding:"required"`
		Code       string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	secret := strings.TrimSpace(config.AppConfig.Security.JWTSecret)
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not configured"})
		return
	}

	claims, err := parseTwoFASetupToken(strings.TrimSpace(req.SetupToken), secret)
	if err != nil || claims == nil || claims.UserID == 0 || strings.TrimSpace(claims.Purpose) != "2fa_setup" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_setup_token"})
		return
	}

	globalEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	if !globalEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "global_2fa_disabled"})
		return
	}

	keyMaterial, ok := twoFAKeyMaterial()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "twofa_key_not_configured"})
		return
	}

	var user model.User
	if err := model.DB.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if strings.TrimSpace(user.TwoFASecretEnc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2fa not initialized"})
		return
	}
	seed, err := security.DecryptStringFromStorage(user.TwoFASecretEnc, keyMaterial)
	if err != nil {
		_ = model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"two_fa_enabled":    false,
			"two_fa_secret_enc": "",
		}).Error
		c.JSON(http.StatusUnauthorized, gin.H{"error": "2fa_secret_invalid"})
		return
	}
	if !security.VerifyTOTPCode(seed, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_2fa_code"})
		return
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Update("two_fa_enabled", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2fa"})
		return
	}

	tokenString, err := signJWT(secret, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (h *TwoFAHandler) Setup(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	uid, err := parseUint(raw)
	if err != nil || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	globalEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	if !globalEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "global_2fa_disabled"})
		return
	}

	keyMaterial, ok := twoFAKeyMaterial()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "twofa_key_not_configured"})
		return
	}

	var user model.User
	if err := model.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.TwoFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2fa already enabled"})
		return
	}

	secret, err := security.GenerateTOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
		return
	}
	secretEnc, err := security.EncryptStringForStorage(secret, keyMaterial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt secret"})
		return
	}

	issuer := "ArgusMonitor"
	account := strings.TrimSpace(user.Username)
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	otpauthURL := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		label,
		url.QueryEscape(secret),
		url.QueryEscape(issuer),
	)
	png, err := qrcode.Encode(otpauthURL, qrcode.High, 320)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate qrcode"})
		return
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"two_fa_enabled":    false,
		"two_fa_secret_enc": secretEnc,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save 2fa secret"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"issuer":        issuer,
		"account":       account,
		"secret":        secret,
		"otpauth_url":   otpauthURL,
		"qr_png_base64": base64.StdEncoding.EncodeToString(png),
	})
}

func (h *TwoFAHandler) Enable(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	uid, err := parseUint(raw)
	if err != nil || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	globalEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	if !globalEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "global_2fa_disabled"})
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keyMaterial, ok := twoFAKeyMaterial()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "twofa_key_not_configured"})
		return
	}

	var user model.User
	if err := model.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if strings.TrimSpace(user.TwoFASecretEnc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2fa not initialized"})
		return
	}
	secret, err := security.DecryptStringFromStorage(user.TwoFASecretEnc, keyMaterial)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2fa_secret_invalid"})
		return
	}
	if !security.VerifyTOTPCode(secret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_2fa_code"})
		return
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Update("two_fa_enabled", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2fa"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true})
}

func (h *TwoFAHandler) Disable(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	uid, err := parseUint(raw)
	if err != nil || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	globalEnabled := model.GetBoolSetting(model.SettingKeySecurityTwoFAEnabled, false)
	if globalEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "global_2fa_enforced"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	keyMaterial, ok := twoFAKeyMaterial()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "twofa_key_not_configured"})
		return
	}

	var user model.User
	if err := model.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.TwoFAEnabled {
		secret, err := security.DecryptStringFromStorage(user.TwoFASecretEnc, keyMaterial)
		if err == nil {
			if strings.TrimSpace(req.Code) == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "2fa_required"})
				return
			}
			if !security.VerifyTOTPCode(secret, req.Code) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_2fa_code"})
				return
			}
		}
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"two_fa_enabled":    false,
		"two_fa_secret_enc": "",
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2fa"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": false})
}
