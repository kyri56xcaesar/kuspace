// Package kubernetes defines kubernetes client logic
package kubernetes

import (
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// GetKubeClient initializes and returns a Kubernetes clientset.
// It first attempts to create an in-cluster configuration. If that fails (e.g., when running outside a cluster),
// it falls back to using the local kubeconfig file located at $HOME/.kube/config.
// Returns a Kubernetes clientset or an error if configuration or client creation fails.
func GetKubeClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to local config
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
