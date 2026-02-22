package parser_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/parser"
	"github.com/jonny/opsai-bot/internal/domain/model"
)

// stubParser is a test double implementing inbound.WebhookParser.
type stubParser struct {
	source   string
	canParse bool
}

func (s *stubParser) Source() string                                              { return s.source }
func (s *stubParser) CanParse(r *http.Request) bool                               { return s.canParse }
func (s *stubParser) ValidateSignature(r *http.Request, secret string) error      { return nil }
func (s *stubParser) Parse(ctx context.Context, r *http.Request) ([]model.Alert, error) {
	return nil, nil
}

func TestRegistry_Register_and_Sources(t *testing.T) {
	reg := parser.NewRegistry()

	if sources := reg.Sources(); len(sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(sources))
	}

	reg.Register(&stubParser{source: "grafana"})
	reg.Register(&stubParser{source: "alertmanager"})

	sources := reg.Sources()
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0] != "grafana" || sources[1] != "alertmanager" {
		t.Errorf("unexpected sources: %v", sources)
	}
}

func TestRegistry_Resolve_ReturnsFirstMatch(t *testing.T) {
	reg := parser.NewRegistry()
	reg.Register(&stubParser{source: "no-match", canParse: false})
	reg.Register(&stubParser{source: "grafana", canParse: true})
	reg.Register(&stubParser{source: "also-matches", canParse: true})

	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	p, err := reg.Resolve(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Source() != "grafana" {
		t.Errorf("expected grafana, got %s", p.Source())
	}
}

func TestRegistry_Resolve_NoMatch(t *testing.T) {
	reg := parser.NewRegistry()
	reg.Register(&stubParser{source: "no-match", canParse: false})

	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	_, err := reg.Resolve(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRegistry_Resolve_EmptyRegistry(t *testing.T) {
	reg := parser.NewRegistry()
	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	_, err := reg.Resolve(req)
	if err == nil {
		t.Fatal("expected error for empty registry")
	}
}
