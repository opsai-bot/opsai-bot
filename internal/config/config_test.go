package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected server.port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected server.readTimeout 30s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("expected server.writeTimeout 30s, got %v", cfg.Server.WriteTimeout)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("expected server.shutdownTimeout 15s, got %v", cfg.Server.ShutdownTimeout)
	}
	if cfg.Server.MetricsPort != 9090 {
		t.Errorf("expected server.metricsPort 9090, got %d", cfg.Server.MetricsPort)
	}

	// LLM defaults
	if cfg.LLM.Provider != "ollama" {
		t.Errorf("expected llm.provider ollama, got %q", cfg.LLM.Provider)
	}
	if cfg.LLM.MaxAnalysisRetries != 3 {
		t.Errorf("expected llm.maxAnalysisRetries 3, got %d", cfg.LLM.MaxAnalysisRetries)
	}
	if cfg.LLM.ConfidenceThreshold != 0.6 {
		t.Errorf("expected llm.confidenceThreshold 0.6, got %f", cfg.LLM.ConfidenceThreshold)
	}
	if cfg.LLM.Ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("expected ollama.baseURL http://localhost:11434, got %q", cfg.LLM.Ollama.BaseURL)
	}
	if cfg.LLM.Ollama.Model != "llama3:8b" {
		t.Errorf("expected ollama.model llama3:8b, got %q", cfg.LLM.Ollama.Model)
	}
	if cfg.LLM.Ollama.Temperature != 0.1 {
		t.Errorf("expected ollama.temperature 0.1, got %f", cfg.LLM.Ollama.Temperature)
	}
	if cfg.LLM.Ollama.ContextSize != 8192 {
		t.Errorf("expected ollama.contextSize 8192, got %d", cfg.LLM.Ollama.ContextSize)
	}

	// Kubernetes defaults
	if !cfg.Kubernetes.InCluster {
		t.Error("expected kubernetes.inCluster true")
	}
	if cfg.Kubernetes.ExecTimeout != 30*time.Second {
		t.Errorf("expected kubernetes.execTimeout 30s, got %v", cfg.Kubernetes.ExecTimeout)
	}
	if cfg.Kubernetes.LogTailLines != 100 {
		t.Errorf("expected kubernetes.logTailLines 100, got %d", cfg.Kubernetes.LogTailLines)
	}
	if len(cfg.Kubernetes.BlockedNamespaces) != 3 {
		t.Errorf("expected 3 blocked namespaces, got %d", len(cfg.Kubernetes.BlockedNamespaces))
	}

	// Database defaults
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected database.driver sqlite, got %q", cfg.Database.Driver)
	}
	if cfg.Database.SQLite.Path != "/data/opsai-bot.db" {
		t.Errorf("expected sqlite.path /data/opsai-bot.db, got %q", cfg.Database.SQLite.Path)
	}

	// Slack defaults
	if !cfg.Slack.Enabled {
		t.Error("expected slack.enabled true")
	}
	if cfg.Slack.DefaultChannel != "#ops-alerts" {
		t.Errorf("expected slack.defaultChannel #ops-alerts, got %q", cfg.Slack.DefaultChannel)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("expected logging.level info, got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected logging.format json, got %q", cfg.Logging.Format)
	}
}

