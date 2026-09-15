package rules

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

func podWithEnvFromConfigMap(ns, name, container, cmName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: container,
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					},
				}},
			}},
		},
	}
}

func TestMissingRefFlagsAbsentConfigMap(t *testing.T) {
	snap := &kube.Snapshot{
		Pods:       []corev1.Pod{podWithEnvFromConfigMap("demo", "api-1", "api", "app-config")},
		ConfigMaps: []kube.ObjectRef{},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{MissingRef{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	f := findings[0]
	if !f.Verified {
		t.Error("finding must be marked verified")
	}
	if !strings.Contains(f.Explanation, "app-config") {
		t.Errorf("explanation must name the missing object, got %q", f.Explanation)
	}
	if f.Reproduce == "" {
		t.Error("finding must carry a reproduce command")
	}
}

func TestMissingRefStaysSilentWhenConfigMapExists(t *testing.T) {
	snap := &kube.Snapshot{
		Pods:       []corev1.Pod{podWithEnvFromConfigMap("demo", "api-1", "api", "app-config")},
		ConfigMaps: []kube.ObjectRef{{Namespace: "demo", Name: "app-config"}},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{MissingRef{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 — the ConfigMap exists", len(findings))
	}
}

func TestMissingRefGroupsPodsSharingOneMissingConfigMap(t *testing.T) {
	snap := &kube.Snapshot{
		Pods: []corev1.Pod{
			podWithEnvFromConfigMap("demo", "api-1", "api", "app-config"),
			podWithEnvFromConfigMap("demo", "api-2", "api", "app-config"),
			podWithEnvFromConfigMap("demo", "api-3", "api", "app-config"),
		},
		ConfigMaps: []kube.ObjectRef{},
	}

	findings, err := Run(context.Background(), snap, nil, []Rule{MissingRef{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 grouped finding", len(findings))
	}
	if findings[0].BlastRadius != 3 {
		t.Errorf("got blast radius %d, want 3", findings[0].BlastRadius)
	}
}
