package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/parser"
	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
)

// WebhookSourceConfig holds per-source configuration for a webhook endpoint.
type WebhookSourceConfig struct {
	// Secret is used for signature validation (Bearer token or HMAC secret).
	Secret string
	// ValidateSignature controls whether signature validation is enforced.
	ValidateSignature bool
}

// Handler is the main HTTP handler for incoming webhook alerts.
type Handler struct {
	registry      *parser.Registry
	receiver      inbound.AlertReceiverPort
	sourceConfigs map[string]WebhookSourceConfig
}

// NewHandler creates a new Handler with the given registry, receiver, and per-source configs.
func NewHandler(
	registry *parser.Registry,
	receiver inbound.AlertReceiverPort,
	sourceConfigs map[string]WebhookSourceConfig,
) *Handler {
	return &Handler{
		registry:      registry,
		receiver:      receiver,
		sourceConfigs: sourceConfigs,
	}
}

// ServeHTTP handles an incoming webhook request:
// 1. Resolves the correct parser for the request.
// 2. Optionally validates the signature using the source config.
// 3. Parses the payload into alerts.
// 4. Sends alerts to the receiver.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.registry.Resolve(r)
	if err != nil {
		http.Error(w, "unsupported webhook source", http.StatusBadRequest)
		return
	}

	cfg, hasCfg := h.sourceConfigs[p.Source()]
	if hasCfg && cfg.ValidateSignature {
		if err := p.ValidateSignature(r, cfg.Secret); err != nil {
			http.Error(w, "signature validation failed", http.StatusUnauthorized)
			return
		}
	}

	alerts, err := p.Parse(r.Context(), r)
	if err != nil {
		http.Error(w, "failed to parse webhook payload", http.StatusBadRequest)
		return
	}

	if len(alerts) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.receiver.ReceiveAlerts(r.Context(), alerts); err != nil {
		http.Error(w, "failed to process alerts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"accepted": len(alerts),
	})
}

// HealthHandler returns an http.HandlerFunc for the /health endpoint.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
