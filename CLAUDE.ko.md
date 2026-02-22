# opsai-bot - 개발 가이드

AI 기반 Kubernetes 디버깅 및 자동 조치 도구의 프로젝트 고유 개발 지침

## 프로젝트 개요

**opsai-bot**은 Kubernetes 클러스터의 운영 이슈를 자동으로 진단하고 조치하는 지능형 봇입니다.

- Webhook 수신: Grafana, AlertManager, PagerDuty 등
- LLM 기반 분석: Ollama (로컬), Claude, OpenAI
- 환경별 정책: dev(auto_fix), staging(warn_auto), prod(approval_required)
- Slack 인터랙션: 스레드 기반 대화형 디버깅
- 감사 로그: 모든 작업 추적

## 기술 스택

### 언어 & 런타임
- **Go 1.25.6** - 메인 언어
- **CGO 활성화** - SQLite 지원 필요

### 코어 의존성
- `k8s.io/client-go v0.35.1` - Kubernetes 조작
- `slack-go/slack v0.18.0` - Slack Bot (Socket Mode)
- `mattn/go-sqlite3 v1.14.34` - SQLite 드라이버 (CGO 필요)
- `gopkg.in/yaml.v3 v3.0.1` - 설정 파일 파싱

### 아키텍처 패턴
- **Hexagonal Architecture** (Ports & Adapters 패턴)
- 도메인과 외부 시스템 완전 분리
- 테스트 용이한 인터페이스 기반 설계

### 데이터베이스
- **SQLite** (기본값, 로컬/개발용)
  - 경로: `/data/opsai-bot.db`
  - WAL 모드 활성화
  - 마이그레이션: `internal/adapter/outbound/persistence/sqlite/migration/`
- **PostgreSQL** (프로덕션, 선택사항)
  - 환경변수 기반 설정
  - 커넥션 풀 지원

### LLM 프로바이더
- **Ollama** (로컬, 기본값)
  - HTTP API 기반
  - 모델: `llama3:8b`, `mistral`, `neural-chat` 등
- **Claude** (Anthropic)
  - API 키: `CLAUDE_API_KEY`
- **OpenAI**
  - API 키: `OPENAI_API_KEY`

## 디렉토리 구조

