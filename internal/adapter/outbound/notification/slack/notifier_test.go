package slack_test

import (
	"testing"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// These tests verify block construction via the template functions.
// Actual Slack API calls are not made; the Notifier is not instantiated.

func TestNotifierBlocks_AlertBlock(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:     "alert-notifier-1",
		Title:       "Service Down",
		Summary:     "Payment service is not responding",
		Severity:    "critical",
		Environment: "production",
		Source:      "alertmanager",
	}

	blocks := template.BuildAlertBlocks(n)
	if len(blocks) == 0 {
		t.Fatal("expected alert blocks to be non-empty")
	}
}

func TestNotifierBlocks_AnalysisBlock(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-notifier-2",
		ThreadID:   "ts.123",
		RootCause:  "Database connection pool exhausted",
		Confidence: 0.92,
		Severity:   "critical",
	}

	blocks := template.BuildAnalysisBlocks(n)
	if len(blocks) == 0 {
		t.Fatal("expected analysis blocks to be non-empty")
	}
}

func TestNotifierBlocks_ApprovalBlock(t *testing.T) {
	req := outbound.ApprovalNotification{
		AlertID:     "alert-notifier-3",
		ThreadID:    "ts.456",
		ActionID:    "action-restart",
		Description: "Restart database pod",
		Risk:        "medium",
		Environment: "production",
		RequestedBy: "opsai",
	}

	blocks := template.BuildApprovalBlocks(req)
	if len(blocks) == 0 {
		t.Fatal("expected approval blocks to be non-empty")
	}
}

func TestNotifierBlocks_ApprovalActionIDs(t *testing.T) {
	if template.ActionIDApprove == "" {
		t.Error("ActionIDApprove should not be empty")
	}
	if template.ActionIDReject == "" {
		t.Error("ActionIDReject should not be empty")
	}
	if template.ActionIDApprove == template.ActionIDReject {
		t.Error("ActionIDApprove and ActionIDReject must be distinct")
	}
}
