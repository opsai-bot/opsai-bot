package kubernetes

import (
	"context"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

// NoopExecutor is a no-op K8s executor for local development without a cluster.
// All methods return empty results without error, except HealthCheck which returns
// a descriptive message indicating Kubernetes is unavailable.
type NoopExecutor struct{}

// NewNoopExecutor creates a NoopExecutor suitable for local dev mode.
func NewNoopExecutor() *NoopExecutor {
	return &NoopExecutor{}
}

func (n *NoopExecutor) GetResource(_ context.Context, _ outbound.ResourceQuery) (outbound.ResourceResult, error) {
	return outbound.ResourceResult{Raw: "", Metadata: map[string]string{}}, nil
}

func (n *NoopExecutor) GetPodLogs(_ context.Context, _, _, _ string, _ int64) (string, error) {
	return "", nil
}

func (n *NoopExecutor) GetEvents(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (n *NoopExecutor) DescribeResource(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (n *NoopExecutor) GetClusterContext(_ context.Context) (string, error) {
	return "noop (no kubernetes cluster)", nil
}

func (n *NoopExecutor) ValidateCommand(_ []string) outbound.CommandValidation {
	return outbound.CommandValidation{Allowed: false, Reason: "kubernetes unavailable in local dev mode", Risk: "none"}
}

func (n *NoopExecutor) Exec(_ context.Context, _ outbound.ExecRequest) (outbound.ExecResult, error) {
	return outbound.ExecResult{}, nil
}

func (n *NoopExecutor) RestartDeployment(_ context.Context, _, _ string) error {
	return nil
}

func (n *NoopExecutor) ScaleDeployment(_ context.Context, _, _ string, _ int32) error {
	return nil
}

func (n *NoopExecutor) DeletePod(_ context.Context, _, _ string) error {
	return nil
}

func (n *NoopExecutor) HealthCheck(_ context.Context) error {
	return fmt.Errorf("kubernetes unavailable: running in local dev mode (noop executor)")
}
