package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	LLM        LLMConfig        `yaml:"llm"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Webhook    WebhookConfig    `yaml:"webhook"`
	Slack      SlackConfig      `yaml:"slack"`
	Policy     PolicyConfig     `yaml:"policy"`
	Database   DatabaseConfig   `yaml:"database"`
	Logging    LoggingConfig    `yaml:"logging"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
	MetricsPort     int           `yaml:"metricsPort"`
}

type LLMConfig struct {
	Provider            string       `yaml:"provider"`
	Ollama              OllamaConfig `yaml:"ollama"`
	Claude              ClaudeConfig `yaml:"claude"`
	OpenAI              OpenAIConfig `yaml:"openai"`
	MaxAnalysisRetries  int          `yaml:"maxAnalysisRetries"`
	ConfidenceThreshold float64      `yaml:"confidenceThreshold"`
}

type OllamaConfig struct {
	BaseURL      string        `yaml:"baseURL"`
	Model        string        `yaml:"model"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxRetries   int           `yaml:"maxRetries"`
	SystemPrompt string        `yaml:"systemPrompt"`
	Temperature  float64       `yaml:"temperature"`
	ContextSize  int           `yaml:"contextSize"`
}

type ClaudeConfig struct {
	APIKey    string        `yaml:"apiKey"`
	Model     string        `yaml:"model"`
	MaxTokens int           `yaml:"maxTokens"`
	Timeout   time.Duration `yaml:"timeout"`
}

type OpenAIConfig struct {
	APIKey    string        `yaml:"apiKey"`
	Model     string        `yaml:"model"`
	MaxTokens int           `yaml:"maxTokens"`
	Timeout   time.Duration `yaml:"timeout"`
}

type KubernetesConfig struct {
	InCluster         bool            `yaml:"inCluster"`
	Kubeconfig        string          `yaml:"kubeconfig"`
	Whitelist         WhitelistConfig `yaml:"whitelist"`
	BlockedNamespaces []string        `yaml:"blockedNamespaces"`
	ExecTimeout       time.Duration   `yaml:"execTimeout"`
	LogTailLines      int64           `yaml:"logTailLines"`
}

type WhitelistConfig struct {
	ReadOnly    []string `yaml:"readOnly"`
	Exec        []string `yaml:"exec"`
	Remediation []string `yaml:"remediation"`
}

type WebhookConfig struct {
	Sources       map[string]WebhookSourceConfig `yaml:"sources"`
	Deduplication DeduplicationConfig            `yaml:"deduplication"`
	RateLimit     RateLimitConfig                `yaml:"rateLimit"`
}

type WebhookSourceConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Path     string `yaml:"path"`
	Secret   string `yaml:"secret"`
	AuthType string `yaml:"authType"`
}

type DeduplicationConfig struct {
	Enabled bool          `yaml:"enabled"`
	Window  time.Duration `yaml:"window"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requestsPerMinute"`
}

type SlackConfig struct {
	Enabled        bool              `yaml:"enabled"`
	BotToken       string            `yaml:"botToken"`
	AppToken       string            `yaml:"appToken"`
	SigningSecret  string            `yaml:"signingSecret"`
	DefaultChannel string            `yaml:"defaultChannel"`
	Channels       map[string]string `yaml:"channels"`
	Interaction    InteractionConfig `yaml:"interaction"`
}

type InteractionConfig struct {
	ApprovalTimeout    time.Duration `yaml:"approvalTimeout"`
	AutoExecDelay      time.Duration `yaml:"autoExecDelay"`
	ThreadHistoryLimit int           `yaml:"threadHistoryLimit"`
}

type PolicyConfig struct {
	Environments map[string]EnvironmentPolicyConfig `yaml:"environments"`
	CustomRules  []CustomRuleConfig                 `yaml:"customRules"`
}

type EnvironmentPolicyConfig struct {
	Mode        string   `yaml:"mode"`
	MaxAutoRisk string   `yaml:"maxAutoRisk"`
	Approvers   []string `yaml:"approvers"`
	Namespaces  []string `yaml:"namespaces"`
}

