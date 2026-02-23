package service_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
	"github.com/jonny/opsai-bot/internal/domain/service"
)

// --- mock repositories ---

type mockAlertRepo struct {
	alerts map[string]model.Alert
	err    error
}

func newMockAlertRepo() *mockAlertRepo {
	return &mockAlertRepo{alerts: make(map[string]model.Alert)}
}

func (r *mockAlertRepo) Create(_ context.Context, a model.Alert) (model.Alert, error) {
	if r.err != nil {
		return model.Alert{}, r.err
	}
	if a.ID == "" {
		a.ID = "alert-mock-id"
	}
	r.alerts[a.ID] = a
	return a, nil
}
func (r *mockAlertRepo) GetByID(_ context.Context, id string) (model.Alert, error) {
	a, ok := r.alerts[id]
	if !ok {
		return model.Alert{}, errors.New("not found")
	}
	return a, nil
}
func (r *mockAlertRepo) Update(_ context.Context, a model.Alert) (model.Alert, error) {
	r.alerts[a.ID] = a
	return a, nil
}
func (r *mockAlertRepo) List(_ context.Context, _ outbound.AlertFilter, _ outbound.PageRequest) (outbound.PageResult[model.Alert], error) {
	return outbound.PageResult[model.Alert]{}, nil
}
func (r *mockAlertRepo) FindDuplicate(_ context.Context, _ string, _ time.Duration) (*model.Alert, error) {
	return nil, nil
}

var _ outbound.AlertRepository = (*mockAlertRepo)(nil)

type mockAnalysisRepo struct{}

func (m *mockAnalysisRepo) Create(_ context.Context, a model.Analysis) (model.Analysis, error) {
	if a.ID == "" {
		a.ID = "analysis-mock-id"
	}
	return a, nil
}
func (m *mockAnalysisRepo) GetByID(_ context.Context, _ string) (model.Analysis, error) {
	return model.Analysis{}, nil
}
func (m *mockAnalysisRepo) GetByAlertID(_ context.Context, _ string) ([]model.Analysis, error) {
	return nil, nil
}
func (m *mockAnalysisRepo) Update(_ context.Context, a model.Analysis) (model.Analysis, error) {
	return a, nil
}

var _ outbound.AnalysisRepository = (*mockAnalysisRepo)(nil)

type mockActionRepo struct {
	actions map[string]model.Action
}

func newMockActionRepo() *mockActionRepo {
	return &mockActionRepo{actions: make(map[string]model.Action)}
}

func (r *mockActionRepo) Create(_ context.Context, a model.Action) (model.Action, error) {
	if a.ID == "" {
		a.ID = "action-mock-id"
	}
	r.actions[a.ID] = a
	return a, nil
}
func (r *mockActionRepo) GetByID(_ context.Context, id string) (model.Action, error) {
	a, ok := r.actions[id]
	if !ok {
		return model.Action{}, errors.New("not found")
	}
	return a, nil
}
func (r *mockActionRepo) GetByAnalysisID(_ context.Context, _ string) ([]model.Action, error) {
	return nil, nil
}
func (r *mockActionRepo) UpdateStatus(_ context.Context, id string, status model.ActionStatus, output string) error {
	if a, ok := r.actions[id]; ok {
		a.Status = status
		a.Output = output
		r.actions[id] = a
	}
	return nil
}
func (r *mockActionRepo) GetPendingApprovals(_ context.Context, _ string) ([]model.Action, error) {
	return nil, nil
}

var _ outbound.ActionRepository = (*mockActionRepo)(nil)

type mockAuditRepo struct{}

func (m *mockAuditRepo) Create(_ context.Context, _ model.AuditLog) error { return nil }
func (m *mockAuditRepo) List(_ context.Context, _ outbound.AuditFilter, _ outbound.PageRequest) (outbound.PageResult[model.AuditLog], error) {
	return outbound.PageResult[model.AuditLog]{}, nil
}

var _ outbound.AuditRepository = (*mockAuditRepo)(nil)

type mockConversationRepo struct {
	threads map[string]model.ConversationThread
}

func newMockConversationRepo() *mockConversationRepo {
	return &mockConversationRepo{threads: make(map[string]model.ConversationThread)}
}

