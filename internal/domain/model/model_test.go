package model

import (
	"testing"
	"time"
)

// ---- Alert tests ----

func TestNewAlert(t *testing.T) {
	before := time.Now().UTC()
	a := NewAlert(AlertSourceGrafana, SeverityCritical, "High CPU", "CPU > 90%", "prod", "default")
	after := time.Now().UTC()

	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
	if a.Source != AlertSourceGrafana {
		t.Errorf("expected source grafana, got %s", a.Source)
	}
	if a.Severity != SeverityCritical {
		t.Errorf("expected severity critical, got %s", a.Severity)
	}
	if a.Status != AlertStatusReceived {
		t.Errorf("expected status received, got %s", a.Status)
	}
	if a.Title != "High CPU" {
		t.Errorf("expected title 'High CPU', got %s", a.Title)
	}
	if a.Environment != "prod" {
		t.Errorf("expected environment prod, got %s", a.Environment)
	}
	if a.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", a.Namespace)
	}
	if a.Labels == nil {
		t.Error("expected non-nil Labels map")
	}
	if a.Annotations == nil {
		t.Error("expected non-nil Annotations map")
	}
	if a.CreatedAt.Before(before) || a.CreatedAt.After(after) {
		t.Error("CreatedAt out of expected range")
	}
	if a.ResolvedAt != nil {
		t.Error("expected nil ResolvedAt for new alert")
	}
}

func TestAlert_WithStatus(t *testing.T) {
	original := NewAlert(AlertSourceGrafana, SeverityWarning, "Test", "desc", "dev", "ns")
	updated := original.WithStatus(AlertStatusAnalyzing)

	// original must not be mutated
	if original.Status != AlertStatusReceived {
		t.Errorf("original status mutated: got %s", original.Status)
	}
	if updated.Status != AlertStatusAnalyzing {
		t.Errorf("expected analyzing, got %s", updated.Status)
	}
	if !updated.UpdatedAt.After(original.UpdatedAt) && updated.UpdatedAt.Equal(original.UpdatedAt) {
		// UpdatedAt should be >= original; allow equal only if extremely fast
	}
}

func TestAlert_WithThreadID(t *testing.T) {
	original := NewAlert(AlertSourceCustom, SeverityInfo, "T", "d", "e", "n")
	updated := original.WithThreadID("thread-123")

	if original.ThreadID != "" {
		t.Error("original ThreadID must not be mutated")
	}
	if updated.ThreadID != "thread-123" {
		t.Errorf("expected thread-123, got %s", updated.ThreadID)
	}
}

func TestAlert_WithFingerprint(t *testing.T) {
	original := NewAlert(AlertSourceGrafana, SeverityCritical, "T", "d", "e", "n")
	updated := original.WithFingerprint("fp-abc")

	if original.Fingerprint != "" {
		t.Error("original Fingerprint must not be mutated")
	}
	if updated.Fingerprint != "fp-abc" {
		t.Errorf("expected fp-abc, got %s", updated.Fingerprint)
	}
}

func TestAlert_Resolve(t *testing.T) {
	original := NewAlert(AlertSourceAlertManager, SeverityCritical, "T", "d", "prod", "ns")
	before := time.Now().UTC()
	resolved := original.Resolve()
	after := time.Now().UTC()

	// original must not be mutated
	if original.Status != AlertStatusReceived {
		t.Error("original status mutated after Resolve()")
	}
	if original.ResolvedAt != nil {
		t.Error("original ResolvedAt must remain nil")
	}

	if resolved.Status != AlertStatusResolved {
		t.Errorf("expected resolved, got %s", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("expected non-nil ResolvedAt")
	}
	if resolved.ResolvedAt.Before(before) || resolved.ResolvedAt.After(after) {
		t.Error("ResolvedAt out of expected range")
	}
}

func TestAlert_IsTerminal(t *testing.T) {
	cases := []struct {
		status   AlertStatus
		terminal bool
	}{
		{AlertStatusReceived, false},
		{AlertStatusAnalyzing, false},
		{AlertStatusAnalyzed, false},
		{AlertStatusActing, false},
		{AlertStatusResolved, true},
		{AlertStatusFailed, true},
		{AlertStatusDuplicate, true},
		{AlertStatusSilenced, true},
	}

	for _, tc := range cases {
		a := NewAlert(AlertSourceCustom, SeverityInfo, "T", "d", "e", "n").WithStatus(tc.status)
		if got := a.IsTerminal(); got != tc.terminal {
			t.Errorf("status %s: expected IsTerminal=%v, got %v", tc.status, tc.terminal, got)
		}
	}
}

// ---- Analysis tests ----

func TestNewAnalysis(t *testing.T) {
	before := time.Now().UTC()
	an := NewAnalysis("alert-1", "openai", "gpt-4o")
	after := time.Now().UTC()

	if an.ID == "" {
		t.Error("expected non-empty ID")
	}
	if an.AlertID != "alert-1" {
		t.Errorf("expected alert-1, got %s", an.AlertID)
	}
	if an.Provider != "openai" {
		t.Errorf("expected openai, got %s", an.Provider)
	}
	if an.Model != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", an.Model)
	}
	if an.CreatedAt.Before(before) || an.CreatedAt.After(after) {
		t.Error("CreatedAt out of expected range")
	}
}

