# opsai-bot

AI 기반 Kubernetes 디버깅 및 자동 조치 도구 (AI-Powered Kubernetes Debugging & Automated Remediation Tool)

## 프로젝트 개요

**opsai-bot**은 Kubernetes 클러스터의 운영 이슈를 자동으로 진단하고 조치하는 지능형 봇입니다.

### 핵심 기능

- **통합 Webhook 수신**: Grafana, AlertManager, PagerDuty 등 다양한 모니터링 플랫폼에서 알림 수신
- **LLM 기반 분석**: Ollama, Claude, OpenAI 등 다양한 LLM을 활용한 근본 원인 분석 (RCA)
- **화이트리스트 기반 K8s 실행**: 안전하고 제한된 Kubernetes 명령 실행
- **환경별 정책 엔진**: 개발(auto_fix), 스테이징(warn_auto), 운영(approval_required) 별 정책 관리
- **대화형 Slack 인터페이스**: Slack 스레드를 통한 인터랙티브 디버깅
- **감시 감사 로그**: 모든 작업의 완전한 감시 추적 (Audit Trail)

### 사용 사례

```
Grafana Alert
    ↓
opsai-bot이 수신 및 분석
    ↓
LLM이 근본 원인 파악
    ↓
정책 검사 (환경별 자동화 레벨)
    ↓
승인 필요 → Slack에서 승인 / 자동 실행
    ↓
작업 실행 및 결과 보고
    ↓
Slack으로 완료 알림
```

---

## 아키텍처

### 시스템 다이어그램

```
┌────────────────────────────────────────────────────────────────┐
│ 외부 시스템 (External Systems)                                   │
│ ┌──────────────┬──────────────────┬──────────────┐              │
│ │   Grafana    │  AlertManager    │  PagerDuty   │              │
│ └──────┬───────┴────────┬─────────┴──────┬───────┘              │
│        │                │                │                     │
└────────┼────────────────┼────────────────┼─────────────────────┘
         │                │                │
         └─── Webhook ───→┌─────────────────────────────────────┐
                          │  HTTP Server & Parser (Inbound)    │
                          │ ├─ Webhook Handler                  │
                          │ ├─ Grafana Parser                   │
                          │ ├─ AlertManager Parser              │
                          │ └─ Generic Parser                   │
                          │  (middleware: auth, logging, etc)   │
                          └──────────────┬──────────────────────┘
                                         │
                                         ▼
┌──────────────────────────────────────────────────────────────────┐
│ Domain Services (Business Logic)                                 │
│ ┌──────────────┬──────────────────┬──────────────────────┐      │
│ │ Orchestrator │   PolicyEvaluator│  ActionPlanner       │      │
│ │              │                  │                      │      │
│ │ • AlertFlow  │ • Policy Match   │ • Generate Actions   │      │
│ │ • Execution  │ • Risk Eval      │ • Command Building   │      │
│ │ • Status Mgmt│ • Auto/Approval  │ • Suggestion List    │      │
│ └──────────────┴──────────────────┴──────────────────────┘      │
│        ▲            ▲                    ▲                      │
│        │            │                    │                      │
│    ┌───┴────────────┴────────────────────┴─────┐                │
│    │        Analyzer (LLM Interface)           │                │
│    │ • Alert Analysis                          │                │
│    │ • Conversation Handling                   │                │
│    │ • Prompt Template Management              │                │
│    └───┬────────────────────────────────────────┘                │
└────────┼──────────────────────────────────────────────────────────┘
         │
         ▼
┌──────────────────────────────────────────────────────────────────┐
│ Outbound Adapters                                                │
│ ┌─────────────────────┬────────────┬──────────────────┐          │
│ │   LLM Providers     │  K8s       │  Notification    │          │
│ │ ┌─ Ollama          │ ┌─ Reader  │ ┌─ Slack         │          │
│ │ ┌─ Claude API      │ ├─ Executor│ └─ Webhook       │          │
│ │ ├─ OpenAI API      │ └─ Client  │                  │          │
│ │ └─ Prompt Builder  │   (K8s)    │                  │          │
│ └─────────────────────┴────────────┴──────────────────┘          │
│ ┌─────────────────────────────────────────────┐                 │
│ │  Data Persistence                           │                 │
│ │ ┌─ SQLite (default)  ┌─ PostgreSQL (optional)                │
│ │ │ Repository Pattern │ Repository Pattern                    │
│ │ │ • AlertRepository  │ • ActionRepository                    │
│ │ │ • AnalysisRepo     │ • AuditRepository                     │
│ │ │ • ConversationRepo │ • PolicyRepository                    │
│ │ └──────────────────┴──────────────────────┘                  │
│ └─────────────────────────────────────────────┘                 │
└──────────────────────────────────────────────────────────────────┘
         │                          │
         └──────┬───────────────────┘
                ▼
┌──────────────────────────────────────────────────────────────────┐
│ Slack Bot (Inbound Socket Mode)                                  │
│ ┌──────────────┬─────────────┐                                  │
│ │  Interaction │  Templates  │                                  │
│ │  Handler     │             │                                  │
│ │              │ • Alert Card│                                  │
│ │              │ • Analysis  │                                  │
│ │              │ • Approval  │                                  │
│ └──────────────┴─────────────┘                                  │
└──────────────────────────────────────────────────────────────────┘
         │
         ▼ (Thread interaction)
    Kubernetes Cluster
```

