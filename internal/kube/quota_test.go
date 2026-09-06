package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetResourceUsage(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "tostada"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "tostada"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "worker",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: "test-quota", Namespace: "tostada"},
			Status: corev1.ResourceQuotaStatus{
				Hard: corev1.ResourceList{
					corev1.ResourceName("requests.cpu"):    resource.MustParse("4"),
					corev1.ResourceName("limits.cpu"):      resource.MustParse("8"),
					corev1.ResourceName("requests.memory"): resource.MustParse("8Gi"),
					corev1.ResourcePods:                    resource.MustParse("10"),
				},
			},
		},
	)

	qc := NewQuotaClientFromClientset(clientset, "tostada")
	usage, err := qc.GetResourceUsage(context.Background())
	if err != nil {
		t.Fatalf("GetResourceUsage() error: %v", err)
	}
	if len(usage) != 5 {
		t.Fatalf("len(usage) = %d, want 5", len(usage))
	}

	m := map[string]ResourceUsage{}
	for _, u := range usage {
		m[u.Resource] = u
	}

	if m["cpu requests"].Hard != "4" {
		t.Errorf("cpu requests hard = %q, want %q", m["cpu requests"].Hard, "4")
	}
	if m["cpu limits"].Hard != "8" {
		t.Errorf("cpu limits hard = %q, want %q", m["cpu limits"].Hard, "8")
	}
	if m["memory requests"].Hard != "8Gi" {
		t.Errorf("memory requests hard = %q, want %q", m["memory requests"].Hard, "8Gi")
	}
	if m["memory limits"].Hard != "∞" {
		t.Errorf("memory limits hard = %q, want %q", m["memory limits"].Hard, "∞")
	}
	if m["pods"].Hard != "10" {
		t.Errorf("pods hard = %q, want %q", m["pods"].Hard, "10")
	}
	if m["pods"].Used != "2" {
		t.Errorf("pods used = %q, want %q", m["pods"].Used, "2")
	}
}

func TestGetResourceUsage_NoQuota(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "tostada"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	qc := NewQuotaClientFromClientset(clientset, "tostada")
	usage, err := qc.GetResourceUsage(context.Background())
	if err != nil {
		t.Fatalf("GetResourceUsage() error: %v", err)
	}

	m := map[string]ResourceUsage{}
	for _, u := range usage {
		m[u.Resource] = u
	}

	for _, res := range []string{"cpu requests", "cpu limits", "memory requests", "memory limits", "pods"} {
		if m[res].Hard != "∞" {
			t.Errorf("%s hard = %q, want %q", res, m[res].Hard, "∞")
		}
	}
	if m["pods"].Used != "1" {
		t.Errorf("pods used = %q, want %q", m["pods"].Used, "1")
	}
}

func TestGetResourceUsage_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	qc := NewQuotaClientFromClientset(clientset, "tostada")
	usage, err := qc.GetResourceUsage(context.Background())
	if err != nil {
		t.Fatalf("GetResourceUsage() error: %v", err)
	}
	if len(usage) != 5 {
		t.Fatalf("len(usage) = %d, want 5", len(usage))
	}
	if usage[4].Resource != "pods" {
		t.Errorf("last resource = %q, want pods", usage[4].Resource)
	}
	if usage[4].Used != "0" {
		t.Errorf("pods used = %q, want %q", usage[4].Used, "0")
	}
}
