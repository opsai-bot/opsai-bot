-- Alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id TEXT PRIMARY KEY,
    external_id TEXT,
    fingerprint TEXT,
    source TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'received',
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    environment TEXT NOT NULL,
    namespace TEXT,
    resource TEXT,
    labels TEXT DEFAULT '{}',
    annotations TEXT DEFAULT '{}',
    raw_payload TEXT,
    thread_id TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    resolved_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_environment ON alerts(environment);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at);

-- Analyses table
CREATE TABLE IF NOT EXISTS analyses (
    id TEXT PRIMARY KEY,
    alert_id TEXT NOT NULL REFERENCES alerts(id),
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    root_cause TEXT,
    severity TEXT,
    confidence REAL DEFAULT 0,
    explanation TEXT,
    k8s_context TEXT,
    prompt_tokens INTEGER DEFAULT 0,
    response_tokens INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_analyses_alert_id ON analyses(alert_id);

-- Actions table
CREATE TABLE IF NOT EXISTS actions (
    id TEXT PRIMARY KEY,
    analysis_id TEXT NOT NULL REFERENCES analyses(id),
    alert_id TEXT NOT NULL REFERENCES alerts(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'planned',
    description TEXT,
    commands TEXT DEFAULT '[]',
    risk TEXT NOT NULL DEFAULT 'low',
    reversible BOOLEAN DEFAULT 0,
    output TEXT,
    error_message TEXT,
    approved_by TEXT,
    approved_at DATETIME,
    executed_at DATETIME,
    completed_at DATETIME,
    environment TEXT,
    namespace TEXT,
    target_resource TEXT,
    metadata TEXT DEFAULT '{}',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_actions_alert_id ON actions(alert_id);
CREATE INDEX IF NOT EXISTS idx_actions_analysis_id ON actions(analysis_id);
CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status);

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    alert_id TEXT,
    action_id TEXT,
    actor TEXT NOT NULL,
    environment TEXT,
    description TEXT,
    metadata TEXT DEFAULT '{}',
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_alert_id ON audit_logs(alert_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs(event_type);

-- Policies table
CREATE TABLE IF NOT EXISTS policies (
    id TEXT PRIMARY KEY,
    environment TEXT NOT NULL UNIQUE,
    mode TEXT NOT NULL,
    max_auto_risk TEXT DEFAULT 'low',
    approvers TEXT DEFAULT '[]',
    namespaces TEXT DEFAULT '[]',
    custom_rules TEXT DEFAULT '[]',
    enabled BOOLEAN DEFAULT 1
);

-- Conversations table
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    alert_id TEXT REFERENCES alerts(id),
    thread_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    messages TEXT DEFAULT '[]',
    active BOOLEAN DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversations_thread_id ON conversations(thread_id);
CREATE INDEX IF NOT EXISTS idx_conversations_alert_id ON conversations(alert_id);
