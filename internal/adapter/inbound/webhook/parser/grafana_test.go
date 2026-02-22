package parser_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/parser"
	"github.com/jonny/opsai-bot/internal/domain/model"
)

func TestGrafanaParser_Source(t *testing.T) {
	p := parser.NewGrafanaParser()
	if p.Source() != "grafana" {
		t.Errorf("expected 'grafana', got %q", p.Source())
	}
}

func TestGrafanaParser_CanParse(t *testing.T) {
	p := parser.NewGrafanaParser()

	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected bool
	}{
		{
			name: "X-Grafana-Origin header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Grafana-Origin", "alert")
			},
			expected: true,
		},
		{
			name: "grafana in path",
			setup: func(r *http.Request) {
				r.URL.Path = "/webhook/grafana"
			},
			expected: true,
		},
		{
			name: "grafana User-Agent",
			setup: func(r *http.Request) {
				r.Header.Set("User-Agent", "Grafana/9.0")
			},
			expected: true,
		},
		{
			name:     "unrelated request",
			setup:    func(r *http.Request) {},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
			tc.setup(req)
			if got := p.CanParse(req); got != tc.expected {
				t.Errorf("CanParse() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestGrafanaParser_ParseV2(t *testing.T) {
	payload := `{
		"receiver": "ops-team",
		"status": "firing",
		"alerts": [
			{
				"status": "firing",
				"labels": {
					"alertname": "HighCPU",
					"severity": "critical",
					"environment": "prod",
					"namespace": "default",
					"instance": "web-1"
				},
				"annotations": {
					"summary": "CPU usage is high",
					"description": "CPU usage exceeded 90%"
				},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "abc123"
			}
		]
	}`

	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/grafana", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Title != "CPU usage is high" {
		t.Errorf("title = %q, want %q", a.Title, "CPU usage is high")
	}
	if a.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical", a.Severity)
	}
	if a.Status != model.AlertStatusReceived {
		t.Errorf("status = %v, want received", a.Status)
	}
	if a.Source != model.AlertSourceGrafana {
		t.Errorf("source = %v, want grafana", a.Source)
	}
	if a.Fingerprint != "abc123" {
		t.Errorf("fingerprint = %q, want abc123", a.Fingerprint)
	}
	if a.Environment != "prod" {
		t.Errorf("environment = %q, want prod", a.Environment)
	}
	if a.Namespace != "default" {
		t.Errorf("namespace = %q, want default", a.Namespace)
	}
}

func TestGrafanaParser_ParseV2_Resolved(t *testing.T) {
	payload := `{
		"status": "resolved",
		"alerts": [
			{
				"status": "resolved",
				"labels": {"alertname": "TestAlert", "severity": "warning"},
				"annotations": {"summary": "Test alert resolved"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "2024-01-01T01:00:00Z",
				"fingerprint": "def456"
			}
		]
	}`

	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/grafana", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Status != model.AlertStatusResolved {
		t.Errorf("status = %v, want resolved", a.Status)
	}
	if a.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestGrafanaParser_ParseV1(t *testing.T) {
	payload := `{
		"title": "[Alerting] HighMemory",
		"message": "Memory usage is high",
		"state": "alerting",
		"ruleId": 42,
		"ruleName": "HighMemory",
		"ruleUrl": "http://grafana/alert/42",
		"tags": {
			"environment": "staging",
			"instance": "db-1"
		},
		"evalMatches": [
			{"value": 90, "metric": "memory_usage", "tags": {}}
		]
	}`

	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/grafana", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Title != "[Alerting] HighMemory" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Source != model.AlertSourceGrafana {
		t.Errorf("source = %v", a.Source)
	}
	if a.Status != model.AlertStatusReceived {
		t.Errorf("status = %v, want received", a.Status)
	}
}

func TestGrafanaParser_ParseV1_OK_State(t *testing.T) {
	payload := `{
		"title": "Alert resolved",
		"message": "Everything is fine",
		"state": "ok",
		"tags": {}
	}`

	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/grafana", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerts[0].Status != model.AlertStatusResolved {
		t.Errorf("expected resolved, got %v", alerts[0].Status)
	}
}

func TestGrafanaParser_ValidateSignature_NoSecret(t *testing.T) {
	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if err := p.ValidateSignature(req, ""); err != nil {
		t.Errorf("expected no error with empty secret, got %v", err)
	}
}

func TestGrafanaParser_ValidateSignature_Valid(t *testing.T) {
	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("Authorization", "Bearer mysecret")
	if err := p.ValidateSignature(req, "mysecret"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGrafanaParser_ValidateSignature_Invalid(t *testing.T) {
	p := parser.NewGrafanaParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	if err := p.ValidateSignature(req, "mysecret"); err == nil {
		t.Error("expected error for invalid token")
	}
}
