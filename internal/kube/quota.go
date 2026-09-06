package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ResourceUsage struct {
	Resource string `json:"resource"`
	Hard     string `json:"hard"`
	Used     string `json:"used"`
}

type QuotaInfo struct {
	Name      string          `json:"name"`
	Resources []ResourceUsage `json:"resources"`
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

func (c *QuotaClient) ListQuotas(ctx context.Context) ([]QuotaInfo, error) {
	quotas, err := c.clientset.CoreV1().ResourceQuotas(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resource quotas: %w", err)
	}
	return toQuotaInfos(quotas.Items), nil
}

func toQuotaInfos(quotas []corev1.ResourceQuota) []QuotaInfo {
	var result []QuotaInfo
	for _, q := range quotas {
		info := QuotaInfo{Name: q.Name}
		for resource, hard := range q.Status.Hard {
			used := q.Status.Used[resource]
			info.Resources = append(info.Resources, ResourceUsage{
				Resource: string(resource),
				Hard:     hard.String(),
				Used:     used.String(),
			})
		}
		result = append(result, info)
	}
	return result
}
