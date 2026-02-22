package prompt

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

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
