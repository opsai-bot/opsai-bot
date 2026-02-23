package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// AuditRepo implements outbound.AuditRepository using SQLite.
type AuditRepo struct {
	db *sql.DB
}

// NewAuditRepo creates a new AuditRepo backed by the given store.
func NewAuditRepo(store *Store) *AuditRepo {
	return &AuditRepo{db: store.DB}
}

// Create inserts a new audit log row.
func (r *AuditRepo) Create(ctx context.Context, log model.AuditLog) error {
	meta, err := marshalStringMap(log.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling audit metadata: %w", err)
	}

	const q = `INSERT INTO audit_logs
		(id, event_type, alert_id, action_id, actor, environment, description, metadata, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`

	_, err = r.db.ExecContext(ctx, q,
		log.ID, string(log.EventType),
		log.AlertID, log.ActionID,
		log.Actor, log.Environment, log.Description,
		meta, log.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}
	return nil
}

// allowedAuditOrderColumns defines valid columns for ORDER BY to prevent SQL injection.
var allowedAuditOrderColumns = map[string]bool{
	"created_at": true, "event_type": true, "actor": true,
	"environment": true, "alert_id": true,
}

// List returns a paginated, filtered list of audit logs.
func (r *AuditRepo) List(ctx context.Context, filter outbound.AuditFilter, page outbound.PageRequest) (outbound.PageResult[model.AuditLog], error) {
	where, args := buildAuditWhere(filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs"+where, args...).Scan(&total); err != nil {
		return outbound.PageResult[model.AuditLog]{}, fmt.Errorf("counting audit logs: %w", err)
	}

	orderCol := "created_at"
	if page.OrderBy != "" {
		if !allowedAuditOrderColumns[page.OrderBy] {
			return outbound.PageResult[model.AuditLog]{}, fmt.Errorf("invalid order column: %q", page.OrderBy)
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

	dataQ := fmt.Sprintf(`SELECT id, event_type, alert_id, action_id, actor, environment, description, metadata, created_at
		FROM audit_logs%s ORDER BY %s %s LIMIT ? OFFSET ?`, where, orderCol, dir)

	rows, err := r.db.QueryContext(ctx, dataQ, append(args, size, offset)...)
	if err != nil {
		return outbound.PageResult[model.AuditLog]{}, fmt.Errorf("listing audit logs: %w", err)
	}
	defer rows.Close()

	var items []model.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return outbound.PageResult[model.AuditLog]{}, fmt.Errorf("scanning audit log: %w", err)
		}
		items = append(items, l)
	}
	if err := rows.Err(); err != nil {
		return outbound.PageResult[model.AuditLog]{}, fmt.Errorf("iterating audit logs: %w", err)
	}

	return outbound.PageResult[model.AuditLog]{
		Items:      items,
		TotalCount: total,
		Page:       page.Page,
		Size:       size,
	}, nil
}

// --- helpers ---

type auditScanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(s auditScanner) (model.AuditLog, error) {
	var l model.AuditLog
	var eventType, metaJSON string

	err := s.Scan(
		&l.ID, &eventType, &l.AlertID, &l.ActionID,
		&l.Actor, &l.Environment, &l.Description,
		&metaJSON, &l.CreatedAt,
	)
	if err != nil {
		return model.AuditLog{}, err
	}
	l.EventType = model.AuditEventType(eventType)
	if err := json.Unmarshal([]byte(metaJSON), &l.Metadata); err != nil {
		l.Metadata = make(map[string]string)
	}
	return l, nil
}

func buildAuditWhere(f outbound.AuditFilter) (string, []any) {
	var clauses []string
	var args []any

	if f.AlertID != "" {
		clauses = append(clauses, "alert_id = ?")
		args = append(args, f.AlertID)
	}
	if f.ActionType != "" {
		clauses = append(clauses, "event_type = ?")
		args = append(args, f.ActionType)
	}
	if f.Actor != "" {
		clauses = append(clauses, "actor = ?")
		args = append(args, f.Actor)
	}
	if f.Environment != "" {
		clauses = append(clauses, "environment = ?")
		args = append(args, f.Environment)
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
