package parser

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// grafanaV1Payload represents the legacy Grafana Alerting webhook payload (Grafana < 9).
type grafanaV1Payload struct {
	Title       string                   `json:"title"`
	Message     string                   `json:"message"`
	State       string                   `json:"state"`
	RuleID      int64                    `json:"ruleId"`
	RuleName    string                   `json:"ruleName"`
	RuleURL     string                   `json:"ruleUrl"`
	EvalMatches []grafanaV1EvalMatch     `json:"evalMatches"`
	Tags        map[string]string        `json:"tags"`
}

type grafanaV1EvalMatch struct {
	Value  interface{} `json:"value"`
	Metric string      `json:"metric"`
	Tags   map[string]string `json:"tags"`
}

// grafanaV2Payload represents the Grafana Alerting v2 unified alerting webhook payload (Grafana >= 9).
type grafanaV2Payload struct {
	Receiver          string          `json:"receiver"`
	Status            string          `json:"status"`
	Alerts            []grafanaV2Alert `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string          `json:"externalURL"`
	Title             string          `json:"title"`
	Message           string          `json:"message"`
}

type grafanaV2Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
	SilenceURL   string            `json:"silenceURL"`
	DashboardURL string            `json:"dashboardURL"`
	PanelURL     string            `json:"panelURL"`
	Values       map[string]interface{} `json:"values"`
}

// GrafanaParser parses Grafana webhook payloads (both v1 and v2 formats).
type GrafanaParser struct{}

// NewGrafanaParser creates a new GrafanaParser.
func NewGrafanaParser() *GrafanaParser {
	return &GrafanaParser{}
}

// Source returns the source identifier for Grafana alerts.
func (g *GrafanaParser) Source() string {
	return string(model.AlertSourceGrafana)
}

// CanParse returns true if the request appears to be from Grafana.
// Checks X-Grafana-Origin header, path segments, or Grafana-specific User-Agent.
func (g *GrafanaParser) CanParse(r *http.Request) bool {
	if r.Header.Get("X-Grafana-Origin") != "" {
		return true
	}
	if strings.Contains(strings.ToLower(r.URL.Path), "grafana") {
		return true
	}
	if strings.Contains(strings.ToLower(r.Header.Get("User-Agent")), "grafana") {
		return true
	}
	return false
}

// ValidateSignature validates the Bearer token for Grafana webhooks.
func (g *GrafanaParser) ValidateSignature(r *http.Request, secret string) error {
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
	token := strings.TrimSpace(parts[1])
	if !hmac.Equal([]byte(token), []byte(secret)) {
		return fmt.Errorf("invalid bearer token")
	}
	return nil
}

// Parse extracts model.Alert instances from a Grafana webhook payload.
// Supports both v1 (evalMatches) and v2 (alerts array) formats.
func (g *GrafanaParser) Parse(ctx context.Context, r *http.Request) ([]model.Alert, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("grafana: failed to decode JSON: %w", err)
	}

	// Detect v2 by presence of "alerts" array field
	if _, hasAlerts := raw["alerts"]; hasAlerts {
		return g.parseV2(raw)
	}
	return g.parseV1(raw)
}

func (g *GrafanaParser) parseV2(raw map[string]json.RawMessage) ([]model.Alert, error) {
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("grafana v2: marshal error: %w", err)
	}
	var payload grafanaV2Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("grafana v2: unmarshal error: %w", err)
	}

	alerts := make([]model.Alert, 0, len(payload.Alerts))
	for _, a := range payload.Alerts {
		severity := grafanaSeverityFromLabels(a.Labels)
		status := grafanaStatusFromV2(a.Status)
		title := a.Annotations["summary"]
		if title == "" {
			title = a.Labels["alertname"]
		}
		if title == "" {
			title = payload.Title
		}
		description := a.Annotations["description"]
		if description == "" {
			description = a.Annotations["message"]
		}
		if description == "" {
			description = payload.Message
		}

		alert := model.NewAlert(
			model.AlertSourceGrafana,
			severity,
			title,
			description,
			a.Labels["environment"],
			a.Labels["namespace"],
		)
		alert.Status = status
		alert.Fingerprint = a.Fingerprint
		alert.ExternalID = a.Fingerprint
		alert.Resource = a.Labels["instance"]
		alert.Labels = copyMap(a.Labels)
		alert.Annotations = copyMap(a.Annotations)

		rawBytes, _ := json.Marshal(a)
		alert.RawPayload = string(rawBytes)

		if status == model.AlertStatusResolved {
			t := a.EndsAt
			alert.ResolvedAt = &t
		}

		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func (g *GrafanaParser) parseV1(raw map[string]json.RawMessage) ([]model.Alert, error) {
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("grafana v1: marshal error: %w", err)
	}
	var payload grafanaV1Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("grafana v1: unmarshal error: %w", err)
	}

	severity := grafanaSeverityFromState(payload.State)
	status := grafanaStatusFromV1(payload.State)

	title := payload.Title
	if title == "" {
		title = payload.RuleName
	}

	alert := model.NewAlert(
		model.AlertSourceGrafana,
		severity,
		title,
		payload.Message,
		payload.Tags["environment"],
		payload.Tags["namespace"],
	)
	alert.Status = status
	alert.Resource = payload.Tags["instance"]
	alert.Labels = copyMap(payload.Tags)
	alert.Annotations = map[string]string{
		"ruleUrl": payload.RuleURL,
	}

	rawBytes, _ := json.Marshal(payload)
	alert.RawPayload = string(rawBytes)

	return []model.Alert{alert}, nil
}

// grafanaSeverityFromLabels maps Grafana v2 label severity to model.Severity.
func grafanaSeverityFromLabels(labels map[string]string) model.Severity {
	switch strings.ToLower(labels["severity"]) {
	case "critical", "error", "high":
		return model.SeverityCritical
	case "warning", "warn", "medium":
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}

// grafanaSeverityFromState maps Grafana v1 state to model.Severity.
func grafanaSeverityFromState(state string) model.Severity {
	switch strings.ToLower(state) {
	case "alerting":
		return model.SeverityWarning
	case "critical":
		return model.SeverityCritical
	default:
		return model.SeverityInfo
	}
}

// grafanaStatusFromV2 maps Grafana v2 alert status to model.AlertStatus.
func grafanaStatusFromV2(status string) model.AlertStatus {
	switch strings.ToLower(status) {
	case "resolved":
		return model.AlertStatusResolved
	default:
		return model.AlertStatusReceived
	}
}

// grafanaStatusFromV1 maps Grafana v1 state to model.AlertStatus.
func grafanaStatusFromV1(state string) model.AlertStatus {
	switch strings.ToLower(state) {
	case "ok", "no_data":
		return model.AlertStatusResolved
	default:
		return model.AlertStatusReceived
	}
}

// copyMap returns a shallow copy of a string map, returning nil if src is nil.
func copyMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
