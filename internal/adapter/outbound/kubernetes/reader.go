package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Reader provides read-only access to Kubernetes resources.
type Reader struct {
	clientset kubernetes.Interface
}

// NewReader creates a Reader backed by the given clientset.
func NewReader(clientset kubernetes.Interface) *Reader {
	return &Reader{clientset: clientset}
}

// GetResource retrieves a resource description by namespace/type/name or lists resources
// matching the given label/field selectors. Returns raw YAML-like text.
func (r *Reader) GetResource(ctx context.Context, namespace, resourceType, name, labelSelector, fieldSelector string, limit int64) (string, error) {
	switch strings.ToLower(resourceType) {
	case "pod", "pods":
		return r.getPodResource(ctx, namespace, name, labelSelector, fieldSelector, limit)
	case "deployment", "deployments":
		return r.getDeploymentResource(ctx, namespace, name, labelSelector, fieldSelector, limit)
	case "service", "services", "svc":
		return r.getServiceResource(ctx, namespace, name, labelSelector, fieldSelector, limit)
	case "node", "nodes":
		return r.getNodeResource(ctx, namespace, name, labelSelector, fieldSelector, limit)
	case "namespace", "namespaces", "ns":
		return r.getNamespaceResource(ctx, name)
	default:
		return "", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (r *Reader) getPodResource(ctx context.Context, namespace, name, labelSelector, fieldSelector string, limit int64) (string, error) {
	if name != "" {
		pod, err := r.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting pod %s/%s: %w", namespace, name, err)
		}
		return formatPod(pod), nil
	}

	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	if limit > 0 {
		opts.Limit = limit
	}
	list, err := r.clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("listing pods in %s: %w", namespace, err)
	}
	return formatPodList(list), nil
}

func (r *Reader) getDeploymentResource(ctx context.Context, namespace, name, labelSelector, fieldSelector string, limit int64) (string, error) {
	if name != "" {
		d, err := r.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
		}
		return fmt.Sprintf("deployment/%s namespace=%s replicas=%d/%d",
			d.Name, d.Namespace, d.Status.ReadyReplicas, d.Status.Replicas), nil
	}

	opts := metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector}
	if limit > 0 {
		opts.Limit = limit
	}
	list, err := r.clientset.AppsV1().Deployments(namespace).List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("listing deployments in %s: %w", namespace, err)
	}
	var sb strings.Builder
	for i := range list.Items {
		d := &list.Items[i]
		fmt.Fprintf(&sb, "deployment/%s namespace=%s replicas=%d/%d\n",
			d.Name, d.Namespace, d.Status.ReadyReplicas, d.Status.Replicas)
	}
	return sb.String(), nil
}

func (r *Reader) getServiceResource(ctx context.Context, namespace, name, labelSelector, fieldSelector string, limit int64) (string, error) {
	if name != "" {
		svc, err := r.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting service %s/%s: %w", namespace, name, err)
		}
		return fmt.Sprintf("service/%s namespace=%s type=%s clusterIP=%s",
			svc.Name, svc.Namespace, svc.Spec.Type, svc.Spec.ClusterIP), nil
	}

	opts := metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector}
	if limit > 0 {
		opts.Limit = limit
	}
	list, err := r.clientset.CoreV1().Services(namespace).List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("listing services in %s: %w", namespace, err)
	}
	var sb strings.Builder
	for i := range list.Items {
		s := &list.Items[i]
		fmt.Fprintf(&sb, "service/%s namespace=%s type=%s clusterIP=%s\n",
			s.Name, s.Namespace, s.Spec.Type, s.Spec.ClusterIP)
	}
	return sb.String(), nil
}

func (r *Reader) getNodeResource(ctx context.Context, _, name, labelSelector, fieldSelector string, limit int64) (string, error) {
	if name != "" {
		node, err := r.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting node %s: %w", name, err)
		}
		return fmt.Sprintf("node/%s ready=%v", node.Name, isNodeReady(node)), nil
	}

	opts := metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector}
	if limit > 0 {
		opts.Limit = limit
	}
	list, err := r.clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("listing nodes: %w", err)
	}
	var sb strings.Builder
	for i := range list.Items {
		n := &list.Items[i]
		fmt.Fprintf(&sb, "node/%s ready=%v\n", n.Name, isNodeReady(n))
	}
	return sb.String(), nil
}

