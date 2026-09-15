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

// MissingRef is C1: a pod references a ConfigMap or Secret that does not exist.
//
// Verification is exact — the object is either in the snapshot or it is not.
type MissingRef struct{}

func (MissingRef) Name() string { return "missing_ref" }

// ref is one reference from a pod to a config object.
type ref struct {
	kind      string // "ConfigMap" or "Secret"
	namespace string
	name      string
}

// refsIn returns every ConfigMap/Secret a pod depends on.
func refsIn(p corev1.Pod) []ref {
	var out []ref
	add := func(kind, name string) {
		if name != "" {
			out = append(out, ref{kind: kind, namespace: p.Namespace, name: name})
		}
	}

	all := append(append([]corev1.Container{}, p.Spec.Containers...), p.Spec.InitContainers...)
	for _, c := range all {
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				add("ConfigMap", ef.ConfigMapRef.Name)
			}
			if ef.SecretRef != nil {
				add("Secret", ef.SecretRef.Name)
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom == nil {
				continue
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				add("ConfigMap", e.ValueFrom.ConfigMapKeyRef.Name)
			}
			if e.ValueFrom.SecretKeyRef != nil {
				add("Secret", e.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
	for _, v := range p.Spec.Volumes {
		if v.ConfigMap != nil {
			add("ConfigMap", v.ConfigMap.Name)
		}
		if v.Secret != nil {
			add("Secret", v.Secret.SecretName)
		}
	}
	return out
}

// Detect groups pods by the missing object they share, so forty pods blocked by
// one absent ConfigMap become one suspicion with a blast radius of forty.
func (m MissingRef) Detect(s *kube.Snapshot) []Suspicion {
	type key struct{ kind, namespace, name string }
	affected := map[key][]string{}

	for _, p := range s.Pods {
		for _, r := range refsIn(p) {
			exists := s.HasConfigMap(r.namespace, r.name)
			if r.kind == "Secret" {
				exists = s.HasSecret(r.namespace, r.name)
			}
			if exists {
				continue
			}
			k := key{r.kind, r.namespace, r.name}
			affected[k] = append(affected[k], p.Namespace+"/"+p.Name)
		}
	}

	var out []Suspicion
	for k, pods := range affected {
		sort.Strings(pods)
		out = append(out, Suspicion{
			Namespace: k.namespace,
			Name:      k.name,
			Data: map[string]string{
				"kind": k.kind,
				"pods": strings.Join(pods, ","),
			},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Verify re-checks existence. Detect already looked, but Verify is the gate that
// decides whether anything is emitted, so it checks again rather than trusting.
func (m MissingRef) Verify(_ context.Context, s *kube.Snapshot, _ kube.LogFetcher, sus Suspicion) (*model.Finding, error) {
	kind := sus.Data["kind"]

	exists := s.HasConfigMap(sus.Namespace, sus.Name)
	lower := "configmap"
	if kind == "Secret" {
		exists = s.HasSecret(sus.Namespace, sus.Name)
		lower = "secret"
	}
	if exists {
		return nil, nil // looked, and it is fine
	}

	pods := strings.Split(sus.Data["pods"], ",")
	evidence := make([]model.Evidence, 0, len(pods))
	for _, p := range pods {
		evidence = append(evidence, model.Evidence{
			Kind: "Pod", Name: p,
			Detail: fmt.Sprintf("references %s %s/%s", kind, sus.Namespace, sus.Name),
		})
	}

	return &model.Finding{
		ID:                 fmt.Sprintf("c1-%s-%s-%s", strings.ToLower(kind), sus.Namespace, sus.Name),
		Rule:               m.Name(),
		Severity:           model.SeverityHigh,
		Verified:           true,
		VerificationMethod: "object_existence_check",
		Title:              fmt.Sprintf("%s %s/%s does not exist", kind, sus.Namespace, sus.Name),
		Explanation: fmt.Sprintf("%d pod(s) reference %s %q in namespace %q, and it is not present in the cluster.",
			len(pods), kind, sus.Name, sus.Namespace),
		Evidence:        evidence,
		Affected:        pods,
		BlastRadius:     len(pods),
		SuggestedAction: fmt.Sprintf("Create %s %s/%s, or correct the reference in the pod spec.", kind, sus.Namespace, sus.Name),
		Reproduce:       fmt.Sprintf("kubectl get %s %s -n %s", lower, sus.Name, sus.Namespace),
	}, nil
}