### 아키텍처 패턴

**Hexagonal Architecture (포트 & 어댑터 패턴)**

```
internal/
├── domain/               # 비즈니스 로직
│   ├── model/           # 도메인 엔티티 (Alert, Action, Analysis, etc)
│   ├── port/            # 인터페이스 정의
│   │   ├── inbound/     # 들어오는 포트 (AlertReceiver, Interaction)
│   │   └── outbound/    # 나가는 포트 (LLM, K8s, Notifier, Repository)
│   └── service/         # 비즈니스 로직 (Orchestrator, Analyzer, etc)
│
├── adapter/             # 외부 시스템 통합
│   ├── inbound/         # 들어오는 어댑터
│   │   ├── webhook/     # HTTP 웹훅 서버
│   │   └── slackbot/    # Slack Bot (Socket Mode)
│   └── outbound/        # 나가는 어댑터
│       ├── llm/         # LLM 클라이언트 (Ollama, Claude, OpenAI)
│       ├── kubernetes/  # K8s 클라이언트
│       ├── notification/# 알림 서비스 (Slack, Webhook)
│       └── persistence/ # 데이터 저장 (SQLite, PostgreSQL)
│
└── config/              # 설정 관리
```

---

## 주요 기능 상세

### 1. 범용 Webhook 수신

다양한 모니터링 플랫폼에서 알림을 받습니다.

**지원하는 소스:**
- Grafana (alert notification)
- Prometheus AlertManager
- PagerDuty (이벤트)
- 커스텀 Webhook

**기능:**
- 인증 (Bearer Token, 서명 검증)
- 요청 속도 제한 (Rate Limiting)
- 중복 제거 (Deduplication window)
- 구조화된 로깅

### 2. LLM 기반 분석

여러 LLM 프로바이더를 지원합니다.

**프로바이더:**
- **Ollama** (로컬, 비용 무료) - 기본값
- **Claude** (Anthropic API)
- **OpenAI** (GPT-4 등)

**분석 기능:**
- 알림 근본 원인 분석 (RCA)
- 신뢰도 점수 (Confidence Score)
- 개선 제안 자동 생성
- 대화형 추가 진단

### 3. 화이트리스트 기반 K8s 실행

안전성을 최우선으로 제한된 명령만 실행합니다.

**화이트리스트 카테고리:**

```yaml
readOnly:
  - get, describe, logs, top, events

exec:
  - cat, ls, df, free, top, ps
  - netstat, ss, curl, nslookup, dig, ping

remediation:
  - "rollout restart"
  - scale
  - "delete pod"
```

**보호 기능:**
- 차단된 네임스페이스 (kube-system, kube-public, kube-node-lease)
- 명령 타임아웃 (30초 기본)
- 출력 크기 제한

### 4. 환경별 정책 엔진

환경에 따라 다른 자동화 수준을 적용합니다.

```yaml
policy:
  environments:
    dev:
      mode: auto_fix                  # 자동 실행 (검사 없음)
      maxAutoRisk: medium             # 중위험까지 자동

    staging:
      mode: warn_auto                 # 알림 후 자동 실행
      maxAutoRisk: medium

    prod:
      mode: approval_required         # Slack에서 수동 승인 필수
      maxAutoRisk: low                # 저위험만 가능
      approvers:
        - "@oncall-team"
```

**정책 평가:**
- 알림 환경 감지
- 작업 위험도 분류 (low, medium, high)
- 정책 규칙 적용
- 승인 여부 결정

### 5. 대화형 Slack 인터페이스

Slack을 통한 인터랙티브 디버깅:

