package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bikigrg11/verdict/internal/model"
)

func TestTextRendersFindingWithReproduceCommand(t *testing.T) {
	findings := []model.Finding{{
		ID:                 "c2-demo-api",
		Rule:               "service_no_endpoints",
		Severity:           model.SeverityCritical,
		Verified:           true,
		VerificationMethod: "selector_match_scan",
		Title:              "Service demo/api has no backing pods",
		Explanation:        "Selector app=api,tier=web matches 0 pods.",
		BlastRadius:        3,
		SuggestedAction:    "Reconcile the Service selector with the pod template labels.",
		Reproduce:          "kubectl get endpoints -n demo api",
	}}

	var buf bytes.Buffer
	if err := Text(&buf, findings, "kind-devlab"); err != nil {
		t.Fatalf("Text returned error: %v", err)
	}
	got := buf.String()

	for _, want := range []string{
		"CRITICAL",
		"Service demo/api has no backing pods",
		"Selector app=api,tier=web matches 0 pods.",
		"kubectl get endpoints -n demo api",
		"kind-devlab",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

func TestTextReportsAllClearWhenNoFindings(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, nil, "kind-devlab"); err != nil {
		t.Fatalf("Text returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "No problems found") {
		t.Errorf("empty findings should report all clear, got:\n%s", buf.String())
	}
}

func TestSortOrdersBySeverityThenBlastRadius(t *testing.T) {
	findings := []model.Finding{
		{Title: "medium-small", Severity: model.SeverityMedium, BlastRadius: 1},
		{Title: "high-small", Severity: model.SeverityHigh, BlastRadius: 2},
		{Title: "critical", Severity: model.SeverityCritical, BlastRadius: 1},
		{Title: "high-big", Severity: model.SeverityHigh, BlastRadius: 40},
	}
	model.Sort(findings)

	want := []string{"critical", "high-big", "high-small", "medium-small"}
	for i, w := range want {
		if findings[i].Title != w {
			t.Errorf("position %d: got %q, want %q", i, findings[i].Title, w)
		}
	}
}
