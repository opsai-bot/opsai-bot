package inbound

import "context"

// InteractionPort handles user interactions from messaging platforms.
type InteractionPort interface {
	HandleMessage(ctx context.Context, req MessageRequest) (MessageResponse, error)
	HandleApproval(ctx context.Context, req ApprovalRequest) error
}

type MessageRequest struct {
	ThreadID  string
	ChannelID string
	UserID    string
	UserName  string
	Text      string
	AlertID   string
}

type MessageResponse struct {
	Text            string
	SuggestedActions []SuggestedActionInfo
	NeedsApproval   bool
}

type SuggestedActionInfo struct {
	Description string
	Commands    []string
	Risk        string
}

type ApprovalRequest struct {
	ActionID   string
	Approved   bool
	ApprovedBy string
	Reason     string
}
