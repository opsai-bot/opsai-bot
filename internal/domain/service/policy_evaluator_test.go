package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
	"github.com/jonny/opsai-bot/internal/domain/service"
)

// --- mock PolicyRepository ---

type mockPolicyRepo struct {
	policy model.EnvironmentPolicy
	err    error
}

func (m *mockPolicyRepo) GetByEnvironment(_ context.Context, _ string) (model.EnvironmentPolicy, error) {
	return m.policy, m.err
}

func (m *mockPolicyRepo) GetAll(_ context.Context) ([]model.EnvironmentPolicy, error) {
	return []model.EnvironmentPolicy{m.policy}, m.err
}

func (m *mockPolicyRepo) Upsert(_ context.Context, _ model.EnvironmentPolicy) error {
	return nil
}

// Ensure mockPolicyRepo satisfies the interface.
var _ outbound.PolicyRepository = (*mockPolicyRepo)(nil)

func lowRiskAction() model.Action {
	return model.NewAction("aid", "alid", model.ActionTypeKubectl, "describe pod", []string{"kubectl describe pod foo"}, model.RiskLow)
}

func highRiskAction() model.Action {
	return model.NewAction("aid", "alid", model.ActionTypeDeletePod, "delete pod", []string{"kubectl delete pod foo"}, model.RiskHigh)
}

func TestPolicyEvaluator_DevAutoFix(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "dev",
			Mode:        model.PolicyModeAutoFix,
			MaxAutoRisk: "high",
			Enabled:     true,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	decision, err := eval.Evaluate(context.Background(), "dev", lowRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("expected Allowed=true, got false")
	}
	if !decision.AutoExecute {
		t.Errorf("expected AutoExecute=true for dev auto_fix with low risk")
	}
	if decision.NeedsApproval {
		t.Errorf("expected NeedsApproval=false, got true")
	}
}

func TestPolicyEvaluator_DevAutoFix_RiskTooHigh(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "dev",
			Mode:        model.PolicyModeAutoFix,
			MaxAutoRisk: "low",
			Approvers:   []string{"alice"},
			Enabled:     true,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	decision, err := eval.Evaluate(context.Background(), "dev", highRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("expected Allowed=true")
	}
	if !decision.NeedsApproval {
		t.Errorf("expected NeedsApproval=true when risk exceeds limit")
	}
	if decision.AutoExecute {
		t.Errorf("expected AutoExecute=false")
	}
}

func TestPolicyEvaluator_StagingWarnAuto(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "staging",
			Mode:        model.PolicyModeWarnAuto,
			MaxAutoRisk: "medium",
			Approvers:   []string{"bob"},
			Enabled:     true,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	// Low risk should auto-execute.
	decision, err := eval.Evaluate(context.Background(), "staging", lowRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.AutoExecute {
		t.Errorf("expected AutoExecute=true for low-risk in warn_auto")
	}

	// High risk should require approval.
	decision, err = eval.Evaluate(context.Background(), "staging", highRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.NeedsApproval {
		t.Errorf("expected NeedsApproval=true for high-risk in warn_auto with medium limit")
	}
}

func TestPolicyEvaluator_ProdApprovalRequired(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "prod",
			Mode:        model.PolicyModeApprovalRequired,
			MaxAutoRisk: "low",
			Approvers:   []string{"admin"},
			Enabled:     true,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	// Even low-risk actions need approval in prod.
	decision, err := eval.Evaluate(context.Background(), "prod", lowRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.NeedsApproval {
		t.Errorf("expected NeedsApproval=true for prod")
	}
	if decision.AutoExecute {
		t.Errorf("expected AutoExecute=false for prod")
	}
	if len(decision.Approvers) == 0 {
		t.Errorf("expected non-empty Approvers for prod")
	}
}

func TestPolicyEvaluator_DisabledPolicy(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "dev",
			Mode:        model.PolicyModeAutoFix,
			Enabled:     false,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	decision, err := eval.Evaluate(context.Background(), "dev", lowRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Errorf("expected Allowed=false for disabled policy")
	}
}

func TestPolicyEvaluator_RepoError_DefaultsToApprovalRequired(t *testing.T) {
	repo := &mockPolicyRepo{err: errors.New("db unavailable")}
	eval := service.NewPolicyEvaluator(repo)

	decision, err := eval.Evaluate(context.Background(), "dev", lowRiskAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.NeedsApproval {
		t.Errorf("expected NeedsApproval=true on repo error")
	}
	if decision.AutoExecute {
		t.Errorf("expected AutoExecute=false on repo error")
	}
}

func TestIsRiskAcceptable(t *testing.T) {
	repo := &mockPolicyRepo{
		policy: model.EnvironmentPolicy{
			Environment: "dev",
			Mode:        model.PolicyModeAutoFix,
			MaxAutoRisk: "medium",
			Enabled:     true,
		},
	}
	eval := service.NewPolicyEvaluator(repo)

	cases := []struct {
		action  model.RiskLevel
		wantAuto bool
	}{
		{model.RiskLow, true},
		{model.RiskMedium, true},
		{model.RiskHigh, false},
		{model.RiskCritical, false},
	}
	for _, tc := range cases {
		action := model.NewAction("a", "b", model.ActionTypeKubectl, "desc", nil, tc.action)
		decision, err := eval.Evaluate(context.Background(), "dev", action)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.AutoExecute != tc.wantAuto {
			t.Errorf("risk=%s: want AutoExecute=%v got %v", tc.action, tc.wantAuto, decision.AutoExecute)
		}
	}
}
