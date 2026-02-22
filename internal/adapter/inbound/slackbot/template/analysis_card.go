package template

import (
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// confidenceBar builds a visual progress bar string for confidence percentage.
func confidenceBar(confidence float64) string {
	pct := int(confidence * 100)
	filled := pct / 10
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled
	return fmt.Sprintf("[%s%s] %d%%",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		pct,
	)
}

// BuildAnalysisBlocks constructs Block Kit blocks for an analysis notification.
func BuildAnalysisBlocks(n outbound.AnalysisNotification) []slackapi.Block {
	header := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			":brain: *AI Analysis Complete*", false, false),
		nil, nil,
	)

	divider := slackapi.NewDividerBlock()

	confidenceBlock := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Confidence*\n`%s`", confidenceBar(n.Confidence)), false, false),
		nil, nil,
	)

	rootCauseBlock := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Root Cause*\n%s", n.RootCause), false, false),
		nil, nil,
	)

	blocks := []slackapi.Block{header, divider, confidenceBlock, rootCauseBlock}

	if n.Explanation != "" {
		explanationBlock := slackapi.NewSectionBlock(
			slackapi.NewTextBlockObject(slackapi.MarkdownType,
				fmt.Sprintf("*Explanation*\n%s", n.Explanation), false, false),
			nil, nil,
		)
		blocks = append(blocks, explanationBlock)
	}

	if n.Severity != "" {
		severityBlock := slackapi.NewSectionBlock(
			slackapi.NewTextBlockObject(slackapi.MarkdownType,
				fmt.Sprintf("*Assessed Severity*\n%s %s",
					severityEmoji(n.Severity), strings.ToUpper(n.Severity)), false, false),
			nil, nil,
		)
		blocks = append(blocks, severityBlock)
	}

	if len(n.Actions) > 0 {
		blocks = append(blocks, slackapi.NewDividerBlock())
		actionLines := make([]string, 0, len(n.Actions))
		for i, a := range n.Actions {
			riskTag := ""
			if a.Risk != "" {
				riskTag = fmt.Sprintf(" _(risk: %s)_", a.Risk)
			}
			actionLines = append(actionLines,
				fmt.Sprintf("%d. *%s*%s", i+1, a.Description, riskTag))
			if a.Command != "" {
				actionLines = append(actionLines, fmt.Sprintf("   `%s`", a.Command))
			}
		}
		actionsBlock := slackapi.NewSectionBlock(
			slackapi.NewTextBlockObject(slackapi.MarkdownType,
				"*Suggested Actions*\n"+strings.Join(actionLines, "\n"), false, false),
			nil, nil,
		)
		blocks = append(blocks, actionsBlock)
	}

	return blocks
}
