package service

import (
	"context"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// riskOrder maps risk level strings to comparable integers.
var riskOrder = map[string]int{
	"low":      0,
	"medium":   1,
	"high":     2,
	"critical": 3,
}

// PolicyDecision holds the result of a policy evaluation.
type PolicyDecision struct {
	Allowed      bool
	NeedsApproval bool
	AutoExecute  bool
	Reason       string
	Approvers    []string
	MaxRiskLevel string
}

// PolicyEvaluator evaluates environment policies to decide auto-fix vs approval-required.
type PolicyEvaluator struct {
	repo outbound.PolicyRepository
}

// NewPolicyEvaluator creates a new PolicyEvaluator backed by the given repository.
func NewPolicyEvaluator(repo outbound.PolicyRepository) *PolicyEvaluator {
	return &PolicyEvaluator{repo: repo}
}

// Evaluate looks up the policy for the given environment and determines how the
// requested action should be handled.
func (e *PolicyEvaluator) Evaluate(ctx context.Context, environment string, action model.Action) (PolicyDecision, error) {
	policy, err := e.repo.GetByEnvironment(ctx, environment)
	if err != nil {
		// Fall back to approval-required on policy lookup failure.
		return PolicyDecision{
			Allowed:       true,
			NeedsApproval: true,
			AutoExecute:   false,
			Reason:        fmt.Sprintf("policy lookup failed (%v); defaulting to approval-required", err),
			MaxRiskLevel:  "low",
		}, nil
	}

	if !policy.Enabled {
		return PolicyDecision{
			Allowed:       false,
			NeedsApproval: false,
			AutoExecute:   false,
			Reason:        fmt.Sprintf("policy for environment %q is disabled", environment),
			MaxRiskLevel:  policy.MaxAutoRisk,
		}, nil
	}

	actionRisk := string(action.Risk)

	switch policy.Mode {
	case model.PolicyModeAutoFix:
		if !e.isRiskAcceptable(actionRisk, policy.MaxAutoRisk) {
			return PolicyDecision{
				Allowed:       true,
				NeedsApproval: true,
				AutoExecute:   false,
				Reason:        fmt.Sprintf("action risk %q exceeds max auto-risk %q for env %q", actionRisk, policy.MaxAutoRisk, environment),
				Approvers:     policy.Approvers,
				MaxRiskLevel:  policy.MaxAutoRisk,
			}, nil
		}
		return PolicyDecision{
			Allowed:      true,
			NeedsApproval: false,
			AutoExecute:  true,
			Reason:       fmt.Sprintf("auto_fix policy: risk %q is within limit %q", actionRisk, policy.MaxAutoRisk),
			MaxRiskLevel: policy.MaxAutoRisk,
		}, nil

	case model.PolicyModeWarnAuto:
		if !e.isRiskAcceptable(actionRisk, policy.MaxAutoRisk) {
			return PolicyDecision{
				Allowed:       true,
				NeedsApproval: true,
				AutoExecute:   false,
				Reason:        fmt.Sprintf("warn_auto policy: action risk %q exceeds limit %q; approval required", actionRisk, policy.MaxAutoRisk),
				Approvers:     policy.Approvers,
				MaxRiskLevel:  policy.MaxAutoRisk,
			}, nil
		}
		return PolicyDecision{
			Allowed:      true,
			NeedsApproval: false,
			AutoExecute:  true,
			Reason:       fmt.Sprintf("warn_auto policy: executing with warning (risk %q)", actionRisk),
			MaxRiskLevel: policy.MaxAutoRisk,
		}, nil

	case model.PolicyModeApprovalRequired:
		return PolicyDecision{
			Allowed:       true,
			NeedsApproval: true,
			AutoExecute:   false,
			Reason:        fmt.Sprintf("approval_required policy for environment %q", environment),
			Approvers:     policy.Approvers,
			MaxRiskLevel:  policy.MaxAutoRisk,
		}, nil

	default:
		return PolicyDecision{
			Allowed:       true,
			NeedsApproval: true,
			AutoExecute:   false,
			Reason:        fmt.Sprintf("unknown policy mode %q; defaulting to approval-required", policy.Mode),
			Approvers:     policy.Approvers,
			MaxRiskLevel:  policy.MaxAutoRisk,
		}, nil
	}
}

// isRiskAcceptable returns true when actionRisk is less than or equal to maxRisk
// in the ordering: low < medium < high < critical.
func (e *PolicyEvaluator) isRiskAcceptable(actionRisk, maxRisk string) bool {
	actionOrder, actionOK := riskOrder[actionRisk]
	maxOrder, maxOK := riskOrder[maxRisk]
	if !actionOK || !maxOK {
		// Unknown risk level: treat as unacceptable.
		return false
	}
	return actionOrder <= maxOrder
}
