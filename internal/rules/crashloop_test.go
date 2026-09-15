package rules

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

type fakeLogs struct {
	body string
	err  error
}

func (f fakeLogs) PreviousLogs(_ context.Context, _, _, _ string, _ int) (string, error) {
	return f.body, f.err
}

func crashingPod(ns, name, container string, exitCode int32) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: container}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         container,
				RestartCount: 7,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: exitCode},
				},
			}},
		},
	}
}

func TestCrashLoopReportsTheActualErrorFromLogs(t *testing.T) {
	logs := strings.Join([]string{
		"2026-09-15T10:00:00Z starting api server",
		"2026-09-15T10:00:01Z connecting to database",
		"panic: dial tcp 10.0.0.5:5432: connection refused",
		"goroutine 1 [running]:",
	}, "\n")

	snap := &kube.Snapshot{Pods: []corev1.Pod{crashingPod("demo", "api-1", "api", 2)}}

	findings, err := Run(context.Background(), snap, fakeLogs{body: logs}, []Rule{CrashLoop{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	f := findings[0]
	if !strings.Contains(f.Explanation, "connection refused") {
		t.Errorf("explanation must contain the real error, got %q", f.Explanation)
	}
	if !strings.Contains(f.Explanation, "exit code 2") {
		t.Errorf("explanation must contain the exit code, got %q", f.Explanation)
	}
}

func TestCrashLoopStillReportsWhenLogsAreUnavailable(t *testing.T) {
	snap := &kube.Snapshot{Pods: []corev1.Pod{crashingPod("demo", "api-1", "api", 137)}}

	findings, err := Run(context.Background(), snap, fakeLogs{err: context.DeadlineExceeded}, []Rule{CrashLoop{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 — the crashloop is still a fact", len(findings))
	}
	if !strings.Contains(findings[0].Explanation, "SIGKILL") {
		t.Errorf("exit code 137 should be explained, got %q", findings[0].Explanation)
	}
}

func TestCrashLoopSilentForHealthyPod(t *testing.T) {
	healthy := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ok-1", Namespace: "demo"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "api", Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	snap := &kube.Snapshot{Pods: []corev1.Pod{healthy}}

	findings, err := Run(context.Background(), snap, fakeLogs{}, []Rule{CrashLoop{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0", len(findings))
	}
}

func TestCrashLoopIgnoresSucceededPod(t *testing.T) {
	done := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "demo"},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "task",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
				},
			}},
		},
	}
	snap := &kube.Snapshot{Pods: []corev1.Pod{done}}

	findings, err := Run(context.Background(), snap, fakeLogs{}, []Rule{CrashLoop{}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 — the pod completed successfully", len(findings))
	}
}

func TestExtractErrorRecognisesCommonRuntimes(t *testing.T) {
	cases := []struct{ name, logs, want string }{
		{"go panic", "ok\npanic: runtime error: index out of range\nstack", "panic: runtime error: index out of range"},
		{"python traceback", "Traceback (most recent call last):\n  File x\nValueError: bad config", "ValueError: bad config"},
		{"java exception", "log\nException in thread \"main\" java.lang.NullPointerException", "Exception in thread \"main\" java.lang.NullPointerException"},
		{"generic error", "starting\nERROR: cannot bind port 8080", "ERROR: cannot bind port 8080"},
		{"nothing useful", "starting\nlistening on 8080", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractError(tc.logs); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
