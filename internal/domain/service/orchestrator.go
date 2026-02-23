package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// Repositories groups all repository dependencies for the orchestrator.
type Repositories struct {
	Alerts        outbound.AlertRepository
	Analyses      outbound.AnalysisRepository
	Actions       outbound.ActionRepository
	Audits        outbound.AuditRepository
	Conversations outbound.ConversationRepository
}

// Orchestrator ties the analysis, planning and policy sub-services together and
// implements both AlertReceiverPort and InteractionPort.
type Orchestrator struct {
	analyzer   *Analyzer
	planner    *ActionPlanner
	policyEval *PolicyEvaluator
	notifier   outbound.Notifier
	k8s        outbound.K8sExecutor
	repos      Repositories
	logger     *slog.Logger
}

// NewOrchestrator creates an Orchestrator with all required dependencies.
func NewOrchestrator(
	analyzer *Analyzer,
	planner *ActionPlanner,
	policyEval *PolicyEvaluator,
	notifier outbound.Notifier,
	k8s outbound.K8sExecutor,
	repos Repositories,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		analyzer:   analyzer,
		planner:    planner,
		policyEval: policyEval,
		notifier:   notifier,
		k8s:        k8s,
		repos:      repos,
		logger:     logger,
	}
}

// logAudit creates an audit log, logging on failure instead of silently discarding.
func (o *Orchestrator) logAudit(ctx context.Context, log model.AuditLog) {
	if err := o.repos.Audits.Create(ctx, log); err != nil {
		o.logger.Error("failed to write audit log",
			"error", err,
			"event_type", string(log.EventType),
			"alert_id", log.AlertID,
		)
	}
}

// Ensure Orchestrator satisfies the inbound ports at compile time.
var _ inbound.AlertReceiverPort = (*Orchestrator)(nil)
var _ inbound.InteractionPort = (*Orchestrator)(nil)

// ReceiveAlert implements inbound.AlertReceiverPort for a single alert.
func (o *Orchestrator) ReceiveAlert(ctx context.Context, alert model.Alert) error {
	return o.HandleAlert(ctx, alert)
}

