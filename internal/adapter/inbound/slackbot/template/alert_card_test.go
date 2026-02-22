package template_test

import (
	"testing"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func TestBuildAlertBlocks_Critical(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:     "alert-123",
		Title:       "High CPU Usage",
		Summary:     "CPU usage exceeded 95% on node-1",
		Severity:    "critical",
		Environment: "production",
		Source:      "prometheus",
		Labels:      map[string]string{"node": "node-1"},
	}

	blocks := template.BuildAlertBlocks(n)

	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// First block should be a section with the title.
	section, ok := blocks[0].(*slackapi.SectionBlock)
	if !ok {
		t.Fatalf("expected first block to be SectionBlock, got %T", blocks[0])
	}
	if section.Text == nil {
		t.Fatal("expected section text to be non-nil")
	}
	if section.Text.Text == "" {
		t.Fatal("expected section text to be non-empty")
	}
	// Should contain the title.
	if !containsString(section.Text.Text, "High CPU Usage") {
		t.Errorf("expected title in header block, got: %s", section.Text.Text)
	}
	// Should contain critical emoji.
	if !containsString(section.Text.Text, ":red_circle:") {
		t.Errorf("expected red_circle emoji for critical severity, got: %s", section.Text.Text)
	}
}

func TestBuildAlertBlocks_Warning(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:  "alert-456",
		Title:    "Memory Pressure",
		Severity: "warning",
	}

	blocks := template.BuildAlertBlocks(n)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	section, ok := blocks[0].(*slackapi.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock, got %T", blocks[0])
	}
	if !containsString(section.Text.Text, ":large_yellow_circle:") {
		t.Errorf("expected yellow emoji for warning, got: %s", section.Text.Text)
	}
}

func TestBuildAlertBlocks_Info(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:  "alert-789",
		Title:    "Deployment Started",
		Severity: "info",
	}

	blocks := template.BuildAlertBlocks(n)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	section, ok := blocks[0].(*slackapi.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock, got %T", blocks[0])
	}
	if !containsString(section.Text.Text, ":large_blue_circle:") {
		t.Errorf("expected blue emoji for info, got: %s", section.Text.Text)
	}
}

func TestBuildAlertBlocks_NoLabels(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:  "alert-000",
		Title:    "Test Alert",
		Severity: "warning",
		Labels:   nil,
	}

	blocks := template.BuildAlertBlocks(n)
	// Without labels there should be no context block appended.
	for _, b := range blocks {
		if _, ok := b.(*slackapi.ContextBlock); ok {
			t.Error("expected no context block when no labels")
		}
	}
}

func TestBuildAlertBlocks_WithLabels(t *testing.T) {
	n := outbound.AlertNotification{
		AlertID:  "alert-111",
		Title:    "Test Alert",
		Severity: "info",
		Labels:   map[string]string{"env": "staging", "team": "platform"},
	}

	blocks := template.BuildAlertBlocks(n)
	hasContext := false
	for _, b := range blocks {
		if _, ok := b.(*slackapi.ContextBlock); ok {
			hasContext = true
		}
	}
	if !hasContext {
		t.Error("expected context block with labels")
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
