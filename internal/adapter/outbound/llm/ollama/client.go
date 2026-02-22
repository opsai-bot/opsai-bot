package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/adapter/outbound/llm/prompt"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// Config holds configuration for the Ollama client.
type Config struct {
	BaseURL      string
	Model        string
	Timeout      time.Duration
	MaxRetries   int
	SystemPrompt string
	Temperature  float64
}

// Client implements outbound.LLMProvider using the Ollama API.
type Client struct {
	config     Config
	httpClient *http.Client
	builder    *prompt.Builder
}

// NewClient creates a new Ollama Client with the given configuration.
func NewClient(cfg Config) (*Client, error) {
	builder, err := prompt.NewBuilder()
	if err != nil {
		return nil, fmt.Errorf("creating prompt builder: %w", err)
	}
	return &Client{
		config:     cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		builder:    builder,
	}, nil
}

// --- Ollama API types ---

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  chatOptions   `json:"options,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
}

type chatResponse struct {
	Message         chatMessage `json:"message"`
	TotalDuration   int64       `json:"total_duration"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	EvalCount       int         `json:"eval_count"`
}

// llmDiagnosisResult mirrors the JSON the LLM returns for diagnosis.
type llmDiagnosisResult struct {
	RootCause        string               `json:"root_cause"`
	Severity         string               `json:"severity"`
	Confidence       float64              `json:"confidence"`
	Explanation      string               `json:"explanation"`
	SuggestedActions []llmSuggestedAction `json:"suggested_actions"`
	NeedsMoreInfo    bool                 `json:"needs_more_info"`
	FollowUpQueries  []string             `json:"follow_up_queries"`
}

type llmSuggestedAction struct {
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
	Risk        string   `json:"risk"`
	Reversible  bool     `json:"reversible"`
}

// llmConversationResult mirrors the JSON the LLM returns for conversation.
type llmConversationResult struct {
	Reply            string               `json:"reply"`
	SuggestedActions []llmSuggestedAction `json:"suggested_actions"`
	NeedsApproval    bool                 `json:"needs_approval"`
}

// tagsResponse is returned by GET /api/tags.
type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// --- LLMProvider implementation ---

// Diagnose builds a diagnosis prompt and calls Ollama, then parses the result.
func (c *Client) Diagnose(ctx context.Context, req outbound.DiagnosisRequest) (outbound.DiagnosisResult, error) {
	actions := make([]prompt.ActionInput, len(req.PreviousActions))
	for i, a := range req.PreviousActions {
		actions[i] = prompt.ActionInput{Command: a.Command, Result: a.Result}
	}

	promptText, err := c.builder.BuildDiagnosePrompt(prompt.DiagnoseInput{
		AlertSummary:    req.AlertSummary,
		K8sContext:      req.K8sContext,
		Environment:     req.Environment,
		Constraints:     req.Constraints,
		PreviousActions: actions,
	})
	if err != nil {
		return outbound.DiagnosisResult{}, fmt.Errorf("building diagnose prompt: %w", err)
	}

	messages := []chatMessage{}
	if c.config.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: c.config.SystemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: promptText})

	raw, err := c.doChat(ctx, messages)
	if err != nil {
		return outbound.DiagnosisResult{}, err
	}

	var llmResult llmDiagnosisResult
	if err := parseJSONFromContent(raw, &llmResult); err != nil {
		return outbound.DiagnosisResult{}, fmt.Errorf("parsing diagnosis response: %w", err)
	}

	return mapDiagnosisResult(llmResult), nil
}

