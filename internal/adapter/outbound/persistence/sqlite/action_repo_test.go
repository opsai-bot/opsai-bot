package sqlite_test

import (
	"context"
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/outbound/persistence/sqlite"
	"github.com/jonny/opsai-bot/internal/domain/model"
)

func seedAlertAndAnalysis(t *testing.T, store *sqlite.Store) (alertID, analysisID string) {
	t.Helper()
	ctx := context.Background()

	alert := makeAlert("seed-alert", "production")
	alertRepo := sqlite.NewAlertRepo(store)
	if _, err := alertRepo.Create(ctx, alert); err != nil {
		t.Fatalf("seed alert: %v", err)
	}

	analysis := model.NewAnalysis(alert.ID, "ollama", "llama3")
	analysisRepo := sqlite.NewAnalysisRepo(store)
	if _, err := analysisRepo.Create(ctx, analysis); err != nil {
		t.Fatalf("seed analysis: %v", err)
	}
	return alert.ID, analysis.ID
}

func makeAction(analysisID, alertID string) model.Action {
	return model.NewAction(
		analysisID, alertID,
		model.ActionTypeRestart,
		"Restart deployment",
		[]string{"kubectl rollout restart deploy/myapp"},
		model.RiskLow,
	)
}

func TestActionRepo_CreateAndGetByID(t *testing.T) {
	store := newTestStore(t)
	alertID, analysisID := seedAlertAndAnalysis(t, store)
	repo := sqlite.NewActionRepo(store)
	ctx := context.Background()

	action := makeAction(analysisID, alertID)
	action.Environment = "production"
	action.Namespace = "default"
	action.Metadata = map[string]string{"reason": "oom"}

	created, err := repo.Create(ctx, action)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != action.ID {
		t.Errorf("ID mismatch")
	}

	got, err := repo.GetByID(ctx, action.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Type != model.ActionTypeRestart {
		t.Errorf("Type: got %s", got.Type)
	}
	if got.Risk != model.RiskLow {
		t.Errorf("Risk: got %s", got.Risk)
	}
	if len(got.Commands) != 1 || got.Commands[0] != "kubectl rollout restart deploy/myapp" {
		t.Errorf("Commands: got %v", got.Commands)
	}
	if got.Metadata["reason"] != "oom" {
		t.Errorf("Metadata: got %v", got.Metadata)
	}
}

func TestActionRepo_UpdateStatus(t *testing.T) {
	store := newTestStore(t)
	alertID, analysisID := seedAlertAndAnalysis(t, store)
	repo := sqlite.NewActionRepo(store)
	ctx := context.Background()

	action := makeAction(analysisID, alertID)
	if _, err := repo.Create(ctx, action); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := repo.UpdateStatus(ctx, action.ID, model.ActionStatusCompleted, "rollout restarted successfully")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.GetByID(ctx, action.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != model.ActionStatusCompleted {
		t.Errorf("Status: got %s want completed", got.Status)
	}
	if got.Output != "rollout restarted successfully" {
		t.Errorf("Output: got %q", got.Output)
	}
}

func TestActionRepo_GetPendingApprovals(t *testing.T) {
	store := newTestStore(t)
	alertID, analysisID := seedAlertAndAnalysis(t, store)
	repo := sqlite.NewActionRepo(store)
	ctx := context.Background()

	// Create pending action in production
	a1 := makeAction(analysisID, alertID)
	a1.Status = model.ActionStatusPending
	a1.Environment = "production"
	if _, err := repo.Create(ctx, a1); err != nil {
		t.Fatalf("Create a1: %v", err)
	}

	// Create pending action in staging (should NOT appear in production query)
	a2 := makeAction(analysisID, alertID)
	a2.Status = model.ActionStatusPending
	a2.Environment = "staging"
	if _, err := repo.Create(ctx, a2); err != nil {
		t.Fatalf("Create a2: %v", err)
	}

	// Create completed action in production (should NOT appear)
	a3 := makeAction(analysisID, alertID)
	a3.Status = model.ActionStatusCompleted
	a3.Environment = "production"
	if _, err := repo.Create(ctx, a3); err != nil {
		t.Fatalf("Create a3: %v", err)
	}

	results, err := repo.GetPendingApprovals(ctx, "production")
	if err != nil {
		t.Fatalf("GetPendingApprovals: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 pending approval, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != a1.ID {
		t.Errorf("wrong action returned: got %s want %s", results[0].ID, a1.ID)
	}
}
