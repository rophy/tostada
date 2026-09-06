package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ResourceUsage struct {
	Resource string `json:"resource"`
	Used     string `json:"used"`
	Hard     string `json:"hard"`
}

type QuotaClient struct {
	clientset kubernetes.Interface
	namespace string
}

func NewQuotaClient(namespace string) (*QuotaClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &QuotaClient{clientset: clientset, namespace: namespace}, nil
}

func NewQuotaClientFromClientset(clientset kubernetes.Interface, namespace string) *QuotaClient {
	return &QuotaClient{clientset: clientset, namespace: namespace}
}

func (c *QuotaClient) GetResourceUsage(ctx context.Context) ([]ResourceUsage, error) {
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var cpuReq, cpuLim, memReq, memLim resource.Quantity
	podCount := 0
	for _, pod := range pods.Items {
		podCount++
		for _, c := range pod.Spec.Containers {
			if r, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
				cpuReq.Add(r)
			}
			if l, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
				cpuLim.Add(l)
			}
			if r, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
				memReq.Add(r)
			}
			if l, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
				memLim.Add(l)
			}
		}
	}

	hardLimits := map[string]string{}
	quotas, err := c.clientset.CoreV1().ResourceQuotas(c.namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, q := range quotas.Items {
			for res, qty := range q.Status.Hard {
				hardLimits[string(res)] = qty.String()
			}
		}
	}

	hard := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := hardLimits[k]; ok {
				return v
			}
		}
		return "∞"
	}

	return []ResourceUsage{
		{Resource: "cpu requests", Used: cpuReq.String(), Hard: hard("requests.cpu", "cpu")},
		{Resource: "cpu limits", Used: cpuLim.String(), Hard: hard("limits.cpu")},
		{Resource: "memory requests", Used: memReq.String(), Hard: hard("requests.memory", "memory")},
		{Resource: "memory limits", Used: memLim.String(), Hard: hard("limits.memory")},
		{Resource: "pods", Used: fmt.Sprintf("%d", podCount), Hard: hard("pods")},
	}, nil
}
