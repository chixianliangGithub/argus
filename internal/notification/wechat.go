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

type WeChatSender struct {
	client *http.Client
}

func NewWeChatSender() *WeChatSender {
	return &WeChatSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *WeChatSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		return fmt.Errorf("missing webhook_url in config")
	}

	msg := config["template"]
	if msg == "" {
		msg = fmt.Sprintf("[Argus Alarm] %s\nStatus: %s\nTime: %s", alarm.Content, alarm.Status, alarm.StartsAt.Format(time.RFC3339))
	}
	format := config["format"]
	if format == "" {
		format = "text"
	}
	title := strings.TrimSpace(config["title"])
	level := strings.TrimSpace(config["alert_level"])
	status := strings.TrimSpace(config["alert_status"])

	var payload map[string]interface{}
	if format == "markdown" || title != "" {
		head := ""
		if title != "" {
			color := "comment"
			if status == "resolved" {
				color = "info"
			} else if level == "critical" {
				color = "warning"
			} else if level == "warning" {
				color = "warning"
			} else if level == "info" {
				color = "comment"
			}
			head = fmt.Sprintf(`<font color="%s">%s</font>`, color, title)
		}
		content := msg
		if head != "" {
			content = head + "\n" + msg
		}
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": content,
			},
		}
	} else {
		payload = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": msg,
			},
		}
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
		return fmt.Errorf("failed to send wechat notification, status code: %d", resp.StatusCode)
	}

	return nil
}