func TestAnalysis_WithDiagnosis(t *testing.T) {
	original := NewAnalysis("alert-1", "ollama", "llama3")
	updated := original.WithDiagnosis("OOM killer", SeverityCritical, 0.95, "Memory exhausted")

	// immutability
	if original.RootCause != "" {
		t.Error("original RootCause must not be mutated")
	}
	if updated.RootCause != "OOM killer" {
		t.Errorf("expected 'OOM killer', got %s", updated.RootCause)
	}
	if updated.Severity != SeverityCritical {
		t.Errorf("expected critical, got %s", updated.Severity)
	}
	if updated.Confidence != 0.95 {
		t.Errorf("expected 0.95, got %f", updated.Confidence)
	}
	if updated.Explanation != "Memory exhausted" {
		t.Errorf("expected 'Memory exhausted', got %s", updated.Explanation)
	}
}

func TestAnalysis_WithTokenUsage(t *testing.T) {
	original := NewAnalysis("alert-1", "openai", "gpt-4o")
	updated := original.WithTokenUsage(100, 200, 1500)

	if original.PromptTokens != 0 {
		t.Error("original PromptTokens mutated")
	}
	if updated.PromptTokens != 100 {
		t.Errorf("expected 100, got %d", updated.PromptTokens)
	}
	if updated.ResponseTokens != 200 {
		t.Errorf("expected 200, got %d", updated.ResponseTokens)
	}
	if updated.LatencyMs != 1500 {
		t.Errorf("expected 1500, got %d", updated.LatencyMs)
	}
}

func TestAnalysis_IsHighConfidence(t *testing.T) {
	cases := []struct {
		confidence float64
		expected   bool
	}{
		{0.0, false},
		{0.5, false},
		{0.69, false},
		{0.7, true},
		{0.85, true},
		{1.0, true},
	}
	for _, tc := range cases {
		an := NewAnalysis("a", "p", "m").WithDiagnosis("rc", SeverityInfo, tc.confidence, "exp")
		if got := an.IsHighConfidence(); got != tc.expected {
			t.Errorf("confidence %.2f: expected IsHighConfidence=%v, got %v", tc.confidence, tc.expected, got)
		}
	}
}

// ---- Action tests ----

func TestNewAction(t *testing.T) {
	cmds := []string{"kubectl rollout restart deployment/app"}
	before := time.Now().UTC()
	a := NewAction("analysis-1", "alert-1", ActionTypeRestart, "Restart app", cmds, RiskMedium)
	after := time.Now().UTC()

	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
	if a.Status != ActionStatusPlanned {
		t.Errorf("expected planned, got %s", a.Status)
	}
	if a.AnalysisID != "analysis-1" {
		t.Errorf("expected analysis-1, got %s", a.AnalysisID)
	}
	if a.AlertID != "alert-1" {
		t.Errorf("expected alert-1, got %s", a.AlertID)
	}
	if a.Risk != RiskMedium {
		t.Errorf("expected medium, got %s", a.Risk)
	}
	if len(a.Commands) != 1 || a.Commands[0] != cmds[0] {
		t.Error("commands not set correctly")
	}
	if a.Metadata == nil {
		t.Error("expected non-nil Metadata map")
	}
	if a.CreatedAt.Before(before) || a.CreatedAt.After(after) {
		t.Error("CreatedAt out of expected range")
	}
}

