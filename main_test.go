package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bikigrg11/verdict/internal/kube"
)

func TestReportExitsZeroOnHealthyCluster(t *testing.T) {
	snap := &kube.Snapshot{Context: "kind-devlab"}
	var buf bytes.Buffer

	code, err := report(context.Background(), snap, nil, &buf, false)
	if err != nil {
		t.Fatalf("report returned error: %v", err)
	}
	if code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(buf.String(), "No problems found") {
		t.Errorf("expected all-clear message, got:\n%s", buf.String())
	}
}

func TestReportExitsOneWhenProblemsExist(t *testing.T) {
	snap := &kube.Snapshot{
		Context: "kind-devlab",
		Services: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "demo"},
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "api"}},
		}},
	}
	var buf bytes.Buffer

	code, err := report(context.Background(), snap, nil, &buf, false)
	if err != nil {
		t.Fatalf("report returned error: %v", err)
	}
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
}

func TestReportEmitsValidJSON(t *testing.T) {
	snap := &kube.Snapshot{Context: "kind-devlab"}
	var buf bytes.Buffer

	if _, err := report(context.Background(), snap, nil, &buf, true); err != nil {
		t.Fatalf("report returned error: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "[") {
		t.Errorf("json mode must emit an array, got:\n%s", buf.String())
	}
}
