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

func TestAlertManagerParser_Source(t *testing.T) {
	p := parser.NewAlertManagerParser()
	if p.Source() != "alertmanager" {
		t.Errorf("expected 'alertmanager', got %q", p.Source())
	}
}

func TestAlertManagerParser_CanParse(t *testing.T) {
	p := parser.NewAlertManagerParser()

	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected bool
	}{
		{
			name: "X-Prometheus-Alert header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Prometheus-Alert", "HighCPU")
			},
			expected: true,
		},
		{
			name: "alertmanager in path",
			setup: func(r *http.Request) {
				r.URL.Path = "/webhook/alertmanager"
			},
			expected: true,
		},
		{
			name: "prometheus in path",
			setup: func(r *http.Request) {
				r.URL.Path = "/webhook/prometheus"
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

func TestAlertManagerParser_Parse_Firing(t *testing.T) {
	payload := `{
		"version": "4",
		"groupKey": "{}:{alertname=\"HighCPU\"}",
		"status": "firing",
		"receiver": "ops",
		"groupLabels": {"alertname": "HighCPU"},
		"commonLabels": {"env": "prod", "severity": "critical"},
		"commonAnnotations": {"summary": "CPU is too high"},
		"externalURL": "http://alertmanager",
		"alerts": [
			{
				"status": "firing",
				"labels": {
					"alertname": "HighCPU",
					"severity": "critical",
					"environment": "prod",
					"namespace": "kube-system",
					"instance": "node-1"
				},
				"annotations": {
					"summary": "CPU is too high",
					"description": "CPU exceeded 90% for 5 minutes"
				},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "aabbcc"
			}
		]
	}`

	p := parser.NewAlertManagerParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Source != model.AlertSourceAlertManager {
		t.Errorf("source = %v, want alertmanager", a.Source)
	}
	if a.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical", a.Severity)
	}
	if a.Status != model.AlertStatusReceived {
		t.Errorf("status = %v, want received", a.Status)
	}
	if a.Title != "CPU is too high" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Description != "CPU exceeded 90% for 5 minutes" {
		t.Errorf("description = %q", a.Description)
	}
	if a.Fingerprint != "aabbcc" {
		t.Errorf("fingerprint = %q, want aabbcc", a.Fingerprint)
	}
	if a.Namespace != "kube-system" {
		t.Errorf("namespace = %q", a.Namespace)
	}
	if a.Resource != "node-1" {
		t.Errorf("resource = %q", a.Resource)
	}
}

func TestAlertManagerParser_Parse_Resolved(t *testing.T) {
	payload := `{
		"version": "4",
		"status": "resolved",
		"receiver": "ops",
		"groupLabels": {},
		"commonLabels": {},
		"commonAnnotations": {},
		"alerts": [
			{
				"status": "resolved",
				"labels": {"alertname": "TestAlert", "severity": "warning"},
				"annotations": {"summary": "Test resolved"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "2024-01-01T01:00:00Z",
				"fingerprint": "ff1122"
			}
		]
	}`

	p := parser.NewAlertManagerParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", strings.NewReader(payload))

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

func TestAlertManagerParser_Parse_FingerprintFromLabels(t *testing.T) {
	payload := `{
		"version": "4",
		"status": "firing",
		"receiver": "ops",
		"groupLabels": {},
		"commonLabels": {},
		"commonAnnotations": {},
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "NoFingerprint"},
				"annotations": {"summary": "Test"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z"
			}
		]
	}`

	p := parser.NewAlertManagerParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerts[0].Fingerprint == "" {
		t.Error("expected fingerprint to be derived from labels")
	}
}

func TestAlertManagerParser_Parse_MultipleAlerts(t *testing.T) {
	payload := `{
		"version": "4",
		"status": "firing",
		"receiver": "ops",
		"groupLabels": {},
		"commonLabels": {},
		"commonAnnotations": {},
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "Alert1", "severity": "critical"},
				"annotations": {"summary": "First alert"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "fp1"
			},
			{
				"status": "firing",
				"labels": {"alertname": "Alert2", "severity": "warning"},
				"annotations": {"summary": "Second alert"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "fp2"
			}
		]
	}`

	p := parser.NewAlertManagerParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}
}
