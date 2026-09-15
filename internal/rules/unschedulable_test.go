package rules

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

func node(name string, labels map[string]string, cpu, mem string, taints []corev1.Taint) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       corev1.NodeSpec{Taints: taints},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
			},
		},
	}
}

func pendingPod(ns, name string, selector map[string]string, cpu, mem string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			NodeSelector: selector,
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpu),
						corev1.ResourceMemory: resource.MustParse(mem),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable",
			}},
		},
	}
}

func TestUnschedulableExplainsMemoryShortfallPerNode(t *testing.T) {
	snap := &kube.Snapshot{
		Nodes: []corev1.Node{node("n1", nil, "4", "4Gi", nil)},
		Pods:  []corev1.Pod{pendingPod("demo", "big-1", nil, "1", "8Gi")},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{Unschedulable{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if !strings.Contains(findings[0].Explanation, "memory") {
		t.Errorf("explanation must name the insufficient resource, got %q", findings[0].Explanation)
	}
}

func TestUnschedulableExplainsLabelMismatch(t *testing.T) {
	snap := &kube.Snapshot{
		Nodes: []corev1.Node{node("n1", map[string]string{"disk": "hdd"}, "4", "8Gi", nil)},
		Pods:  []corev1.Pod{pendingPod("demo", "gpu-1", map[string]string{"gpu": "true"}, "1", "1Gi")},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{Unschedulable{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if !strings.Contains(findings[0].Explanation, "gpu") {
		t.Errorf("explanation must name the missing label, got %q", findings[0].Explanation)
	}
}

func TestUnschedulableExplainsTaint(t *testing.T) {
	taints := []corev1.Taint{{Key: "dedicated", Value: "ml", Effect: corev1.TaintEffectNoSchedule}}
	snap := &kube.Snapshot{
		Nodes: []corev1.Node{node("n1", nil, "4", "8Gi", taints)},
		Pods:  []corev1.Pod{pendingPod("demo", "app-1", nil, "1", "1Gi")},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{Unschedulable{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if !strings.Contains(findings[0].Explanation, "dedicated") {
		t.Errorf("explanation must name the taint, got %q", findings[0].Explanation)
	}
}

func TestUnschedulableIgnoresPendingWithoutUnschedulableCondition(t *testing.T) {
	p := pendingPod("demo", "starting-1", nil, "1", "1Gi")
	p.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionTrue,
	}}
	snap := &kube.Snapshot{
		Nodes: []corev1.Node{node("n1", nil, "4", "8Gi", nil)},
		Pods:  []corev1.Pod{p},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{Unschedulable{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 — the pod is scheduling normally", len(findings))
	}
}