func TestLoad(t *testing.T) {
	yaml := `
server:
  port: 9000
  metricsPort: 9091
llm:
  provider: ollama
  ollama:
    baseURL: "http://ollama:11434"
    model: "mistral:7b"
database:
  driver: sqlite
  sqlite:
    path: "/tmp/test.db"
slack:
  enabled: false
`
	f := writeTempYAML(t, yaml)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.MetricsPort != 9091 {
		t.Errorf("expected metricsPort 9091, got %d", cfg.Server.MetricsPort)
	}
	if cfg.LLM.Ollama.BaseURL != "http://ollama:11434" {
		t.Errorf("expected ollama baseURL http://ollama:11434, got %q", cfg.LLM.Ollama.BaseURL)
	}
	if cfg.LLM.Ollama.Model != "mistral:7b" {
		t.Errorf("expected ollama model mistral:7b, got %q", cfg.LLM.Ollama.Model)
	}
	if cfg.Database.SQLite.Path != "/tmp/test.db" {
		t.Errorf("expected sqlite path /tmp/test.db, got %q", cfg.Database.SQLite.Path)
	}
	if cfg.Slack.Enabled {
		t.Error("expected slack.enabled false")
	}
	// Verify defaults still apply to unset fields
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected default readTimeout 30s, got %v", cfg.Server.ReadTimeout)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f := writeTempYAML(t, ":::invalid yaml:::")
	_, err := Load(f)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("TEST_TOKEN", "secret-token-123")
	t.Setenv("TEST_PORT", "9999")

	input := "token: ${TEST_TOKEN}\nport: ${TEST_PORT}\nmissing: ${MISSING_VAR}"
	result := expandEnvVars(input)

	if result != "token: secret-token-123\nport: 9999\nmissing: ${MISSING_VAR}" {
		t.Errorf("unexpected expansion result:\n%s", result)
	}
}

func TestExpandEnvVars_InLoad(t *testing.T) {
	t.Setenv("OPSAI_DB_PATH", "/tmp/envtest.db")

	yaml := `
llm:
  provider: ollama
  ollama:
    baseURL: "http://localhost:11434"
database:
  driver: sqlite
  sqlite:
    path: "${OPSAI_DB_PATH}"
slack:
  enabled: false
`
	f := writeTempYAML(t, yaml)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.SQLite.Path != "/tmp/envtest.db" {
		t.Errorf("expected env-expanded path /tmp/envtest.db, got %q", cfg.Database.SQLite.Path)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	// DefaultConfig has slack.enabled=true but no tokens; disable for a clean valid config.
	cfg.Slack.Enabled = false

	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config to pass validation, got: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.Server.Port = 0

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for port 0, got nil")
	}
}

func TestValidate_InvalidPort_TooHigh(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.Server.Port = 99999

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for port 99999, got nil")
	}
}

func TestValidate_InvalidProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.LLM.Provider = "unknown"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for unknown provider, got nil")
	}
}

func TestValidate_InvalidDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.Database.Driver = "mongodb"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for unknown driver, got nil")
	}
}

func TestValidate_ClaudeRequiresAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.LLM.Provider = "claude"
	cfg.LLM.Claude.APIKey = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for missing claude API key, got nil")
	}
}

func TestValidate_OpenAIRequiresAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.LLM.Provider = "openai"
	cfg.LLM.OpenAI.APIKey = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for missing openai API key, got nil")
	}
}

func TestValidate_SlackRequiresTokens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = true
	cfg.Slack.BotToken = ""
	cfg.Slack.AppToken = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for missing slack tokens, got nil")
	}
}

func TestValidate_InvalidPolicyMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Slack.Enabled = false
	cfg.Policy.Environments["dev"] = EnvironmentPolicyConfig{
		Mode:        "invalid_mode",
		MaxAutoRisk: "medium",
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for invalid policy mode, got nil")
	}
}

func TestChannelForEnvironment(t *testing.T) {
	slack := &SlackConfig{
		DefaultChannel: "#ops-alerts",
		Channels: map[string]string{
			"dev":     "#ops-alerts-dev",
			"staging": "#ops-alerts-staging",
			"prod":    "#ops-alerts-prod",
		},
	}

	tests := []struct {
		env      string
		expected string
	}{
		{"dev", "#ops-alerts-dev"},
		{"staging", "#ops-alerts-staging"},
		{"prod", "#ops-alerts-prod"},
		{"unknown", "#ops-alerts"},
		{"", "#ops-alerts"},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			got := slack.ChannelForEnvironment(tt.env)
			if got != tt.expected {
				t.Errorf("ChannelForEnvironment(%q) = %q, want %q", tt.env, got, tt.expected)
			}
		})
	}
}

// writeTempYAML writes content to a temp file and returns its path.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp yaml: %v", err)
	}
	return f
}
