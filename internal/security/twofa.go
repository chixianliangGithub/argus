package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TwoFA 相关约定：
// - secret：Google Authenticator 使用的 TOTP seed（base32，无 padding）
// - secretEnc：secret 使用服务端密钥加密后的密文（base64，包含 nonce + ciphertext）
//
// 说明：这里不引入“恢复码”等高级特性，只实现基础 TOTP 二次验证。

func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToUpper(enc.EncodeToString(buf)), nil
}

func VerifyTOTPCode(secret, code string) bool {
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return false
	}
	ok, _ := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return ok
}

func EncryptStringForStorage(plaintext, keyMaterial string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	keyMaterial = strings.TrimSpace(keyMaterial)
	if plaintext == "" {
		return "", errors.New("plaintext is empty")
	}
	if keyMaterial == "" {
		return "", errors.New("encryption key is empty")
	}

	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func DecryptStringFromStorage(ciphertextB64, keyMaterial string) (string, error) {
	ciphertextB64 = strings.TrimSpace(ciphertextB64)
	keyMaterial = strings.TrimSpace(keyMaterial)
	if ciphertextB64 == "" {
		return "", errors.New("ciphertext is empty")
	}
	if keyMaterial == "" {
		return "", errors.New("encryption key is empty")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce := raw[:ns]
	ct := raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
