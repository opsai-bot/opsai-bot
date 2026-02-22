package inbound

import (
	"context"
	"net/http"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// WebhookParser parses source-specific webhook payloads into standard Alert models.
type WebhookParser interface {
	Source() string
	CanParse(r *http.Request) bool
	Parse(ctx context.Context, r *http.Request) ([]model.Alert, error)
	ValidateSignature(r *http.Request, secret string) error
}

// ParserRegistry manages WebhookParser instances.
type ParserRegistry interface {
	Register(parser WebhookParser)
	Resolve(r *http.Request) (WebhookParser, error)
	Sources() []string
}

// AlertReceiverPort delivers parsed alerts to the domain orchestrator.
type AlertReceiverPort interface {
	ReceiveAlert(ctx context.Context, alert model.Alert) error
	ReceiveAlerts(ctx context.Context, alerts []model.Alert) error
}
