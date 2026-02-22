# opsai-bot - Development Guide

Project-specific development guidelines for the AI-powered Kubernetes debugging and automated remediation tool.

## Project Overview

**opsai-bot** is an intelligent bot that automatically diagnoses and remediates operational issues in Kubernetes clusters.

- Webhook ingestion: Grafana, AlertManager, PagerDuty, etc.
- LLM-based analysis: Ollama (local), Claude, OpenAI
- Environment-specific policies: dev (auto_fix), staging (warn_auto), prod (approval_required)
- Slack interaction: thread-based conversational debugging
- Audit logging: full traceability of all actions

## Tech Stack

### Language & Runtime
- **Go 1.25.6** - primary language
- **CGO enabled** - required for SQLite support

### Core Dependencies
- `k8s.io/client-go v0.35.1` - Kubernetes operations
- `slack-go/slack v0.18.0` - Slack Bot (Socket Mode)
- `mattn/go-sqlite3 v1.14.34` - SQLite driver (requires CGO)
- `gopkg.in/yaml.v3 v3.0.1` - configuration file parsing

### Architecture Pattern
- **Hexagonal Architecture** (Ports & Adapters pattern)
- Complete separation between domain and external systems
- Interface-driven design for testability

### Database
- **SQLite** (default, for local/development use)
  - Path: `/data/opsai-bot.db`
  - WAL mode enabled
  - Migrations: `internal/adapter/outbound/persistence/sqlite/migration/`
- **PostgreSQL** (production, optional)
  - Environment variable-based configuration
  - Connection pool support

### LLM Providers
- **Ollama** (local, default)
  - HTTP API based
  - Models: `llama3:8b`, `mistral`, `neural-chat`, etc.
- **Claude** (Anthropic)
  - API key: `CLAUDE_API_KEY`
- **OpenAI**
  - API key: `OPENAI_API_KEY`

## Directory Structure

```
internal/
├── domain/                    # Business logic (no framework dependencies)
│   ├── model/                # Domain entities (value objects)
│   │   ├── alert.go          # Alert (immutable)
│   │   ├── action.go         # Action plan (immutable)
│   │   ├── analysis.go       # LLM analysis result (immutable)
│   │   ├── audit.go          # Audit log (immutable)
│   │   ├── conversation.go   # Slack thread conversation (immutable)
│   │   └── policy.go         # Policy configuration
│   │
│   ├── port/                 # Interfaces (contracts)
│   │   ├── inbound/          # Inbound ports
│   │   │   ├── alert_receiver.go  # AlertReceiverPort
│   │   │   └── interaction.go     # InteractionPort
│   │   └── outbound/         # Outbound ports
│   │       ├── k8s_executor.go    # K8sExecutor
│   │       ├── llm_provider.go    # LLMProvider
│   │       ├── notifier.go        # Notifier
│   │       └── repository.go      # Repository interfaces
│   │
│   └── service/              # Business logic (orchestration, analysis)
│       ├── orchestrator.go        # Primary orchestration engine
│       ├── orchestrator_test.go
│       ├── analyzer.go            # LLM analysis logic
│       ├── analyzer_test.go
│       ├── action_planner.go      # Action planning
│       ├── action_planner_test.go
│       ├── policy_evaluator.go    # Policy evaluation
│       └── policy_evaluator_test.go
│
├── adapter/                  # External system integrations
│   ├── inbound/             # Inbound adapters
│   │   ├── webhook/         # HTTP Webhook receiver
│   │   │   ├── handler.go
│   │   │   ├── handler_test.go
│   │   │   ├── server.go
│   │   │   ├── middleware/  # Auth, logging, rate limiting
│   │   │   │   ├── auth.go
│   │   │   │   ├── logging.go
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── bodyreader.go
│   │   │   └── parser/      # Webhook parsers
│   │   │       ├── registry.go
│   │   │       ├── grafana.go
│   │   │       ├── grafana_test.go
│   │   │       ├── alertmanager.go
│   │   │       ├── alertmanager_test.go
│   │   │       ├── generic.go
│   │   │       └── generic_test.go
│   │   │
│   │   └── slackbot/        # Slack Bot (Socket Mode)
│   │       ├── bot.go
│   │       ├── interaction.go
│   │       └── template/    # Block Kit templates
│   │           ├── alert_card.go
│   │           ├── analysis_card.go
│   │           ├── approval_card.go
│   │           └── *_test.go
│   │
│   └── outbound/            # Outbound adapters
│       ├── llm/             # LLM clients
│       │   ├── ollama/
│       │   │   ├── client.go
│       │   │   └── client_test.go
│       │   ├── claude/      # stub
│       │   ├── openai/      # stub
│       │   └── prompt/      # Prompt builder
│       │       ├── builder.go
│       │       ├── builder_test.go
│       │       └── templates/
│       │           ├── diagnose.tmpl
│       │           └── conversation.tmpl
│       │
│       ├── kubernetes/      # K8s client
│       │   ├── client.go    # Client initialization
│       │   ├── executor.go  # Command execution
│       │   ├── executor_test.go
│       │   ├── reader.go    # Information retrieval
│       │   ├── whitelist.go # Whitelist
│       │   └── whitelist_test.go
│       │
│       ├── notification/    # Notifications
│       │   ├── slack/
│       │   │   ├── notifier.go
│       │   │   └── notifier_test.go
│       │   └── webhook/
│       │
│       └── persistence/     # Data storage
│           ├── sqlite/
│           │   ├── store.go
│           │   ├── migration/
│           │   │   ├── migrator.go
│           │   │   └── migrations/
│           │   │       └── 001_initial.sql
│           │   ├── alert_repo.go
│           │   ├── alert_repo_test.go
│           │   ├── action_repo.go
│           │   ├── action_repo_test.go
│           │   ├── analysis_repo.go
│           │   ├── audit_repo.go
│           │   ├── conversation_repo.go
│           │   └── policy_repo.go
│           │
│           └── postgres/    # PostgreSQL implementation (stub)
│
└── config/                  # Configuration management
    ├── config.go           # YAML loading & structs
    ├── config_test.go
    └── validate.go         # Validation

cmd/
└── opsai/
    └── main.go             # Entrypoint & DI

pkg/
├── version/
│   └── version.go          # Build information
├── health/
│   └── health.go           # Health check
└── ...

deploy/
├── helm/
│   └── opsai-bot/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/

configs/
├── config.yaml             # Default configuration
└── policies/               # Policy files

Makefile, Dockerfile, go.mod, go.sum, README.md
```