```
internal/
├── domain/                    # 비즈니스 로직 (프레임워크 의존성 없음)
│   ├── model/                # 도메인 엔티티 (값 객체)
│   │   ├── alert.go          # 알림 (불변)
│   │   ├── action.go         # 작업 계획 (불변)
│   │   ├── analysis.go       # LLM 분석 결과 (불변)
│   │   ├── audit.go          # 감사 로그 (불변)
│   │   ├── conversation.go   # Slack 스레드 대화 (불변)
│   │   └── policy.go         # 정책 설정
│   │
│   ├── port/                 # 인터페이스 (계약)
│   │   ├── inbound/          # 들어오는 포트
│   │   │   ├── alert_receiver.go  # AlertReceiverPort
│   │   │   └── interaction.go     # InteractionPort
│   │   └── outbound/         # 나가는 포트
│   │       ├── k8s_executor.go    # K8sExecutor
│   │       ├── llm_provider.go    # LLMProvider
│   │       ├── notifier.go        # Notifier
│   │       └── repository.go      # Repository 인터페이스들
│   │
│   └── service/              # 비즈니스 로직 (조율, 분석)
│       ├── orchestrator.go        # 주 조율 엔진
│       ├── orchestrator_test.go
│       ├── analyzer.go            # LLM 분석 로직
│       ├── analyzer_test.go
│       ├── action_planner.go      # 작업 계획
│       ├── action_planner_test.go
│       ├── policy_evaluator.go    # 정책 평가
│       └── policy_evaluator_test.go
│
├── adapter/                  # 외부 시스템 통합
│   ├── inbound/             # 들어오는 어댑터
│   │   ├── webhook/         # HTTP Webhook 수신
│   │   │   ├── handler.go
│   │   │   ├── handler_test.go
│   │   │   ├── server.go
│   │   │   ├── middleware/  # 인증, 로깅, 속도 제한
│   │   │   │   ├── auth.go
│   │   │   │   ├── logging.go
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── bodyreader.go
│   │   │   └── parser/      # Webhook 파서
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
│   │       └── template/    # Block Kit 템플릿
│   │           ├── alert_card.go
│   │           ├── analysis_card.go
│   │           ├── approval_card.go
│   │           └── *_test.go
│   │
│   └── outbound/            # 나가는 어댑터
│       ├── llm/             # LLM 클라이언트
│       │   ├── ollama/
│       │   │   ├── client.go
│       │   │   └── client_test.go
│       │   ├── claude/      # 스텁
│       │   ├── openai/      # 스텁
│       │   └── prompt/      # 프롬프트 빌더
│       │       ├── builder.go
│       │       ├── builder_test.go
│       │       └── templates/
│       │           ├── diagnose.tmpl
│       │           └── conversation.tmpl
│       │
│       ├── kubernetes/      # K8s 클라이언트
│       │   ├── client.go    # 클라이언트 초기화
│       │   ├── executor.go  # 명령 실행
│       │   ├── executor_test.go
│       │   ├── reader.go    # 정보 조회
│       │   ├── whitelist.go # 화이트리스트
│       │   └── whitelist_test.go
│       │
│       ├── notification/    # 알림
│       │   ├── slack/
│       │   │   ├── notifier.go
│       │   │   └── notifier_test.go
│       │   └── webhook/
│       │
│       └── persistence/     # 데이터 저장소
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
│           └── postgres/    # PostgreSQL 구현 (스텁)
│
└── config/                  # 설정 관리
    ├── config.go           # YAML 로드 & 구조체
    ├── config_test.go
    └── validate.go         # 검증

cmd/
└── opsai/
    └── main.go             # 엔트리포인트 & DI

pkg/
├── version/
│   └── version.go          # 빌드 정보
├── health/
│   └── health.go           # 헬스 체크
└── ...

deploy/
├── helm/
│   └── opsai-bot/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│
configs/
├── config.yaml             # 기본 설정
└── policies/               # 정책 파일들

Makefile, Dockerfile, go.mod, go.sum, README.md
```

## 코딩 규칙

### 불변성 (Immutability) - 핵심!

**모든 도메인 모델은 불변입니다. 항상 새 객체를 반환하세요.**

```go
// WRONG - 뮤테이션
func (a *Alert) SetStatus(status AlertStatus) {
    a.Status = status        // 절대 금지!
    a.UpdatedAt = time.Now()
}

// CORRECT - 불변
func (a Alert) WithStatus(status AlertStatus) Alert {
    a.Status = status
    a.UpdatedAt = time.Now().UTC()
    return a                 // 새 객체 반환
}
```

**사용 예:**
```go
// 상태 업데이트
alert := alert.WithStatus(model.AlertStatusAnalyzing)

// 스레드 ID 설정
alert = alert.WithThreadID(threadID)

// 저장소에 업데이트
savedAlert, err := repo.Update(ctx, alert)
```

### 값 수신자 (Value Receivers)

모든 도메인 메서드는 값 수신자를 사용합니다:

```go
// 도메인 모델
func (a Alert) WithStatus(status AlertStatus) Alert { ... }
func (a Alert) IsTerminal() bool { ... }

// 어댑터는 포인터 수신자 가능
func (c *OllamaClient) Analyze(ctx context.Context, prompt string) (string, error) { ... }
```

### 에러 처리

항상 컨텍스트와 함께 에러를 래핑하세요:

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

### 인터페이스 분리

포트는 최소한으로 유지하고, 구현은 외부 어댑터에만:

```go
// port/outbound/k8s_executor.go - 최소한의 인터페이스
type K8sExecutor interface {
    Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
    Read(ctx context.Context, req ReadRequest) (ReadResult, error)
}

// adapter/outbound/kubernetes/executor.go - 구현
type Executor struct {
    clientset kubernetes.Interface
    whitelist *Whitelist
}

func (e *Executor) Exec(ctx context.Context, req outbound.ExecRequest) (outbound.ExecResult, error) {
    // 구현
}
```