// ReceiveAlerts implements inbound.AlertReceiverPort for a batch of alerts.
func (o *Orchestrator) ReceiveAlerts(ctx context.Context, alerts []model.Alert) error {
	var errs []string
	for _, alert := range alerts {
		if err := o.HandleAlert(ctx, alert); err != nil {
			errs = append(errs, fmt.Sprintf("alert %s: %v", alert.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("batch receive errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// HandleMessage implements inbound.InteractionPort.
func (o *Orchestrator) HandleMessage(ctx context.Context, req inbound.MessageRequest) (inbound.MessageResponse, error) {
	thread, err := o.repos.Conversations.GetByThreadID(ctx, req.ThreadID)
	if err != nil {
		// Start a new conversation thread if none exists.
		thread = model.NewConversationThread(req.AlertID, req.ThreadID, req.ChannelID)
		thread, err = o.repos.Conversations.Create(ctx, thread)
		if err != nil {
			return inbound.MessageResponse{}, fmt.Errorf("create conversation thread: %w", err)
		}
	}

	// Append the user message.
	thread = thread.AddMessage(model.MessageRoleUser, req.Text, req.UserID)

	resp, err := o.analyzer.HandleConversation(ctx, thread, req.Text, req.AlertID)
	if err != nil {
		return inbound.MessageResponse{}, fmt.Errorf("handle conversation: %w", err)
	}

	// Persist assistant reply.
	thread = thread.AddMessage(model.MessageRoleAssistant, resp.Reply, "system")
	if _, err = o.repos.Conversations.Update(ctx, thread); err != nil {
		return inbound.MessageResponse{}, fmt.Errorf("update conversation thread: %w", err)
	}

	// Audit log.
	auditLog := model.NewAuditLog(
		model.AuditConversation,
		req.AlertID,
		req.UserID,
		"",
		fmt.Sprintf("user %s sent message in thread %s", req.UserName, req.ThreadID),
	)
	o.logAudit(ctx, auditLog)

	suggested := make([]inbound.SuggestedActionInfo, 0, len(resp.SuggestedActions))
	for _, sa := range resp.SuggestedActions {
		suggested = append(suggested, inbound.SuggestedActionInfo{
			Description: sa.Description,
			Commands:    sa.Commands,
			Risk:        sa.Risk,
		})
	}

	return inbound.MessageResponse{
		Text:             resp.Reply,
		SuggestedActions: suggested,
		NeedsApproval:    resp.NeedsApproval,
	}, nil
}

// HandleApproval implements inbound.InteractionPort.
func (o *Orchestrator) HandleApproval(ctx context.Context, req inbound.ApprovalRequest) error {
	return o.processApproval(ctx, req.ActionID, req.Approved, req.ApprovedBy, req.Reason)
}

// HandleAlert runs the full alert processing pipeline.
func (o *Orchestrator) HandleAlert(ctx context.Context, alert model.Alert) error {
	// 1. Persist alert.
	saved, err := o.repos.Alerts.Create(ctx, alert)
	if err != nil {
		return fmt.Errorf("save alert: %w", err)
	}
	alert = saved

	// 2. Notify Slack.
	threadID, err := o.notifier.NotifyAlert(ctx, outbound.AlertNotification{
		AlertID:     alert.ID,
		Title:       alert.Title,
		Summary:     alert.Description,
		Severity:    string(alert.Severity),
		Environment: alert.Environment,
		Source:      string(alert.Source),
		Labels:      alert.Labels,
	})
	if err != nil {
		return fmt.Errorf("notify alert: %w", err)
	}
	alert = alert.WithThreadID(threadID)
	if _, err = o.repos.Alerts.Update(ctx, alert); err != nil {
		return fmt.Errorf("update alert thread ID: %w", err)
	}

	// Audit: received.
	o.logAudit(ctx, model.NewAuditLog(
		model.AuditAlertReceived,
		alert.ID,
		"system",
		alert.Environment,
		fmt.Sprintf("alert received from %s", alert.Source),
	))

	// 3. Update status to analyzing.
	alert = alert.WithStatus(model.AlertStatusAnalyzing)
	if _, err = o.repos.Alerts.Update(ctx, alert); err != nil {
		return fmt.Errorf("update alert status: %w", err)
	}

	// Audit: analysis started.
	o.logAudit(ctx, model.NewAuditLog(
		model.AuditAnalysisStarted,
		alert.ID,
		"system",
		alert.Environment,
		"analysis started",
	))

	// 4. Analyze.
	analysis, suggestions, err := o.analyzer.AnalyzeAlert(ctx, alert)
	if err != nil {
		alert = alert.WithStatus(model.AlertStatusFailed)
		_, _ = o.repos.Alerts.Update(ctx, alert)
		return fmt.Errorf("analyze alert: %w", err)
	}

	analysis, err = o.repos.Analyses.Create(ctx, analysis)
	if err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}

	// Audit: analysis completed.
	o.logAudit(ctx, model.NewAuditLog(
		model.AuditAnalysisCompleted,
		alert.ID,
		"system",
		alert.Environment,
		fmt.Sprintf("root cause: %s (confidence %.2f)", analysis.RootCause, analysis.Confidence),
	))

	// 5. Plan actions.
	actions, err := o.planner.Plan(ctx, analysis.ID, alert.ID, suggestions, alert.Environment, alert.Namespace)
	if err != nil {
		return fmt.Errorf("plan actions: %w", err)
	}

	// Notify analysis result.
	actionNotifs := make([]outbound.ActionNotification, 0, len(actions))
	for _, a := range actions {
		actionNotifs = append(actionNotifs, outbound.ActionNotification{
			Description: a.Description,
			Command:     strings.Join(a.Commands, " && "),
			Status:      string(a.Status),
			Risk:        string(a.Risk),
		})
	}
	if notifyErr := o.notifier.NotifyAnalysis(ctx, outbound.AnalysisNotification{
		AlertID:     alert.ID,
		ThreadID:    threadID,
		RootCause:   analysis.RootCause,
		Confidence:  analysis.Confidence,
		Severity:    string(analysis.Severity),
		Actions:     actionNotifs,
		Explanation: analysis.Explanation,
	}); notifyErr != nil {
		o.logger.Error("failed to notify analysis", "error", notifyErr, "alert_id", alert.ID)
	}

	// 6. For each action: evaluate policy and execute or request approval.
	alert = alert.WithStatus(model.AlertStatusActing)
	if _, updateErr := o.repos.Alerts.Update(ctx, alert); updateErr != nil {
		o.logger.Error("failed to update alert status", "error", updateErr, "alert_id", alert.ID)
	}

	allResolved := true
	for _, action := range actions {
		decision, evalErr := o.policyEval.Evaluate(ctx, alert.Environment, action)
		if evalErr != nil {
			allResolved = false
			continue
		}

		// Audit: policy evaluated.
		o.logAudit(ctx, model.NewAuditLog(
			model.AuditPolicyEvaluated,
			alert.ID,
			"system",
			alert.Environment,
			fmt.Sprintf("policy decision for action %s: allowed=%v needsApproval=%v", action.Description, decision.Allowed, decision.NeedsApproval),
		).WithActionID(action.ID))

		if !decision.Allowed {
			action = action.WithStatus(model.ActionStatusRejected)
			action, createErr := o.repos.Actions.Create(ctx, action)
			if createErr != nil {
				o.logger.Error("failed to create action", "error", createErr, "alert_id", alert.ID)
			}
			_ = action
			continue
		}

		if decision.NeedsApproval {
			action = action.WithStatus(model.ActionStatusPending)
			action, err = o.repos.Actions.Create(ctx, action)
			if err != nil {
				allResolved = false
				continue
			}

			if notifyErr := o.notifier.RequestApproval(ctx, outbound.ApprovalNotification{
				AlertID:     alert.ID,
				ThreadID:    threadID,
				ActionID:    action.ID,
				Description: action.Description,
				Commands:    action.Commands,
				Risk:        string(action.Risk),
				Environment: alert.Environment,
				RequestedBy: "system",
			}); notifyErr != nil {
				o.logger.Error("failed to request approval", "error", notifyErr, "alert_id", alert.ID, "action_id", action.ID)
			}
			allResolved = false
			continue
		}

		// Auto-execute.
		action, createErr := o.repos.Actions.Create(ctx, action)
		if createErr != nil {
			o.logger.Error("failed to create action", "error", createErr, "alert_id", alert.ID)
			allResolved = false
			continue
		}
		executedAction, execErr := o.executeAction(ctx, action)
		if execErr != nil {
			allResolved = false
		}
		if updateErr := o.repos.Actions.UpdateStatus(ctx, executedAction.ID, executedAction.Status, executedAction.Output); updateErr != nil {
			o.logger.Error("failed to update action status", "error", updateErr, "action_id", executedAction.ID)
		}
	}

	// 7. Update final alert status.
	if allResolved {
		alert = alert.WithStatus(model.AlertStatusResolved)
	}
	if _, updateErr := o.repos.Alerts.Update(ctx, alert); updateErr != nil {
		o.logger.Error("failed to update alert status", "error", updateErr, "alert_id", alert.ID)
	}

	return nil
}

// processApproval handles the approval or rejection of a pending action.
func (o *Orchestrator) processApproval(ctx context.Context, actionID string, approved bool, approvedBy, reason string) error {
	action, err := o.repos.Actions.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("get action %s: %w", actionID, err)
	}

	if approved {
		action = action.Approve(approvedBy)
		if err = o.repos.Actions.UpdateStatus(ctx, action.ID, action.Status, ""); err != nil {
			return fmt.Errorf("update action status: %w", err)
		}

		o.logAudit(ctx, model.NewAuditLog(
			model.AuditActionApproved,
			action.AlertID,
			approvedBy,
			action.Environment,
			fmt.Sprintf("action %q approved: %s", action.Description, reason),
		).WithActionID(actionID))

		executedAction, execErr := o.executeAction(ctx, action)
		if execErr != nil {
			return fmt.Errorf("execute action after approval: %w", execErr)
		}
		return o.repos.Actions.UpdateStatus(ctx, executedAction.ID, executedAction.Status, executedAction.Output)
	}

	action = action.Reject(approvedBy)
	if err = o.repos.Actions.UpdateStatus(ctx, action.ID, action.Status, ""); err != nil {
		return fmt.Errorf("update action status: %w", err)
	}

	o.logAudit(ctx, model.NewAuditLog(
		model.AuditActionRejected,
		action.AlertID,
		approvedBy,
		action.Environment,
		fmt.Sprintf("action %q rejected: %s", action.Description, reason),
	).WithActionID(actionID))

	return nil
}

// executeAction runs all commands for an action and returns the updated action.
func (o *Orchestrator) executeAction(ctx context.Context, action model.Action) (model.Action, error) {
	action = action.WithExecutedAt(time.Now().UTC())

	var outputs []string
	var execErr error

	for _, cmd := range action.Commands {
		parts := strings.Fields(cmd)
		result, err := o.k8s.Exec(ctx, outbound.ExecRequest{
			Namespace: action.Namespace,
			Command:   parts,
			Timeout:   60,
		})
		if err != nil {
			execErr = fmt.Errorf("exec command %q: %w", cmd, err)
			outputs = append(outputs, fmt.Sprintf("ERROR: %v", err))
			break
		}
		if result.ExitCode != 0 {
			execErr = fmt.Errorf("command %q exited with code %d: %s", cmd, result.ExitCode, result.Stderr)
			outputs = append(outputs, fmt.Sprintf("EXIT %d: %s", result.ExitCode, result.Stderr))
			break
		}
		outputs = append(outputs, result.Stdout)
	}

	output := strings.Join(outputs, "\n")

	// Audit.
	auditType := model.AuditActionCompleted
	if execErr != nil {
		auditType = model.AuditActionFailed
		action = action.Fail(execErr.Error())
	} else {
		action = action.Complete(output)
	}

	o.logAudit(ctx, model.NewAuditLog(
		auditType,
		action.AlertID,
		"system",
		action.Environment,
		fmt.Sprintf("action %q executed", action.Description),
	).WithActionID(action.ID))

	// Notify result.
	threadID := ""
	if v, ok := action.Metadata["thread_id"]; ok {
		threadID = v
	}
	if notifyErr := o.notifier.NotifyAction(ctx, threadID, outbound.ActionNotification{
		Description: action.Description,
		Command:     strings.Join(action.Commands, " && "),
		Status:      string(action.Status),
		Output:      output,
		Risk:        string(action.Risk),
	}); notifyErr != nil {
		o.logger.Error("failed to notify action", "error", notifyErr, "action_id", action.ID)
	}

	return action, execErr
}