- **알림 카드**: 알림 발생 즉시 Slack에 스레드 생성
- **분석 결과**: 근본 원인, 신뢰도, 추천 작업 표시
- **승인 버튼**: prod 환경에서 작업 승인/거부
- **대화형 Q&A**: 스레드에서 추가 질문 가능
- **실행 결과**: 작업 완료 시 결과 피드백

### 6. 감사 로그

모든 작업을 추적 가능하게 기록합니다.

**기록 항목:**
- 알림 수신 (AlertReceived)
- 분석 시작/완료 (AnalysisStarted, AnalysisCompleted)
- 정책 평가 (PolicyEvaluated)
- 작업 승인/거부 (ActionApproved, ActionRejected)
- 작업 실행/완료/실패 (ActionExecuting, ActionCompleted, ActionFailed)
- 대화 (Conversation)

---

## 기술 스택

| 레이어 | 기술 |
|--------|------|
| **언어** | Go 1.25.6 |
| **패턴** | Hexagonal Architecture (Ports & Adapters) |
| **K8s** | client-go v0.35.1 |
| **LLM** | Ollama API (로컬), Claude/OpenAI (클라우드) |
| **메시징** | Slack Socket Mode (slack-go/slack v0.18.0) |
| **데이터베이스** | SQLite v1.14.34 (기본), PostgreSQL (선택사항) |
| **설정** | YAML (gopkg.in/yaml.v3) |
| **배포** | Helm 차트, Docker |

---

## 빠른 시작

### 사전 요구사항

- **Go 1.23+** (빌드용)
- **Docker & Docker Compose** (로컬 테스트용)
- **Kubernetes 클러스터** (1.20+)
- **LLM 프로바이더** (Ollama 로컬 또는 Claude/OpenAI API)
- **Slack Workspace** (Bot 권한)

### 1단계: 저장소 클론 및 설정

```bash
git clone https://github.com/jonny/opsai-bot.git
cd opsai-bot

# 의존성 다운로드
go mod download
```

### 2단계: 설정 파일 준비

```bash
# 기본 설정 파일 확인
cat configs/config.yaml

# 필요시 환경변수 설정
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
export OLLAMA_BASE_URL="http://localhost:11434"
```

### 3단계: 로컬 빌드 및 테스트

```bash
# 바이너리 빌드
make build

# 테스트 실행
make test

# 커버리지 확인
make coverage

# 린터 실행
make lint

# 실행
make run
```

### 4단계: Kubernetes 배포 (Helm)

```bash
cd deploy/helm/opsai-bot

# 네임스페이스 생성
kubectl create namespace opsai

# Helm 차트 배포
helm install opsai-bot . \
  --namespace opsai \
  --values values.yaml

# 상태 확인
kubectl get pods -n opsai
kubectl logs -n opsai -l app=opsai-bot -f
```

### 5단계: Webhook 설정

Grafana, AlertManager 등에서 opsai-bot Webhook 엔드포인트로 알림 설정:

```yaml
# Grafana Notification Channel
URL: http://opsai-bot.opsai.svc.cluster.local:8080/webhooks/grafana
Auth: Bearer ${GRAFANA_WEBHOOK_SECRET}

# AlertManager
receivers:
  - name: opsai
    webhook_configs:
      - url: http://opsai-bot.opsai.svc.cluster.local:8080/webhooks/alertmanager
        send_resolved: true
        headers:
          Authorization: "Bearer ${ALERTMANAGER_WEBHOOK_SECRET}"
```

---

## 설정 가이드

### config.yaml 구조

#### Server 설정

```yaml
server:
  port: 8080                    # HTTP 포트
  readTimeout: 30s              # 읽기 타임아웃
  writeTimeout: 30s             # 쓰기 타임아웃
  shutdownTimeout: 15s          # 종료 타임아웃
  metricsPort: 9090             # Prometheus 메트릭 포트
```

#### LLM 설정

```yaml
llm:
  provider: ollama              # ollama | claude | openai
  maxAnalysisRetries: 3         # 분석 재시도 횟수
  confidenceThreshold: 0.6      # 신뢰도 임계값

  ollama:
    baseURL: "http://localhost:11434"
    model: "llama3:8b"          # 또는 mistral, neural-chat 등
    timeout: 120s
    maxRetries: 3
    temperature: 0.1            # 낮을수록 정확함
    contextSize: 8192           # 컨텍스트 윈도우
```

#### Kubernetes 설정

