package outbound

import (
	"context"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

type PageRequest struct {
	Page    int
	Size    int
	OrderBy string
	Desc    bool
}

type PageResult[T any] struct {
	Items      []T
	TotalCount int64
	Page       int
	Size       int
}

type AlertFilter struct {
	Source      string
	Severity    string
	Environment string
	Status      string
	Since       *time.Time
	Until       *time.Time
}

type AuditFilter struct {
	AlertID     string
	ActionType  string
	Actor       string
	Environment string
	Since       *time.Time
	Until       *time.Time
}

type AlertRepository interface {
	Create(ctx context.Context, alert model.Alert) (model.Alert, error)
	GetByID(ctx context.Context, id string) (model.Alert, error)
	Update(ctx context.Context, alert model.Alert) (model.Alert, error)
	List(ctx context.Context, filter AlertFilter, page PageRequest) (PageResult[model.Alert], error)
	FindDuplicate(ctx context.Context, fingerprint string, window time.Duration) (*model.Alert, error)
}

type AnalysisRepository interface {
	Create(ctx context.Context, analysis model.Analysis) (model.Analysis, error)
	GetByID(ctx context.Context, id string) (model.Analysis, error)
	GetByAlertID(ctx context.Context, alertID string) ([]model.Analysis, error)
	Update(ctx context.Context, analysis model.Analysis) (model.Analysis, error)
}

type ActionRepository interface {
	Create(ctx context.Context, action model.Action) (model.Action, error)
	GetByID(ctx context.Context, id string) (model.Action, error)
	GetByAnalysisID(ctx context.Context, analysisID string) ([]model.Action, error)
	UpdateStatus(ctx context.Context, id string, status model.ActionStatus, output string) error
	GetPendingApprovals(ctx context.Context, environment string) ([]model.Action, error)
}

type AuditRepository interface {
	Create(ctx context.Context, log model.AuditLog) error
	List(ctx context.Context, filter AuditFilter, page PageRequest) (PageResult[model.AuditLog], error)
}

type PolicyRepository interface {
	GetByEnvironment(ctx context.Context, env string) (model.EnvironmentPolicy, error)
	GetAll(ctx context.Context) ([]model.EnvironmentPolicy, error)
	Upsert(ctx context.Context, policy model.EnvironmentPolicy) error
}

type ConversationRepository interface {
	Create(ctx context.Context, thread model.ConversationThread) (model.ConversationThread, error)
	GetByThreadID(ctx context.Context, threadID string) (model.ConversationThread, error)
	GetByAlertID(ctx context.Context, alertID string) (*model.ConversationThread, error)
	Update(ctx context.Context, thread model.ConversationThread) (model.ConversationThread, error)
}