func (r *Reader) getNamespaceResource(ctx context.Context, name string) (string, error) {
	if name != "" {
		ns, err := r.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting namespace %s: %w", name, err)
		}
		return fmt.Sprintf("namespace/%s status=%s", ns.Name, ns.Status.Phase), nil
	}

	list, err := r.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing namespaces: %w", err)
	}
	var sb strings.Builder
	for i := range list.Items {
		ns := &list.Items[i]
		fmt.Fprintf(&sb, "namespace/%s status=%s\n", ns.Name, ns.Status.Phase)
	}
	return sb.String(), nil
}

// GetPodLogs returns the last tailLines lines of logs from the specified pod/container.
func (r *Reader) GetPodLogs(ctx context.Context, namespace, pod, container string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{Container: container}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	req := r.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("streaming logs for pod %s/%s: %w", namespace, pod, err)
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(stream); err != nil {
		return "", fmt.Errorf("reading log stream: %w", err)
	}
	return buf.String(), nil
}

// GetEvents returns events for namespace optionally filtered to the named involvedObject.
func (r *Reader) GetEvents(ctx context.Context, namespace, involvedObject string) (string, error) {
	opts := metav1.ListOptions{}
	if involvedObject != "" {
		opts.FieldSelector = "involvedObject.name=" + involvedObject
	}

	list, err := r.clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("listing events in %s: %w", namespace, err)
	}

	var sb strings.Builder
	for i := range list.Items {
		e := &list.Items[i]
		fmt.Fprintf(&sb, "[%s] %s/%s: %s\n",
			e.Type, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Message)
	}
	return sb.String(), nil
}

// DescribeResource combines a Get with related Events for the named resource.
func (r *Reader) DescribeResource(ctx context.Context, namespace, resourceType, name string) (string, error) {
	resource, err := r.GetResource(ctx, namespace, resourceType, name, "", "", 0)
	if err != nil {
		return "", err
	}

	events, err := r.GetEvents(ctx, namespace, name)
	if err != nil {
		// Events are best-effort; don't fail the entire describe.
		events = fmt.Sprintf("(could not retrieve events: %v)", err)
	}

	return fmt.Sprintf("=== Resource ===\n%s\n=== Events ===\n%s", resource, events), nil
}

// GetClusterContext returns a summary of nodes, pods, and namespaces.
func (r *Reader) GetClusterContext(ctx context.Context) (string, error) {
	nodes, err := r.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing nodes for cluster context: %w", err)
	}

	pods, err := r.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing pods for cluster context: %w", err)
	}

	namespaces, err := r.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing namespaces for cluster context: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Nodes: %d\n", len(nodes.Items))
	readyNodes := 0
	for i := range nodes.Items {
		if isNodeReady(&nodes.Items[i]) {
			readyNodes++
		}
	}
	fmt.Fprintf(&sb, "Ready nodes: %d\n", readyNodes)
	fmt.Fprintf(&sb, "Total pods: %d\n", len(pods.Items))
	fmt.Fprintf(&sb, "Namespaces (%d):", len(namespaces.Items))
	for i := range namespaces.Items {
		fmt.Fprintf(&sb, " %s", namespaces.Items[i].Name)
	}
	fmt.Fprintln(&sb)
	return sb.String(), nil
}

// --- formatting helpers ---

func formatPod(pod *corev1.Pod) string {
	return fmt.Sprintf("pod/%s namespace=%s phase=%s node=%s",
		pod.Name, pod.Namespace, pod.Status.Phase, pod.Spec.NodeName)
}

func formatPodList(list *corev1.PodList) string {
	var sb strings.Builder
	for i := range list.Items {
		fmt.Fprintln(&sb, formatPod(&list.Items[i]))
	}
	return sb.String()
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
