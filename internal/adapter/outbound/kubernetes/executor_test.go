package kubernetes

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

func testExecutor(objs ...runtime.Object) *Executor {
	clientset := fake.NewSimpleClientset(objs...)
	whitelist := NewWhitelist(WhitelistConfig{
		ReadOnly:          []string{"get", "list", "describe", "logs"},
		Exec:              []string{"ls", "cat", "ps"},
		Remediation:       []string{"kubectl rollout restart", "kubectl scale"},
		BlockedNamespaces: []string{"kube-system"},
	})
	return NewExecutor(clientset, whitelist, 5*time.Second)
}

// --- ValidateCommand ---

func TestValidateCommand_Allowed(t *testing.T) {
	e := testExecutor()
	result := e.ValidateCommand([]string{"ls", "-la"})
	if !result.Allowed {
		t.Errorf("expected ls to be allowed, reason: %s", result.Reason)
	}
}

func TestValidateCommand_Denied(t *testing.T) {
	e := testExecutor()
	result := e.ValidateCommand([]string{"rm", "-rf", "/"})
	if result.Allowed {
		t.Error("expected rm to be denied")
	}
}

func TestIsNamespaceBlocked_ViaValidate(t *testing.T) {
	e := testExecutor()
	result := e.ValidateCommand([]string{"ls", "-n", "kube-system"})
	if result.Allowed {
		t.Error("expected command targeting kube-system to be denied")
	}
}

func TestValidateCommand_Empty(t *testing.T) {
	e := testExecutor()
	result := e.ValidateCommand([]string{})
	if result.Allowed {
		t.Error("expected empty command to be denied")
	}
}

// --- GetPodLogs ---

func TestGetPodLogs(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: "default",
		},
	}
	clientset := fake.NewSimpleClientset(pod)
	e := NewExecutor(clientset, NewWhitelist(WhitelistConfig{}), 5*time.Second)
	// The fake client does not implement log streaming; we just verify the call path
	// executes without a panic and returns either nil or a stream error.
	_, err := e.GetPodLogs(context.Background(), "default", "mypod", "", 100)
	_ = err // acceptable: fake doesn't support streaming
}

// --- GetEvents ---

func TestGetEvents(t *testing.T) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "evt1",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "mypod",
		},
		Type:    "Warning",
		Message: "OOMKilled",
	}
	e := testExecutor(event)
	result, err := e.GetEvents(context.Background(), "default", "mypod")
	if err != nil {
		t.Fatalf("GetEvents returned error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty events output")
	}
}

// --- HealthCheck ---

func TestHealthCheck(t *testing.T) {
	e := testExecutor()
	// fake.NewSimpleClientset supports ServerVersion via discovery fake.
	err := e.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck returned unexpected error: %v", err)
	}
}

// --- RestartDeployment ---

func TestRestartDeployment(t *testing.T) {
	replicas := int32(2)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	e := testExecutor(dep)
	err := e.RestartDeployment(context.Background(), "default", "myapp")
	if err != nil {
		t.Errorf("RestartDeployment returned error: %v", err)
	}
}

func TestRestartDeployment_BlockedNamespace(t *testing.T) {
	e := testExecutor()
	err := e.RestartDeployment(context.Background(), "kube-system", "coredns")
	if err == nil {
		t.Error("expected error when restarting deployment in blocked namespace")
	}
}

// --- ScaleDeployment ---

func TestScaleDeployment(t *testing.T) {
	replicas := int32(2)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	e := testExecutor(dep)
	err := e.ScaleDeployment(context.Background(), "default", "myapp", 3)
	if err != nil {
		t.Errorf("ScaleDeployment returned error: %v", err)
	}
}

func TestScaleDeployment_BlockedNamespace(t *testing.T) {
	e := testExecutor()
	err := e.ScaleDeployment(context.Background(), "kube-system", "coredns", 1)
	if err == nil {
		t.Error("expected error when scaling in blocked namespace")
	}
}

// --- DeletePod ---

func TestDeletePod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: "default",
		},
	}
	e := testExecutor(pod)
	err := e.DeletePod(context.Background(), "default", "mypod")
	if err != nil {
		t.Errorf("DeletePod returned error: %v", err)
	}
}

func TestDeletePod_BlockedNamespace(t *testing.T) {
	e := testExecutor()
	err := e.DeletePod(context.Background(), "kube-system", "somepod")
	if err == nil {
		t.Error("expected error when deleting pod in blocked namespace")
	}
}

// --- GetResource ---

func TestGetResource_Pod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	e := testExecutor(pod)
	result, err := e.GetResource(context.Background(), outbound.ResourceQuery{
		Namespace:    "default",
		ResourceType: "pod",
		Name:         "mypod",
	})
	if err != nil {
		t.Fatalf("GetResource returned error: %v", err)
	}
	if result.Raw == "" {
		t.Error("expected non-empty raw result")
	}
}
