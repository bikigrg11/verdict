package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/bikigrg11/verdict/internal/kube"
	"github.com/bikigrg11/verdict/internal/model"
)

// NoEndpoints is C2: a Service whose selector matches no pods.
//
// Verification does not stop at "zero endpoints" — it scans for near-miss pods
// and names the exact label that differs, which is the answer the user wants.
type NoEndpoints struct{}

func (NoEndpoints) Name() string { return "service_no_endpoints" }

func selectorMatches(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// Detect flags Services that have a selector but match nothing.
func (n NoEndpoints) Detect(s *kube.Snapshot) []Suspicion {
	var out []Suspicion
	for _, service := range s.Services {
		// No selector means headless or ExternalName — endpoints are managed
		// elsewhere on purpose. Not a problem.
		if len(service.Spec.Selector) == 0 {
			continue
		}
		if service.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}

		matched := 0
		for _, p := range s.Pods {
			if p.Namespace == service.Namespace && selectorMatches(service.Spec.Selector, p.Labels) {
				matched++
			}
		}
		if matched == 0 {
			out = append(out, Suspicion{Namespace: service.Namespace, Name: service.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// nearMiss finds pods matching some but not all selector keys, and reports which
// keys differ. This is what turns "no endpoints" into an actionable answer.
func nearMiss(service corev1.Service, pods []corev1.Pod) (corev1.Pod, []string, bool) {
	best := -1
	var bestPod corev1.Pod
	var bestDiff []string

	for _, p := range pods {
		if p.Namespace != service.Namespace {
			continue
		}
		hits, diff := 0, []string{}
		for k, v := range service.Spec.Selector {
			if p.Labels[k] == v {
				hits++
			} else {
				diff = append(diff, fmt.Sprintf("%s: want %q, pod has %q", k, v, p.Labels[k]))
			}
		}
		if hits > best && hits > 0 {
			best, bestPod, bestDiff = hits, p, diff
		}
	}
	sort.Strings(bestDiff)
	return bestPod, bestDiff, best > 0
}

func formatSelector(sel map[string]string) string {
	parts := make([]string, 0, len(sel))
	for k, v := range sel {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (n NoEndpoints) Verify(_ context.Context, s *kube.Snapshot, _ kube.LogFetcher, sus Suspicion) (*model.Finding, error) {
	var service corev1.Service
	found := false
	for _, candidate := range s.Services {
		if candidate.Namespace == sus.Namespace && candidate.Name == sus.Name {
			service, found = candidate, true
			break
		}
	}
	if !found {
		return nil, nil
	}

	// Re-confirm zero matches before emitting anything.
	for _, p := range s.Pods {
		if p.Namespace == service.Namespace && selectorMatches(service.Spec.Selector, p.Labels) {
			return nil, nil
		}
	}

	sel := formatSelector(service.Spec.Selector)
	evidence := []model.Evidence{{
		Kind: "Service", Name: sus.Namespace + "/" + sus.Name,
		Detail: "selector " + sel,
	}}

	explanation := fmt.Sprintf("Selector %s matches 0 pods in namespace %s.", sel, sus.Namespace)
	action := "Reconcile the Service selector with the pod template labels."

	if pod, diff, ok := nearMiss(service, s.Pods); ok {
		explanation = fmt.Sprintf("Selector %s matches 0 pods. Closest pod %s/%s differs on — %s.",
			sel, pod.Namespace, pod.Name, strings.Join(diff, "; "))
		evidence = append(evidence, model.Evidence{
			Kind: "Pod", Name: pod.Namespace + "/" + pod.Name,
			Detail: "labels " + formatSelector(pod.Labels),
		})
		action = fmt.Sprintf("Align the Service selector with pod %s/%s, or relabel the pods.", pod.Namespace, pod.Name)
	}

	return &model.Finding{
		ID:                 fmt.Sprintf("c2-%s-%s", sus.Namespace, sus.Name),
		Rule:               n.Name(),
		Severity:           model.SeverityCritical,
		Verified:           true,
		VerificationMethod: "selector_match_scan",
		Title:              fmt.Sprintf("Service %s/%s has no backing pods", sus.Namespace, sus.Name),
		Explanation:        explanation,
		Evidence:           evidence,
		Affected:           []string{sus.Namespace + "/" + sus.Name},
		BlastRadius:        1,
		SuggestedAction:    action,
		Reproduce:          fmt.Sprintf("kubectl get endpoints %s -n %s", sus.Name, sus.Namespace),
	}, nil
}
