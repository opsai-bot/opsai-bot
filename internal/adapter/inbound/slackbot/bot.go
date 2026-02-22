package slackbot

import (
	"context"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
)

// Config holds Slack bot configuration.
type Config struct {
	BotToken string
	AppToken string
}

// Bot handles incoming Slack events via Socket Mode.
type Bot struct {
	client      *slackapi.Client
	socketMode  *socketmode.Client
	interaction inbound.InteractionPort
}

// NewBot creates a new Bot with Socket Mode enabled.
func NewBot(cfg Config, interaction inbound.InteractionPort) *Bot {
	client := slackapi.New(cfg.BotToken, slackapi.OptionAppLevelToken(cfg.AppToken))
	sm := socketmode.New(client)
	return &Bot{
		client:      client,
		socketMode:  sm,
		interaction: interaction,
	}
}

// Start begins processing Slack events. It blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	go b.handleEvents(ctx)
	return b.socketMode.RunContext(ctx)
}

// handleEvents dispatches incoming Socket Mode events to the appropriate handler.
func (b *Bot) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-b.socketMode.Events:
			if !ok {
				return
			}
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				b.handleEventsAPI(ctx, evt)
			case socketmode.EventTypeInteractive:
				b.handleInteraction(ctx, evt)
			case socketmode.EventTypeSlashCommand:
				b.handleSlashCommand(ctx, evt)
			default:
				b.socketMode.Ack(*evt.Request)
			}
		}
	}
}