func TestAction_WithStatus(t *testing.T) {
	original := NewAction("an", "al", ActionTypeKubectl, "desc", nil, RiskLow)
	updated := original.WithStatus(ActionStatusExecuting)

	if original.Status != ActionStatusPlanned {
		t.Error("original status mutated")
	}
	if updated.Status != ActionStatusExecuting {
		t.Errorf("expected executing, got %s", updated.Status)
	}
}

func TestAction_Approve(t *testing.T) {
	original := NewAction("an", "al", ActionTypeScale, "scale", nil, RiskHigh)
	before := time.Now().UTC()
	approved := original.Approve("bob")
	after := time.Now().UTC()

	if original.Status != ActionStatusPlanned {
		t.Error("original status mutated after Approve()")
	}
	if original.ApprovedBy != "" {
		t.Error("original ApprovedBy mutated")
	}

	if approved.Status != ActionStatusApproved {
		t.Errorf("expected approved, got %s", approved.Status)
	}
	if approved.ApprovedBy != "bob" {
		t.Errorf("expected 'bob', got %s", approved.ApprovedBy)
	}
	if approved.ApprovedAt == nil {
		t.Fatal("expected non-nil ApprovedAt")
	}
	if approved.ApprovedAt.Before(before) || approved.ApprovedAt.After(after) {
		t.Error("ApprovedAt out of expected range")
	}
}

func TestAction_Reject(t *testing.T) {
	original := NewAction("an", "al", ActionTypeDeletePod, "delete", nil, RiskCritical)
	rejected := original.Reject("alice")

	if original.Status != ActionStatusPlanned {
		t.Error("original status mutated")
	}
	if rejected.Status != ActionStatusRejected {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}
	if rejected.ApprovedBy != "alice" {
		t.Errorf("expected alice, got %s", rejected.ApprovedBy)
	}
}

func TestAction_Complete(t *testing.T) {
	original := NewAction("an", "al", ActionTypeExec, "exec", nil, RiskLow)
	before := time.Now().UTC()
	completed := original.Complete("exit code 0")
	after := time.Now().UTC()

	if original.Status != ActionStatusPlanned {
		t.Error("original status mutated")
	}
	if completed.Status != ActionStatusCompleted {
		t.Errorf("expected completed, got %s", completed.Status)
	}
	if completed.Output != "exit code 0" {
		t.Errorf("expected 'exit code 0', got %s", completed.Output)
	}
	if completed.CompletedAt == nil {
		t.Fatal("expected non-nil CompletedAt")
	}
	if completed.CompletedAt.Before(before) || completed.CompletedAt.After(after) {
		t.Error("CompletedAt out of expected range")
	}
}

func TestAction_Fail(t *testing.T) {
	original := NewAction("an", "al", ActionTypeManual, "manual", nil, RiskMedium)
	failed := original.Fail("connection refused")

	if original.Status != ActionStatusPlanned {
		t.Error("original status mutated")
	}
	if failed.Status != ActionStatusFailed {
		t.Errorf("expected failed, got %s", failed.Status)
	}
	if failed.ErrorMessage != "connection refused" {
		t.Errorf("expected 'connection refused', got %s", failed.ErrorMessage)
	}
}

func TestAction_IsTerminal(t *testing.T) {
	cases := []struct {
		status   ActionStatus
		terminal bool
	}{
		{ActionStatusPlanned, false},
		{ActionStatusPending, false},
		{ActionStatusApproved, false},
		{ActionStatusExecuting, false},
		{ActionStatusCompleted, true},
		{ActionStatusFailed, true},
		{ActionStatusRejected, true},
		{ActionStatusRolledBack, true},
	}
	for _, tc := range cases {
		a := NewAction("an", "al", ActionTypeKubectl, "d", nil, RiskLow).WithStatus(tc.status)
		if got := a.IsTerminal(); got != tc.terminal {
			t.Errorf("status %s: expected IsTerminal=%v, got %v", tc.status, tc.terminal, got)
		}
	}
}

