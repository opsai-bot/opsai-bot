package kubernetes

import "strings"

// WhitelistConfig holds the lists of allowed commands and blocked namespaces.
type WhitelistConfig struct {
	ReadOnly          []string
	Exec              []string
	Remediation       []string
	BlockedNamespaces []string
}

// Whitelist enforces command and namespace access control.
type Whitelist struct {
	readOnly    map[string]bool
	exec        map[string]bool
	remediation map[string]bool
	blockedNS   map[string]bool
}

// NewWhitelist creates a Whitelist from the given configuration.
func NewWhitelist(cfg WhitelistConfig) *Whitelist {
	return &Whitelist{
		readOnly:    toSet(cfg.ReadOnly),
		exec:        toSet(cfg.Exec),
		remediation: toSet(cfg.Remediation),
		blockedNS:   toSet(cfg.BlockedNamespaces),
	}
}

// IsReadOnlyAllowed reports whether verb is in the read-only whitelist.
func (w *Whitelist) IsReadOnlyAllowed(verb string) bool {
	return w.readOnly[strings.ToLower(verb)]
}

// IsExecAllowed reports whether the first token of command is in the exec whitelist.
func (w *Whitelist) IsExecAllowed(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	return w.exec[strings.ToLower(parts[0])]
}

// IsRemediationAllowed reports whether command matches any remediation prefix.
func (w *Whitelist) IsRemediationAllowed(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	for allowed := range w.remediation {
		if strings.HasPrefix(cmd, allowed) {
			return true
		}
	}
	return false
}

// IsNamespaceBlocked reports whether ns is in the blocked-namespace set.
func (w *Whitelist) IsNamespaceBlocked(ns string) bool {
	return w.blockedNS[ns]
}

// ValidateExecCommand checks whether the first element of command is exec-allowed.
func (w *Whitelist) ValidateExecCommand(command []string) (allowed bool, reason string) {
	if len(command) == 0 {
		return false, "empty command"
	}
	if !w.IsExecAllowed(command[0]) {
		return false, "command not in exec whitelist: " + command[0]
	}
	return true, "command allowed"
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[strings.ToLower(item)] = true
	}
	return s
}
