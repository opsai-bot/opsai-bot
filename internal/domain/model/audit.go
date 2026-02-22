package model

import "time"

type AuditEventType string

const (
	AuditAlertReceived     AuditEventType = "alert.received"
	AuditAnalysisStarted   AuditEventType = "analysis.started"
	AuditAnalysisCompleted AuditEventType = "analysis.completed"
	AuditActionPlanned     AuditEventType = "action.planned"
	AuditActionApproved    AuditEventType = "action.approved"
	AuditActionRejected    AuditEventType = "action.rejected"
	AuditActionExecuted    AuditEventType = "action.executed"
	AuditActionCompleted   AuditEventType = "action.completed"
	AuditActionFailed      AuditEventType = "action.failed"
	AuditPolicyEvaluated   AuditEventType = "policy.evaluated"
	AuditConversation      AuditEventType = "conversation.message"
)

type AuditLog struct {
	ID          string            `json:"id"`
	EventType   AuditEventType    `json:"event_type"`
	AlertID     string            `json:"alert_id"`
	ActionID    string            `json:"action_id"`
	Actor       string            `json:"actor"`
	Environment string            `json:"environment"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

func NewAuditLog(eventType AuditEventType, alertID, actor, environment, description string) AuditLog {
	return AuditLog{
		ID:          generateID(),
		EventType:   eventType,
		AlertID:     alertID,
		Actor:       actor,
		Environment: environment,
		Description: description,
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now().UTC(),
	}
}

func (a AuditLog) WithActionID(actionID string) AuditLog {
	a.ActionID = actionID
	return a
}

func (a AuditLog) WithMetadata(key, value string) AuditLog {
	meta := make(map[string]string, len(a.Metadata)+1)
	for k, v := range a.Metadata {
		meta[k] = v
	}
	meta[key] = value
	a.Metadata = meta
	return a
}
