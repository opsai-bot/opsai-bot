package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	cfg := Config{
		BaseURL:     baseURL,
		Model:       "llama3",
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		Temperature: 0.1,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func makeChatResponse(content string) chatResponse {
	return chatResponse{
		Message: chatMessage{Role: "assistant", Content: content},
	}
}

func TestDiagnose_Success(t *testing.T) {
	diagJSON := `{
		"root_cause": "OOMKilled pod",
		"severity": "critical",
		"confidence": 0.95,
		"explanation": "Pod ran out of memory",
		"suggested_actions": [
			{
				"description": "Increase memory limit",
				"commands": ["kubectl edit deployment myapp"],
				"risk": "low",
				"reversible": true
			}
		],
		"needs_more_info": false,
		"follow_up_queries": []
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeChatResponse(diagJSON))
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	req := outbound.DiagnosisRequest{
		AlertSummary: "Pod OOMKilled in namespace default",
		K8sContext:   "pod/myapp: OOMKilled",
		Environment:  "production",
	}

	result, err := client.Diagnose(context.Background(), req)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}

	if result.RootCause != "OOMKilled pod" {
		t.Errorf("RootCause = %q, want %q", result.RootCause, "OOMKilled pod")
	}
	if result.Severity != "critical" {
		t.Errorf("Severity = %q, want %q", result.Severity, "critical")
	}
	if result.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", result.Confidence)
	}
	if len(result.SuggestedActions) != 1 {
		t.Errorf("SuggestedActions len = %d, want 1", len(result.SuggestedActions))
	}
	if result.NeedsMoreInfo {
		t.Error("NeedsMoreInfo should be false")
	}
}

func TestDiagnose_RetryOnError(t *testing.T) {
	attempts := 0
	diagJSON := `{"root_cause":"retry worked","severity":"info","confidence":0.5,"explanation":"ok","suggested_actions":[],"needs_more_info":false,"follow_up_queries":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeChatResponse(diagJSON))
	}))
	defer srv.Close()

	cfg := Config{
		BaseURL:     srv.URL,
		Model:       "llama3",
		Timeout:     5 * time.Second,
		MaxRetries:  2,
		Temperature: 0.1,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.Diagnose(context.Background(), outbound.DiagnosisRequest{
		AlertSummary: "test alert",
		K8sContext:   "ctx",
		Environment:  "test",
	})
	if err != nil {
		t.Fatalf("Diagnose with retry: %v", err)
	}
	if result.RootCause != "retry worked" {
		t.Errorf("RootCause = %q, want %q", result.RootCause, "retry worked")
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestConverse_Success(t *testing.T) {
	convJSON := `{
		"reply": "Check the pod logs for OOM signals.",
		"suggested_actions": [],
		"needs_approval": false
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeChatResponse(convJSON))
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	req := outbound.ConversationRequest{
		ThreadID:    "thread-1",
		UserMessage: "What should I check first?",
		AlertID:     "alert-123",
		K8sContext:  "pod/myapp: Running",
	}

	resp, err := client.Converse(context.Background(), req)
	if err != nil {
		t.Fatalf("Converse: %v", err)
	}
	if resp.Reply != "Check the pod logs for OOM signals." {
		t.Errorf("Reply = %q", resp.Reply)
	}
	if resp.NeedsApproval {
		t.Error("NeedsApproval should be false")
	}
}

func TestHealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tagsResponse{})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	if err := client.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck should succeed: %v", err)
	}
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	if err := client.HealthCheck(context.Background()); err == nil {
		t.Error("HealthCheck should fail when server returns 503")
	}
}

func TestModelInfo(t *testing.T) {
	client := newTestClient(t, "http://localhost:11434")
	info, err := client.ModelInfo(context.Background())
	if err != nil {
		t.Fatalf("ModelInfo: %v", err)
	}
	if info.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", info.Provider, "ollama")
	}
	if info.Model != "llama3" {
		t.Errorf("Model = %q, want %q", info.Model, "llama3")
	}
}