```yaml
kubernetes:
  inCluster: true               # Pod에서 실행 시 true
  kubeconfig: ""                # 로컬 테스트 시 경로 지정
  execTimeout: 30s
  logTailLines: 100             # 로그 조회 라인 수
  blockedNamespaces:            # 명령 금지 네임스페이스
    - kube-system
    - kube-public
    - kube-node-lease
```

#### Webhook 설정

```yaml
webhook:
  sources:
    grafana:
      enabled: true
      path: /webhooks/grafana
      secret: "${GRAFANA_WEBHOOK_SECRET}"  # 환경변수
      authType: bearer
    alertmanager:
      enabled: true
      path: /webhooks/alertmanager
      secret: "${ALERTMANAGER_WEBHOOK_SECRET}"
      authType: bearer

  deduplication:
    enabled: true
    window: 5m                  # 5분 이내 중복 제거

  rateLimit:
    enabled: true
    requestsPerMinute: 60
```

#### Slack 설정

```yaml
slack:
  enabled: true
  botToken: "${SLACK_BOT_TOKEN}"
  appToken: "${SLACK_APP_TOKEN}"
  signingSecret: "${SLACK_SIGNING_SECRET}"

  defaultChannel: "#ops-alerts"
  channels:
    dev: "#ops-alerts-dev"
    staging: "#ops-alerts-staging"
    prod: "#ops-alerts-prod"

  interaction:
    approvalTimeout: 30m        # 승인 대기 시간
    autoExecDelay: 5s           # 자동 실행 대기
    threadHistoryLimit: 50      # 스레드 메시지 기록
```

#### 정책 설정

```yaml
policy:
  environments:
    dev:
      mode: auto_fix            # 정책: 자동 실행
      maxAutoRisk: medium       # 위험도: 중 이하
      namespaces: []            # 빈 값 = 모든 네임스페이스

    staging:
      mode: warn_auto           # 정책: 알림 후 자동
      maxAutoRisk: medium
      namespaces: []

    prod:
      mode: approval_required   # 정책: 승인 필수
      maxAutoRisk: low          # 위험도: 저만 가능
      approvers:
        - "@oncall-team"        # 승인자 Slack 멘션
      namespaces: []
```

#### 데이터베이스 설정

```yaml
database:
  driver: sqlite                # sqlite | postgres

  sqlite:
    path: /data/opsai-bot.db
    maxOpenConns: 1             # SQLite 제한
    pragmaJournalMode: wal      # Write-Ahead Logging
    pragmaBusyTimeout: 5000     # 밀리초

  postgres:
    host: "${POSTGRES_HOST}"
    port: 5432
    user: "${POSTGRES_USER}"
    password: "${POSTGRES_PASSWORD}"
    database: "${POSTGRES_DB}"
    sslMode: require
    maxOpenConns: 25
```

#### 로깅 설정

```yaml
logging:
  level: info                   # debug, info, warn, error
  format: json                  # json 또는 text
  output: stdout                # stdout 또는 파일 경로
```

---

## API 엔드포인트

### Webhook 엔드포인트

#### POST /webhooks/grafana
Grafana 알림 수신

**요청:**
```bash
curl -X POST http://localhost:8080/webhooks/grafana \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${GRAFANA_WEBHOOK_SECRET}" \
  -d '{
    "title": "High CPU Usage",
    "message": "CPU > 80%",
    "state": "alerting",
    "evalMatches": [{"metric": "cpu", "value": 85}]
  }'
```

**응답:**
```json
{
  "status": "accepted",
  "alert_id": "alert-uuid-here"
}
```

#### POST /webhooks/alertmanager
AlertManager 알림 수신

**요청:**
```bash
curl -X POST http://localhost:8080/webhooks/alertmanager \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ALERTMANAGER_WEBHOOK_SECRET}" \
  -d '{
    "alerts": [
      {
        "status": "firing",
        "labels": {"alertname": "HighMemory", "env": "prod"},
        "annotations": {"summary": "High memory usage"}
      }
    ]
  }'
```

#### POST /webhooks/generic
범용 Webhook 엔드포인트

**요청:**
```bash
curl -X POST http://localhost:8080/webhooks/generic \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${GENERIC_WEBHOOK_SECRET}" \
  -d '{
    "title": "Custom Alert",
    "description": "Something happened",
    "severity": "warning",
    "environment": "staging"
  }'
```

### 헬스 체크 엔드포인트

#### GET /healthz
기본 헬스 체크 (항상 200 OK)

```bash
curl http://localhost:8080/healthz
# 응답: {"status": "ok"}
```

