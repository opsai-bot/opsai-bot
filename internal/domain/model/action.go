package model

import "time"

type ActionStatus string

const (
	ActionStatusPlanned    ActionStatus = "planned"
	ActionStatusPending    ActionStatus = "pending"
	ActionStatusApproved   ActionStatus = "approved"
	ActionStatusRejected   ActionStatus = "rejected"
	ActionStatusExecuting  ActionStatus = "executing"
	ActionStatusCompleted  ActionStatus = "completed"
	ActionStatusFailed     ActionStatus = "failed"
	ActionStatusRolledBack ActionStatus = "rolled_back"
)

type ActionType string

const (
	ActionTypeKubectl   ActionType = "kubectl"
	ActionTypeRestart   ActionType = "restart"
	ActionTypeScale     ActionType = "scale"
	ActionTypeDeletePod ActionType = "delete_pod"
	ActionTypeExec      ActionType = "exec"
	ActionTypeManual    ActionType = "manual"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Action struct {
	ID             string            `json:"id"`
	AnalysisID     string            `json:"analysis_id"`
	AlertID        string            `json:"alert_id"`
	Type           ActionType        `json:"type"`
	Status         ActionStatus      `json:"status"`
	Description    string            `json:"description"`
	Commands       []string          `json:"commands"`
	Risk           RiskLevel         `json:"risk"`
	Reversible     bool              `json:"reversible"`
	Output         string            `json:"output"`
	ErrorMessage   string            `json:"error_message"`
	ApprovedBy     string            `json:"approved_by"`
	ApprovedAt     *time.Time        `json:"approved_at"`
	ExecutedAt     *time.Time        `json:"executed_at"`
	CompletedAt    *time.Time        `json:"completed_at"`
	Environment    string            `json:"environment"`
	Namespace      string            `json:"namespace"`
	TargetResource string            `json:"target_resource"`
	Metadata       map[string]string `json:"metadata"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func NewAction(analysisID, alertID string, actionType ActionType, description string, commands []string, risk RiskLevel) Action {
	now := time.Now().UTC()
	return Action{
		ID:          generateID(),
		AnalysisID:  analysisID,
		AlertID:     alertID,
		Type:        actionType,
		Status:      ActionStatusPlanned,
		Description: description,
		Commands:    commands,
		Risk:        risk,
		Metadata:    make(map[string]string),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (a Action) WithStatus(status ActionStatus) Action {
	a.Status = status
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) Approve(approvedBy string) Action {
	now := time.Now().UTC()
	a.Status = ActionStatusApproved
	a.ApprovedBy = approvedBy
	a.ApprovedAt = &now
	a.UpdatedAt = now
	return a
}

func (a Action) Reject(rejectedBy string) Action {
	a.Status = ActionStatusRejected
	a.ApprovedBy = rejectedBy
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) Complete(output string) Action {
	now := time.Now().UTC()
	a.Status = ActionStatusCompleted
	a.Output = output
	a.CompletedAt = &now
	a.UpdatedAt = now
	return a
}

func (a Action) WithExecutedAt(t time.Time) Action {
	a.Status = ActionStatusExecuting
	a.ExecutedAt = &t
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) Fail(errMsg string) Action {
	a.Status = ActionStatusFailed
	a.ErrorMessage = errMsg
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) WithEnvironment(env string) Action {
	a.Environment = env
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) WithNamespace(ns string) Action {
	a.Namespace = ns
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) WithReversible(reversible bool) Action {
	a.Reversible = reversible
	a.UpdatedAt = time.Now().UTC()
	return a
}

func (a Action) NeedsApproval() bool {
	return a.Status == ActionStatusPending
}

func (a Action) IsTerminal() bool {
	switch a.Status {
	case ActionStatusCompleted, ActionStatusFailed, ActionStatusRejected, ActionStatusRolledBack:
		return true
	}
	return false
}

func (a Action) RiskOrder() int {
	switch a.Risk {
	case RiskLow:
		return 0
	case RiskMedium:
		return 1
	case RiskHigh:
		return 2
	case RiskCritical:
		return 3
	default:
		return -1
	}
}