### 테스트

표준 `testing` 패키지만 사용하고, 외부 의존성은 Mock/Fake로 처리:

```go
// Unit 테스트
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

### 파일 크기

- 도메인 모델: 100-200줄
- 서비스: 300-500줄
- 어댑터: 200-400줄
- 테스트: 같은 크기 또는 더 큼

파일이 800줄을 넘으면 분할하세요.

### 네이밍

**변수/함수/타입:**
- `camelCase` (Go 관례)
- 의미 있는 이름: `err` 대신 `parseErr`, `validationErr`

**패키지:**
- 소문자: `webhook`, `sqlite`, `kubernetes`
- 단수형: `adapter` 아님 `adapters`

**인터페이스:**
- 단일 메서드: `Reader`, `Writer`, `Parser`
- 다중 메서드: 역할 기반 `Repository`, `Executor`

## 빌드 & 테스트

### 빌드

```bash
# 기본 빌드
make build

# 또는 직접
CGO_ENABLED=1 go build -o bin/opsai-bot ./cmd/opsai/

# 버전 정보 포함
make build  # Makefile에 LDFLAGS 정의됨
```

**CGO_ENABLED=1이 필수인 이유:** SQLite 드라이버가 C 라이브러리를 필요로 함

### 테스트

```bash
# 전체 테스트
make test

# 단일 패키지
go test -v ./internal/domain/service/

# 커버리지
make coverage

# 특정 테스트
go test -run TestOrchestratorHandleAlert ./internal/domain/service/
```

**테스트 규칙:**
- 모든 `_test.go` 파일은 같은 패키지
- 외부 의존성은 반드시 Mock/Fake
- 테스트 헬퍼는 `testdata/` 또는 `fake_*` 구조체로

### 린트

```bash
make lint

# 또는
golangci-lint run ./...
```

**.golangci.yml 확인:** 프로젝트의 린트 규칙은 이 파일에 정의됨

## 주요 인터페이스 가이드

### LLMProvider (LLM 추상화)

```go
type LLMProvider interface {
    Analyze(ctx context.Context, prompt string) (string, error)
    IsHealthy(ctx context.Context) error
}
```

**새 프로바이더 추가 시:**
1. `internal/adapter/outbound/llm/{provider}/client.go` 생성
2. 인터페이스 구현
3. `cmd/opsai/main.go`에 초기화 로직 추가
4. `config.yaml`에 설정 섹션 추가

### K8sExecutor (Kubernetes 실행)

```go
type K8sExecutor interface {
    Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
    Read(ctx context.Context, req ReadRequest) (ReadResult, error)
}
```

**제약사항:**
- 화이트리스트 명령만 실행
- 차단된 네임스페이스 검사
- 타임아웃 적용 (기본 30초)

### Repository 인터페이스들

```go
type AlertRepository interface {
    Create(ctx context.Context, alert Alert) (Alert, error)
    Update(ctx context.Context, alert Alert) (Alert, error)
    GetByID(ctx context.Context, id string) (Alert, error)
    GetByFingerprint(ctx context.Context, fp string) (Alert, error)
    ListByEnvironment(ctx context.Context, env string, limit int) ([]Alert, error)
}
```

**특징:**
- 컨텍스트 활용 (취소, 타임아웃)
- 불변 모델 반환
- 원본 수정 없이 새 객체 반환

## 설정 관리

### 구조

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

### 환경 변수 오버라이드

```yaml
# config.yaml
slack:
  botToken: "${SLACK_BOT_TOKEN}"      # 환경변수 참조
  appToken: "${SLACK_APP_TOKEN}"
