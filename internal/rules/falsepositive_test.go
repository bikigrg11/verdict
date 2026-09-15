package rules

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

// A healthy cluster, plus every edge case that naively looks broken, must
// produce exactly zero findings. Spec §9.2.
func TestNoFalsePositivesOnHealthyCluster(t *testing.T) {
	cases := []struct {
		name string
		snap *kube.Snapshot
	}{
		{
			name: "completed job pod",
			snap: &kube.Snapshot{Pods: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "demo"},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "task",
						State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"}},
					}},
				},
			}}},
		},
		{
			name: "deployment scaled to zero leaves no pods and no service",
			snap: &kube.Snapshot{},
		},
		{
			name: "headless service with no selector",
			snap: &kube.Snapshot{Services: []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "demo"},
				Spec:       corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone},
			}}},
		},
		{
			name: "externalname service",
			snap: &kube.Snapshot{Services: []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "ext", Namespace: "demo"},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "example.com"},
			}}},
		},
		{
			name: "pod with historical restarts that is now running fine",
			snap: &kube.Snapshot{Pods: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "demo"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{
						Name: "api", Ready: true, RestartCount: 42,
						State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					}},
				},
			}}},
		},
		{
			name: "pod mid-scheduling with PodScheduled true",
			snap: &kube.Snapshot{
				Nodes: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "n1"}}},
				Pods: []corev1.Pod{{
					ObjectMeta: metav1.ObjectMeta{Name: "new-1", Namespace: "demo"},
					Status: corev1.PodStatus{
						Phase:      corev1.PodPending,
						Conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue}},
					},
				}},
			},
		},
		{
			name: "service whose selector matches a running pod",
			snap: &kube.Snapshot{
				Services: []corev1.Service{{
					ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "demo"},
					Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "api"}},
				}},
				Pods: []corev1.Pod{{
					ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "demo", Labels: map[string]string{"app": "api", "extra": "ok"}},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := Run(context.Background(), tc.snap, nil, []Rule{
				MissingRef{}, NoEndpoints{}, CrashLoop{}, Unschedulable{},
			})
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if len(findings) != 0 {
				for _, f := range findings {
					t.Errorf("false positive: [%s] %s — %s", f.Severity, f.Title, f.Explanation)
				}
				t.Fatalf("got %d findings, want 0", len(findings))
			}
		})
	}
}
