package slackbot

import (
	"context"
	"fmt"
	"log"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/slackbot/template"
	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
)

// handleEventsAPI processes Slack Events API payloads (e.g. message events).
func (b *Bot) handleEventsAPI(ctx context.Context, evt socketmode.Event) {
	b.socketMode.Ack(*evt.Request)

	eventsPayload, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	innerEvent := eventsPayload.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		b.processMessageEvent(ctx, ev)
	}
}

// processMessageEvent routes a Slack message event to the InteractionPort.
func (b *Bot) processMessageEvent(ctx context.Context, ev *slackevents.MessageEvent) {
	// Ignore bot messages to prevent loops.
	if ev.BotID != "" || ev.SubType == "bot_message" {
		return
	}
	// Only handle threaded replies (ThreadTimeStamp set).
	if ev.ThreadTimeStamp == "" {
		return
	}

	alertID := extractAlertID(ev.ThreadTimeStamp)

	req := inbound.MessageRequest{
		ThreadID:  ev.ThreadTimeStamp,
		ChannelID: ev.Channel,
		UserID:    ev.User,
		UserName:  ev.User,
		Text:      ev.Text,
		AlertID:   alertID,
	}

	resp, err := b.interaction.HandleMessage(ctx, req)
	if err != nil {
		log.Printf("handleMessage error: %v", err)
		return
	}

	if resp.Text != "" {
		_, _, postErr := b.client.PostMessageContext(ctx, ev.Channel,
			slackapi.MsgOptionText(resp.Text, false),
			slackapi.MsgOptionTS(ev.ThreadTimeStamp),
		)
		if postErr != nil {
			log.Printf("post reply error: %v", postErr)
		}
	}
}

// handleInteraction processes Slack interactive component payloads (button clicks).
func (b *Bot) handleInteraction(ctx context.Context, evt socketmode.Event) {
	b.socketMode.Ack(*evt.Request)

	callback, ok := evt.Data.(slackapi.InteractionCallback)
	if !ok {
		return
	}

	for _, actionBlock := range callback.ActionCallback.BlockActions {
		switch actionBlock.ActionID {
		case template.ActionIDApprove:
			b.processApproval(ctx, callback, actionBlock, true)
		case template.ActionIDReject:
			b.processApproval(ctx, callback, actionBlock, false)
		}
	}
}

// processApproval routes an approve/reject button click to the InteractionPort.
func (b *Bot) processApproval(ctx context.Context, callback slackapi.InteractionCallback, action *slackapi.BlockAction, approved bool) {
	// Value format: "approve:<actionID>" or "reject:<actionID>"
	parts := strings.SplitN(action.Value, ":", 2)
	actionID := ""
	if len(parts) == 2 {
		actionID = parts[1]
	}

	req := inbound.ApprovalRequest{
		ActionID:   actionID,
		Approved:   approved,
		ApprovedBy: callback.User.ID,
		Reason:     "",
	}

	if err := b.interaction.HandleApproval(ctx, req); err != nil {
		log.Printf("handleApproval error: %v", err)
		return
	}

	status := "approved"
	if !approved {
		status = "rejected"
	}
	responseText := fmt.Sprintf(":white_check_mark: Action `%s` has been *%s* by <@%s>",
		actionID, status, callback.User.ID)

	_, _, err := b.client.PostMessageContext(ctx, callback.Channel.ID,
		slackapi.MsgOptionText(responseText, false),
		slackapi.MsgOptionTS(callback.Message.ThreadTimestamp),
	)
	if err != nil {
		log.Printf("post approval response error: %v", err)
	}
}

// handleSlashCommand processes /opsai slash commands.
func (b *Bot) handleSlashCommand(ctx context.Context, evt socketmode.Event) {
	cmd, ok := evt.Data.(slackapi.SlashCommand)
	if !ok {
		b.socketMode.Ack(*evt.Request)
		return
	}

	var responseText string
	switch strings.TrimSpace(strings.ToLower(cmd.Text)) {
	case "status":
		responseText = ":robot_face: *OpsAI Bot* is running and monitoring your infrastructure."
	case "help":
		responseText = buildHelpText()
	default:
		sanitized := cmd.Text
		if len(sanitized) > 100 {
			sanitized = sanitized[:100]
		}
		sanitized = strings.ReplaceAll(sanitized, "`", "'")
		responseText = fmt.Sprintf(":question: Unknown command `%s`. Try `/opsai help`.", sanitized)
	}

	b.socketMode.Ack(*evt.Request, map[string]string{
		"text": responseText,
	})
}

// extractAlertID derives an alertID from a Slack thread timestamp.
func extractAlertID(threadTS string) string {
	return threadTS
}

// buildHelpText returns the help message for the /opsai slash command.
func buildHelpText() string {
	return strings.Join([]string{
		":robot_face: *OpsAI Bot Commands*",
		"",
		"*Slash Commands:*",
		"\u2022 `/opsai status` \u2014 Check bot status",
		"\u2022 `/opsai help` \u2014 Show this help message",
		"",
		"*Thread Interaction:*",
		"\u2022 Reply in an alert thread to interact with AI analysis",
		"\u2022 Use Approve/Reject buttons to manage actions",
	}, "\n")
}
