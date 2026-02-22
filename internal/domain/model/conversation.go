package model

import "time"

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

type ConversationThread struct {
	ID        string        `json:"id"`
	AlertID   string        `json:"alert_id"`
	ThreadID  string        `json:"thread_id"`
	ChannelID string        `json:"channel_id"`
	Messages  []ChatMessage `json:"messages"`
	Active    bool          `json:"active"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type ChatMessage struct {
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	UserID    string      `json:"user_id"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewConversationThread(alertID, threadID, channelID string) ConversationThread {
	now := time.Now().UTC()
	return ConversationThread{
		ID:        generateID(),
		AlertID:   alertID,
		ThreadID:  threadID,
		ChannelID: channelID,
		Messages:  []ChatMessage{},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (c ConversationThread) AddMessage(role MessageRole, content, userID string) ConversationThread {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		UserID:    userID,
		Timestamp: time.Now().UTC(),
	}
	messages := make([]ChatMessage, len(c.Messages), len(c.Messages)+1)
	copy(messages, c.Messages)
	messages = append(messages, msg)
	c.Messages = messages
	c.UpdatedAt = time.Now().UTC()
	return c
}

func (c ConversationThread) Close() ConversationThread {
	c.Active = false
	c.UpdatedAt = time.Now().UTC()
	return c
}

func (c ConversationThread) LastNMessages(n int) []ChatMessage {
	if n >= len(c.Messages) {
		return c.Messages
	}
	return c.Messages[len(c.Messages)-n:]
}
