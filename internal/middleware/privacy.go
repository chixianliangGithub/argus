package middleware

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// IP Address (IPv4)
	ipRegex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Phone Number (Simple 11-digit or standard US format)
	phoneRegex = regexp.MustCompile(`\b(?:\+?86)?1[3-9]\d{9}\b|\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`)
	// Email
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	// Bearer Token
	tokenRegex = regexp.MustCompile(`Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*`)
)

// PrivacyMiddleware sanitizes sensitive data in the response body.
func PrivacyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &responseBodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()

		ct := strings.ToLower(strings.TrimSpace(w.Header().Get("Content-Type")))
		if ct == "" {
			ct = strings.ToLower(strings.TrimSpace(c.Writer.Header().Get("Content-Type")))
		}
		if ct != "" && !(strings.Contains(ct, "application/json") || strings.HasPrefix(ct, "text/")) {
			w.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(w.body.Len()))
			w.ResponseWriter.Write(w.body.Bytes())
			return
		}
		responseBody := w.body.String()
		sanitizedBody := Sanitize(responseBody)

		// Update Content-Length
		w.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(sanitizedBody)))
		w.ResponseWriter.Write([]byte(sanitizedBody))
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseBodyWriter) WriteString(s string) (int, error) {
	return r.body.WriteString(s)
}

// Sanitize replaces sensitive data with placeholders.
func Sanitize(input string) string {
	input = ipRegex.ReplaceAllString(input, "[IP]")
	input = phoneRegex.ReplaceAllString(input, "[PHONE]")
	input = emailRegex.ReplaceAllString(input, "[EMAIL]")
	input = tokenRegex.ReplaceAllString(input, "Bearer [TOKEN]")
	return input
}
