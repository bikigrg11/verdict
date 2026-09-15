package rules

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

func svc(ns, name string, selector map[string]string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.ServiceSpec{Selector: selector, Type: corev1.ServiceTypeClusterIP},
	}
}

func labelledPod(ns, name string, labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestNoEndpointsNamesTheMismatchedLabel(t *testing.T) {
	snap := &kube.Snapshot{
		Services: []corev1.Service{svc("demo", "api", map[string]string{"app": "api", "tier": "web"})},
		Pods: []corev1.Pod{
			labelledPod("demo", "api-1", map[string]string{"app": "api", "tier": "backend"}),
			labelledPod("demo", "api-2", map[string]string{"app": "api", "tier": "backend"}),
		},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{NoEndpoints{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	f := findings[0]
	if f.Severity != "critical" {
		t.Errorf("got severity %q, want critical", f.Severity)
	}
	if !strings.Contains(f.Explanation, "tier") {
		t.Errorf("explanation must name the mismatched label key, got %q", f.Explanation)
	}
}

func TestNoEndpointsSilentWhenPodsMatch(t *testing.T) {
	snap := &kube.Snapshot{
		Services: []corev1.Service{svc("demo", "api", map[string]string{"app": "api"})},
		Pods:     []corev1.Pod{labelledPod("demo", "api-1", map[string]string{"app": "api"})},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{NoEndpoints{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 — the selector matches", len(findings))
	}
}

func TestNoEndpointsIgnoresSelectorlessServices(t *testing.T) {
	snap := &kube.Snapshot{
		Services: []corev1.Service{svc("demo", "external", nil)},
		Pods:     nil,
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{NoEndpoints{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 — selectorless Services are intentional", len(findings))
	}
}

func TestNoEndpointsIgnoresKubernetesAPIService(t *testing.T) {
	snap := &kube.Snapshot{
		Services: []corev1.Service{svc("default", "kubernetes", nil)},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{NoEndpoints{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0", len(findings))
	}
}
