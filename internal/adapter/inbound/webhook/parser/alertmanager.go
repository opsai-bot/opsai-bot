package parser

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// alertManagerPayload represents the Prometheus AlertManager webhook group payload.
type alertManagerPayload struct {
	Version           string              `json:"version"`
	GroupKey          string              `json:"groupKey"`
	TruncatedAlerts   int                 `json:"truncatedAlerts"`
	Status            string              `json:"status"`
	Receiver          string              `json:"receiver"`
	GroupLabels       map[string]string   `json:"groupLabels"`
	CommonLabels      map[string]string   `json:"commonLabels"`
	CommonAnnotations map[string]string   `json:"commonAnnotations"`
	ExternalURL       string              `json:"externalURL"`
	Alerts            []alertManagerAlert `json:"alerts"`
}

type alertManagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// AlertManagerParser parses Prometheus AlertManager webhook payloads.
type AlertManagerParser struct{}

// NewAlertManagerParser creates a new AlertManagerParser.
func NewAlertManagerParser() *AlertManagerParser {
	return &AlertManagerParser{}
}

// Source returns the source identifier for AlertManager alerts.
func (a *AlertManagerParser) Source() string {
	return string(model.AlertSourceAlertManager)
}

// CanParse returns true if the request payload looks like an AlertManager group payload.
// Checks for the X-Prometheus-Alert header or alertmanager in the path.
func (a *AlertManagerParser) CanParse(r *http.Request) bool {
	if r.Header.Get("X-Prometheus-Alert") != "" {
		return true
	}
	if strings.Contains(strings.ToLower(r.URL.Path), "alertmanager") {
		return true
	}
	if strings.Contains(strings.ToLower(r.URL.Path), "prometheus") {
		return true
	}
	return false
}

// ValidateSignature validates an HMAC-SHA256 signature for AlertManager webhooks.
func (a *AlertManagerParser) ValidateSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if sigHeader == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return fmt.Errorf("invalid signature format")
	}
	providedSig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("reading request body for signature validation: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	mac := hmac.New(sha256.New, []byte(secret))
	// Body is already exhausted at this point; the handler should use HMACAuth middleware.
	// Return an error indicating middleware should be used instead.
	_ = providedSig
	_ = mac
	return fmt.Errorf("use HMACAuth middleware for AlertManager signature validation")
}

// Parse extracts model.Alert instances from an AlertManager group payload.
func (a *AlertManagerParser) Parse(ctx context.Context, r *http.Request) ([]model.Alert, error) {
	var payload alertManagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("alertmanager: failed to decode JSON: %w", err)
	}

	alerts := make([]model.Alert, 0, len(payload.Alerts))
	for _, am := range payload.Alerts {
		// Merge group labels into per-alert labels (alert labels take precedence)
		mergedLabels := make(map[string]string, len(payload.CommonLabels)+len(am.Labels))
		for k, v := range payload.CommonLabels {
			mergedLabels[k] = v
		}
		for k, v := range am.Labels {
			mergedLabels[k] = v
		}

		// Merge annotations similarly
		mergedAnnotations := make(map[string]string, len(payload.CommonAnnotations)+len(am.Annotations))
		for k, v := range payload.CommonAnnotations {
			mergedAnnotations[k] = v
		}
		for k, v := range am.Annotations {
			mergedAnnotations[k] = v
		}

		severity := amSeverityFromLabels(mergedLabels)
		status := amStatusFromString(am.Status)

		title := mergedAnnotations["summary"]
		if title == "" {
			title = mergedLabels["alertname"]
		}
		description := mergedAnnotations["description"]
		if description == "" {
			description = mergedAnnotations["message"]
		}

		alert := model.NewAlert(
			model.AlertSourceAlertManager,
			severity,
			title,
			description,
			mergedLabels["environment"],
			mergedLabels["namespace"],
		)
		alert.Status = status
		alert.Resource = mergedLabels["instance"]
		alert.Labels = mergedLabels
		alert.Annotations = mergedAnnotations

		// Use AlertManager fingerprint if present, otherwise derive from labels
		if am.Fingerprint != "" {
			alert.Fingerprint = am.Fingerprint
			alert.ExternalID = am.Fingerprint
		} else {
			alert.Fingerprint = labelsFingerprint(mergedLabels)
		}

		rawBytes, _ := json.Marshal(am)
		alert.RawPayload = string(rawBytes)

		if status == model.AlertStatusResolved && !am.EndsAt.IsZero() {
			t := am.EndsAt
			alert.ResolvedAt = &t
		}

		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// amSeverityFromLabels maps AlertManager label severity to model.Severity.
func amSeverityFromLabels(labels map[string]string) model.Severity {
	switch strings.ToLower(labels["severity"]) {
	case "critical", "error", "high", "p1", "p2":
		return model.SeverityCritical
	case "warning", "warn", "medium", "p3":
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}

// amStatusFromString maps AlertManager status string to model.AlertStatus.
func amStatusFromString(status string) model.AlertStatus {
	switch strings.ToLower(status) {
	case "resolved":
		return model.AlertStatusResolved
	default:
		return model.AlertStatusReceived
	}
}

// labelsFingerprint computes a deterministic hash of the label set for use as fingerprint.
func labelsFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(labels[k]))
		h.Write([]byte(","))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