## Coding Rules

### Immutability - Critical!

**All domain models are immutable. Always return a new object.**

```go
// WRONG - mutation
func (a *Alert) SetStatus(status AlertStatus) {
    a.Status = status        // Never do this!
    a.UpdatedAt = time.Now()
}

// CORRECT - immutable
func (a Alert) WithStatus(status AlertStatus) Alert {
    a.Status = status
    a.UpdatedAt = time.Now().UTC()
    return a                 // Return new object
}
```

**Usage examples:**
```go
// Update status
alert := alert.WithStatus(model.AlertStatusAnalyzing)

// Set thread ID
alert = alert.WithThreadID(threadID)

// Update in repository
savedAlert, err := repo.Update(ctx, alert)
```

### Value Receivers

All domain methods use value receivers:

```go
// Domain model
func (a Alert) WithStatus(status AlertStatus) Alert { ... }
func (a Alert) IsTerminal() bool { ... }

// Adapters may use pointer receivers
func (c *OllamaClient) Analyze(ctx context.Context, prompt string) (string, error) { ... }
```

### Error Handling

Always wrap errors with context:

```go
// WRONG
return err

// CORRECT
return fmt.Errorf("context: %w", err)

// EXAMPLE
if err := o.repos.Alerts.Create(ctx, alert); err != nil {
    return fmt.Errorf("save alert: %w", err)
}
```

### Interface Segregation

Keep ports minimal; implementations belong only in external adapters:

```go
// port/outbound/k8s_executor.go - minimal interface
type K8sExecutor interface {
    Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
    Read(ctx context.Context, req ReadRequest) (ReadResult, error)
}

// adapter/outbound/kubernetes/executor.go - implementation
type Executor struct {
    clientset kubernetes.Interface
    whitelist *Whitelist
}

func (e *Executor) Exec(ctx context.Context, req outbound.ExecRequest) (outbound.ExecResult, error) {
    // implementation
}
```