type CustomRuleConfig struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Condition   ConditionConfig `yaml:"condition"`
	Effect      string          `yaml:"effect"`
	Priority    int             `yaml:"priority"`
}

type ConditionConfig struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
	Value    string `yaml:"value"`
}

type DatabaseConfig struct {
	Driver   string         `yaml:"driver"`
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path              string `yaml:"path"`
	MaxOpenConns      int    `yaml:"maxOpenConns"`
	PragmaJournalMode string `yaml:"pragmaJournalMode"`
	PragmaBusyTimeout int    `yaml:"pragmaBusyTimeout"`
}

type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"sslMode"`
	MaxOpenConns    int           `yaml:"maxOpenConns"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// Load reads a YAML config file and returns a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded := expandEnvVars(string(data))

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 15 * time.Second,
			MetricsPort:     9090,
		},
		LLM: LLMConfig{
			Provider:            "ollama",
			MaxAnalysisRetries:  3,
			ConfidenceThreshold: 0.6,
			Ollama: OllamaConfig{
				BaseURL:     "http://localhost:11434",
				Model:       "llama3:8b",
				Timeout:     120 * time.Second,
				MaxRetries:  3,
				Temperature: 0.1,
				ContextSize: 8192,
			},
		},
		Kubernetes: KubernetesConfig{
			InCluster:    true,
			ExecTimeout:  30 * time.Second,
			LogTailLines: 100,
			BlockedNamespaces: []string{"kube-system", "kube-public", "kube-node-lease"},
			Whitelist: WhitelistConfig{
				ReadOnly:    []string{"get", "describe", "logs", "top", "events"},
				Exec:        []string{"cat", "ls", "df", "free", "top", "ps", "netstat", "ss", "curl", "nslookup", "dig", "ping"},
				Remediation: []string{"rollout restart", "scale", "delete pod"},
			},
		},
		Webhook: WebhookConfig{
			Sources: map[string]WebhookSourceConfig{
				"grafana":      {Enabled: true, Path: "/webhooks/grafana", AuthType: "bearer"},
				"alertmanager": {Enabled: true, Path: "/webhooks/alertmanager", AuthType: "bearer"},
				"generic":      {Enabled: true, Path: "/webhooks/generic", AuthType: "bearer"},
			},
			Deduplication: DeduplicationConfig{Enabled: true, Window: 5 * time.Minute},
			RateLimit:     RateLimitConfig{Enabled: true, RequestsPerMinute: 60},
		},
		Slack: SlackConfig{
			Enabled:        true,
			DefaultChannel: "#ops-alerts",
			Channels:       map[string]string{"dev": "#ops-alerts-dev", "staging": "#ops-alerts-staging", "prod": "#ops-alerts-prod"},
			Interaction: InteractionConfig{
				ApprovalTimeout:    30 * time.Minute,
				AutoExecDelay:      5 * time.Second,
				ThreadHistoryLimit: 50,
			},
		},
		Policy: PolicyConfig{
			Environments: map[string]EnvironmentPolicyConfig{
				"dev":     {Mode: "auto_fix", MaxAutoRisk: "medium"},
				"staging": {Mode: "warn_auto", MaxAutoRisk: "medium"},
				"prod":    {Mode: "approval_required", MaxAutoRisk: "low", Approvers: []string{"@oncall-team"}},
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{
				Path:              "/data/opsai-bot.db",
				MaxOpenConns:      1,
				PragmaJournalMode: "wal",
				PragmaBusyTimeout: 5000,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// expandEnvVars replaces ${VAR} patterns with environment variable values.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return "${" + key + "}"
	})
}

// ChannelForEnvironment returns the Slack channel for the given environment.
func (c *SlackConfig) ChannelForEnvironment(env string) string {
	if ch, ok := c.Channels[env]; ok {
		return ch
	}
	return c.DefaultChannel
}
