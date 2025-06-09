package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	version   = "dev"
	startTime = time.Now()
)

// GetSystemMetrics collects and returns various system metrics for a given Kubernetes namespace.
// It gathers information such as pod statuses, total CPU and memory usage, persistent volume claim (PVC) disk usage,
// recent events within the namespace, and build/uptime information. The function attempts to use in-cluster
// configuration, falling back to the local kubeconfig if necessary. Any errors encountered during metric collection
// are aggregated and returned as part of the result map, along with a partial error if applicable.
//
// Parameters:
//
//	ns string - The Kubernetes namespace to collect metrics from.
//
// Returns:
//
//	map[string]any - A map containing collected metrics and information, including pod statuses, resource usage,
//	                 PVC disk info, recent events, build info, and any errors encountered.
//	error          - An error if any part of the metric collection failed, otherwise nil.
func GetSystemMetrics(ns string) (map[string]any, error) {
	var allErrors []string
	result := map[string]any{
		"namespace": ns,
	}

	// Config setup
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("config error: %v", err))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		allErrors = append(allErrors, fmt.Sprintf("clientset error: %v", err))
	}

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		allErrors = append(allErrors, fmt.Sprintf("metrics client error: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ---- POD STATUS ----
	podStatuses := map[string]int{
		"Running":   0,
		"Pending":   0,
		"Succeeded": 0,
		"Failed":    0,
		"Unknown":   0,
	}

	if clientset != nil {
		pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("list pods error: %v", err))
		} else {
			for _, pod := range pods.Items {
				podStatuses[string(pod.Status.Phase)]++
			}
		}
	}
	result["pods"] = podStatuses

	// ---- CPU / MEMORY ----
	totalCPU := int64(0)
	totalMem := int64(0)

	if metricsClient != nil {
		podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("fetch metrics error: %v", err))
		} else {
			for _, podMetrics := range podMetricsList.Items {
				for _, container := range podMetrics.Containers {
					totalCPU += container.Usage.Cpu().MilliValue()
					totalMem += container.Usage.Memory().Value()
				}
			}
		}
	}
	result["total_cpu_milli"] = totalCPU
	result["total_mem_bytes"] = totalMem

	// ---- PVCs (DISK USAGE) ----
	diskInfo := make([]map[string]string, 0)
	if clientset != nil {
		pvcs, err := clientset.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("list pvcs error: %v", err))
		} else {
			for _, pvc := range pvcs.Items {
				capacity := pvc.Status.Capacity[corev1.ResourceStorage]
				diskInfo = append(diskInfo, map[string]string{
					"name":       pvc.Name,
					"volumeName": pvc.Spec.VolumeName,
					"capacity":   capacity.String(),
					"status":     string(pvc.Status.Phase),
				})
			}
		}
	}
	result["disk"] = diskInfo

	// ---- EVENTS ----
	recentEvents := []map[string]interface{}{}
	if clientset != nil {
		events, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("fetch events error: %v", err))
		} else {
			for _, e := range events.Items {
				if time.Since(e.LastTimestamp.Time) < 6*time.Hour {
					recentEvents = append(recentEvents, map[string]interface{}{
						"type":         e.Type,
						"reason":       e.Reason,
						"message":      e.Message,
						"count":        e.Count,
						"firstSeen":    e.FirstTimestamp.Time,
						"lastSeen":     e.LastTimestamp.Time,
						"involved":     e.InvolvedObject.Name,
						"involvedKind": e.InvolvedObject.Kind,
					})
				}
			}
		}
	}
	result["recent_events"] = recentEvents

	// ---- BUILD INFO / UPTIME ----
	result["build_info"] = map[string]string{
		"version": version,
		"uptime":  time.Since(startTime).String(),
	}

	// ---- ERRORS ----
	if len(allErrors) > 0 {
		result["errors"] = allErrors

		return result, fmt.Errorf("partial system metrics with %d errors", len(allErrors))
	}

	return result, nil
}