### Testing

Use the standard `testing` package only; handle external dependencies with Mock/Fake:

```go
// Unit test
func TestOrchestratorHandleAlert(t *testing.T) {
    // Arrange
    mockAnalyzer := &mockAnalyzer{}
    mockK8s := &mockK8sExecutor{}
    repo := &fakeRepository{}
    orchestrator := service.NewOrchestrator(mockAnalyzer, ..., repo)

    alert := model.NewAlert(...)

    // Act
    err := orchestrator.HandleAlert(context.Background(), alert)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

### File Size

- Domain models: 100-200 lines
- Services: 300-500 lines
- Adapters: 200-400 lines
- Tests: same size or larger

Split any file that exceeds 800 lines.

### Naming

**Variables / Functions / Types:**
- `camelCase` (Go convention)
- Meaningful names: use `parseErr`, `validationErr` instead of just `err`

**Packages:**
- Lowercase: `webhook`, `sqlite`, `kubernetes`
- Singular: `adapter` not `adapters`

**Interfaces:**
- Single method: `Reader`, `Writer`, `Parser`
- Multiple methods: role-based `Repository`, `Executor`

## Build & Test

### Build

```bash
# Default build
make build

# Or directly
CGO_ENABLED=1 go build -o bin/opsai-bot ./cmd/opsai/

# With version information
make build  # LDFLAGS defined in Makefile
```

**Why CGO_ENABLED=1 is required:** The SQLite driver depends on a C library.

### Test

```bash
# Full test suite
make test

# Single package
go test -v ./internal/domain/service/

# Coverage
make coverage

# Specific test
go test -run TestOrchestratorHandleAlert ./internal/domain/service/
```

**Test rules:**
- All `_test.go` files belong to the same package
- External dependencies must always be Mock/Fake
- Test helpers go in `testdata/` or `fake_*` structs

### Lint

```bash
make lint

# Or
golangci-lint run ./...
```

**Check `.golangci.yml`:** Project lint rules are defined in this file.

## Key Interface Guide

### LLMProvider (LLM Abstraction)

```go
type LLMProvider interface {
    Analyze(ctx context.Context, prompt string) (string, error)
    IsHealthy(ctx context.Context) error
}
```

**When adding a new provider:**
1. Create `internal/adapter/outbound/llm/{provider}/client.go`
2. Implement the interface
3. Add initialization logic to `cmd/opsai/main.go`
4. Add a configuration section to `config.yaml`

### K8sExecutor (Kubernetes Execution)

```go
type K8sExecutor interface {
    Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
    Read(ctx context.Context, req ReadRequest) (ReadResult, error)
}
```

**Constraints:**
- Only whitelisted commands are executed
- Blocked namespace checks enforced
- Timeout applied (default 30 seconds)

### Repository Interfaces

```go
type AlertRepository interface {
    Create(ctx context.Context, alert Alert) (Alert, error)
    Update(ctx context.Context, alert Alert) (Alert, error)
    GetByID(ctx context.Context, id string) (Alert, error)
    GetByFingerprint(ctx context.Context, fp string) (Alert, error)
    ListByEnvironment(ctx context.Context, env string, limit int) ([]Alert, error)
}
```

**Characteristics:**
- Context propagation (cancellation, timeouts)
- Returns immutable models
- Returns new objects without modifying the original

## Configuration Management

### Structure

```go
// internal/config/config.go
type Config struct {
    Server     ServerConfig
    LLM        LLMConfig
    Kubernetes K8sConfig
    Webhook    WebhookConfig
    Slack      SlackConfig
    Policy     PolicyConfig
    Database   DatabaseConfig
    Logging    LoggingConfig
}
```

### Environment Variable Override

```yaml
# config.yaml
slack:
  botToken: "${SLACK_BOT_TOKEN}"      # Environment variable reference
  appToken: "${SLACK_APP_TOKEN}"
