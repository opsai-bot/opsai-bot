package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/parser"
	"github.com/jonny/opsai-bot/internal/adapter/outbound/kubernetes"
	"github.com/jonny/opsai-bot/internal/adapter/outbound/llm/ollama"
	slacknotifier "github.com/jonny/opsai-bot/internal/adapter/outbound/notification/slack"
	"github.com/jonny/opsai-bot/internal/adapter/outbound/persistence/sqlite"
	"github.com/jonny/opsai-bot/internal/config"
	"github.com/jonny/opsai-bot/internal/domain/service"
	"github.com/jonny/opsai-bot/pkg/health"
	"github.com/jonny/opsai-bot/pkg/version"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println(version.String())
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger = buildLogger(cfg.Logging)

	// --- Database ---
	store, err := sqlite.NewStore(sqlite.Config{
		Path:              cfg.Database.SQLite.Path,
		MaxOpenConns:      cfg.Database.SQLite.MaxOpenConns,
		PragmaJournalMode: cfg.Database.SQLite.PragmaJournalMode,
		PragmaBusyTimeout: cfg.Database.SQLite.PragmaBusyTimeout,
	})
	if err != nil {
		logger.Error("failed to open sqlite store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// --- Repositories ---
	alertRepo := sqlite.NewAlertRepo(store)
	analysisRepo := sqlite.NewAnalysisRepo(store)
	actionRepo := sqlite.NewActionRepo(store)
	auditRepo := sqlite.NewAuditRepo(store)
	conversationRepo := sqlite.NewConversationRepo(store)
	policyRepo := sqlite.NewPolicyRepo(store)

	repos := service.Repositories{
		Alerts:        alertRepo,
		Analyses:      analysisRepo,
		Actions:       actionRepo,
		Audits:        auditRepo,
		Conversations: conversationRepo,
	}

	// --- Kubernetes ---
	whitelist := kubernetes.NewWhitelist(kubernetes.WhitelistConfig{
		ReadOnly:          cfg.Kubernetes.Whitelist.ReadOnly,
		Exec:              cfg.Kubernetes.Whitelist.Exec,
		Remediation:       cfg.Kubernetes.Whitelist.Remediation,
		BlockedNamespaces: cfg.Kubernetes.BlockedNamespaces,
	})

	k8sClientset, err := kubernetes.NewClientset(cfg.Kubernetes.InCluster, cfg.Kubernetes.Kubeconfig)
	if err != nil {
		logger.Warn("kubernetes clientset unavailable (local dev mode)", "error", err)
	}

	var k8sExecutor *kubernetes.Executor
	if k8sClientset != nil {
		k8sExecutor = kubernetes.NewExecutor(k8sClientset, whitelist, cfg.Kubernetes.ExecTimeout)
	}

	// --- LLM ---
	llmClient, err := ollama.NewClient(ollama.Config{
		BaseURL:      cfg.LLM.Ollama.BaseURL,
		Model:        cfg.LLM.Ollama.Model,
		Timeout:      cfg.LLM.Ollama.Timeout,
		MaxRetries:   cfg.LLM.Ollama.MaxRetries,
		SystemPrompt: cfg.LLM.Ollama.SystemPrompt,
		Temperature:  cfg.LLM.Ollama.Temperature,
	})
	if err != nil {
		logger.Error("failed to create LLM client", "error", err)
		os.Exit(1)
	}

	// --- Notifier ---
	notifier := slacknotifier.NewNotifier(slacknotifier.Config{
		BotToken:       cfg.Slack.BotToken,
		DefaultChannel: cfg.Slack.DefaultChannel,
		Channels:       cfg.Slack.Channels,
	})

	// --- Domain services ---
	if k8sExecutor == nil {
		logger.Error("kubernetes executor is required but unavailable; set kubernetes.inCluster=false and provide a kubeconfig for local dev")
		os.Exit(1)
	}

	analyzer := service.NewAnalyzer(llmClient, k8sExecutor)
	planner := service.NewActionPlanner(k8sExecutor)
	policyEval := service.NewPolicyEvaluator(policyRepo)
	orchestrator := service.NewOrchestrator(analyzer, planner, policyEval, notifier, k8sExecutor, repos)

	// --- Webhook ---
	reg := parser.NewRegistry()

	sourceConfigs := make(map[string]webhook.WebhookSourceConfig)
	for name, src := range cfg.Webhook.Sources {
		if src.Enabled {
			sourceConfigs[name] = webhook.WebhookSourceConfig{
				Secret:            src.Secret,
				ValidateSignature: src.Secret != "",
			}
		}
	}

	webhookHandler := webhook.NewHandler(reg, orchestrator, sourceConfigs)
	webhookServer := webhook.NewServer(webhook.ServerConfig{
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}, webhookHandler)

	// --- Health checker ---
	checker := health.NewChecker()
	checker.Register("database", func(ctx context.Context) error {
		return store.DB.PingContext(ctx)
	})
	if k8sClientset != nil {
		checker.Register("kubernetes", func(ctx context.Context) error {
			return k8sExecutor.HealthCheck(ctx)
		})
	}
	checker.Register("llm", func(ctx context.Context) error {
		return llmClient.HealthCheck(ctx)
	})

	// --- Metrics server ---
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/healthz", checker.LivenessHandler())
	metricsMux.HandleFunc("/readyz", checker.ReadinessHandler())
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.MetricsPort),
		Handler: metricsMux,
	}

	// --- Signal handling & startup ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gCtx := errgroup.WithContext(ctx)

	// Webhook HTTP server.
	g.Go(func() error {
		logger.Info("starting webhook server", "port", cfg.Server.Port)
		return webhookServer.Start(gCtx)
	})

	// Metrics/health server.
	g.Go(func() error {
		logger.Info("starting metrics server", "port", cfg.Server.MetricsPort)
		errCh := make(chan error, 1)
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
			close(errCh)
		}()
		select {
		case <-gCtx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
			defer cancel()
			return metricsServer.Shutdown(shutdownCtx)
		case err := <-errCh:
			return err
		}
	})

	// Slack bot (optional).
	if cfg.Slack.Enabled && cfg.Slack.BotToken != "" && cfg.Slack.AppToken != "" {
		g.Go(func() error {
			logger.Info("starting slack bot")
			bot := slackbot.NewBot(slackbot.Config{
				BotToken: cfg.Slack.BotToken,
				AppToken: cfg.Slack.AppToken,
			}, orchestrator)
			return bot.Start(gCtx)
		})
	} else {
		logger.Info("slack bot disabled or tokens not configured")
	}

	logger.Info("opsai-bot started", "version", version.String())

	if err := g.Wait(); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("opsai-bot stopped")
}

// buildLogger constructs a slog.Logger based on config.
func buildLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
