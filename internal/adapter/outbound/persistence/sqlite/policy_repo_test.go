package sqlite_test

import (
	"context"
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/outbound/persistence/sqlite"
	"github.com/jonny/opsai-bot/internal/domain/model"
)

func makePolicy(env string, mode model.PolicyMode) model.EnvironmentPolicy {
	return model.EnvironmentPolicy{
		ID:          "policy-" + env,
		Environment: env,
		Mode:        mode,
		MaxAutoRisk: "low",
		Approvers:   []string{"alice", "bob"},
		Namespaces:  []string{"default", "kube-system"},
		CustomRules: []model.PolicyRule{
			{
				Name:        "deny-prod-delete",
				Description: "Deny pod deletion in production",
				Condition: model.PolicyCondition{
					Field:    "action.type",
					Operator: "eq",
					Value:    "delete_pod",
				},
				Effect:   model.PolicyEffectDeny,
				Priority: 10,
			},
		},
		Enabled: true,
	}
}

func TestPolicyRepo_UpsertAndGet(t *testing.T) {
	store := newTestStore(t)
	repo := sqlite.NewPolicyRepo(store)
	ctx := context.Background()

	policy := makePolicy("production", model.PolicyModeApprovalRequired)

	// Insert
	if err := repo.Upsert(ctx, policy); err != nil {
		t.Fatalf("Upsert (insert): %v", err)
	}

	got, err := repo.GetByEnvironment(ctx, "production")
	if err != nil {
		t.Fatalf("GetByEnvironment: %v", err)
	}
	if got.Mode != model.PolicyModeApprovalRequired {
		t.Errorf("Mode: got %s", got.Mode)
	}
	if len(got.Approvers) != 2 || got.Approvers[0] != "alice" {
		t.Errorf("Approvers: got %v", got.Approvers)
	}
	if len(got.Namespaces) != 2 {
		t.Errorf("Namespaces: got %v", got.Namespaces)
	}
	if len(got.CustomRules) != 1 {
		t.Errorf("CustomRules: got %v", got.CustomRules)
	}
	if got.CustomRules[0].Effect != model.PolicyEffectDeny {
		t.Errorf("CustomRule Effect: got %s", got.CustomRules[0].Effect)
	}

	// Update via upsert
	updated := policy
	updated.Mode = model.PolicyModeAutoFix
	updated.Approvers = []string{"charlie"}
	if err := repo.Upsert(ctx, updated); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}

	got2, err := repo.GetByEnvironment(ctx, "production")
	if err != nil {
		t.Fatalf("GetByEnvironment after update: %v", err)
	}
	if got2.Mode != model.PolicyModeAutoFix {
		t.Errorf("Mode after update: got %s", got2.Mode)
	}
	if len(got2.Approvers) != 1 || got2.Approvers[0] != "charlie" {
		t.Errorf("Approvers after update: got %v", got2.Approvers)
	}

	// GetAll
	if err := repo.Upsert(ctx, makePolicy("staging", model.PolicyModeWarnAuto)); err != nil {
		t.Fatalf("Upsert staging: %v", err)
	}
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("GetAll len: got %d want 2", len(all))
	}
}
