package outbound

import "context"

type NotificationLevel string

const (
	NotificationInfo     NotificationLevel = "info"
	NotificationWarning  NotificationLevel = "warning"
	NotificationCritical NotificationLevel = "critical"
	NotificationResolved NotificationLevel = "resolved"
)

type AlertNotification struct {
	AlertID     string
	Title       string
	Summary     string
	Severity    string
	Environment string
	Source      string
	Labels      map[string]string
	ThreadID    string
}

type AnalysisNotification struct {
	AlertID    string
	ThreadID   string
	RootCause  string
	Confidence float64
	Severity   string
	Actions    []ActionNotification
	Explanation string
}

type ActionNotification struct {
	Description string
	Command     string
	Status      string
	Output      string
	Risk        string
}

type ApprovalNotification struct {
	AlertID     string
	ThreadID    string
	ActionID    string
	Description string
	Commands    []string
	Risk        string
	Environment string
	RequestedBy string
}

// Notifier sends notifications to users via messaging platforms.
type Notifier interface {
	NotifyAlert(ctx context.Context, notification AlertNotification) (threadID string, err error)
	NotifyAnalysis(ctx context.Context, notification AnalysisNotification) error
	NotifyAction(ctx context.Context, threadID string, action ActionNotification) error
	RequestApproval(ctx context.Context, req ApprovalNotification) error
	SendMessage(ctx context.Context, threadID string, message string, level NotificationLevel) error
}
