package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
	"github.com/jonny/opsai-bot/internal/domain/service"
)

// --- mock LLMProvider ---

type mockLLM struct {
	diagnoseResult outbound.DiagnosisResult
	diagnoseErr    error
	converseResult outbound.ConversationResponse
	converseErr    error
	diagnoseCallCount int
}

func (m *mockLLM) Diagnose(_ context.Context, _ outbound.DiagnosisRequest) (outbound.DiagnosisResult, error) {
	m.diagnoseCallCount++
	return m.diagnoseResult, m.diagnoseErr
}

func (m *mockLLM) Converse(_ context.Context, _ outbound.ConversationRequest) (outbound.ConversationResponse, error) {
	return m.converseResult, m.converseErr
}

func (m *mockLLM) HealthCheck(_ context.Context) error { return nil }

func (m *mockLLM) ModelInfo(_ context.Context) (outbound.ModelInfo, error) {
	return outbound.ModelInfo{Provider: "test", Model: "test-model"}, nil
}

var _ outbound.LLMProvider = (*mockLLM)(nil)

// --- mock K8sExecutor ---

type mockK8s struct {
	resourceResult outbound.ResourceResult
	resourceErr    error
	logsResult     string
	logsErr        error
	eventsResult   string
	eventsErr      error
	describeResult string
	describeErr    error
	clusterCtx     string
	clusterErr     error
	validateResult outbound.CommandValidation
	execResult     outbound.ExecResult
	execErr        error
}

func (m *mockK8s) GetResource(_ context.Context, _ outbound.ResourceQuery) (outbound.ResourceResult, error) {
	return m.resourceResult, m.resourceErr
}
func (m *mockK8s) GetPodLogs(_ context.Context, _, _, _ string, _ int64) (string, error) {
	return m.logsResult, m.logsErr
}
func (m *mockK8s) GetEvents(_ context.Context, _, _ string) (string, error) {
	return m.eventsResult, m.eventsErr
}
func (m *mockK8s) DescribeResource(_ context.Context, _, _, _ string) (string, error) {
	return m.describeResult, m.describeErr
}
func (m *mockK8s) GetClusterContext(_ context.Context) (string, error) {
	return m.clusterCtx, m.clusterErr
}
func (m *mockK8s) ValidateCommand(_ []string) outbound.CommandValidation {
	return m.validateResult
}
func (m *mockK8s) Exec(_ context.Context, _ outbound.ExecRequest) (outbound.ExecResult, error) {
	return m.execResult, m.execErr
}
func (m *mockK8s) RestartDeployment(_ context.Context, _, _ string) error { return nil }
func (m *mockK8s) ScaleDeployment(_ context.Context, _, _ string, _ int32) error { return nil }
func (m *mockK8s) DeletePod(_ context.Context, _, _ string) error { return nil }
func (m *mockK8s) HealthCheck(_ context.Context) error { return nil }

var _ outbound.K8sExecutor = (*mockK8s)(nil)

func testAlert() model.Alert {
	return model.NewAlert(model.AlertSourceGrafana, model.SeverityCritical, "OOMKilled", "pod killed", "dev", "default")
}

func TestAnalyzer_AnalyzeAlert_Simple(t *testing.T) {
	llm := &mockLLM{
		diagnoseResult: outbound.DiagnosisResult{
			RootCause:  "memory leak",
			Severity:   "critical",
			Confidence: 0.9,
			Explanation: "pod consumed too much memory",
			SuggestedActions: []outbound.SuggestedAction{
				{Description: "restart pod", Commands: []string{"kubectl rollout restart deployment/app"}, Risk: "low"},
			},
		},
	}
	k8s := &mockK8s{
		resourceResult: outbound.ResourceResult{Raw: "pod info"},
		logsResult:     "OOM error logs",
		eventsResult:   "OOM event",
	}

	alert := testAlert()
	alert.Resource = "app-pod"

	analyzer := service.NewAnalyzer(llm, k8s)
	analysis, actions, err := analyzer.AnalyzeAlert(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.RootCause != "memory leak" {
		t.Errorf("expected root cause 'memory leak', got %q", analysis.RootCause)
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 suggested action, got %d", len(actions))
	}
	if llm.diagnoseCallCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.diagnoseCallCount)
	}
}

