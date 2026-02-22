package template_test

import (
	"testing"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func TestBuildApprovalBlocks_Basic(t *testing.T) {
	req := outbound.ApprovalNotification{
		AlertID:     "alert-123",
		ThreadID:    "1234567890.123",
		ActionID:    "action-abc",
		Description: "Restart the payment service deployment",
		Commands:    []string{"kubectl rollout restart deployment/payment"},
		Risk:        "low",
		Environment: "production",
		RequestedBy: "opsai-bot",
	}

	blocks := template.BuildApprovalBlocks(req)

	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// Header should mention Approval.
	header, ok := blocks[0].(*slackapi.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock header, got %T", blocks[0])
	}
	if !containsString(header.Text.Text, "Approval") {
		t.Errorf("expected 'Approval' in header, got: %s", header.Text.Text)
	}
}

func TestBuildApprovalBlocks_HasApproveButton(t *testing.T) {
	req := outbound.ApprovalNotification{
		ActionID:    "action-xyz",
		Description: "Delete stale pods",
		Risk:        "medium",
		Environment: "staging",
	}

	blocks := template.BuildApprovalBlocks(req)

	approveFound := false
	rejectFound := false
	for _, b := range blocks {
		actionBlock, ok := b.(*slackapi.ActionBlock)
		if !ok {
			continue
		}
		for _, elem := range actionBlock.Elements.ElementSet {
			btn, ok := elem.(*slackapi.ButtonBlockElement)
			if !ok {
				continue
			}
			if btn.ActionID == template.ActionIDApprove {
				approveFound = true
			}
			if btn.ActionID == template.ActionIDReject {
				rejectFound = true
			}
		}
	}

	if !approveFound {
		t.Error("expected Approve button with ActionIDApprove")
	}
	if !rejectFound {
		t.Error("expected Reject button with ActionIDReject")
	}
}

func TestBuildApprovalBlocks_ButtonValues(t *testing.T) {
	req := outbound.ApprovalNotification{
		ActionID:    "action-123",
		Description: "Scale down",
		Risk:        "high",
		Environment: "production",
	}

	blocks := template.BuildApprovalBlocks(req)

	for _, b := range blocks {
		actionBlock, ok := b.(*slackapi.ActionBlock)
		if !ok {
			continue
		}
		for _, elem := range actionBlock.Elements.ElementSet {
			btn, ok := elem.(*slackapi.ButtonBlockElement)
			if !ok {
				continue
			}
			if btn.ActionID == template.ActionIDApprove {
				if !containsString(btn.Value, "action-123") {
					t.Errorf("approve button value should contain actionID, got: %s", btn.Value)
				}
			}
			if btn.ActionID == template.ActionIDReject {
				if !containsString(btn.Value, "action-123") {
					t.Errorf("reject button value should contain actionID, got: %s", btn.Value)
				}
			}
		}
	}
}

func TestBuildApprovalBlocks_WithCommands(t *testing.T) {
	req := outbound.ApprovalNotification{
		ActionID:    "action-cmd",
		Description: "Run cleanup",
		Commands:    []string{"kubectl delete pod stale-pod-1", "kubectl delete pod stale-pod-2"},
		Risk:        "low",
		Environment: "dev",
	}

	blocks := template.BuildApprovalBlocks(req)

	found := false
	for _, b := range blocks {
		s, ok := b.(*slackapi.SectionBlock)
		if !ok || s.Text == nil {
			continue
		}
		if containsString(s.Text.Text, "Commands") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Commands block when commands are present")
	}
}

func TestBuildApprovalBlocks_NoCommands(t *testing.T) {
	req := outbound.ApprovalNotification{
		ActionID:    "action-nocmd",
		Description: "Manual review required",
		Commands:    nil,
		Risk:        "high",
		Environment: "production",
	}

	blocks := template.BuildApprovalBlocks(req)

	for _, b := range blocks {
		s, ok := b.(*slackapi.SectionBlock)
		if !ok || s.Text == nil {
			continue
		}
		if containsString(s.Text.Text, "Commands") {
			t.Error("expected no Commands block when commands slice is nil")
		}
	}
}
