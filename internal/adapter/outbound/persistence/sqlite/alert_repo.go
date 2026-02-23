package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// AlertRepo implements outbound.AlertRepository using SQLite.
type AlertRepo struct {
	db *sql.DB
}

// NewAlertRepo creates a new AlertRepo backed by the given store.
func NewAlertRepo(store *Store) *AlertRepo {
	return &AlertRepo{db: store.DB}
}

// Create inserts a new alert row and returns the stored alert.
func (r *AlertRepo) Create(ctx context.Context, alert model.Alert) (model.Alert, error) {
	labels, err := marshalStringMap(alert.Labels)
	if err != nil {
		return model.Alert{}, fmt.Errorf("marshaling labels: %w", err)
	}
	annotations, err := marshalStringMap(alert.Annotations)
	if err != nil {
		return model.Alert{}, fmt.Errorf("marshaling annotations: %w", err)
	}

	const q = `INSERT INTO alerts
		(id, external_id, fingerprint, source, status, severity, title, description,
		 environment, namespace, resource, labels, annotations, raw_payload, thread_id,
		 created_at, updated_at, resolved_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	_, err = r.db.ExecContext(ctx, q,
		alert.ID, alert.ExternalID, alert.Fingerprint,
		string(alert.Source), string(alert.Status), string(alert.Severity),
		alert.Title, alert.Description, alert.Environment,
		alert.Namespace, alert.Resource,
		labels, annotations, alert.RawPayload, alert.ThreadID,
		alert.CreatedAt.UTC(), alert.UpdatedAt.UTC(),
		nullableTime(alert.ResolvedAt),
	)
	if err != nil {
		return model.Alert{}, fmt.Errorf("inserting alert: %w", err)
	}
	return alert, nil
}

// GetByID fetches a single alert by primary key.
func (r *AlertRepo) GetByID(ctx context.Context, id string) (model.Alert, error) {
	const q = `SELECT id, external_id, fingerprint, source, status, severity, title, description,
		environment, namespace, resource, labels, annotations, raw_payload, thread_id,
		created_at, updated_at, resolved_at FROM alerts WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, id)
	alert, err := scanAlert(row)
	if err == sql.ErrNoRows {
		return model.Alert{}, fmt.Errorf("alert %s not found", id)
	}
	if err != nil {
		return model.Alert{}, fmt.Errorf("fetching alert: %w", err)
	}
	return alert, nil
}

// Update replaces all mutable fields of the alert row.
func (r *AlertRepo) Update(ctx context.Context, alert model.Alert) (model.Alert, error) {
	labels, err := marshalStringMap(alert.Labels)
	if err != nil {
		return model.Alert{}, fmt.Errorf("marshaling labels: %w", err)
	}
	annotations, err := marshalStringMap(alert.Annotations)
	if err != nil {
		return model.Alert{}, fmt.Errorf("marshaling annotations: %w", err)
	}

	const q = `UPDATE alerts SET
		external_id=?, fingerprint=?, source=?, status=?, severity=?, title=?, description=?,
		environment=?, namespace=?, resource=?, labels=?, annotations=?, raw_payload=?,
		thread_id=?, updated_at=?, resolved_at=?
		WHERE id=?`

	res, err := r.db.ExecContext(ctx, q,
		alert.ExternalID, alert.Fingerprint,
		string(alert.Source), string(alert.Status), string(alert.Severity),
		alert.Title, alert.Description, alert.Environment,
		alert.Namespace, alert.Resource,
		labels, annotations, alert.RawPayload, alert.ThreadID,
		alert.UpdatedAt.UTC(), nullableTime(alert.ResolvedAt),
		alert.ID,
	)
	if err != nil {
		return model.Alert{}, fmt.Errorf("updating alert: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.Alert{}, fmt.Errorf("alert %s not found", alert.ID)
	}
	return alert, nil
}

// allowedAlertOrderColumns defines valid columns for ORDER BY to prevent SQL injection.
var allowedAlertOrderColumns = map[string]bool{
	"created_at": true, "updated_at": true, "severity": true,
	"status": true, "environment": true, "source": true, "title": true,
}

// List returns a paginated, filtered list of alerts.
func (r *AlertRepo) List(ctx context.Context, filter outbound.AlertFilter, page outbound.PageRequest) (outbound.PageResult[model.Alert], error) {
	where, args := buildAlertWhere(filter)

	// Count total
	countQ := "SELECT COUNT(*) FROM alerts" + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return outbound.PageResult[model.Alert]{}, fmt.Errorf("counting alerts: %w", err)
	}

	// Fetch page
	orderCol := "created_at"
	if page.OrderBy != "" {
		if !allowedAlertOrderColumns[page.OrderBy] {
			return outbound.PageResult[model.Alert]{}, fmt.Errorf("invalid order column: %q", page.OrderBy)
		}
		orderCol = page.OrderBy
	}
	dir := "ASC"
	if page.Desc {
		dir = "DESC"
	}
	size := page.Size
	if size <= 0 {
		size = 20
	}
	offset := page.Page * size

	dataQ := fmt.Sprintf(`SELECT id, external_id, fingerprint, source, status, severity, title, description,
		environment, namespace, resource, labels, annotations, raw_payload, thread_id,
		created_at, updated_at, resolved_at FROM alerts%s ORDER BY %s %s LIMIT ? OFFSET ?`,
		where, orderCol, dir)

	rows, err := r.db.QueryContext(ctx, dataQ, append(args, size, offset)...)
	if err != nil {
		return outbound.PageResult[model.Alert]{}, fmt.Errorf("listing alerts: %w", err)
	}
	defer rows.Close()

	var items []model.Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return outbound.PageResult[model.Alert]{}, fmt.Errorf("scanning alert: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return outbound.PageResult[model.Alert]{}, fmt.Errorf("iterating alerts: %w", err)
	}

	return outbound.PageResult[model.Alert]{
		Items:      items,
		TotalCount: total,
		Page:       page.Page,
		Size:       size,
	}, nil
}

