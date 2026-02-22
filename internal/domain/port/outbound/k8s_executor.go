package outbound

import "context"

type ResourceQuery struct {
	Namespace     string
	ResourceType  string
	Name          string
	LabelSelector string
	FieldSelector string
	Limit         int64
}

type ResourceResult struct {
	Raw      string
	Metadata map[string]string
}

type ExecRequest struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Timeout   int
}

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandValidation struct {
	Allowed bool
	Reason  string
	Risk    string
}

// K8sExecutor abstracts Kubernetes cluster interactions.
type K8sExecutor interface {
	GetResource(ctx context.Context, query ResourceQuery) (ResourceResult, error)
	GetPodLogs(ctx context.Context, namespace, pod, container string, tailLines int64) (string, error)
	GetEvents(ctx context.Context, namespace string, involvedObject string) (string, error)
	DescribeResource(ctx context.Context, namespace, resourceType, name string) (string, error)
	GetClusterContext(ctx context.Context) (string, error)
	ValidateCommand(command []string) CommandValidation
	Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
	RestartDeployment(ctx context.Context, namespace, name string) error
	ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error
	DeletePod(ctx context.Context, namespace, name string) error
	HealthCheck(ctx context.Context) error
}
