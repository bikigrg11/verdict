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

// A reference marked optional:true is absent on purpose. Kubernetes itself ships
// these — k3s CoreDNS mounts an optional coredns-custom ConfigMap — so flagging
// them is a false positive, caught against a real cluster on the first run.
func TestMissingRefIgnoresOptionalReferences(t *testing.T) {
	yes := true

	optionalVolume := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "coredns-1", Namespace: "kube-system"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "coredns"}},
			Volumes: []corev1.Volume{{
				Name: "custom-config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "coredns-custom"},
						Optional:             &yes,
					},
				},
			}},
		},
	}

	optionalEnvFrom := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "demo"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "api",
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "optional-config"},
						Optional:             &yes,
					},
				}},
				Env: []corev1.EnvVar{{
					Name: "TOKEN",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "optional-secret"},
							Key:                  "token",
							Optional:             &yes,
						},
					},
				}},
			}},
		},
	}

	snap := &kube.Snapshot{Pods: []corev1.Pod{optionalVolume, optionalEnvFrom}}

	findings, err := Run(context.Background(), snap, nil, []Rule{MissingRef{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("false positive: %s — %s", f.Title, f.Explanation)
		}
		t.Fatalf("got %d findings, want 0 — every reference is optional", len(findings))
	}
}
