package service_test

import (
	"context"
	"testing"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
	"github.com/jonny/opsai-bot/internal/domain/service"
)

func suggestions() []outbound.SuggestedAction {
	return []outbound.SuggestedAction{
		{
			Description: "restart deployment",
			Commands:    []string{"kubectl rollout restart deployment/app"},
			Risk:        "low",
			Reversible:  true,
		},
		{
			Description: "scale down",
			Commands:    []string{"kubectl scale deployment/app --replicas=0"},
			Risk:        "medium",
		},
	}
}

func TestActionPlanner_Plan_AllAllowed(t *testing.T) {
	k8s := &mockK8s{
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "low"},
	}
	planner := service.NewActionPlanner(k8s)

	actions, err := planner.Plan(context.Background(), "analysis-1", "alert-1", suggestions(), "dev", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if a.AnalysisID != "analysis-1" {
			t.Errorf("expected analysisID=analysis-1, got %s", a.AnalysisID)
		}
		if a.AlertID != "alert-1" {
			t.Errorf("expected alertID=alert-1, got %s", a.AlertID)
		}
		if a.Environment != "dev" {
			t.Errorf("expected env=dev, got %s", a.Environment)
		}
		if a.Namespace != "default" {
			t.Errorf("expected namespace=default, got %s", a.Namespace)
		}
	}
}

func TestActionPlanner_Plan_DeniedCommands_Filtered(t *testing.T) {
	k8s := &mockK8s{
		validateResult: outbound.CommandValidation{Allowed: false, Reason: "command not whitelisted"},
	}
	planner := service.NewActionPlanner(k8s)

	actions, err := planner.Plan(context.Background(), "analysis-1", "alert-1", suggestions(), "prod", "kube-system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions when all commands denied, got %d", len(actions))
	}
}

func TestActionPlanner_Plan_PartialDeny(t *testing.T) {
	// First command allowed, second denied.
	callCount := 0
	k8s := &callCountK8s{
		responses: []outbound.CommandValidation{
			{Allowed: true, Risk: "low"},
			{Allowed: false, Reason: "denied"},
		},
		callIdx: &callCount,
	}
	planner := service.NewActionPlanner(k8s)

	twoCommandSuggestion := []outbound.SuggestedAction{
		{
			Description: "multi-step",
			Commands:    []string{"kubectl get pods", "kubectl exec -it pod -- sh"},
			Risk:        "medium",
		},
	}

	actions, err := planner.Plan(context.Background(), "a", "b", twoCommandSuggestion, "staging", "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should keep the one allowed command.
	if len(actions) != 1 {
		t.Errorf("expected 1 action (partial deny), got %d", len(actions))
	}
	if len(actions[0].Commands) != 1 {
		t.Errorf("expected 1 command in action, got %d", len(actions[0].Commands))
	}
}

func TestActionPlanner_Plan_InferActionType(t *testing.T) {
	k8s := &mockK8s{
		validateResult: outbound.CommandValidation{Allowed: true, Risk: "low"},
	}
	planner := service.NewActionPlanner(k8s)

	cases := []struct {
		cmds     []string
		wantType model.ActionType
	}{
		{[]string{"kubectl rollout restart deployment/app"}, model.ActionTypeRestart},
		{[]string{"kubectl scale deployment/app --replicas=2"}, model.ActionTypeScale},
		{[]string{"kubectl delete pod app-abc"}, model.ActionTypeDeletePod},
		{[]string{"kubectl exec -it pod -- sh"}, model.ActionTypeExec},
		{[]string{"kubectl get pods"}, model.ActionTypeKubectl},
	}

	for _, tc := range cases {
		sug := []outbound.SuggestedAction{{Description: "test", Commands: tc.cmds, Risk: "low"}}
		actions, err := planner.Plan(context.Background(), "a", "b", sug, "dev", "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Type != tc.wantType {
			t.Errorf("cmd=%v: want type %s, got %s", tc.cmds, tc.wantType, actions[0].Type)
		}
	}
}

func TestActionPlanner_Plan_EmptySuggestions(t *testing.T) {
	k8s := &mockK8s{}
	planner := service.NewActionPlanner(k8s)

	actions, err := planner.Plan(context.Background(), "a", "b", nil, "dev", "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty suggestions, got %d", len(actions))
	}
}

// callCountK8s cycles through a list of CommandValidation responses.
type callCountK8s struct {
	responses []outbound.CommandValidation
	callIdx   *int
	mockK8s
}

func (c *callCountK8s) ValidateCommand(_ []string) outbound.CommandValidation {
	if *c.callIdx >= len(c.responses) {
		return c.responses[len(c.responses)-1]
	}
	r := c.responses[*c.callIdx]
	*c.callIdx++
	return r
}
