package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonny/opsai-bot/internal/adapter/outbound/persistence/sqlite"
	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.NewStore(sqlite.Config{
		Path:              ":memory:",
		MaxOpenConns:      1,
		PragmaJournalMode: "WAL",
		PragmaBusyTimeout: 5000,
	})
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func makeAlert(title, env string) model.Alert {
	a := model.NewAlert(
		model.AlertSourceGrafana,
		model.SeverityCritical,
		title, "desc", env, "default",
	)
	a.Labels = map[string]string{"app": "test"}
	a.Annotations = map[string]string{"runbook": "http://example.com"}
	return a
}

func TestAlertRepo_CreateAndGetByID(t *testing.T) {
	store := newTestStore(t)
	repo := sqlite.NewAlertRepo(store)
	ctx := context.Background()

	alert := makeAlert("High CPU", "production")
	created, err := repo.Create(ctx, alert)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != alert.ID {
		t.Errorf("ID mismatch: got %s want %s", created.ID, alert.ID)
	}

	got, err := repo.GetByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != alert.Title {
		t.Errorf("Title: got %q want %q", got.Title, alert.Title)
	}
	if got.Labels["app"] != "test" {
		t.Errorf("Labels: got %v", got.Labels)
	}
	if got.Annotations["runbook"] != "http://example.com" {
		t.Errorf("Annotations: got %v", got.Annotations)
	}
	if got.Source != model.AlertSourceGrafana {
		t.Errorf("Source: got %s", got.Source)
	}
	if got.Severity != model.SeverityCritical {
		t.Errorf("Severity: got %s", got.Severity)
	}
}

func TestAlertRepo_Update(t *testing.T) {
	store := newTestStore(t)
	repo := sqlite.NewAlertRepo(store)
	ctx := context.Background()

	alert := makeAlert("OOM Kill", "staging")
	if _, err := repo.Create(ctx, alert); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated := alert.WithStatus(model.AlertStatusAnalyzing)
	updated.ThreadID = "T12345"
	stored, err := repo.Update(ctx, updated)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if stored.Status != model.AlertStatusAnalyzing {
		t.Errorf("Status: got %s", stored.Status)
	}

	got, err := repo.GetByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Status != model.AlertStatusAnalyzing {
		t.Errorf("persisted Status: got %s", got.Status)
	}
	if got.ThreadID != "T12345" {
		t.Errorf("ThreadID: got %s", got.ThreadID)
	}
}

func TestAlertRepo_List_WithFilters(t *testing.T) {
	store := newTestStore(t)
	repo := sqlite.NewAlertRepo(store)
	ctx := context.Background()

	a1 := makeAlert("Alert prod 1", "production")
	a2 := makeAlert("Alert staging 1", "staging")
	a3 := makeAlert("Alert prod 2", "production")
	a3 = a3.WithStatus(model.AlertStatusResolved)

	for _, a := range []model.Alert{a1, a2, a3} {
		if _, err := repo.Create(ctx, a); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Filter by environment
	result, err := repo.List(ctx, outbound.AlertFilter{Environment: "production"}, outbound.PageRequest{Page: 0, Size: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("TotalCount: got %d want 2", result.TotalCount)
	}

	// Filter by status
	result, err = repo.List(ctx, outbound.AlertFilter{Status: "resolved"}, outbound.PageRequest{Page: 0, Size: 10})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if result.TotalCount != 1 {
		t.Errorf("TotalCount for resolved: got %d want 1", result.TotalCount)
	}

	// Pagination
	result, err = repo.List(ctx, outbound.AlertFilter{}, outbound.PageRequest{Page: 0, Size: 2})
	if err != nil {
		t.Fatalf("List paginated: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("Items len: got %d want 2", len(result.Items))
	}
	if result.TotalCount != 3 {
		t.Errorf("TotalCount all: got %d want 3", result.TotalCount)
	}
}

func TestAlertRepo_FindDuplicate(t *testing.T) {
	store := newTestStore(t)
	repo := sqlite.NewAlertRepo(store)
	ctx := context.Background()

	fp := "sha256:abc123"
	alert := makeAlert("Dup alert", "production")
	alert.Fingerprint = fp
	if _, err := repo.Create(ctx, alert); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Should find duplicate within 1-hour window
	dup, err := repo.FindDuplicate(ctx, fp, time.Hour)
	if err != nil {
		t.Fatalf("FindDuplicate: %v", err)
	}
	if dup == nil {
		t.Fatal("expected duplicate, got nil")
	}
	if dup.ID != alert.ID {
		t.Errorf("ID: got %s want %s", dup.ID, alert.ID)
	}

	// Should not find with different fingerprint
	dup2, err := repo.FindDuplicate(ctx, "other:fp", time.Hour)
	if err != nil {
		t.Fatalf("FindDuplicate other fp: %v", err)
	}
	if dup2 != nil {
		t.Error("expected nil, got duplicate")
	}

	// Resolved alert should not be returned as duplicate
	resolved := alert.Resolve()
	if _, err := repo.Update(ctx, resolved); err != nil {
		t.Fatalf("Update resolved: %v", err)
	}
	dup3, err := repo.FindDuplicate(ctx, fp, time.Hour)
	if err != nil {
		t.Fatalf("FindDuplicate resolved: %v", err)
	}
	if dup3 != nil {
		t.Error("resolved alert should not be returned as duplicate")
	}
}