func (r *mockConversationRepo) Create(_ context.Context, t model.ConversationThread) (model.ConversationThread, error) {
	r.threads[t.ThreadID] = t
	return t, nil
}
func (r *mockConversationRepo) GetByThreadID(_ context.Context, id string) (model.ConversationThread, error) {
	t, ok := r.threads[id]
	if !ok {
		return model.ConversationThread{}, errors.New("not found")
	}
	return t, nil
}
func (r *mockConversationRepo) GetByAlertID(_ context.Context, _ string) (*model.ConversationThread, error) {
	return nil, nil
}
func (r *mockConversationRepo) Update(_ context.Context, t model.ConversationThread) (model.ConversationThread, error) {
	r.threads[t.ThreadID] = t
	return t, nil
}

var _ outbound.ConversationRepository = (*mockConversationRepo)(nil)

// --- mock Notifier ---

type mockNotifier struct {
	threadID      string
	notifyAlertFn func(outbound.AlertNotification)
	requestApprovalCalled bool
}

func (m *mockNotifier) NotifyAlert(_ context.Context, n outbound.AlertNotification) (string, error) {
	if m.notifyAlertFn != nil {
		m.notifyAlertFn(n)
	}
	return m.threadID, nil
}
func (m *mockNotifier) NotifyAnalysis(_ context.Context, _ outbound.AnalysisNotification) error {
	return nil
}
func (m *mockNotifier) NotifyAction(_ context.Context, _ string, _ outbound.ActionNotification) error {
	return nil
}
func (m *mockNotifier) RequestApproval(_ context.Context, _ outbound.ApprovalNotification) error {
	m.requestApprovalCalled = true
	return nil
}
func (m *mockNotifier) SendMessage(_ context.Context, _ string, _ string, _ outbound.NotificationLevel) error {
	return nil
}

var _ outbound.Notifier = (*mockNotifier)(nil)

// --- helpers ---

func buildOrchestrator(
	llm outbound.LLMProvider,
	k8sMock outbound.K8sExecutor,
	policyRepo outbound.PolicyRepository,
	notifier outbound.Notifier,
	actionRepo *mockActionRepo,
) *service.Orchestrator {
	analyzer := service.NewAnalyzer(llm, k8sMock)
	planner := service.NewActionPlanner(k8sMock)
	policyEval := service.NewPolicyEvaluator(policyRepo)
	repos := service.Repositories{
		Alerts:        newMockAlertRepo(),
		Analyses:      &mockAnalysisRepo{},
		Actions:       actionRepo,
		Audits:        &mockAuditRepo{},
		Conversations: newMockConversationRepo(),
	}
	return service.NewOrchestrator(analyzer, planner, policyEval, notifier, k8sMock, repos, slog.Default())
}

// --- tests ---

func TestOrchestrator_HandleAlert_AutoExecute(t *testing.T) {
	llm := &mockLLM{
		diagnoseResult: outbound.DiagnosisResult{
			RootCause:  "OOM",
			Severity:   "critical",
			Confidence: 0.9,
			SuggestedActions: []outbound.SuggestedAction{
				{Description: "restart", Commands: []string{"kubectl rollout restart deployment/app"}, Risk: "low"},
			},
		},
	}
	k8sMock := &mockK8s{
		resourceResult: outbound.ResourceResult{Raw: "pod info"},
		logsResult:     "logs",
		eventsResult:   "events",
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "low"},
		execResult:     outbound.ExecResult{Stdout: "restarted", ExitCode: 0},
	}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "dev",
			Mode:        model.PolicyModeAutoFix,
			MaxAutoRisk: "high",
			Enabled:     true,
		},
	}
	notifier := &mockNotifier{threadID: "thread-123"}
	actionRepo := newMockActionRepo()

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	alert := testAlert()
	alert.Resource = "app-pod"

	err := orch.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notifier.requestApprovalCalled {
		t.Errorf("expected no approval request for dev auto_fix policy")
	}
}

func TestOrchestrator_HandleAlert_ApprovalRequired(t *testing.T) {
	llm := &mockLLM{
		diagnoseResult: outbound.DiagnosisResult{
			RootCause:  "high load",
			Severity:   "warning",
			Confidence: 0.8,
			SuggestedActions: []outbound.SuggestedAction{
				{Description: "delete pod", Commands: []string{"kubectl delete pod app"}, Risk: "high"},
			},
		},
	}
	k8sMock := &mockK8s{
		resourceResult: outbound.ResourceResult{Raw: "pod info"},
		logsResult:     "logs",
		eventsResult:   "events",
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "high"},
	}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "prod",
			Mode:        model.PolicyModeApprovalRequired,
			MaxAutoRisk: "low",
			Approvers:   []string{"admin"},
			Enabled:     true,
		},
	}
	notifier := &mockNotifier{threadID: "thread-456"}
	actionRepo := newMockActionRepo()

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	alert := testAlert()
	alert.Environment = "prod"
	alert.Resource = "app-pod"

	err := orch.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !notifier.requestApprovalCalled {
		t.Errorf("expected approval request for prod policy")
	}
}

