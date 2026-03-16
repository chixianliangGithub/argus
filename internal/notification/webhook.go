package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type WebhookSender struct {
	client *http.Client
}

func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func webhookDebugEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ARGUS_DEBUG_WEBHOOK")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (s *WebhookSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		webhookURL = config["url"]
	}
	if webhookURL == "" {
		return fmt.Errorf("missing url in config")
	}

	payload := map[string]interface{}{
		"alarm":     alarm,
		"timestamp": time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers if needed
	if token, ok := config["token"]; ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if webhookDebugEnabled() && model.DB != nil && alarm != nil {
			_ = model.DB.Create(&model.AlertLog{
				AlarmID:     alarm.ID,
				AlertRuleID: alarm.AlertRuleID,
				Status:      "webhook_debug",
				Message:     fmt.Sprintf("url=%s err=%v", webhookURL, err),
				TriggeredAt: time.Now(),
			}).Error
		}
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	respMsg := string(bytes.TrimSpace(respBody))
	if webhookDebugEnabled() && model.DB != nil && alarm != nil {
		msg := fmt.Sprintf("url=%s status=%d", webhookURL, resp.StatusCode)
		if respMsg != "" {
			msg += " body=" + respMsg
		}
		_ = model.DB.Create(&model.AlertLog{
			AlarmID:     alarm.ID,
			AlertRuleID: alarm.AlertRuleID,
			Status:      "webhook_debug",
			Message:     msg,
			TriggeredAt: time.Now(),
		}).Error
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if respMsg != "" {
			return fmt.Errorf("webhook failed with status=%d body=%s", resp.StatusCode, respMsg)
		}
		return fmt.Errorf("webhook failed with status=%d", resp.StatusCode)
	}

	return nil
}
