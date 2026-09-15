package kube

import (
	"context"
	"os"
	"strings"
	"testing"
)

// fakeRunner returns canned bytes per kubectl resource argument.
type fakeRunner struct {
	responses map[string][]byte
	calls     [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	for key, body := range f.responses {
		for _, a := range args {
			if a == key {
				return body, nil
			}
		}
	}
	return []byte(`{"items":[]}`), nil
}

func TestFetchParsesPods(t *testing.T) {
	pods, err := os.ReadFile("testdata/pods.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	r := &fakeRunner{responses: map[string][]byte{"pods": pods}}

	snap, err := Fetch(context.Background(), r)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(snap.Pods) != 1 {
		t.Fatalf("got %d pods, want 1", len(snap.Pods))
	}
	if snap.Pods[0].Name != "api-7d9f" {
		t.Errorf("got pod name %q, want %q", snap.Pods[0].Name, "api-7d9f")
	}
	if snap.Pods[0].Namespace != "demo" {
		t.Errorf("got namespace %q, want %q", snap.Pods[0].Namespace, "demo")
	}
}

func TestFetchNeverIssuesAWriteVerb(t *testing.T) {
	r := &fakeRunner{responses: map[string][]byte{}}
	if _, err := Fetch(context.Background(), r); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	banned := []string{"apply", "delete", "scale", "patch", "exec", "edit", "create"}
	for _, call := range r.calls {
		joined := strings.Join(call, " ")
		for _, verb := range banned {
			if strings.Contains(joined, verb) {
				t.Errorf("Fetch issued a forbidden verb %q in call: %s", verb, joined)
			}
		}
	}
}
