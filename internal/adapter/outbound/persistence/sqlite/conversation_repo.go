package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// ConversationRepo implements outbound.ConversationRepository using SQLite.
type ConversationRepo struct {
	db *sql.DB
}

// NewConversationRepo creates a new ConversationRepo backed by the given store.
func NewConversationRepo(store *Store) *ConversationRepo {
	return &ConversationRepo{db: store.DB}
}

// Create inserts a new conversation thread row.
func (r *ConversationRepo) Create(ctx context.Context, t model.ConversationThread) (model.ConversationThread, error) {
	msgs, err := json.Marshal(t.Messages)
	if err != nil {
		return model.ConversationThread{}, fmt.Errorf("marshaling messages: %w", err)
	}

	const q = `INSERT INTO conversations
		(id, alert_id, thread_id, channel_id, messages, active, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`

	_, err = r.db.ExecContext(ctx, q,
		t.ID, t.AlertID, t.ThreadID, t.ChannelID,
		string(msgs), t.Active,
		t.CreatedAt.UTC(), t.UpdatedAt.UTC(),
	)
	if err != nil {
		return model.ConversationThread{}, fmt.Errorf("inserting conversation: %w", err)
	}
	return t, nil
}

// GetByThreadID fetches a conversation thread by its Slack thread_id.
func (r *ConversationRepo) GetByThreadID(ctx context.Context, threadID string) (model.ConversationThread, error) {
	const q = `SELECT id, alert_id, thread_id, channel_id, messages, active, created_at, updated_at
		FROM conversations WHERE thread_id = ? ORDER BY created_at DESC LIMIT 1`

	row := r.db.QueryRowContext(ctx, q, threadID)
	t, err := scanConversation(row)
	if err == sql.ErrNoRows {
		return model.ConversationThread{}, fmt.Errorf("conversation for thread %s not found", threadID)
	}
	if err != nil {
		return model.ConversationThread{}, fmt.Errorf("fetching conversation by thread: %w", err)
	}
	return t, nil
}

// GetByAlertID fetches the most recent conversation thread for an alert.
func (r *ConversationRepo) GetByAlertID(ctx context.Context, alertID string) (*model.ConversationThread, error) {
	const q = `SELECT id, alert_id, thread_id, channel_id, messages, active, created_at, updated_at
		FROM conversations WHERE alert_id = ? ORDER BY created_at DESC LIMIT 1`

	row := r.db.QueryRowContext(ctx, q, alertID)
	t, err := scanConversation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetching conversation by alert: %w", err)
	}
	return &t, nil
}

// Update replaces all mutable fields of a conversation thread row.
func (r *ConversationRepo) Update(ctx context.Context, t model.ConversationThread) (model.ConversationThread, error) {
	msgs, err := json.Marshal(t.Messages)
	if err != nil {
		return model.ConversationThread{}, fmt.Errorf("marshaling messages: %w", err)
	}

	const q = `UPDATE conversations SET
		alert_id=?, thread_id=?, channel_id=?, messages=?, active=?, updated_at=?
		WHERE id=?`

	res, err := r.db.ExecContext(ctx, q,
		t.AlertID, t.ThreadID, t.ChannelID,
		string(msgs), t.Active, t.UpdatedAt.UTC(),
		t.ID,
	)
	if err != nil {
		return model.ConversationThread{}, fmt.Errorf("updating conversation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ConversationThread{}, fmt.Errorf("conversation %s not found", t.ID)
	}
	return t, nil
}

// --- helpers ---

type conversationScanner interface {
	Scan(dest ...any) error
}

func scanConversation(s conversationScanner) (model.ConversationThread, error) {
	var t model.ConversationThread
	var msgsJSON string

	err := s.Scan(
		&t.ID, &t.AlertID, &t.ThreadID, &t.ChannelID,
		&msgsJSON, &t.Active,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return model.ConversationThread{}, err
	}
	if err := json.Unmarshal([]byte(msgsJSON), &t.Messages); err != nil {
		t.Messages = []model.ChatMessage{}
	}
	return t, nil
}
