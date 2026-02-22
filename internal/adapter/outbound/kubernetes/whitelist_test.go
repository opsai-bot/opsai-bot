package kubernetes

import (
	"testing"
)

func defaultTestWhitelist() *Whitelist {
	return NewWhitelist(WhitelistConfig{
		ReadOnly:          []string{"get", "list", "describe", "logs"},
		Exec:              []string{"ls", "cat", "ps", "df"},
		Remediation:       []string{"kubectl rollout restart", "kubectl scale"},
		BlockedNamespaces: []string{"kube-system", "kube-public"},
	})
}

func TestIsReadOnlyAllowed(t *testing.T) {
	w := defaultTestWhitelist()

	allowed := []string{"get", "list", "describe", "logs", "GET", "LIST"}
	for _, v := range allowed {
		if !w.IsReadOnlyAllowed(v) {
			t.Errorf("expected %q to be read-only allowed", v)
		}
	}

	denied := []string{"delete", "patch", "apply", "exec"}
	for _, v := range denied {
		if w.IsReadOnlyAllowed(v) {
			t.Errorf("expected %q to be read-only denied", v)
		}
	}
}

func TestIsExecAllowed(t *testing.T) {
	w := defaultTestWhitelist()

	if !w.IsExecAllowed("ls -la /tmp") {
		t.Error("expected 'ls -la /tmp' to be exec-allowed")
	}
	if !w.IsExecAllowed("ps aux") {
		t.Error("expected 'ps aux' to be exec-allowed")
	}
	if w.IsExecAllowed("rm -rf /") {
		t.Error("expected 'rm -rf /' to be exec-denied")
	}
	if w.IsExecAllowed("") {
		t.Error("expected empty string to be exec-denied")
	}
}

func TestIsRemediationAllowed(t *testing.T) {
	w := defaultTestWhitelist()

	if !w.IsRemediationAllowed("kubectl rollout restart deployment/myapp") {
		t.Error("expected rollout restart to be remediation-allowed")
	}
	if !w.IsRemediationAllowed("kubectl scale deployment myapp --replicas=3") {
		t.Error("expected scale to be remediation-allowed")
	}
	if w.IsRemediationAllowed("kubectl delete pod myapp-xxx") {
		t.Error("expected kubectl delete to be remediation-denied")
	}
}

func TestIsNamespaceBlocked(t *testing.T) {
	w := defaultTestWhitelist()

	if !w.IsNamespaceBlocked("kube-system") {
		t.Error("expected kube-system to be blocked")
	}
	if !w.IsNamespaceBlocked("kube-public") {
		t.Error("expected kube-public to be blocked")
	}
	if w.IsNamespaceBlocked("default") {
		t.Error("expected default to not be blocked")
	}
	if w.IsNamespaceBlocked("production") {
		t.Error("expected production to not be blocked")
	}
}

func TestValidateExecCommand(t *testing.T) {
	w := defaultTestWhitelist()

	allowed, reason := w.ValidateExecCommand([]string{"ls", "-la"})
	if !allowed {
		t.Errorf("expected ls to be allowed, got reason: %s", reason)
	}

	allowed, reason = w.ValidateExecCommand([]string{"rm", "-rf", "/"})
	if allowed {
		t.Errorf("expected rm to be denied")
	}
	if reason == "" {
		t.Error("expected a deny reason")
	}

	allowed, reason = w.ValidateExecCommand([]string{})
	if allowed {
		t.Error("expected empty command to be denied")
	}
	if reason != "empty command" {
		t.Errorf("expected 'empty command' reason, got: %s", reason)
	}
}
