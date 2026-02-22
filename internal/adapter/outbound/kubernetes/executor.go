package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// Executor implements outbound.K8sExecutor using a real Kubernetes clientset.
type Executor struct {
	clientset   kubernetes.Interface
	whitelist   *Whitelist
	execTimeout time.Duration
	reader      *Reader
}

// NewExecutor creates an Executor.
func NewExecutor(clientset kubernetes.Interface, whitelist *Whitelist, execTimeout time.Duration) *Executor {
	return &Executor{
		clientset:   clientset,
		whitelist:   whitelist,
		execTimeout: execTimeout,
		reader:      NewReader(clientset),
	}
}

// GetResource delegates to Reader.
func (e *Executor) GetResource(ctx context.Context, query outbound.ResourceQuery) (outbound.ResourceResult, error) {
	raw, err := e.reader.GetResource(ctx,
		query.Namespace, query.ResourceType, query.Name,
		query.LabelSelector, query.FieldSelector, query.Limit)
	if err != nil {
		return outbound.ResourceResult{}, err
	}
	return outbound.ResourceResult{
		Raw:      raw,
		Metadata: map[string]string{"namespace": query.Namespace, "resourceType": query.ResourceType},
	}, nil
}

// GetPodLogs delegates to Reader.
func (e *Executor) GetPodLogs(ctx context.Context, namespace, pod, container string, tailLines int64) (string, error) {
	return e.reader.GetPodLogs(ctx, namespace, pod, container, tailLines)
}

// GetEvents delegates to Reader.
func (e *Executor) GetEvents(ctx context.Context, namespace, involvedObject string) (string, error) {
	return e.reader.GetEvents(ctx, namespace, involvedObject)
}

// DescribeResource delegates to Reader.
func (e *Executor) DescribeResource(ctx context.Context, namespace, resourceType, name string) (string, error) {
	return e.reader.DescribeResource(ctx, namespace, resourceType, name)
}

// GetClusterContext delegates to Reader.
func (e *Executor) GetClusterContext(ctx context.Context) (string, error) {
	return e.reader.GetClusterContext(ctx)
}

// ValidateCommand checks whitelist and blocked-namespace constraints.
func (e *Executor) ValidateCommand(command []string) outbound.CommandValidation {
	if len(command) == 0 {
		return outbound.CommandValidation{Allowed: false, Reason: "empty command", Risk: "none"}
	}

	allowed, reason := e.whitelist.ValidateExecCommand(command)
	if !allowed {
		return outbound.CommandValidation{Allowed: false, Reason: reason, Risk: "blocked"}
	}

	// Scan for a namespace flag and check if it is blocked.
	ns := extractNamespaceFlag(command)
	if ns != "" && e.whitelist.IsNamespaceBlocked(ns) {
		return outbound.CommandValidation{
			Allowed: false,
			Reason:  "namespace is blocked: " + ns,
			Risk:    "blocked",
		}
	}

	return outbound.CommandValidation{Allowed: true, Reason: reason, Risk: "low"}
}

// Exec runs the command inside the specified pod/container by shelling out to kubectl.
// This approach avoids SPDY/streaming complexity and is straightforward to test/mock.
func (e *Executor) Exec(ctx context.Context, req outbound.ExecRequest) (outbound.ExecResult, error) {
	allowed, reason := e.whitelist.ValidateExecCommand(req.Command)
	if !allowed {
		return outbound.ExecResult{ExitCode: 1}, fmt.Errorf("exec denied: %s", reason)
	}

	if e.whitelist.IsNamespaceBlocked(req.Namespace) {
		return outbound.ExecResult{ExitCode: 1}, fmt.Errorf("exec denied: namespace %s is blocked", req.Namespace)
	}

	timeout := e.execTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build: kubectl exec -n <ns> <pod> [-c <container>] -- <cmd...>
	kubectlArgs := []string{"exec", "-n", req.Namespace, req.Pod}
	if req.Container != "" {
		kubectlArgs = append(kubectlArgs, "-c", req.Container)
	}
	kubectlArgs = append(kubectlArgs, "--")
	kubectlArgs = append(kubectlArgs, req.Command...)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(execCtx, "kubectl", kubectlArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return outbound.ExecResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: 1,
			}, fmt.Errorf("running kubectl exec: %w", err)
		}
	}

	return outbound.ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// RestartDeployment triggers a rollout restart by patching the pod template annotation.
func (e *Executor) RestartDeployment(ctx context.Context, namespace, name string) error {
	if e.whitelist.IsNamespaceBlocked(namespace) {
		return fmt.Errorf("restart denied: namespace %s is blocked", namespace)
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						"kubectl.kubernetes.io/restartedAt": time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling restart patch: %w", err)
	}

	_, err = e.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patching deployment %s/%s for restart: %w", namespace, name, err)
	}
	return nil
}

// ScaleDeployment sets the replica count on a deployment via a strategic-merge patch.
func (e *Executor) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	if e.whitelist.IsNamespaceBlocked(namespace) {
		return fmt.Errorf("scale denied: namespace %s is blocked", namespace)
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling scale patch: %w", err)
	}

	_, err = e.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patching deployment %s/%s for scale: %w", namespace, name, err)
	}
	return nil
}

// DeletePod deletes a pod with a zero grace period.
func (e *Executor) DeletePod(ctx context.Context, namespace, name string) error {
	if e.whitelist.IsNamespaceBlocked(namespace) {
		return fmt.Errorf("delete denied: namespace %s is blocked", namespace)
	}

	gracePeriod := int64(0)
	err := e.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if err != nil {
		return fmt.Errorf("deleting pod %s/%s: %w", namespace, name, err)
	}
	return nil
}

// HealthCheck verifies connectivity to the API server via ServerVersion.
func (e *Executor) HealthCheck(ctx context.Context) error {
	_, err := e.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("k8s health check failed: %w", err)
	}
	return nil
}

// --- helpers ---

// extractNamespaceFlag scans command tokens for -n or --namespace and returns the value.
func extractNamespaceFlag(command []string) string {
	for i, token := range command {
		switch {
		case token == "-n" || token == "--namespace":
			if i+1 < len(command) {
				return command[i+1]
			}
		case strings.HasPrefix(token, "--namespace="):
			return strings.TrimPrefix(token, "--namespace=")
		}
	}
	return ""
}

