package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bikigrg11/verdict/internal/kube"
	"github.com/bikigrg11/verdict/internal/model"
)

// Unschedulable is C5: a pod the scheduler cannot place.
//
// Verification computes, per node, the specific constraint that failed — label,
// taint, or insufficient resource — rather than repeating the scheduler's message.
type Unschedulable struct{}

func (Unschedulable) Name() string { return "not_scheduled" }

// podRequests totals the cpu and memory a pod asks for.
func podRequests(p corev1.Pod) (cpu, mem resource.Quantity) {
	for _, c := range p.Spec.Containers {
		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			cpu.Add(q)
		}
		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			mem.Add(q)
		}
	}
	return cpu, mem
}

// tolerates reports whether a pod tolerates a taint.
func tolerates(p corev1.Pod, t corev1.Taint) bool {
	for _, tol := range p.Spec.Tolerations {
		if tol.Operator == corev1.TolerationOpExists && (tol.Key == "" || tol.Key == t.Key) {
			return true
		}
		if tol.Key == t.Key && tol.Value == t.Value {
			return true
		}
	}
	return false
}

// whyNotNode returns the reason a pod cannot be placed on a node, or "" if it can.
func whyNotNode(p corev1.Pod, n corev1.Node, usedCPU, usedMem resource.Quantity) string {
	for k, v := range p.Spec.NodeSelector {
		if n.Labels[k] != v {
			return fmt.Sprintf("no matching label %s=%s", k, v)
		}
	}
	for _, t := range n.Spec.Taints {
		if t.Effect == corev1.TaintEffectNoSchedule && !tolerates(p, t) {
			return fmt.Sprintf("untolerated taint %s=%s:%s", t.Key, t.Value, t.Effect)
		}
	}

	wantCPU, wantMem := podRequests(p)

	freeCPU := n.Status.Allocatable[corev1.ResourceCPU]
	freeCPU.Sub(usedCPU)
	if wantCPU.Cmp(freeCPU) > 0 {
		return fmt.Sprintf("insufficient cpu (needs %s, %s free)", wantCPU.String(), freeCPU.String())
	}

	freeMem := n.Status.Allocatable[corev1.ResourceMemory]
	freeMem.Sub(usedMem)
	if wantMem.Cmp(freeMem) > 0 {
		return fmt.Sprintf("insufficient memory (needs %s, %s free)", wantMem.String(), freeMem.String())
	}
	return ""
}

func (u Unschedulable) Detect(s *kube.Snapshot) []Suspicion {
	var out []Suspicion
	for _, p := range s.Pods {
		if p.Status.Phase != corev1.PodPending || p.Spec.NodeName != "" {
			continue
		}
		for _, cond := range p.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
				out = append(out, Suspicion{Namespace: p.Namespace, Name: p.Name})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (u Unschedulable) Verify(_ context.Context, s *kube.Snapshot, _ kube.LogFetcher, sus Suspicion) (*model.Finding, error) {
	var pod corev1.Pod
	found := false
	for _, p := range s.Pods {
		if p.Namespace == sus.Namespace && p.Name == sus.Name {
			pod, found = p, true
			break
		}
	}
	if !found || pod.Spec.NodeName != "" {
		return nil, nil // it got scheduled in the meantime
	}
	if len(s.Nodes) == 0 {
		return nil, nil
	}

	// Sum what each node is already committed to.
	usedCPU := map[string]resource.Quantity{}
	usedMem := map[string]resource.Quantity{}
	for _, p := range s.Pods {
		if p.Spec.NodeName == "" {
			continue
		}
		cpu, mem := podRequests(p)
		c, m := usedCPU[p.Spec.NodeName], usedMem[p.Spec.NodeName]
		c.Add(cpu)
		m.Add(mem)
		usedCPU[p.Spec.NodeName], usedMem[p.Spec.NodeName] = c, m
	}

	reasons := map[string][]string{}
	var evidence []model.Evidence
	for _, n := range s.Nodes {
		why := whyNotNode(pod, n, usedCPU[n.Name], usedMem[n.Name])
		if why == "" {
			// A node that fits means our reasoning is incomplete — stay silent
			// rather than assert something we cannot explain.
			return nil, nil
		}
		reasons[why] = append(reasons[why], n.Name)
		evidence = append(evidence, model.Evidence{Kind: "Node", Name: n.Name, Detail: why})
	}

	parts := make([]string, 0, len(reasons))
	for why, nodes := range reasons {
		parts = append(parts, fmt.Sprintf("%d node(s): %s", len(nodes), why))
	}
	sort.Strings(parts)

	return &model.Finding{
		ID:                 fmt.Sprintf("c5-%s-%s", sus.Namespace, sus.Name),
		Rule:               u.Name(),
		Severity:           model.SeverityHigh,
		Verified:           true,
		VerificationMethod: "node_fit_computation",
		Title:              fmt.Sprintf("Pod %s/%s cannot be scheduled", sus.Namespace, sus.Name),
		Explanation:        fmt.Sprintf("No node accepts this pod — %s.", strings.Join(parts, "; ")),
		Evidence:           evidence,
		Affected:           []string{sus.Namespace + "/" + sus.Name},
		BlastRadius:        1,
		SuggestedAction:    "Lower the pod's requests, add a matching node, or adjust selectors and tolerations.",
		Reproduce:          fmt.Sprintf("kubectl describe pod %s -n %s", sus.Name, sus.Namespace),
	}, nil
}
