package template_test

import (
	"testing"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func TestBuildAnalysisBlocks_Basic(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-123",
		ThreadID:   "1234567890.123",
		RootCause:  "OOM Killer triggered due to memory leak in app container",
		Confidence: 0.87,
		Severity:   "critical",
		Explanation: "Memory usage grew steadily over 6 hours before hitting the limit.",
		Actions: []outbound.ActionNotification{
			{Description: "Restart deployment", Command: "kubectl rollout restart deployment/app", Risk: "low"},
		},
	}

	blocks := template.BuildAnalysisBlocks(n)

	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// Header should mention AI Analysis.
	header, ok := blocks[0].(*slackapi.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock header, got %T", blocks[0])
	}
	if !containsString(header.Text.Text, "AI Analysis") {
		t.Errorf("expected 'AI Analysis' in header, got: %s", header.Text.Text)
	}
}

func TestBuildAnalysisBlocks_ConfidenceBar(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-456",
		RootCause:  "Network timeout",
		Confidence: 0.5,
	}

	blocks := template.BuildAnalysisBlocks(n)
	// Find a block containing the confidence bar.
	found := false
	for _, b := range blocks {
		if s, ok := b.(*slackapi.SectionBlock); ok && s.Text != nil {
			if containsString(s.Text.Text, "50%") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected confidence percentage in blocks")
	}
}

func TestBuildAnalysisBlocks_NoActions(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-789",
		RootCause:  "Disk pressure",
		Confidence: 0.9,
		Actions:    nil,
	}

	blocks := template.BuildAnalysisBlocks(n)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}
}

func TestBuildAnalysisBlocks_MultipleActions(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-999",
		RootCause:  "High load",
		Confidence: 0.75,
		Actions: []outbound.ActionNotification{
			{Description: "Scale up", Command: "kubectl scale deployment/app --replicas=5", Risk: "low"},
			{Description: "Check logs", Command: "kubectl logs deployment/app", Risk: "none"},
		},
	}

	blocks := template.BuildAnalysisBlocks(n)
	// Find block with suggested actions.
	found := false
	for _, b := range blocks {
		if s, ok := b.(*slackapi.SectionBlock); ok && s.Text != nil {
			if containsString(s.Text.Text, "Suggested Actions") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected 'Suggested Actions' block")
	}
}

func TestBuildAnalysisBlocks_HighConfidence(t *testing.T) {
	n := outbound.AnalysisNotification{
		AlertID:    "alert-100",
		RootCause:  "Config error",
		Confidence: 1.0,
	}

	blocks := template.BuildAnalysisBlocks(n)
	found := false
	for _, b := range blocks {
		if s, ok := b.(*slackapi.SectionBlock); ok && s.Text != nil {
			if containsString(s.Text.Text, "100%") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected 100% confidence in blocks")
	}
}
