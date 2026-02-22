package outbound

import "context"

type DiagnosisRequest struct {
	AlertSummary    string
	K8sContext      string
	PreviousActions []ActionSummary
	Environment     string
	Constraints     []string
}

type ActionSummary struct {
	Command   string
	Result    string
	Timestamp int64
}

type DiagnosisResult struct {
	RootCause        string
	Severity         string
	Confidence       float64
	SuggestedActions []SuggestedAction
	Explanation      string
	NeedsMoreInfo    bool
	FollowUpQueries  []string
}

type SuggestedAction struct {
	Description string
	Commands    []string
	Risk        string
	Reversible  bool
}

type ConversationRequest struct {
	ThreadID    string
	UserMessage string
	History     []Message
	AlertID     string
	K8sContext  string
}

type Message struct {
	Role    string
	Content string
}

type ConversationResponse struct {
	Reply            string
	SuggestedActions []SuggestedAction
	NeedsApproval    bool
}

type ModelInfo struct {
	Provider    string
	Model       string
	MaxTokens   int
	ContextSize int
}

// LLMProvider abstracts interaction with LLM services.
type LLMProvider interface {
	Diagnose(ctx context.Context, req DiagnosisRequest) (DiagnosisResult, error)
	Converse(ctx context.Context, req ConversationRequest) (ConversationResponse, error)
	HealthCheck(ctx context.Context) error
	ModelInfo(ctx context.Context) (ModelInfo, error)
}