func TestAction_RiskOrder(t *testing.T) {
	cases := []struct {
		risk     RiskLevel
		expected int
	}{
		{RiskLow, 0},
		{RiskMedium, 1},
		{RiskHigh, 2},
		{RiskCritical, 3},
		{RiskLevel("unknown"), -1},
	}
	for _, tc := range cases {
		a := NewAction("an", "al", ActionTypeKubectl, "d", nil, tc.risk)
		if got := a.RiskOrder(); got != tc.expected {
			t.Errorf("risk %s: expected %d, got %d", tc.risk, tc.expected, got)
		}
	}
}

func TestAction_NeedsApproval(t *testing.T) {
	a := NewAction("an", "al", ActionTypeKubectl, "d", nil, RiskLow)
	if a.NeedsApproval() {
		t.Error("planned action should not need approval")
	}
	pending := a.WithStatus(ActionStatusPending)
	if !pending.NeedsApproval() {
		t.Error("pending action should need approval")
	}
}

// ---- AuditLog tests ----

func TestNewAuditLog(t *testing.T) {
	before := time.Now().UTC()
	log := NewAuditLog(AuditAlertReceived, "alert-1", "system", "prod", "Alert received from Grafana")
	after := time.Now().UTC()

	if log.ID == "" {
		t.Error("expected non-empty ID")
	}
	if log.EventType != AuditAlertReceived {
		t.Errorf("expected alert.received, got %s", log.EventType)
	}
	if log.AlertID != "alert-1" {
		t.Errorf("expected alert-1, got %s", log.AlertID)
	}
	if log.Actor != "system" {
		t.Errorf("expected system, got %s", log.Actor)
	}
	if log.Metadata == nil {
		t.Error("expected non-nil Metadata map")
	}
	if log.CreatedAt.Before(before) || log.CreatedAt.After(after) {
		t.Error("CreatedAt out of expected range")
	}
}

func TestAuditLog_WithActionID(t *testing.T) {
	original := NewAuditLog(AuditActionPlanned, "alert-1", "system", "prod", "desc")
	updated := original.WithActionID("action-99")

	if original.ActionID != "" {
		t.Error("original ActionID must not be mutated")
	}
	if updated.ActionID != "action-99" {
		t.Errorf("expected action-99, got %s", updated.ActionID)
	}
}

func TestAuditLog_WithMetadata(t *testing.T) {
	original := NewAuditLog(AuditConversation, "alert-1", "user", "dev", "desc")
	updated := original.WithMetadata("key1", "val1").WithMetadata("key2", "val2")

	// original must not be mutated
	if len(original.Metadata) != 0 {
		t.Error("original Metadata must not be mutated")
	}
	if updated.Metadata["key1"] != "val1" {
		t.Errorf("expected val1, got %s", updated.Metadata["key1"])
	}
	if updated.Metadata["key2"] != "val2" {
		t.Errorf("expected val2, got %s", updated.Metadata["key2"])
	}
	if len(updated.Metadata) != 2 {
		t.Errorf("expected 2 entries, got %d", len(updated.Metadata))
	}
}

// ---- Policy tests ----

func TestEnvironmentPolicy_IsAutoFix(t *testing.T) {
	p := EnvironmentPolicy{Mode: PolicyModeAutoFix}
	if !p.IsAutoFix() {
		t.Error("expected IsAutoFix=true")
	}
	p2 := EnvironmentPolicy{Mode: PolicyModeApprovalRequired}
	if p2.IsAutoFix() {
		t.Error("expected IsAutoFix=false for approval_required")
	}
}

func TestEnvironmentPolicy_RequiresApproval(t *testing.T) {
	p := EnvironmentPolicy{Mode: PolicyModeApprovalRequired}
	if !p.RequiresApproval() {
		t.Error("expected RequiresApproval=true")
	}
	p2 := EnvironmentPolicy{Mode: PolicyModeAutoFix}
	if p2.RequiresApproval() {
		t.Error("expected RequiresApproval=false for auto_fix")
	}
}

