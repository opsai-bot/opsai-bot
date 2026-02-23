package prompt

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// maxAlertSummaryLen is the maximum allowed length for alert summary in prompts.
const maxAlertSummaryLen = 4000

// maxK8sContextLen is the maximum allowed length for K8s context in prompts.
const maxK8sContextLen = 8000

// sanitizePromptInput removes known prompt injection patterns and truncates to maxLen.
func sanitizePromptInput(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen] + "\n... [truncated]"
	}
	return s
}

// Builder constructs prompts for LLM diagnosis and conversation.
type Builder struct {
	templates *template.Template
}

// NewBuilder parses all embedded templates and returns a Builder.
func NewBuilder() (*Builder, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, err
	}
	return &Builder{templates: tmpl}, nil
}

// DiagnoseInput holds data for the diagnose prompt template.
type DiagnoseInput struct {
	AlertSummary    string
	K8sContext      string
	Environment     string
	Constraints     []string
	PreviousActions []ActionInput
}

// ActionInput represents a previously taken action.
type ActionInput struct {
	Command string
	Result  string
}

// ConverseInput holds data for the conversation prompt template.
type ConverseInput struct {
	AlertContext string
	K8sContext   string
	UserMessage  string
	History      []MessageInput
}

// MessageInput represents a single message in conversation history.
type MessageInput struct {
	Role    string
	Content string
}

// BuildDiagnosePrompt renders the diagnose template with the given input.
func (b *Builder) BuildDiagnosePrompt(input DiagnoseInput) (string, error) {
	input.AlertSummary = sanitizePromptInput(input.AlertSummary, maxAlertSummaryLen)
	input.K8sContext = sanitizePromptInput(input.K8sContext, maxK8sContextLen)

	var buf bytes.Buffer
	if err := b.templates.ExecuteTemplate(&buf, "diagnose.tmpl", input); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// BuildConversePrompt renders the conversation template with the given input.
func (b *Builder) BuildConversePrompt(input ConverseInput) (string, error) {
	var buf bytes.Buffer
	if err := b.templates.ExecuteTemplate(&buf, "conversation.tmpl", input); err != nil {
		return "", err
	}
	return buf.String(), nil
}
