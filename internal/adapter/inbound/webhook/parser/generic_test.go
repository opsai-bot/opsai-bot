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

func TestGenericParser_Source(t *testing.T) {
	p := parser.NewGenericParser()
	if p.Source() != "custom" {
		t.Errorf("expected 'custom', got %q", p.Source())
	}
}

func TestGenericParser_CanParse(t *testing.T) {
	p := parser.NewGenericParser()

	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", false},
		{"", false},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("Content-Type", tc.contentType)
		if got := p.CanParse(req); got != tc.expected {
			t.Errorf("CanParse(%q) = %v, want %v", tc.contentType, got, tc.expected)
		}
	}
}

func TestGenericParser_Parse_Full(t *testing.T) {
	payload := `{
		"title": "Disk Full",
		"description": "Disk usage exceeded 95%",
		"severity": "critical",
		"environment": "production",
		"namespace": "monitoring",
		"resource": "storage-1",
		"labels": {"team": "ops", "region": "us-east-1"}
	}`

	p := parser.NewGenericParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Title != "Disk Full" {
		t.Errorf("title = %q, want 'Disk Full'", a.Title)
	}
	if a.Description != "Disk usage exceeded 95%" {
		t.Errorf("description = %q", a.Description)
	}
	if a.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical", a.Severity)
	}
	if a.Environment != "production" {
		t.Errorf("environment = %q, want production", a.Environment)
	}
	if a.Namespace != "monitoring" {
		t.Errorf("namespace = %q, want monitoring", a.Namespace)
	}
	if a.Resource != "storage-1" {
		t.Errorf("resource = %q, want storage-1", a.Resource)
	}
	if a.Source != model.AlertSourceCustom {
		t.Errorf("source = %v, want custom", a.Source)
	}
	if a.Labels["team"] != "ops" {
		t.Errorf("labels[team] = %q, want ops", a.Labels["team"])
	}
}

func TestGenericParser_Parse_MissingTitle(t *testing.T) {
	payload := `{"description": "no title here", "severity": "warning"}`

	p := parser.NewGenericParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", strings.NewReader(payload))

	_, err := p.Parse(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestGenericParser_Parse_InvalidJSON(t *testing.T) {
	p := parser.NewGenericParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", strings.NewReader("{not json}"))

	_, err := p.Parse(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGenericParser_Parse_DefaultSeverity(t *testing.T) {
	payload := `{"title": "Test Alert"}`

	p := parser.NewGenericParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", strings.NewReader(payload))

	alerts, err := p.Parse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerts[0].Severity != model.SeverityInfo {
		t.Errorf("expected info severity for unknown, got %v", alerts[0].Severity)
	}
}

func TestGenericParser_SeverityMapping(t *testing.T) {
	cases := []struct {
		input    string
		expected model.Severity
	}{
		{"critical", model.SeverityCritical},
		{"error", model.SeverityCritical},
		{"high", model.SeverityCritical},
		{"warning", model.SeverityWarning},
		{"warn", model.SeverityWarning},
		{"info", model.SeverityInfo},
		{"", model.SeverityInfo},
		{"unknown", model.SeverityInfo},
	}

	p := parser.NewGenericParser()
	for _, tc := range cases {
		payload := `{"title":"Test","severity":"` + tc.input + `"}`
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		alerts, err := p.Parse(context.Background(), req)
		if err != nil {
			t.Fatalf("severity=%q: unexpected error: %v", tc.input, err)
		}
		if alerts[0].Severity != tc.expected {
			t.Errorf("severity=%q: got %v, want %v", tc.input, alerts[0].Severity, tc.expected)
		}
	}
}

func TestGenericParser_ValidateSignature_NoOp(t *testing.T) {
	p := parser.NewGenericParser()
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if err := p.ValidateSignature(req, "any-secret"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
