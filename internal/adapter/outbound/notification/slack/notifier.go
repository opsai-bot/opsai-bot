package slack

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// Config holds Slack notifier configuration.
type Config struct {
	BotToken       string
	DefaultChannel string
	Channels       map[string]string // env -> channel ID
}

// Notifier implements outbound.Notifier via the Slack API.
type Notifier struct {
	client *slackapi.Client
	config Config
}

// NewNotifier creates a new Slack Notifier.
func NewNotifier(cfg Config) *Notifier {
	return &Notifier{
		client: slackapi.New(cfg.BotToken),
		config: cfg,
	}
}

// channelFor returns the channel to post to for a given environment.
func (n *Notifier) channelFor(env string) string {
	if ch, ok := n.config.Channels[env]; ok {
		return ch
	}
	return n.config.DefaultChannel
}

// NotifyAlert posts a rich Block Kit alert card and returns the thread timestamp as threadID.
func (n *Notifier) NotifyAlert(ctx context.Context, notification outbound.AlertNotification) (string, error) {
	blocks := template.BuildAlertBlocks(notification)
	channel := n.channelFor(notification.Environment)

	_, ts, err := n.client.PostMessageContext(ctx, channel,
		slackapi.MsgOptionBlocks(blocks...),
		slackapi.MsgOptionText(fmt.Sprintf("[%s] %s", strings.ToUpper(notification.Severity), notification.Title), false),
	)
	if err != nil {
		return "", fmt.Errorf("slack NotifyAlert: %w", err)
	}
	return ts, nil
}

// NotifyAnalysis posts an analysis result in the alert thread.
func (n *Notifier) NotifyAnalysis(ctx context.Context, notification outbound.AnalysisNotification) error {
	blocks := template.BuildAnalysisBlocks(notification)
	channel := n.channelFor("")

	_, _, err := n.client.PostMessageContext(ctx, channel,
		slackapi.MsgOptionBlocks(blocks...),
		slackapi.MsgOptionTS(notification.ThreadID),
		slackapi.MsgOptionText("AI Analysis Complete", false),
	)
	if err != nil {
		return fmt.Errorf("slack NotifyAnalysis: %w", err)
	}
	return nil
}

// NotifyAction posts an action status update in the alert thread.
func (n *Notifier) NotifyAction(ctx context.Context, threadID string, action outbound.ActionNotification) error {
	statusEmoji := actionStatusEmoji(action.Status)
	lines := []string{
		fmt.Sprintf("%s *%s*", statusEmoji, action.Description),
	}
	if action.Command != "" {
		lines = append(lines, fmt.Sprintf("`%s`", action.Command))
	}
	if action.Output != "" {
		lines = append(lines, fmt.Sprintf("```\n%s\n```", action.Output))
	}
	if action.Risk != "" {
		lines = append(lines, fmt.Sprintf("_Risk: %s_", action.Risk))
	}

	block := slackapi.NewSectionBlock(
		slackapi.NewTextBlockObject(slackapi.MarkdownType, strings.Join(lines, "\n"), false, false),
		nil, nil,
	)

	channel := n.channelFor("")
	_, _, err := n.client.PostMessageContext(ctx, channel,
		slackapi.MsgOptionBlocks(block),
		slackapi.MsgOptionTS(threadID),
		slackapi.MsgOptionText(fmt.Sprintf("Action: %s", action.Description), false),
	)
	if err != nil {
		return fmt.Errorf("slack NotifyAction: %w", err)
	}
	return nil
}

// RequestApproval posts an approval card with Approve/Reject buttons in the alert thread.
func (n *Notifier) RequestApproval(ctx context.Context, req outbound.ApprovalNotification) error {
	blocks := template.BuildApprovalBlocks(req)
	channel := n.channelFor(req.Environment)

	_, _, err := n.client.PostMessageContext(ctx, channel,
		slackapi.MsgOptionBlocks(blocks...),
		slackapi.MsgOptionTS(req.ThreadID),
		slackapi.MsgOptionText("Action Approval Required", false),
	)
	if err != nil {
		return fmt.Errorf("slack RequestApproval: %w", err)
	}
	return nil
}

// SendMessage posts a simple text message in the thread with an emoji for the level.
func (n *Notifier) SendMessage(ctx context.Context, threadID string, message string, level outbound.NotificationLevel) error {
	emoji := levelEmoji(level)
	text := fmt.Sprintf("%s %s", emoji, message)

	channel := n.channelFor("")
	_, _, err := n.client.PostMessageContext(ctx, channel,
		slackapi.MsgOptionText(text, false),
		slackapi.MsgOptionTS(threadID),
	)
	if err != nil {
		return fmt.Errorf("slack SendMessage: %w", err)
	}
	return nil
}

// actionStatusEmoji maps action status to an emoji.
func actionStatusEmoji(status string) string {
	switch strings.ToLower(status) {
	case "success", "completed":
		return ":white_check_mark:"
	case "failed", "error":
		return ":x:"
	case "running", "in_progress":
		return ":hourglass_flowing_sand:"
	case "skipped":
		return ":fast_forward:"
	default:
		return ":arrow_right:"
	}
}

// levelEmoji maps a notification level to an emoji.
func levelEmoji(level outbound.NotificationLevel) string {
	switch level {
	case outbound.NotificationCritical:
		return ":red_circle:"
	case outbound.NotificationWarning:
		return ":large_yellow_circle:"
	case outbound.NotificationResolved:
		return ":large_green_circle:"
	default:
		return ":information_source:"
	}
}
