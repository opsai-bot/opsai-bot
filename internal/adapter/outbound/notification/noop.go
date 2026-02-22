package notification

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// NoopNotifier is a no-op notifier that logs notifications instead of sending them.
// Used in local development when Slack is not configured.
type NoopNotifier struct {
	logger *slog.Logger
}

// NewNoopNotifier creates a new NoopNotifier.
func NewNoopNotifier(logger *slog.Logger) *NoopNotifier {
	return &NoopNotifier{logger: logger}
}

func (n *NoopNotifier) NotifyAlert(_ context.Context, notification outbound.AlertNotification) (string, error) {
	n.logger.Info("noop: alert notification",
		"alertID", notification.AlertID,
		"title", notification.Title,
		"severity", notification.Severity,
		"environment", notification.Environment,
	)
	return fmt.Sprintf("noop-thread-%s", notification.AlertID), nil
}

func (n *NoopNotifier) NotifyAnalysis(_ context.Context, notification outbound.AnalysisNotification) error {
	n.logger.Info("noop: analysis notification",
		"alertID", notification.AlertID,
		"rootCause", notification.RootCause,
		"confidence", notification.Confidence,
	)
	return nil
}

func (n *NoopNotifier) NotifyAction(_ context.Context, threadID string, action outbound.ActionNotification) error {
	n.logger.Info("noop: action notification",
		"threadID", threadID,
		"description", action.Description,
		"status", action.Status,
	)
	return nil
}

func (n *NoopNotifier) RequestApproval(_ context.Context, req outbound.ApprovalNotification) error {
	n.logger.Info("noop: approval request",
		"actionID", req.ActionID,
		"description", req.Description,
		"environment", req.Environment,
	)
	return nil
}

func (n *NoopNotifier) SendMessage(_ context.Context, threadID string, message string, level outbound.NotificationLevel) error {
	n.logger.Info("noop: message",
		"threadID", threadID,
		"message", message,
		"level", level,
	)
	return nil
}