func TestOrchestrator_HandleApproval_Approve(t *testing.T) {
	llm := &mockLLM{}
	k8sMock := &mockK8s{
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "low"},
		execResult:     outbound.ExecResult{Stdout: "done", ExitCode: 0},
	}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{Enabled: true, Mode: model.PolicyModeAutoFix, MaxAutoRisk: "high"},
	}
	notifier := &mockNotifier{threadID: "t1"}
	actionRepo := newMockActionRepo()

	// Pre-create a pending action.
	action := model.NewAction("analysis-1", "alert-1", model.ActionTypeRestart, "restart", []string{"kubectl rollout restart deployment/app"}, model.RiskLow)
	action.Status = model.ActionStatusPending
	action, _ = actionRepo.Create(context.Background(), action)

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	err := orch.HandleApproval(context.Background(), inbound.ApprovalRequest{
		ActionID:   action.ID,
		Approved:   true,
		ApprovedBy: "admin",
		Reason:     "looks good",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := actionRepo.actions[action.ID]
	if stored.Status != model.ActionStatusCompleted {
		t.Errorf("expected action status=completed after approval+exec, got %s", stored.Status)
	}
}

func TestOrchestrator_HandleApproval_Reject(t *testing.T) {
	llm := &mockLLM{}
	k8sMock := &mockK8s{}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{Enabled: true, Mode: model.PolicyModeAutoFix, MaxAutoRisk: "high"},
	}
	notifier := &mockNotifier{}
	actionRepo := newMockActionRepo()

	action := model.NewAction("analysis-1", "alert-1", model.ActionTypeRestart, "restart", []string{"kubectl rollout restart deployment/app"}, model.RiskLow)
	action.Status = model.ActionStatusPending
	action, _ = actionRepo.Create(context.Background(), action)

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	err := orch.HandleApproval(context.Background(), inbound.ApprovalRequest{
		ActionID:   action.ID,
		Approved:   false,
		ApprovedBy: "admin",
		Reason:     "too risky",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := actionRepo.actions[action.ID]
	if stored.Status != model.ActionStatusRejected {
		t.Errorf("expected status=rejected, got %s", stored.Status)
	}
}

func TestOrchestrator_HandleMessage(t *testing.T) {
	llm := &mockLLM{
		converseResult: outbound.ConversationResponse{Reply: "Check the logs."},
	}
	k8sMock := &mockK8s{}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{Enabled: true, Mode: model.PolicyModeAutoFix, MaxAutoRisk: "high"},
	}
	notifier := &mockNotifier{}
	actionRepo := newMockActionRepo()

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	resp, err := orch.HandleMessage(context.Background(), inbound.MessageRequest{
		ThreadID:  "thread-999",
		ChannelID: "channel-1",
		UserID:    "user-1",
		UserName:  "alice",
		Text:      "What's wrong?",
		AlertID:   "alert-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text == "" {
		t.Errorf("expected non-empty response text")
	}
}

func TestOrchestrator_ReceiveAlerts_Batch(t *testing.T) {
	llm := &mockLLM{
		diagnoseResult: outbound.DiagnosisResult{
			RootCause: "test", Severity: "info", Confidence: 0.5,
		},
	}
	k8sMock := &mockK8s{
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "low"},
		execResult:     outbound.ExecResult{Stdout: "ok", ExitCode: 0},
	}
	policyRepo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{Enabled: true, Mode: model.PolicyModeAutoFix, MaxAutoRisk: "high"},
	}
	notifier := &mockNotifier{threadID: "t1"}
	actionRepo := newMockActionRepo()

	orch := buildOrchestrator(llm, k8sMock, policyRepo, notifier, actionRepo)

	alerts := []model.Alert{testAlert(), testAlert()}
	err := orch.ReceiveAlerts(context.Background(), alerts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
