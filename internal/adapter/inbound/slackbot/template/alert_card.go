package template

import (
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// severityColor maps alert severity to Slack attachment color.
func severityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	case "info", "resolved":
		return "good"
	default:
		return "#439FE0"
	}
}

// severityEmoji maps severity to an emoji prefix.
func severityEmoji(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return ":red_circle:"
	case "warning":
		return ":large_yellow_circle:"
	case "resolved":
		return ":large_green_circle:"
	default:
		return ":large_blue_circle:"
	}
}

// BuildAlertBlocks constructs Block Kit blocks for an alert notification.
func BuildAlertBlocks(n outbound.AlertNotification) []slackapi.Block {
	emoji := severityEmoji(n.Severity)
	header := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("%s *%s*", emoji, n.Title), false, false),
		nil, nil,
	)

	divider := slackapi.NewDividerBlock()

	fields := []*slackapi.TextBlockObject{
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Severity*\n%s", strings.ToUpper(n.Severity)), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Environment*\n%s", n.Environment), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Source*\n%s", n.Source), false, false),
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Alert ID*\n`%s`", n.AlertID), false, false),
	}

	fieldBlock := slackapi.NewSectionBlock(nil, fields, nil)

	summaryBlock := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType,
			fmt.Sprintf("*Summary*\n%s", n.Summary), false, false),
		nil, nil,
	)

	blocks := []slackapi.Block{header, divider, fieldBlock, summaryBlock}

	if len(n.Labels) > 0 {
		labels := make([]string, 0, len(n.Labels))
		for k, v := range n.Labels {
			labels = append(labels, fmt.Sprintf("`%s=%s`", k, v))
		}
		labelBlock := slackapi.NewContextBlock("",
			slackapi.NewTextBlockObject(slackapi.MarkdownType,
				"Labels: "+strings.Join(labels, "  "), false, false),
		)
		blocks = append(blocks, labelBlock)
	}

	return blocks
}
