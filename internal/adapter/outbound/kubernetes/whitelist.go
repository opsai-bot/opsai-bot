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

// dangerousPatterns contains shell metacharacters and injection patterns.
var dangerousPatterns = []string{"$(", "`", "|", ">>", "<<", ";", "&&", "||"}

// sensitivePathFragments contains path fragments that should never appear in exec arguments.
var sensitivePathFragments = []string{
	"/etc/shadow", "/etc/passwd", "/etc/master.passwd",
	"/proc/self/environ", "/proc/self/cmdline",
	"/.ssh/", "/.kube/config", "/.env",
	"/var/run/secrets",
}

// IsExecAllowed reports whether the command is safe to execute.
// It validates the binary against the exec whitelist AND checks arguments for
// dangerous patterns (command substitution, sensitive file access, external URLs).
func (w *Whitelist) IsExecAllowed(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	binary := strings.ToLower(parts[0])
	if !w.exec[binary] {
		return false
	}

	fullCmd := strings.ToLower(command)

	// Block dangerous shell metacharacters and command substitution.
	for _, pattern := range dangerousPatterns {
		if strings.Contains(fullCmd, pattern) {
			return false
		}
	}

	// Block access to sensitive file paths.
	for _, fragment := range sensitivePathFragments {
		if strings.Contains(fullCmd, fragment) {
			return false
		}
	}

	// For curl: only allow internal cluster URLs.
	if binary == "curl" {
		for _, arg := range parts[1:] {
			argLower := strings.ToLower(arg)
			if strings.HasPrefix(argLower, "http://") || strings.HasPrefix(argLower, "https://") {
				if !isInternalClusterURL(argLower) {
					return false
				}
			}
		}
	}

	return true
}

// isInternalClusterURL checks if a URL points to cluster-internal services.
func isInternalClusterURL(u string) bool {
	internalPatterns := []string{
		"localhost", "127.0.0.1", "::1",
		".svc.cluster.local", ".svc.cluster.",
		".local:", ".local/",
	}
	for _, pattern := range internalPatterns {
		if strings.Contains(u, pattern) {
			return true
		}
	}
	return false
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

// ValidateExecCommand checks whether command is exec-allowed.
// It joins all tokens into a single string so that IsExecAllowed can inspect
// every argument, not just the binary name.
func (w *Whitelist) ValidateExecCommand(command []string) (allowed bool, reason string) {
	if len(command) == 0 {
		return false, "empty command"
	}
	fullCmd := strings.Join(command, " ")
	if !w.IsExecAllowed(fullCmd) {
		return false, "command not allowed: " + command[0]
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
