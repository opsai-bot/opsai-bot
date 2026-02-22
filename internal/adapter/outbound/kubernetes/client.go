package kubernetes

import (
	"fmt"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClientset creates a Kubernetes clientset from in-cluster config or a kubeconfig file.
func NewClientset(inCluster bool, kubeconfigPath string) (k8s.Interface, error) {
	var config *rest.Config
	var err error

	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	if err != nil {
		return nil, fmt.Errorf("building k8s config: %w", err)
	}

	return k8s.NewForConfig(config)
}
