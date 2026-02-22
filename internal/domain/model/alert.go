package model

import "time"

type AlertStatus string

const (
	AlertStatusReceived  AlertStatus = "received"
	AlertStatusAnalyzing AlertStatus = "analyzing"
	AlertStatusAnalyzed  AlertStatus = "analyzed"
	AlertStatusActing    AlertStatus = "acting"
	AlertStatusResolved  AlertStatus = "resolved"
	AlertStatusFailed    AlertStatus = "failed"
	AlertStatusDuplicate AlertStatus = "duplicate"
	AlertStatusSilenced  AlertStatus = "silenced"
)

type AlertSource string

const (
	AlertSourceGrafana      AlertSource = "grafana"
	AlertSourceAlertManager AlertSource = "alertmanager"
	AlertSourcePagerDuty    AlertSource = "pagerduty"
	AlertSourceCustom       AlertSource = "custom"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type Alert struct {
	ID          string            `json:"id"`
	ExternalID  string            `json:"external_id"`
	Fingerprint string            `json:"fingerprint"`
	Source      AlertSource       `json:"source"`
	Status      AlertStatus       `json:"status"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Environment string            `json:"environment"`
	Namespace   string            `json:"namespace"`
	Resource    string            `json:"resource"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	RawPayload  string            `json:"raw_payload"`
	ThreadID    string            `json:"thread_id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ResolvedAt  *time.Time        `json:"resolved_at"`
}

// NewAlert creates a new Alert with generated ID and timestamps
func NewAlert(source AlertSource, severity Severity, title, description, environment, namespace string) Alert {
	now := time.Now().UTC()
	return Alert{
		ID:          generateID(),
		Source:      source,
		Status:      AlertStatusReceived,
		Severity:    severity,
		Title:       title,
		Description: description,
		Environment: environment,
		Namespace:   namespace,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// WithFingerprint returns a new Alert with the given fingerprint set
func (a Alert) WithFingerprint(fp string) Alert {
	a.Fingerprint = fp
	return a
}

// WithStatus returns a new Alert with updated status and timestamp (immutable)
func (a Alert) WithStatus(status AlertStatus) Alert {
	a.Status = status
	a.UpdatedAt = time.Now().UTC()
	return a
}

// WithThreadID returns a new Alert with the thread ID set
func (a Alert) WithThreadID(threadID string) Alert {
	a.ThreadID = threadID
	a.UpdatedAt = time.Now().UTC()
	return a
}

// Resolve returns a new Alert marked as resolved
func (a Alert) Resolve() Alert {
	now := time.Now().UTC()
	a.Status = AlertStatusResolved
	a.ResolvedAt = &now
	a.UpdatedAt = now
	return a
}

// IsTerminal returns true if the alert is in a terminal state
func (a Alert) IsTerminal() bool {
	switch a.Status {
	case AlertStatusResolved, AlertStatusFailed, AlertStatusDuplicate, AlertStatusSilenced:
		return true
	}
	return false
}