#### GET /readyz
준비 상태 체크 (의존성 확인)

```bash
curl http://localhost:8080/readyz
# 응답: {"status": "ready"} 또는 503 Service Unavailable
```

### Metrics 엔드포인트

#### GET /metrics
Prometheus 메트릭 (포트 9090)

```bash
curl http://localhost:9090/metrics
```

**메트릭:**
- `opsai_alerts_received_total`: 받은 알림 수
- `opsai_analysis_duration_seconds`: 분석 시간
- `opsai_actions_executed_total`: 실행된 작업 수
- `opsai_approvals_pending`: 대기 중인 승인

---

## 개발 가이드

### 프로젝트 구조

```
opsai-bot/
├── cmd/
│   └── opsai/
│       └── main.go                 # 엔트리포인트
├── internal/
│   ├── domain/
│   │   ├── model/
│   │   │   ├── alert.go            # Alert 엔티티
│   │   │   ├── action.go           # Action 엔티티
│   │   │   ├── analysis.go         # Analysis 엔티티
│   │   │   ├── audit.go            # Audit 로그
│   │   │   ├── conversation.go     # Slack 대화
│   │   │   └── policy.go           # 정책
│   │   ├── port/
│   │   │   ├── inbound/
│   │   │   │   ├── alert_receiver.go
│   │   │   │   └── interaction.go
│   │   │   └── outbound/
│   │   │       ├── k8s_executor.go
│   │   │       ├── llm_provider.go
│   │   │       ├── notifier.go
│   │   │       └── repository.go
│   │   └── service/
│   │       ├── orchestrator.go     # 주 조율 서비스
│   │       ├── analyzer.go         # LLM 분석
│   │       ├── action_planner.go   # 작업 계획
│   │       └── policy_evaluator.go # 정책 적용
│   ├── adapter/
│   │   ├── inbound/
│   │   │   ├── webhook/
│   │   │   │   ├── handler.go
│   │   │   │   ├── server.go
│   │   │   │   ├── middleware/
│   │   │   │   └── parser/
│   │   │   │       ├── grafana.go
│   │   │   │       ├── alertmanager.go
│   │   │   │       └── generic.go
│   │   │   └── slackbot/
│   │   │       ├── bot.go
│   │   │       ├── interaction.go
│   │   │       └── template/
│   │   └── outbound/
│   │       ├── llm/
│   │       │   ├── ollama/
│   │       │   ├── claude/
│   │       │   ├── openai/
│   │       │   └── prompt/
│   │       ├── kubernetes/
│   │       ├── notification/
│   │       └── persistence/
│   │           ├── sqlite/
│   │           └── postgres/
│   └── config/
│       ├── config.go
│       └── validate.go
├── pkg/
│   ├── version/
│   ├── health/
│   └── ...
├── deploy/
│   └── helm/
│       └── opsai-bot/
├── configs/
│   ├── config.yaml
│   └── policies/
├── Makefile
├── Dockerfile
├── go.mod
└── README.md
```

### 새로운 Webhook 파서 추가

**단계 1**: 파서 구현

```go
// internal/adapter/inbound/webhook/parser/myservice.go
package parser

import (
    "github.com/jonny/opsai-bot/internal/domain/model"
)

type MyServiceParser struct{}

func (p *MyServiceParser) Parse(payload []byte) ([]model.Alert, error) {
    // payload 파싱
    // model.Alert 슬라이스 반환
    return alerts, nil
}

func (p *MyServiceParser) Name() string {
    return "myservice"
}
```

**단계 2**: 레지스트리에 등록

```go
// internal/adapter/inbound/webhook/parser/registry.go
func init() {
    registry["myservice"] = &MyServiceParser{}
}
```

**단계 3**: 설정에 추가

```yaml
webhook:
  sources:
    myservice:
      enabled: true
      path: /webhooks/myservice
      secret: "${MYSERVICE_SECRET}"
      authType: bearer
```

### 새로운 LLM 프로바이더 추가

**단계 1**: 프로바이더 구현

```go
// internal/adapter/outbound/llm/myprovider/client.go
package myprovider

import (
    "context"
    "github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

type Client struct {
    config Config
}

func (c *Client) Analyze(ctx context.Context, prompt string) (string, error) {
    // API 호출
    // 분석 결과 반환
    return result, nil
}

func (c *Client) IsHealthy(ctx context.Context) error {
    // 헬스 체크
    return nil
}
```

**단계 2**: 주 설정에 통합

