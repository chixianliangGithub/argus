package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/argus-monitoring/argus/internal/model"
)

type FeishuSender struct {
	client *http.Client
}

func NewFeishuSender() *FeishuSender {
	return &FeishuSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *FeishuSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		return fmt.Errorf("missing webhook_url in config")
	}

	msg := config["template"]
	if msg == "" {
		msg = fmt.Sprintf("[Argus Alarm] %s\nStatus: %s\nTime: %s", alarm.Content, alarm.Status, alarm.StartsAt.Format(time.RFC3339))
	}
	title := strings.TrimSpace(config["title"])
	if title == "" {
		title = "Argus Alarm"
	}
	level := strings.TrimSpace(config["alert_level"])
	status := strings.TrimSpace(config["alert_status"])
	templateColor := "blue"
	if status == "resolved" {
		templateColor = "green"
	} else if level == "critical" {
		templateColor = "red"
	} else if level == "warning" {
		templateColor = "orange"
	} else if level == "info" {
		templateColor = "blue"
	}

	var payload map[string]interface{}
	payload = map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": map[string]bool{
				"wide_screen_mode": true,
			},
			"header": map[string]interface{}{
				"template": templateColor,
				"title": map[string]string{
					"tag":     "plain_text",
					"content": title,
				},
			},
			"elements": []map[string]interface{}{
				{
					"tag":     "markdown",
					"content": msg,
				},
			},
		},
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

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send feishu notification, status code: %d", resp.StatusCode)
	}

	return nil
}