```

Expanded at load time via `os.ExpandEnv()`.

### Validation

```go
// internal/config/validate.go
func (c *Config) Validate() error {
    if c.LLM.Provider == "" {
        return fmt.Errorf("llm provider must be set")
    }
    // more validation...
}
```

## Environment-Specific Policies

### Policy Model

```go
type EnvironmentPolicy struct {
    Mode         string              // auto_fix, warn_auto, approval_required
    MaxAutoRisk  string              // low, medium, high
    Approvers    []string            // Slack mentions
    Namespaces   []string            // empty = all namespaces
}
```

### Evaluation Logic

```go
// PolicyEvaluator.Evaluate()
// 1. Policy matching (environment + namespace)
// 2. Risk check (action risk <= maxAutoRisk)
// 3. Decision:
//    - auto_fix: no approval required → execute immediately
//    - warn_auto: notify then execute automatically
//    - approval_required: wait for Slack approval
```

## Key Service Flows

### AlertReceiverPort Implementation (Orchestrator.HandleAlert)

```
1. Save alert
2. Slack notification (create thread)
3. Status → Analyzing
4. LLM analysis
5. Action planning
6. For each action:
   a. Policy evaluation
   b. Approval required → request approval
   c. Auto-execute → execute
7. Status → Resolved / Acting / Failed
```

### InteractionPort Implementation (Orchestrator.HandleMessage, HandleApproval)

```
HandleMessage:
1. Existing thread or create new
2. Append user message
3. Conversational analysis (Analyzer.HandleConversation)
4. Save thread
5. Record audit log

HandleApproval:
1. Retrieve action
2. Process approval decision
3. Update status
4. Execute if needed
5. Record audit log
```

## Audit Logging

All significant operations are recorded:

```go
// Audit types
const (
    AuditAlertReceived     AuditType = "alert_received"
    AuditAnalysisStarted             = "analysis_started"
    AuditAnalysisCompleted           = "analysis_completed"
    AuditPolicyEvaluated             = "policy_evaluated"
    AuditActionPending               = "action_pending"
    AuditActionApproved              = "action_approved"
    AuditActionRejected              = "action_rejected"
    AuditActionExecuting             = "action_executing"
    AuditActionCompleted             = "action_completed"
    AuditActionFailed                = "action_failed"
    AuditConversation                = "conversation"
)
```

**How to record:**
```go
auditLog := model.NewAuditLog(
    model.AuditActionApproved,
    alertID,
    "user@slack",
    "prod",
    "action approved by on-call",
)
_ = repos.Audits.Create(ctx, auditLog)
```

## Development Workflow

### Adding a New Feature

1. **Write domain model** (`internal/domain/model/`)
   - Immutable entity
   - Factory method (`New*`)
   - State transition methods (`With*`)

2. **Define ports** (`internal/domain/port/`)
   - Required interfaces
   - Request/Response structs

3. **Implement service** (`internal/domain/service/`)
   - Business logic
   - Write tests

4. **Implement adapter** (`internal/adapter/`)
   - External system integration
   - Port implementation

5. **Add configuration** (`configs/config.yaml` + `internal/config/config.go`)
   - New configuration fields
   - Validation logic

6. **Test & Lint**
   ```bash
   make test
   make lint
   ```

### Fixing a Bug

1. Write the test first (failing test)
2. Fix the implementation
3. Confirm the test passes
4. Run related tests as well

## Main Logic Entry Point

```go
// cmd/opsai/main.go
// 1. Load configuration
// 2. Initialize repository (SQLite)
// 3. Create LLM client
// 4. Create K8s client
// 5. Initialize domain services
// 6. Start Webhook server
// 7. Start Slack Bot
// 8. Wait for signal (graceful shutdown)
```

## Troubleshooting

### SQLite CGO Error
```
error: cgo: C compiler not available
```
→ Build with `CGO_ENABLED=1`, or install a C compiler (GCC/Clang).

### LLM Connection Failure
→ Verify the Ollama server is running: `curl http://localhost:11434/api/tags`

### Insufficient K8s Permissions
→ Check ServiceAccount permissions: `kubectl auth can-i get pods --as=system:serviceaccount:opsai:opsai-bot`

### Whitelist Rejection
→ Check `kubernetes.whitelist` in `configs/config.yaml`

## References

- **Kubernetes client-go**: https://github.com/kubernetes/client-go
- **Slack Go SDK**: https://github.com/slack-go/slack
- **Ollama API**: https://ollama.ai/library
- **Go Best Practices**: https://golang.org/doc/effective_go

---

**Last updated:** 2026-02-22