// FindDuplicate looks for an active, non-terminal alert with the same fingerprint
// created within the given window.
func (r *AlertRepo) FindDuplicate(ctx context.Context, fingerprint string, window time.Duration) (*model.Alert, error) {
	since := time.Now().UTC().Add(-window)
	const q = `SELECT id, external_id, fingerprint, source, status, severity, title, description,
		environment, namespace, resource, labels, annotations, raw_payload, thread_id,
		created_at, updated_at, resolved_at FROM alerts
		WHERE fingerprint = ? AND created_at > ? AND status NOT IN ('resolved','failed','duplicate','silenced')
		ORDER BY created_at DESC LIMIT 1`

	row := r.db.QueryRowContext(ctx, q, fingerprint, since.UTC())
	a, err := scanAlert(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding duplicate: %w", err)
	}
	return &a, nil
}

// --- helpers ---

type alertScanner interface {
	Scan(dest ...any) error
}

func scanAlert(s alertScanner) (model.Alert, error) {
	var a model.Alert
	var labelsJSON, annotationsJSON string
	var resolvedAt sql.NullTime
	var source, status, severity string

	err := s.Scan(
		&a.ID, &a.ExternalID, &a.Fingerprint,
		&source, &status, &severity,
		&a.Title, &a.Description, &a.Environment,
		&a.Namespace, &a.Resource,
		&labelsJSON, &annotationsJSON,
		&a.RawPayload, &a.ThreadID,
		&a.CreatedAt, &a.UpdatedAt, &resolvedAt,
	)
	if err != nil {
		return model.Alert{}, err
	}

	a.Source = model.AlertSource(source)
	a.Status = model.AlertStatus(status)
	a.Severity = model.Severity(severity)

	if err := json.Unmarshal([]byte(labelsJSON), &a.Labels); err != nil {
		a.Labels = make(map[string]string)
	}
	if err := json.Unmarshal([]byte(annotationsJSON), &a.Annotations); err != nil {
		a.Annotations = make(map[string]string)
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		a.ResolvedAt = &t
	}
	return a, nil
}

func buildAlertWhere(f outbound.AlertFilter) (string, []any) {
	var clauses []string
	var args []any

	if f.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, f.Source)
	}
	if f.Severity != "" {
		clauses = append(clauses, "severity = ?")
		args = append(args, f.Severity)
	}
	if f.Environment != "" {
		clauses = append(clauses, "environment = ?")
		args = append(args, f.Environment)
	}
	if f.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, f.Status)
	}
	if f.Since != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.UTC())
	}
	if f.Until != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, f.Until.UTC())
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func marshalStringMap(m map[string]string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}
