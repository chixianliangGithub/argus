package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

func redactAuditValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, vv := range t {
			kl := strings.ToLower(strings.TrimSpace(k))
			if kl == "password" || kl == "passwd" || kl == "token" || kl == "authorization" || kl == "cookie" || kl == "api_key" || kl == "apikey" || kl == "secret" {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactAuditValue(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, it := range t {
			out = append(out, redactAuditValue(it))
		}
		return out
	default:
		return v
	}
}

func buildAuditDetails(req *http.Request, body []byte) string {
	if req == nil {
		return ""
	}
	ct := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Type")))
	if strings.Contains(ct, "application/json") && len(body) > 0 {
		var v interface{}
		if err := json.Unmarshal(body, &v); err == nil {
			v = redactAuditValue(v)
			if b, err := json.Marshal(v); err == nil {
				body = b
			}
		}
	}
	const limit = 2048
	s := string(body)
	if len(s) > limit {
		s = s[:limit]
	}
	return fmt.Sprintf("Request: %s %s Body: %s", req.Method, req.URL.Path, s)
}

func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only audit write operations
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		// Read request body
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		c.Next()

		// Get User Info
		userID, exists := c.Get("user_id")
		if !exists {
			// Anonymous action (e.g. login) or system action
			return
		}

		var username string
		var uid uint

		// Handle float64 which is default for JSON numbers in claims
		switch v := userID.(type) {
		case float64:
			uid = uint(v)
		case uint:
			uid = v
		case int:
			uid = uint(v)
		}

		if uid > 0 {
			var user model.User
			if err := model.DB.Select("username").First(&user, uid).Error; err == nil {
				username = user.Username
			}
		}

		if username == "" {
			username = "unknown"
		}

		log := model.AuditLog{
			UserID:   uid,
			Username: username,
			Action:   c.Request.Method,
			Resource: c.FullPath(),
			ClientIP: c.ClientIP(),
			Details:  buildAuditDetails(c.Request, requestBody),
		}

		// Async save
		go func(l model.AuditLog) {
			model.DB.Create(&l)
		}(log)
	}
}
