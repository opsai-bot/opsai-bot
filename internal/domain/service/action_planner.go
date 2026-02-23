package service

import (
	"context"
	"strings"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// ActionPlanner converts LLM-suggested actions into validated model.Action instances.
type ActionPlanner struct {
	k8s outbound.K8sExecutor
}

// NewActionPlanner creates a new ActionPlanner.
func NewActionPlanner(k8s outbound.K8sExecutor) *ActionPlanner {
	return &ActionPlanner{k8s: k8s}
}

// Plan converts a slice of LLM suggestions into validated model.Action records.
// Commands failing K8s whitelist validation are filtered out. Actions with no
// valid commands are also excluded.
func (p *ActionPlanner) Plan(
	ctx context.Context,
	analysisID, alertID string,
	suggestions []outbound.SuggestedAction,
	env, ns string,
) ([]model.Action, error) {
	actions := make([]model.Action, 0, len(suggestions))

	for _, suggestion := range suggestions {
		validCmds, risk, deny := p.validateCommands(suggestion.Commands)
		if deny != "" {
			// All commands were denied; skip this suggestion.
			continue
		}
		if len(validCmds) == 0 {
			continue
		}

		actionType := p.inferActionType(validCmds)
		riskLevel := p.toRiskLevel(risk)

		action := model.NewAction(analysisID, alertID, actionType, suggestion.Description, validCmds, riskLevel).
			WithEnvironment(env).
			WithNamespace(ns).
			WithReversible(suggestion.Reversible)

		actions = append(actions, action)
	}

	return actions, nil
}

// validateCommands runs each command through the K8s whitelist. It returns the
// list of allowed commands, the highest risk level seen, and a non-empty deny
// reason if ALL commands were denied.
func (p *ActionPlanner) validateCommands(commands []string) (allowed []string, maxRisk string, denyReason string) {
	maxRiskVal := -1
	deniedAll := true

	for _, cmd := range commands {
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}
		v := p.k8s.ValidateCommand(parts)
		if !v.Allowed {
			denyReason = v.Reason
			continue
		}
		deniedAll = false
		allowed = append(allowed, cmd)
		if ord, ok := riskOrder[v.Risk]; ok && ord > maxRiskVal {
			maxRiskVal = ord
			maxRisk = v.Risk
		}
	}

	if deniedAll && len(commands) > 0 {
		return nil, "", denyReason
	}
	if maxRisk == "" {
		maxRisk = "low"
	}
	return allowed, maxRisk, ""
}

// inferActionType guesses the ActionType from the first command token.
func (p *ActionPlanner) inferActionType(commands []string) model.ActionType {
	if len(commands) == 0 {
		return model.ActionTypeManual
	}
	parts := strings.Fields(commands[0])
	if len(parts) < 2 {
		return model.ActionTypeKubectl
	}
	switch parts[1] {
	case "rollout":
		return model.ActionTypeRestart
	case "scale":
		return model.ActionTypeScale
	case "delete":
		if len(parts) >= 3 && parts[2] == "pod" {
			return model.ActionTypeDeletePod
		}
		return model.ActionTypeKubectl
	case "exec":
		return model.ActionTypeExec
	default:
		return model.ActionTypeKubectl
	}
}

// toRiskLevel converts a raw risk string from K8s validation into model.RiskLevel.
func (p *ActionPlanner) toRiskLevel(risk string) model.RiskLevel {
	switch risk {
	case "low":
		return model.RiskLow
	case "medium":
		return model.RiskMedium
	case "high":
		return model.RiskHigh
	case "critical":
		return model.RiskCritical
	default:
		return model.RiskMedium
	}
}
