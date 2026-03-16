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

type SlackSender struct {
	client *http.Client
}

func NewSlackSender() *SlackSender {
	return &SlackSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		return fmt.Errorf("missing webhook_url in config")
	}

	msg := config["template"]
	if msg == "" {
		msg = fmt.Sprintf("*[Argus Alarm]*\n%s\n*Status*: %s\n*Time*: %s", alarm.Content, alarm.Status, alarm.StartsAt.Format(time.RFC3339))
	}
	title := strings.TrimSpace(config["title"])
	if title == "" {
		title = "Argus Alarm"
	}
	level := strings.TrimSpace(config["alert_level"])
	status := strings.TrimSpace(config["alert_status"])
	color := "#1677ff"
	if status == "resolved" {
		color = "#52c41a"
	} else if level == "critical" {
		color = "#a8071a"
	} else if level == "warning" {
		color = "#faad14"
	} else if level == "info" {
		color = "#1677ff"
	}

	payload := map[string]interface{}{
		"text": title,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"blocks": []map[string]interface{}{
					{
						"type": "header",
						"text": map[string]string{
							"type": "plain_text",
							"text": title,
						},
					},
					{
						"type": "section",
						"text": map[string]string{
							"type": "mrkdwn",
							"text": msg,
						},
					},
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
		return fmt.Errorf("failed to send slack notification, status code: %d", resp.StatusCode)
	}

	return nil
}
