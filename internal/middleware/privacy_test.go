package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "My IP is 192.168.1.1",
			expected: "My IP is [IP]",
		},
		{
			input:    "Call me at 13800138000",
			expected: "Call me at [PHONE]",
		},
		{
			input:    "Call me at 123-456-7890",
			expected: "Call me at [PHONE]",
		},
		{
			input:    "My email is test@example.com",
			expected: "My email is [EMAIL]",
		},
		{
			input:    "Authorization: Bearer abcdef123456",
			expected: "Authorization: Bearer [TOKEN]",
		},
		{
			input:    "No sensitive data here",
			expected: "No sensitive data here",
		},
	}

	for _, test := range tests {
		result := Sanitize(test.input)
		if result != test.expected {
			t.Errorf("expected %q, got %q", test.expected, result)
		}
	}
}

func TestPrivacyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PrivacyMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "My IP is 10.0.0.1",
			"phone":   "13912345678",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "[IP]") {
		t.Error("expected body to contain [IP]")
	}
	if !strings.Contains(body, "[PHONE]") {
		t.Error("expected body to contain [PHONE]")
	}
	if strings.Contains(body, "10.0.0.1") {
		t.Error("expected body not to contain IP address")
	}
	if strings.Contains(body, "13912345678") {
		t.Error("expected body not to contain phone number")
	}
}
