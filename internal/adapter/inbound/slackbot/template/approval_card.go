package template

import (
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

const (
	ActionIDApprove = "approval_approve"
	ActionIDReject  = "approval_reject"
)

// BuildApprovalBlocks constructs Block Kit blocks for an approval request.
func BuildApprovalBlocks(req outbound.ApprovalNotification) []slackapi.Block {
	header := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			":warning: *Action Approval Required*", false, false),
		nil, nil,
	)

	divider := slackapi.NewDividerBlock()

	fields := []*slackapi.TextBlockObject{
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Action ID*\n`%s`", req.ActionID), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Environment*\n%s", req.Environment), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Requested By*\n%s", req.RequestedBy), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Risk Level*\n%s", strings.ToUpper(req.Risk)), false, false),
	}
	fieldBlock := slackapi.NewSectionBlock(nil, fields, nil)

	descBlock := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Description*\n%s", req.Description), false, false),
		nil, nil,
	)

	blocks := []slackapi.Block{header, divider, fieldBlock, descBlock}

	if len(req.Commands) > 0 {
		cmdLines := make([]string, len(req.Commands))
		for i, c := range req.Commands {
			cmdLines[i] = fmt.Sprintf("`%s`", c)
		}
		cmdBlock := slackapi.NewSectionBlock(
			slackapi.NewTextBlockObject(slackapi.MarkdownType,
				"*Commands*\n"+strings.Join(cmdLines, "\n"), false, false),
			nil, nil,
		)
		blocks = append(blocks, cmdBlock)
	}

	approveBtn := slackapi.NewButtonBlockElement(
		ActionIDApprove,
		fmt.Sprintf("approve:%s", req.ActionID),
		slackapi.NewTextBlockObject(slackapi.PlainTextType, "Approve", false, false),
	)
	approveBtn.Style = slackapi.StylePrimary

	rejectBtn := slackapi.NewButtonBlockElement(
		ActionIDReject,
		fmt.Sprintf("reject:%s", req.ActionID),
		slackapi.NewTextBlockObject(slackapi.PlainTextType, "Reject", false, false),
	)
	rejectBtn.Style = slackapi.StyleDanger

	actionBlock := slackapi.NewActionBlock("",
		approveBtn,
		rejectBtn,
	)
	blocks = append(blocks, slackapi.NewDividerBlock(), actionBlock)

	return blocks
}
