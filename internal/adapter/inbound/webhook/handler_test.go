package webhook_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/parser"
	"github.com/jonny/opsai-bot/internal/domain/model"
)

// fakeReceiver records received alerts for assertion in tests.
type fakeReceiver struct {
	mu     sync.Mutex
	alerts []model.Alert
	err    error
}

func (f *fakeReceiver) ReceiveAlert(ctx context.Context, alert model.Alert) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.alerts = append(f.alerts, alert)
	return nil
}

func (f *fakeReceiver) ReceiveAlerts(ctx context.Context, alerts []model.Alert) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.alerts = append(f.alerts, alerts...)
	return nil
}

func (f *fakeReceiver) received() []model.Alert {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]model.Alert, len(f.alerts))
	copy(out, f.alerts)
	return out
}

func buildRegistry() *parser.Registry {
	reg := parser.NewRegistry()
	reg.Register(parser.NewGrafanaParser())
	reg.Register(parser.NewAlertManagerParser())
	reg.Register(parser.NewGenericParser())
	return reg
}

func TestHandler_GrafanaV2_FullLifecycle(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := buildRegistry()
	h := webhook.NewHandler(reg, receiver, nil)

	payload := `{
		"status": "firing",
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "HighCPU", "severity": "critical", "environment": "prod"},
				"annotations": {"summary": "CPU High", "description": "CPU at 95%"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "grafana-fp"
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("X-Grafana-Origin", "alert")
	req.Header.Set("Content-Type", "application/json")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body: %s", rw.Code, http.StatusAccepted, rw.Body.String())
	}
	if got := receiver.received(); len(got) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(got))
	}
	if receiver.received()[0].Source != model.AlertSourceGrafana {
		t.Errorf("source = %v", receiver.received()[0].Source)
	}
}

func TestHandler_AlertManager_FullLifecycle(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := buildRegistry()
	h := webhook.NewHandler(reg, receiver, nil)

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
				"labels": {"alertname": "DiskFull", "severity": "warning"},
				"annotations": {"summary": "Disk is full"},
				"startsAt": "2024-01-01T00:00:00Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"fingerprint": "am-fp"
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", strings.NewReader(payload))
	req.Header.Set("X-Prometheus-Alert", "DiskFull")
	req.Header.Set("Content-Type", "application/json")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body: %s", rw.Code, http.StatusAccepted, rw.Body.String())
	}
	if len(receiver.received()) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(receiver.received()))
	}
}

func TestHandler_Generic_FullLifecycle(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := buildRegistry()
	h := webhook.NewHandler(reg, receiver, nil)

	payload := `{"title": "Custom Alert", "severity": "warning"}`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body: %s", rw.Code, http.StatusAccepted, rw.Body.String())
	}
}

func TestHandler_UnknownSource_Returns400(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := parser.NewRegistry() // empty registry
	h := webhook.NewHandler(reg, receiver, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestHandler_ReceiverError_Returns500(t *testing.T) {
	receiver := &fakeReceiver{err: errors.New("receiver failure")}
	reg := buildRegistry()
	h := webhook.NewHandler(reg, receiver, nil)

	payload := `{"title": "Test", "severity": "info"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rw.Code)
	}
}

func TestHandler_EmptyAlerts_Returns204(t *testing.T) {
	// Build a registry with only a parser that returns no alerts
	reg := parser.NewRegistry()
	reg.Register(&emptyParser{})

	receiver := &fakeReceiver{}
	h := webhook.NewHandler(reg, receiver, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Empty", "true")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rw.Code)
	}
}

func TestHandler_SignatureValidation_InvalidToken(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := buildRegistry()

	sourceConfigs := map[string]webhook.WebhookSourceConfig{
		"grafana": {Secret: "correctsecret", ValidateSignature: true},
	}
	h := webhook.NewHandler(reg, receiver, sourceConfigs)

	payload := `{
		"status": "firing",
		"alerts": [{"status":"firing","labels":{"alertname":"Test"},"annotations":{"summary":"Test"},"startsAt":"2024-01-01T00:00:00Z","endsAt":"0001-01-01T00:00:00Z","fingerprint":"fp"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("X-Grafana-Origin", "alert")
	req.Header.Set("Authorization", "Bearer wrongtoken")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rw.Code)
	}
}

func TestHandler_SignatureValidation_ValidToken(t *testing.T) {
	receiver := &fakeReceiver{}
	reg := buildRegistry()

	sourceConfigs := map[string]webhook.WebhookSourceConfig{
		"grafana": {Secret: "correctsecret", ValidateSignature: true},
	}
	h := webhook.NewHandler(reg, receiver, sourceConfigs)

	payload := `{
		"status": "firing",
		"alerts": [{"status":"firing","labels":{"alertname":"Test","severity":"info"},"annotations":{"summary":"Test"},"startsAt":"2024-01-01T00:00:00Z","endsAt":"0001-01-01T00:00:00Z","fingerprint":"fp"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("X-Grafana-Origin", "alert")
	req.Header.Set("Authorization", "Bearer correctsecret")

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", rw.Code, rw.Body.String())
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rw := httptest.NewRecorder()
	webhook.HealthHandler()(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rw.Code)
	}
	body := rw.Body.String()
	if !strings.Contains(body, "ok") {
		t.Errorf("expected 'ok' in response body, got %q", body)
	}
}

// emptyParser always matches but returns zero alerts.
type emptyParser struct{}

func (e *emptyParser) Source() string                                              { return "empty" }
func (e *emptyParser) CanParse(r *http.Request) bool                               { return r.Header.Get("X-Empty") == "true" }
func (e *emptyParser) ValidateSignature(r *http.Request, secret string) error      { return nil }
func (e *emptyParser) Parse(ctx context.Context, r *http.Request) ([]model.Alert, error) {
	return []model.Alert{}, nil
}
