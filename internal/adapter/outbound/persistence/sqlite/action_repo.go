package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// ActionRepo implements outbound.ActionRepository using SQLite.
type ActionRepo struct {
	db *sql.DB
}

// NewActionRepo creates a new ActionRepo backed by the given store.
func NewActionRepo(store *Store) *ActionRepo {
	return &ActionRepo{db: store.DB}
}

// Create inserts a new action row.
func (r *ActionRepo) Create(ctx context.Context, a model.Action) (model.Action, error) {
	cmds, err := json.Marshal(a.Commands)
	if err != nil {
		return model.Action{}, fmt.Errorf("marshaling commands: %w", err)
	}
	meta, err := marshalStringMap(a.Metadata)
	if err != nil {
		return model.Action{}, fmt.Errorf("marshaling metadata: %w", err)
	}

	const q = `INSERT INTO actions
		(id, analysis_id, alert_id, type, status, description, commands, risk, reversible,
		 output, error_message, approved_by, approved_at, executed_at, completed_at,
		 environment, namespace, target_resource, metadata, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	_, err = r.db.ExecContext(ctx, q,
		a.ID, a.AnalysisID, a.AlertID,
		string(a.Type), string(a.Status), a.Description,
		string(cmds), string(a.Risk), a.Reversible,
		a.Output, a.ErrorMessage, a.ApprovedBy,
		nullableTime(a.ApprovedAt), nullableTime(a.ExecutedAt), nullableTime(a.CompletedAt),
		a.Environment, a.Namespace, a.TargetResource,
		meta, a.CreatedAt.UTC(), a.UpdatedAt.UTC(),
	)
	if err != nil {
		return model.Action{}, fmt.Errorf("inserting action: %w", err)
	}
	return a, nil
}

// GetByID fetches a single action by primary key.
func (r *ActionRepo) GetByID(ctx context.Context, id string) (model.Action, error) {
	const q = `SELECT id, analysis_id, alert_id, type, status, description, commands, risk, reversible,
		output, error_message, approved_by, approved_at, executed_at, completed_at,
		environment, namespace, target_resource, metadata, created_at, updated_at
		FROM actions WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, id)
	a, err := scanAction(row)
	if err == sql.ErrNoRows {
		return model.Action{}, fmt.Errorf("action %s not found", id)
	}
	if err != nil {
		return model.Action{}, fmt.Errorf("fetching action: %w", err)
	}
	return a, nil
}

// GetByAnalysisID returns all actions for the given analysis.
func (r *ActionRepo) GetByAnalysisID(ctx context.Context, analysisID string) ([]model.Action, error) {
	const q = `SELECT id, analysis_id, alert_id, type, status, description, commands, risk, reversible,
		output, error_message, approved_by, approved_at, executed_at, completed_at,
		environment, namespace, target_resource, metadata, created_at, updated_at
		FROM actions WHERE analysis_id = ? ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, analysisID)
	if err != nil {
		return nil, fmt.Errorf("querying actions by analysis: %w", err)
	}
	defer rows.Close()

	var results []model.Action
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning action: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// UpdateStatus updates the status and output fields for a given action.
func (r *ActionRepo) UpdateStatus(ctx context.Context, id string, status model.ActionStatus, output string) error {
	const q = `UPDATE actions SET status = ?, output = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(status), output, id)
	if err != nil {
		return fmt.Errorf("updating action status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("action %s not found", id)
	}
	return nil
}

// GetPendingApprovals returns all actions in 'pending' status for the given environment.
func (r *ActionRepo) GetPendingApprovals(ctx context.Context, environment string) ([]model.Action, error) {
	const q = `SELECT id, analysis_id, alert_id, type, status, description, commands, risk, reversible,
		output, error_message, approved_by, approved_at, executed_at, completed_at,
		environment, namespace, target_resource, metadata, created_at, updated_at
		FROM actions WHERE status = 'pending' AND environment = ? ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, environment)
	if err != nil {
		return nil, fmt.Errorf("querying pending approvals: %w", err)
	}
	defer rows.Close()

	var results []model.Action
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning action: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// --- helpers ---

type actionScanner interface {
	Scan(dest ...any) error
}

func scanAction(s actionScanner) (model.Action, error) {
	var a model.Action
	var actionType, status, risk string
	var cmdsJSON, metaJSON string
	var approvedAt, executedAt, completedAt sql.NullTime

	err := s.Scan(
		&a.ID, &a.AnalysisID, &a.AlertID,
		&actionType, &status, &a.Description,
		&cmdsJSON, &risk, &a.Reversible,
		&a.Output, &a.ErrorMessage, &a.ApprovedBy,
		&approvedAt, &executedAt, &completedAt,
		&a.Environment, &a.Namespace, &a.TargetResource,
		&metaJSON, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return model.Action{}, err
	}

	a.Type = model.ActionType(actionType)
	a.Status = model.ActionStatus(status)
	a.Risk = model.RiskLevel(risk)

	if err := json.Unmarshal([]byte(cmdsJSON), &a.Commands); err != nil {
		a.Commands = []string{}
	}
	if err := json.Unmarshal([]byte(metaJSON), &a.Metadata); err != nil {
		a.Metadata = make(map[string]string)
	}
	if approvedAt.Valid {
		t := approvedAt.Time
		a.ApprovedAt = &t
	}
	if executedAt.Valid {
		t := executedAt.Time
		a.ExecutedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		a.CompletedAt = &t
	}
	return a, nil
}