// Converse builds a conversation prompt and calls Ollama.
func (c *Client) Converse(ctx context.Context, req outbound.ConversationRequest) (outbound.ConversationResponse, error) {
	history := make([]prompt.MessageInput, len(req.History))
	for i, m := range req.History {
		history[i] = prompt.MessageInput{Role: m.Role, Content: m.Content}
	}

	promptText, err := c.builder.BuildConversePrompt(prompt.ConverseInput{
		AlertContext: req.AlertID,
		K8sContext:   req.K8sContext,
		UserMessage:  req.UserMessage,
		History:      history,
	})
	if err != nil {
		return outbound.ConversationResponse{}, fmt.Errorf("building converse prompt: %w", err)
	}

	messages := []chatMessage{}
	if c.config.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: c.config.SystemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: promptText})

	raw, err := c.doChat(ctx, messages)
	if err != nil {
		return outbound.ConversationResponse{}, err
	}

	var llmResult llmConversationResult
	if err := parseJSONFromContent(raw, &llmResult); err != nil {
		return outbound.ConversationResponse{}, fmt.Errorf("parsing conversation response: %w", err)
	}

	return mapConversationResponse(llmResult), nil
}

// HealthCheck performs GET /api/tags to verify Ollama is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	url := c.config.BaseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// ModelInfo returns metadata about the configured model.
func (c *Client) ModelInfo(_ context.Context) (outbound.ModelInfo, error) {
	return outbound.ModelInfo{
		Provider:    "ollama",
		Model:       c.config.Model,
		MaxTokens:   0,
		ContextSize: 0,
	}, nil
}

// --- Internal helpers ---

// doChat sends a chat request to Ollama with retry logic for transient errors.
func (c *Client) doChat(ctx context.Context, messages []chatMessage) (string, error) {
	body := chatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Stream:   false,
		Options:  chatOptions{Temperature: c.config.Temperature},
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encoding chat request: %w", err)
	}

	maxRetries := c.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		raw, err := c.postChat(ctx, encoded)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		// Only retry on transient/server errors, not on context cancellation.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", lastErr
}

func (c *Client) postChat(ctx context.Context, body []byte) (string, error) {
	url := c.config.BaseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ollama response: %w", err)
	}

	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("ollama server error %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// parseJSONFromContent extracts JSON from content that may be wrapped in markdown code fences.
func parseJSONFromContent(content string, dst interface{}) error {
	content = strings.TrimSpace(content)

	// Strip markdown code fences if present.
	if idx := strings.Index(content, "```"); idx != -1 {
		content = content[idx+3:]
		// Strip optional language tag (e.g. "json\n")
		if nl := strings.Index(content, "\n"); nl != -1 {
			content = content[nl+1:]
		}
		if end := strings.LastIndex(content, "```"); end != -1 {
			content = content[:end]
		}
		content = strings.TrimSpace(content)
	}

	// Find the outermost JSON object.
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return fmt.Errorf("no JSON object found in LLM response")
	}
	content = content[start : end+1]

	return json.Unmarshal([]byte(content), dst)
}

func mapDiagnosisResult(r llmDiagnosisResult) outbound.DiagnosisResult {
	actions := make([]outbound.SuggestedAction, len(r.SuggestedActions))
	for i, a := range r.SuggestedActions {
		actions[i] = outbound.SuggestedAction{
			Description: a.Description,
			Commands:    a.Commands,
			Risk:        a.Risk,
			Reversible:  a.Reversible,
		}
	}
	return outbound.DiagnosisResult{
		RootCause:        r.RootCause,
		Severity:         r.Severity,
		Confidence:       r.Confidence,
		Explanation:      r.Explanation,
		SuggestedActions: actions,
		NeedsMoreInfo:    r.NeedsMoreInfo,
		FollowUpQueries:  r.FollowUpQueries,
	}
}

func mapConversationResponse(r llmConversationResult) outbound.ConversationResponse {
	actions := make([]outbound.SuggestedAction, len(r.SuggestedActions))
	for i, a := range r.SuggestedActions {
		actions[i] = outbound.SuggestedAction{
			Description: a.Description,
			Commands:    a.Commands,
			Risk:        a.Risk,
			Reversible:  a.Reversible,
		}
	}
	return outbound.ConversationResponse{
		Reply:            r.Reply,
		SuggestedActions: actions,
		NeedsApproval:    r.NeedsApproval,
	}
}