```go
// cmd/opsai/main.go
var llmProvider outbound.LLMProvider
switch cfg.LLM.Provider {
case "myprovider":
    llmProvider, err = myprovider.NewClient(cfg.LLM.MyProvider)
    // ...
}
```

**단계 3**: 설정 추가

```yaml
llm:
  provider: myprovider
  myprovider:
    apiKey: "${MYPROVIDER_API_KEY}"
    model: "model-name"
```

### 테스트 작성

**Unit 테스트:**

```go
// internal/domain/service/orchestrator_test.go
package service

import (
    "context"
    "testing"
    "github.com/jonny/opsai-bot/internal/domain/model"
)

func TestOrchestratorReceiveAlert(t *testing.T) {
    // Arrange
    orchestrator := setupOrchestrator(t)
    alert := model.NewAlert(
        model.AlertSourceGrafana,
        model.SeverityWarning,
        "Test Alert",
        "Test Description",
        "dev",
        "default",
    )

    // Act
    err := orchestrator.ReceiveAlert(context.Background(), alert)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

**테스트 실행:**

```bash
# 전체 테스트
make test

# 특정 패키지 테스트
go test -v ./internal/domain/service/...

# 커버리지와 함께
make coverage
```

### 데이터베이스 마이그레이션

**새 마이그레이션 추가:**

```sql
-- internal/adapter/outbound/persistence/sqlite/migration/migrations/002_add_table.sql
CREATE TABLE IF NOT EXISTS my_table (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

마이그레이션은 시작 시 자동으로 실행됩니다.

---

## 배포

### Docker 이미지 빌드

```bash
# 빌드
docker build -t opsai-bot:latest .

# 실행
docker run -v ./configs:/app/configs opsai-bot:latest
```

### Helm 배포

```bash
# 네임스페이스 생성
kubectl create namespace opsai

# 시크릿 생성
kubectl create secret generic opsai-bot-secrets \
  --from-literal=SLACK_BOT_TOKEN=xoxb-... \
  --from-literal=SLACK_APP_TOKEN=xapp-... \
  -n opsai

# 배포
helm install opsai-bot deploy/helm/opsai-bot \
  -n opsai \
  --values deploy/helm/opsai-bot/values.yaml

# 업그레이드
helm upgrade opsai-bot deploy/helm/opsai-bot \
  -n opsai
```

### 상태 확인

```bash
# Pod 상태
kubectl get pods -n opsai

# 로그 조회
kubectl logs -n opsai -l app=opsai-bot -f

# 헬스 체크
kubectl exec -n opsai <pod-name> -- curl localhost:8080/healthz
```

---

## 트러블슈팅

### 알림이 수신되지 않음

1. **Webhook 엔드포인트 확인**
   ```bash
   kubectl port-forward -n opsai svc/opsai-bot 8080:8080
   # 테스트: curl -X POST http://localhost:8080/webhooks/grafana ...
   ```

2. **인증 토큰 확인**
   - 환경변수에서 시크릿이 올바르게 설정되었는지 확인
   - 알림 소스에서 헤더가 올바르게 전송되는지 확인

3. **로그 확인**
   ```bash
   kubectl logs -n opsai -l app=opsai-bot | grep "webhook"
   ```

### LLM 분석이 실패함

1. **Ollama 연결 확인**
   ```bash
   curl http://localhost:11434/api/tags  # 모델 목록
   ```

2. **프롬프트 토큰 초과**
   - `config.yaml`에서 `maxTokens` 감소
   - LLM `contextSize` 감소

3. **타임아웃**
   - `llm.timeout` 증가
   - LLM 프로바이더 상태 확인

### K8s 명령 실행 실패

1. **권한 확인**
   ```bash
   kubectl auth can-i get pods --as=system:serviceaccount:opsai:opsai-bot
   ```

2. **화이트리스트 확인**
   - 요청된 명령이 `config.yaml`의 화이트리스트에 있는지 확인

3. **네임스페이스 확인**
   - 차단된 네임스페이스는 `blockedNamespaces` 확인

---

## 라이선스

MIT License - 자유롭게 사용, 수정, 배포 가능합니다.

---

## 기여하기

버그 리포트, 기능 요청, Pull Request를 환영합니다!

1. Fork the repository
2. Feature branch 생성 (`git checkout -b feature/amazing-feature`)
3. 변경사항 커밋 (`git commit -m 'Add amazing feature'`)
4. Branch에 push (`git push origin feature/amazing-feature`)
5. Pull Request 생성

---

## 연락처

- 이슈: GitHub Issues
- 토론: GitHub Discussions
