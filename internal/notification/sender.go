package notification

import (
	"context"

	"github.com/argus-monitoring/argus/internal/model"
)

// AlarmSender defines the interface for sending alarm notifications.
type AlarmSender interface {
	// Send sends an alarm notification.
	// The config parameter contains channel-specific configuration (e.g., webhook URL, token).
	Send(ctx context.Context, alarm *model.Alarm, config map[string]string) error
}
