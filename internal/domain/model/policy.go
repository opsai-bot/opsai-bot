package model

type PolicyMode string

const (
	PolicyModeAutoFix          PolicyMode = "auto_fix"
	PolicyModeWarnAuto         PolicyMode = "warn_auto"
	PolicyModeApprovalRequired PolicyMode = "approval_required"
)

type PolicyEffect string

const (
	PolicyEffectAllow           PolicyEffect = "allow"
	PolicyEffectDeny            PolicyEffect = "deny"
	PolicyEffectRequireApproval PolicyEffect = "require_approval"
)

type EnvironmentPolicy struct {
	ID          string       `json:"id" yaml:"id"`
	Environment string       `json:"environment" yaml:"environment"`
	Mode        PolicyMode   `json:"mode" yaml:"mode"`
	MaxAutoRisk string       `json:"max_auto_risk" yaml:"maxAutoRisk"`
	Approvers   []string     `json:"approvers" yaml:"approvers"`
	Namespaces  []string     `json:"namespaces" yaml:"namespaces"`
	CustomRules []PolicyRule `json:"custom_rules" yaml:"customRules"`
	Enabled     bool         `json:"enabled" yaml:"enabled"`
}

type PolicyRule struct {
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description" yaml:"description"`
	Condition   PolicyCondition `json:"condition" yaml:"condition"`
	Effect      PolicyEffect    `json:"effect" yaml:"effect"`
	Priority    int             `json:"priority" yaml:"priority"`
}

type PolicyCondition struct {
	Field    string `json:"field" yaml:"field"`
	Operator string `json:"operator" yaml:"operator"`
	Value    string `json:"value" yaml:"value"`
}

func (p EnvironmentPolicy) IsAutoFix() bool {
	return p.Mode == PolicyModeAutoFix
}

func (p EnvironmentPolicy) RequiresApproval() bool {
	return p.Mode == PolicyModeApprovalRequired
}

func (p EnvironmentPolicy) AppliesToNamespace(ns string) bool {
	if len(p.Namespaces) == 0 {
		return true
	}
	for _, n := range p.Namespaces {
		if n == ns {
			return true
		}
	}
	return false
}