func TestAnalyzer_AnalyzeAlert_FollowUpQueries(t *testing.T) {
	callCount := 0
	llm := &mockLLM{}
	// First call: NeedsMoreInfo=true; second call: resolved.
	llm.diagnoseResult = outbound.DiagnosisResult{
		NeedsMoreInfo:   true,
		FollowUpQueries: []string{"describe app-pod"},
	}

	// Override Diagnose to alternate responses.
	customLLM := &sequencedLLM{
		responses: []outbound.DiagnosisResult{
			{NeedsMoreInfo: true, FollowUpQueries: []string{"describe app-pod"}},
			{RootCause: "OOM", Severity: "critical", Confidence: 0.85},
		},
	}
	_ = callCount

	k8s := &mockK8s{
		describeResult: "pod description output",
		resourceResult: outbound.ResourceResult{Raw: "pod info"},
		logsResult:     "logs",
		eventsResult:   "events",
	}

	alert := testAlert()
	alert.Resource = "app-pod"

	analyzer := service.NewAnalyzer(customLLM, k8s)
	analysis, _, err := analyzer.AnalyzeAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.RootCause != "OOM" {
		t.Errorf("expected root cause OOM after follow-up, got %q", analysis.RootCause)
	}
	if customLLM.callCount != 2 {
		t.Errorf("expected 2 LLM calls (initial + 1 follow-up), got %d", customLLM.callCount)
	}
}

func TestAnalyzer_AnalyzeAlert_LLMError(t *testing.T) {
	llm := &mockLLM{diagnoseErr: errors.New("LLM unavailable")}
	k8s := &mockK8s{resourceResult: outbound.ResourceResult{Raw: "info"}}

	alert := testAlert()
	alert.Resource = "app-pod"

	analyzer := service.NewAnalyzer(llm, k8s)
	_, _, err := analyzer.AnalyzeAlert(context.Background(), alert)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestAnalyzer_HandleConversation(t *testing.T) {
	llm := &mockLLM{
		converseResult: outbound.ConversationResponse{
			Reply: "I see the issue, try restarting the pod.",
		},
	}
	k8s := &mockK8s{}

	thread := model.NewConversationThread("alert-1", "thread-1", "channel-1")
	thread = thread.AddMessage(model.MessageRoleUser, "what's wrong?", "user-1")

	analyzer := service.NewAnalyzer(llm, k8s)
	resp, err := analyzer.HandleConversation(context.Background(), thread, "what's wrong?", "alert-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reply == "" {
		t.Errorf("expected non-empty reply")
	}
}

func TestAnalyzer_HandleConversation_Error(t *testing.T) {
	llm := &mockLLM{converseErr: errors.New("LLM down")}
	k8s := &mockK8s{}

	thread := model.NewConversationThread("alert-1", "thread-1", "channel-1")
	analyzer := service.NewAnalyzer(llm, k8s)
	_, err := analyzer.HandleConversation(context.Background(), thread, "help", "alert-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// sequencedLLM returns responses from a fixed sequence.
type sequencedLLM struct {
	responses []outbound.DiagnosisResult
	callCount int
}

func (s *sequencedLLM) Diagnose(_ context.Context, _ outbound.DiagnosisRequest) (outbound.DiagnosisResult, error) {
	if s.callCount >= len(s.responses) {
		return s.responses[len(s.responses)-1], nil
	}
	r := s.responses[s.callCount]
	s.callCount++
	return r, nil
}
func (s *sequencedLLM) Converse(_ context.Context, _ outbound.ConversationRequest) (outbound.ConversationResponse, error) {
	return outbound.ConversationResponse{}, nil
}
func (s *sequencedLLM) HealthCheck(_ context.Context) error { return nil }
func (s *sequencedLLM) ModelInfo(_ context.Context) (outbound.ModelInfo, error) {
	return outbound.ModelInfo{Provider: "seq", Model: "seq-model"}, nil
}

var _ outbound.LLMProvider = (*sequencedLLM)(nil)
