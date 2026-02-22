package config

import (
	"fmt"
	"strings"
)

// Validate checks the config for errors.
func Validate(cfg *Config) error {
	var errs []string

	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		errs = append(errs, "server.port must be between 1 and 65535")
	}

	validProviders := map[string]bool{"ollama": true, "claude": true, "openai": true}
	if !validProviders[cfg.LLM.Provider] {
		errs = append(errs, fmt.Sprintf("llm.provider must be one of: ollama, claude, openai (got %q)", cfg.LLM.Provider))
	}

	if cfg.LLM.Provider == "ollama" && cfg.LLM.Ollama.BaseURL == "" {
		errs = append(errs, "llm.ollama.baseURL is required when provider is ollama")
	}

	if cfg.LLM.Provider == "claude" && cfg.LLM.Claude.APIKey == "" {
		errs = append(errs, "llm.claude.apiKey is required when provider is claude")
	}

	if cfg.LLM.Provider == "openai" && cfg.LLM.OpenAI.APIKey == "" {
		errs = append(errs, "llm.openai.apiKey is required when provider is openai")
	}

	validDrivers := map[string]bool{"sqlite": true, "postgres": true}
	if !validDrivers[cfg.Database.Driver] {
		errs = append(errs, fmt.Sprintf("database.driver must be sqlite or postgres (got %q)", cfg.Database.Driver))
	}

	if cfg.Database.Driver == "sqlite" && cfg.Database.SQLite.Path == "" {
		errs = append(errs, "database.sqlite.path is required when driver is sqlite")
	}

	if cfg.Slack.Enabled {
		if cfg.Slack.BotToken == "" {
			errs = append(errs, "slack.botToken is required when slack is enabled")
		}
		if cfg.Slack.AppToken == "" {
			errs = append(errs, "slack.appToken is required when slack is enabled")
		}
	}

	for name, env := range cfg.Policy.Environments {
		validModes := map[string]bool{"auto_fix": true, "warn_auto": true, "approval_required": true}
		if !validModes[env.Mode] {
			errs = append(errs, fmt.Sprintf("policy.environments.%s.mode must be auto_fix, warn_auto, or approval_required", name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
