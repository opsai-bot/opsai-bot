package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// genericPayload represents the simplified JSON structure for the generic webhook parser.
type genericPayload struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    string            `json:"severity"`
	Environment string            `json:"environment"`
	Namespace   string            `json:"namespace"`
	Resource    string            `json:"resource"`
	Labels      map[string]string `json:"labels"`
}

// GenericParser is a fallback parser that accepts any JSON payload with
// a simplified structure. It matches any request with a JSON content-type.
type GenericParser struct{}

// NewGenericParser creates a new GenericParser.
func NewGenericParser() *GenericParser {
	return &GenericParser{}
}

// Source returns the source identifier for generic webhook alerts.
func (g *GenericParser) Source() string {
	return string(model.AlertSourceCustom)
}

// CanParse returns true for any request with a JSON Content-Type (fallback parser).
func (g *GenericParser) CanParse(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.Contains(ct, "application/json")
}

// ValidateSignature validates a Bearer token in the Authorization header.
// Returns nil when no secret is configured (authentication disabled).
func (g *GenericParser) ValidateSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return fmt.Errorf("invalid Authorization header format")
	}
	if strings.TrimSpace(parts[1]) != secret {
		return fmt.Errorf("invalid bearer token")
	}
	return nil
}

// Parse extracts a single model.Alert from the simplified generic JSON payload.
func (g *GenericParser) Parse(ctx context.Context, r *http.Request) ([]model.Alert, error) {
	var payload genericPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("generic: failed to decode JSON: %w", err)
	}

	if payload.Title == "" {
		return nil, fmt.Errorf("generic: missing required field 'title'")
	}

	severity := genericSeverity(payload.Severity)

	alert := model.NewAlert(
		model.AlertSourceCustom,
		severity,
		payload.Title,
		payload.Description,
		payload.Environment,
		payload.Namespace,
	)
	alert.Resource = payload.Resource
	if payload.Labels != nil {
		alert.Labels = copyMap(payload.Labels)
	}

	rawBytes, _ := json.Marshal(payload)
	alert.RawPayload = string(rawBytes)

	return []model.Alert{alert}, nil
}

// genericSeverity maps a severity string to model.Severity.
func genericSeverity(s string) model.Severity {
	switch strings.ToLower(s) {
	case "critical", "error", "high":
		return model.SeverityCritical
	case "warning", "warn":
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}
