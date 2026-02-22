package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// AnalysisRepo implements outbound.AnalysisRepository using SQLite.
type AnalysisRepo struct {
	db *sql.DB
}

// NewAnalysisRepo creates a new AnalysisRepo backed by the given store.
func NewAnalysisRepo(store *Store) *AnalysisRepo {
	return &AnalysisRepo{db: store.DB}
}

// Create inserts a new analysis row.
func (r *AnalysisRepo) Create(ctx context.Context, a model.Analysis) (model.Analysis, error) {
	const q = `INSERT INTO analyses
		(id, alert_id, provider, model, root_cause, severity, confidence, explanation,
		 k8s_context, prompt_tokens, response_tokens, latency_ms, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`

	_, err := r.db.ExecContext(ctx, q,
		a.ID, a.AlertID, a.Provider, a.Model,
		a.RootCause, string(a.Severity), a.Confidence, a.Explanation,
		a.K8sContext, a.PromptTokens, a.ResponseTokens, a.LatencyMs,
		a.CreatedAt.UTC(),
	)
	if err != nil {
		return model.Analysis{}, fmt.Errorf("inserting analysis: %w", err)
	}
	return a, nil
}

// GetByID fetches a single analysis by primary key.
func (r *AnalysisRepo) GetByID(ctx context.Context, id string) (model.Analysis, error) {
	const q = `SELECT id, alert_id, provider, model, root_cause, severity, confidence, explanation,
		k8s_context, prompt_tokens, response_tokens, latency_ms, created_at
		FROM analyses WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, id)
	a, err := scanAnalysis(row)
	if err == sql.ErrNoRows {
		return model.Analysis{}, fmt.Errorf("analysis %s not found", id)
	}
	if err != nil {
		return model.Analysis{}, fmt.Errorf("fetching analysis: %w", err)
	}
	return a, nil
}

// GetByAlertID returns all analyses for the given alert.
func (r *AnalysisRepo) GetByAlertID(ctx context.Context, alertID string) ([]model.Analysis, error) {
	const q = `SELECT id, alert_id, provider, model, root_cause, severity, confidence, explanation,
		k8s_context, prompt_tokens, response_tokens, latency_ms, created_at
		FROM analyses WHERE alert_id = ? ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, alertID)
	if err != nil {
		return nil, fmt.Errorf("querying analyses by alert: %w", err)
	}
	defer rows.Close()

	var results []model.Analysis
	for rows.Next() {
		a, err := scanAnalysis(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning analysis: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// Update replaces all mutable fields of an analysis row.
func (r *AnalysisRepo) Update(ctx context.Context, a model.Analysis) (model.Analysis, error) {
	const q = `UPDATE analyses SET
		alert_id=?, provider=?, model=?, root_cause=?, severity=?, confidence=?,
		explanation=?, k8s_context=?, prompt_tokens=?, response_tokens=?, latency_ms=?
		WHERE id=?`

	res, err := r.db.ExecContext(ctx, q,
		a.AlertID, a.Provider, a.Model,
		a.RootCause, string(a.Severity), a.Confidence, a.Explanation,
		a.K8sContext, a.PromptTokens, a.ResponseTokens, a.LatencyMs,
		a.ID,
	)
	if err != nil {
		return model.Analysis{}, fmt.Errorf("updating analysis: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.Analysis{}, fmt.Errorf("analysis %s not found", a.ID)
	}
	return a, nil
}

// --- helpers ---

type analysisScanner interface {
	Scan(dest ...any) error
}

func scanAnalysis(s analysisScanner) (model.Analysis, error) {
	var a model.Analysis
	var severity string
	err := s.Scan(
		&a.ID, &a.AlertID, &a.Provider, &a.Model,
		&a.RootCause, &severity, &a.Confidence, &a.Explanation,
		&a.K8sContext, &a.PromptTokens, &a.ResponseTokens, &a.LatencyMs,
		&a.CreatedAt,
	)
	if err != nil {
		return model.Analysis{}, err
	}
	a.Severity = model.Severity(severity)
	return a, nil
}
