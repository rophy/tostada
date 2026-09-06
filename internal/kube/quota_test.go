package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListQuotas(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-quota",
			Namespace: "tostada",
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
				corev1.ResourcePods:   resource.MustParse("10"),
			},
			Used: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1500m"),
				corev1.ResourceMemory: resource.MustParse("3Gi"),
				corev1.ResourcePods:   resource.MustParse("4"),
			},
		},
	})

	qc := NewQuotaClientFromClientset(clientset, "tostada")
	quotas, err := qc.ListQuotas(context.Background())
	if err != nil {
		t.Fatalf("ListQuotas() error: %v", err)
	}
	if len(quotas) != 1 {
		t.Fatalf("len(quotas) = %d, want 1", len(quotas))
	}
	if quotas[0].Name != "test-quota" {
		t.Errorf("Name = %q, want %q", quotas[0].Name, "test-quota")
	}
	if len(quotas[0].Resources) != 3 {
		t.Errorf("len(Resources) = %d, want 3", len(quotas[0].Resources))
	}

	resourceMap := map[string]ResourceUsage{}
	for _, r := range quotas[0].Resources {
		resourceMap[r.Resource] = r
	}

	if cpu, ok := resourceMap["cpu"]; !ok {
		t.Error("missing cpu resource")
	} else {
		if cpu.Hard != "4" {
			t.Errorf("cpu.Hard = %q, want %q", cpu.Hard, "4")
		}
		if cpu.Used != "1500m" {
			t.Errorf("cpu.Used = %q, want %q", cpu.Used, "1500m")
		}
	}

	if pods, ok := resourceMap["pods"]; !ok {
		t.Error("missing pods resource")
	} else {
		if pods.Hard != "10" {
			t.Errorf("pods.Hard = %q, want %q", pods.Hard, "10")
		}
		if pods.Used != "4" {
			t.Errorf("pods.Used = %q, want %q", pods.Used, "4")
		}
	}
}

func TestListQuotas_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	qc := NewQuotaClientFromClientset(clientset, "tostada")
	quotas, err := qc.ListQuotas(context.Background())
	if err != nil {
		t.Fatalf("ListQuotas() error: %v", err)
	}
	if len(quotas) != 0 {
		t.Errorf("len(quotas) = %d, want 0", len(quotas))
	}
}
