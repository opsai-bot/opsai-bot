package prompt

import (
	"strings"
	"testing"
)

func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	b, err := NewBuilder()
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	return b
}

func TestBuildDiagnosePrompt(t *testing.T) {
	b := newTestBuilder(t)

	input := DiagnoseInput{
		AlertSummary: "Pod OOMKilled in namespace default",
		K8sContext:   "pod/myapp status: OOMKilled",
		Environment:  "production",
		Constraints:  []string{"no restarts allowed", "read-only namespace"},
		PreviousActions: []ActionInput{
			{Command: "kubectl describe pod myapp", Result: "OOMKilled exit code 137"},
		},
	}

	out, err := b.BuildDiagnosePrompt(input)
	if err != nil {
		t.Fatalf("BuildDiagnosePrompt: %v", err)
	}

	checks := []string{
		"Pod OOMKilled in namespace default",
		"pod/myapp status: OOMKilled",
		"production",
		"no restarts allowed",
		"read-only namespace",
		"kubectl describe pod myapp",
		"OOMKilled exit code 137",
		"root_cause",
		"suggested_actions",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("diagnose prompt missing %q", want)
		}
	}
}

func TestBuildDiagnosePrompt_NoConstraintsNoActions(t *testing.T) {
	b := newTestBuilder(t)

	input := DiagnoseInput{
		AlertSummary: "High CPU",
		K8sContext:   "node cpu=95%",
		Environment:  "staging",
	}

	out, err := b.BuildDiagnosePrompt(input)
	if err != nil {
		t.Fatalf("BuildDiagnosePrompt: %v", err)
	}

	if !strings.Contains(out, "High CPU") {
		t.Error("prompt missing AlertSummary")
	}
	// Constraints section should not appear when empty.
	if strings.Contains(out, "## Constraints") {
		t.Error("prompt should not contain Constraints section when empty")
	}
}

func TestBuildConversePrompt(t *testing.T) {
	b := newTestBuilder(t)

	input := ConverseInput{
		AlertContext: "alert-123: CrashLoopBackOff",
		K8sContext:   "pod/myapp: CrashLoopBackOff",
		UserMessage:  "What should I check first?",
		History: []MessageInput{
			{Role: "user", Content: "Is the pod running?"},
			{Role: "assistant", Content: "No, it is in CrashLoopBackOff."},
		},
	}

	out, err := b.BuildConversePrompt(input)
	if err != nil {
		t.Fatalf("BuildConversePrompt: %v", err)
	}

	checks := []string{
		"alert-123: CrashLoopBackOff",
		"pod/myapp: CrashLoopBackOff",
		"What should I check first?",
		"Is the pod running?",
		"No, it is in CrashLoopBackOff.",
		"reply",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("converse prompt missing %q", want)
		}
	}
}

func TestBuildConversePrompt_NoHistory(t *testing.T) {
	b := newTestBuilder(t)

	input := ConverseInput{
		AlertContext: "alert-456",
		K8sContext:   "node: Ready",
		UserMessage:  "Any issues?",
	}

	out, err := b.BuildConversePrompt(input)
	if err != nil {
		t.Fatalf("BuildConversePrompt: %v", err)
	}

	if !strings.Contains(out, "Any issues?") {
		t.Error("prompt missing UserMessage")
	}
	// History section should not appear when empty.
	if strings.Contains(out, "## Conversation History") {
		t.Error("prompt should not contain Conversation History section when empty")
	}
}