```

로드 시점에 `os.ExpandEnv()` 사용하여 확장됨

### 검증

```go
// internal/config/validate.go
func (c *Config) Validate() error {
    if c.LLM.Provider == "" {
        return fmt.Errorf("llm provider must be set")
    }
    // 더 많은 검증...
}
```

## 환경별 정책

### 정책 모델

```go
type EnvironmentPolicy struct {
    Mode         string              // auto_fix, warn_auto, approval_required
    MaxAutoRisk  string              // low, medium, high
    Approvers    []string            // Slack 멘션
    Namespaces   []string            // 빈 = 모든 네임스페이스
}
```

### 평가 로직

```go
// PolicyEvaluator.Evaluate()
// 1. 정책 매칭 (환경 + 네임스페이스)
// 2. 위험도 검사 (작업 위험도 <= maxAutoRisk)
// 3. 결정:
//    - auto_fix: 승인 불필요 → 바로 실행
//    - warn_auto: 알림 후 자동 실행
//    - approval_required: Slack 승인 대기
```

## 주요 서비스 플로우

### AlertReceiverPort 구현 (Orchestrator.HandleAlert)

```
1. Alert 저장
2. Slack 알림 (스레드 생성)
3. 상태 → Analyzing
4. LLM 분석
5. 작업 계획
6. 각 작업마다:
   a. 정책 평가
   b. 승인 필요 → 요청
   c. 자동 실행 → 실행
7. 상태 → Resolved/Acting/Failed
```

### InteractionPort 구현 (Orchestrator.HandleMessage, HandleApproval)

```
HandleMessage:
1. 기존 스레드 또는 신규 생성
2. 사용자 메시지 추가
3. 대화형 분석 (Analyzer.HandleConversation)
4. 스레드 저장
5. 감시 로그 기록

HandleApproval:
1. 작업 조회
2. 승인 여부 처리
3. 상태 업데이트
4. 필요시 실행
5. 감시 로그 기록
```

## 감시 로그 (Audit)

모든 주요 작업을 기록합니다:

```go
// 기록 유형
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

**기록 방법:**
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

## 개발 워크플로우

### 새 기능 추가 시

1. **도메인 모델 작성** (`internal/domain/model/`)
   - 불변 엔티티
   - 팩토리 메서드 (`New*`)
   - 상태 변경 메서드 (`With*`)

2. **포트 정의** (`internal/domain/port/`)
   - 필요한 인터페이스
   - Request/Response 구조체

3. **서비스 구현** (`internal/domain/service/`)
   - 비즈니스 로직
   - 테스트 작성

4. **어댑터 구현** (`internal/adapter/`)
   - 외부 시스템 통합
   - 포트 구현체

5. **설정 추가** (`configs/config.yaml` + `internal/config/config.go`)
   - 새 설정 필드
   - 검증 로직

6. **테스트 & 린트**
   ```bash
   make test
   make lint
   ```

### 버그 수정 시

1. 테스트를 먼저 작성 (실패하는 테스트)
2. 구현 수정
3. 테스트 통과 확인
4. 관련 테스트도 실행

## 메인 로직 시작점

```go
// cmd/opsai/main.go
// 1. 설정 로드
// 2. 저장소 초기화 (SQLite)
// 3. LLM 클라이언트 생성
// 4. K8s 클라이언트 생성
// 5. 도메인 서비스 초기화
// 6. Webhook 서버 시작
// 7. Slack Bot 시작
// 8. 신호 대기 (graceful shutdown)
```

## 문제 해결

### SQLite CGO 에러
```
error: cgo: C compiler not available
```
→ `CGO_ENABLED=1`로 빌드하거나, C 컴파일러 설치 (GCC/Clang)

### LLM 연결 실패
→ Ollama 서버 실행 확인: `curl http://localhost:11434/api/tags`

### K8s 권한 부족
→ ServiceAccount 권한 확인: `kubectl auth can-i get pods --as=system:serviceaccount:opsai:opsai-bot`

### 화이트리스트 거부
→ `configs/config.yaml`의 `kubernetes.whitelist` 확인

## 참고 자료

- **Kubernetes client-go**: https://github.com/kubernetes/client-go
- **Slack Go SDK**: https://github.com/slack-go/slack
- **Ollama API**: https://ollama.ai/library
- **Go Best Practices**: https://golang.org/doc/effective_go

---

**마지막 업데이트:** 2026-02-22