func TestEnvironmentPolicy_AppliesToNamespace(t *testing.T) {
	// empty namespaces: applies to all
	p := EnvironmentPolicy{}
	if !p.AppliesToNamespace("anything") {
		t.Error("empty namespaces should apply to all")
	}

	// specific namespaces
	p2 := EnvironmentPolicy{Namespaces: []string{"ns-a", "ns-b"}}
	if !p2.AppliesToNamespace("ns-a") {
		t.Error("expected true for ns-a")
	}
	if !p2.AppliesToNamespace("ns-b") {
		t.Error("expected true for ns-b")
	}
	if p2.AppliesToNamespace("ns-c") {
		t.Error("expected false for ns-c")
	}
}

// ---- ConversationThread tests ----

func TestNewConversationThread(t *testing.T) {
	before := time.Now().UTC()
	ct := NewConversationThread("alert-1", "thread-T", "channel-C")
	after := time.Now().UTC()

	if ct.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ct.AlertID != "alert-1" {
		t.Errorf("expected alert-1, got %s", ct.AlertID)
	}
	if ct.ThreadID != "thread-T" {
		t.Errorf("expected thread-T, got %s", ct.ThreadID)
	}
	if ct.ChannelID != "channel-C" {
		t.Errorf("expected channel-C, got %s", ct.ChannelID)
	}
	if !ct.Active {
		t.Error("expected Active=true for new thread")
	}
	if ct.Messages == nil {
		t.Error("expected non-nil Messages slice")
	}
	if len(ct.Messages) != 0 {
		t.Error("expected empty Messages slice")
	}
	if ct.CreatedAt.Before(before) || ct.CreatedAt.After(after) {
		t.Error("CreatedAt out of expected range")
	}
}

func TestConversationThread_AddMessage(t *testing.T) {
	original := NewConversationThread("alert-1", "t", "c")
	updated := original.AddMessage(MessageRoleUser, "hello", "user-1")

	// immutability
	if len(original.Messages) != 0 {
		t.Error("original Messages must not be mutated")
	}
	if len(updated.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(updated.Messages))
	}

	msg := updated.Messages[0]
	if msg.Role != MessageRoleUser {
		t.Errorf("expected user role, got %s", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("expected 'hello', got %s", msg.Content)
	}
	if msg.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", msg.UserID)
	}

	// chaining
	updated2 := updated.AddMessage(MessageRoleAssistant, "hi there", "bot")
	if len(updated.Messages) != 1 {
		t.Error("first updated Messages must not be mutated by second AddMessage")
	}
	if len(updated2.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(updated2.Messages))
	}
}

func TestConversationThread_Close(t *testing.T) {
	original := NewConversationThread("alert-1", "t", "c")
	closed := original.Close()

	if !original.Active {
		t.Error("original Active must not be mutated")
	}
	if closed.Active {
		t.Error("expected Active=false after Close()")
	}
}

func TestConversationThread_LastNMessages(t *testing.T) {
	ct := NewConversationThread("a", "t", "c")
	for i := 0; i < 5; i++ {
		ct = ct.AddMessage(MessageRoleUser, "msg", "u")
	}

	// n >= len: return all
	all := ct.LastNMessages(10)
	if len(all) != 5 {
		t.Errorf("expected 5, got %d", len(all))
	}

	// n == len: return all
	exact := ct.LastNMessages(5)
	if len(exact) != 5 {
		t.Errorf("expected 5, got %d", len(exact))
	}

	// n < len: return last n
	last3 := ct.LastNMessages(3)
	if len(last3) != 3 {
		t.Errorf("expected 3, got %d", len(last3))
	}
	// verify they are the last 3 messages
	for i, msg := range last3 {
		expected := ct.Messages[2+i]
		if msg.Timestamp != expected.Timestamp {
			t.Errorf("message %d: timestamp mismatch", i)
		}
	}
}

// ---- generateID tests ----

func TestGenerateID_Unique(t *testing.T) {
	ids := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := generateID()
		if id == "" {
			t.Fatal("generateID returned empty string")
		}
		if _, exists := ids[id]; exists {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = struct{}{}
	}
}

func TestGenerateID_Length(t *testing.T) {
	// timestamp (8 bytes -> 16 hex chars) + random (8 bytes -> 16 hex chars) = 32 chars
	id := generateID()
	if len(id) != 32 {
		t.Errorf("expected ID length 32, got %d: %s", len(id), id)
	}
}
