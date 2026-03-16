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

type DingTalkSender struct {
	client *http.Client
}

func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *DingTalkSender) Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error {
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
	if title == "" {
		title = "Argus Alarm"
	}
	status := strings.TrimSpace(config["alert_status"])
	level := strings.TrimSpace(config["level"])
	if level == "" {
		level = strings.TrimSpace(config["alert_level"])
	}

	atStr := strings.TrimSpace(config["at_mobiles"])
	var atMobiles []string
	if atStr != "" {
		for _, m := range strings.Split(atStr, ",") {
			mm := strings.TrimSpace(m)
			if mm != "" {
				atMobiles = append(atMobiles, mm)
			}
		}
	}
	if len(atMobiles) > 0 {
		for _, m := range atMobiles {
			msg += " @" + m
		}
	}

	var payload map[string]interface{}
	if format == "markdown" {
		text := msg
		if title != "" {
			header := title
			if status == "resolved" {
				header = fmt.Sprintf("<font color=#52c41a>%s</font>", header)
			} else if level != "" {
				if level == "critical" || level == "严重" {
					header = fmt.Sprintf("<font color=#a8071a>%s</font>", header)
				} else if level == "warning" || level == "警告" {
					header = fmt.Sprintf("<font color=#faad14>%s</font>", header)
				} else if level == "info" || level == "信息" {
					header = fmt.Sprintf("<font color=#1677ff>%s</font>", header)
				}
			}
			text = "### " + header + "\n\n" + msg
		}
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  text,
			},
		}
	} else {
		content := msg
		if title != "" {
			content = title + "\n" + msg
		}
		payload = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
		}
	}
	if len(atMobiles) > 0 {
		payload["at"] = map[string]interface{}{
			"atMobiles": atMobiles,
			"isAtAll":   false,
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
		return fmt.Errorf("failed to send dingtalk notification, status code: %d", resp.StatusCode)
	}

	return nil
}
