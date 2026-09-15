package kube

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// ObjectRef names an object without carrying its contents.
type ObjectRef struct {
	Namespace string
	Name      string
}

// Snapshot is one read of the cluster. Rules operate on this and nothing else.
type Snapshot struct {
	Context    string
	Pods       []corev1.Pod
	Services   []corev1.Service
	Nodes      []corev1.Node
	ConfigMaps []ObjectRef
	Secrets    []ObjectRef // names only; values are never decoded
}

// metaOnlyList decodes just the identity of each item. Used for Secrets so that
// secret material is discarded by the decoder and never reaches memory.
type metaOnlyList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"items"`
}

func getJSON(ctx context.Context, r Runner, resource string, into any) error {
	out, err := r.Run(ctx, "get", resource, "--all-namespaces", "-o", "json")
	if err != nil {
		return fmt.Errorf("listing %s: %w", resource, err)
	}
	if err := json.Unmarshal(out, into); err != nil {
		return fmt.Errorf("parsing %s: %w", resource, err)
	}
	return nil
}

func getRefs(ctx context.Context, r Runner, resource string) ([]ObjectRef, error) {
	var list metaOnlyList
	if err := getJSON(ctx, r, resource, &list); err != nil {
		return nil, err
	}
	refs := make([]ObjectRef, 0, len(list.Items))
	for _, item := range list.Items {
		refs = append(refs, ObjectRef{Namespace: item.Metadata.Namespace, Name: item.Metadata.Name})
	}
	return refs, nil
}

// Fetch reads everything v0 needs in one pass.
func Fetch(ctx context.Context, r Runner) (*Snapshot, error) {
	snap := &Snapshot{}

	var pods corev1.PodList
	if err := getJSON(ctx, r, "pods", &pods); err != nil {
		return nil, err
	}
	snap.Pods = pods.Items

	var svcs corev1.ServiceList
	if err := getJSON(ctx, r, "services", &svcs); err != nil {
		return nil, err
	}
	snap.Services = svcs.Items

	// Nodes are not namespaced; --all-namespaces is harmless but noisy, so ask directly.
	nodeOut, err := r.Run(ctx, "get", "nodes", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	var nodes corev1.NodeList
	if err := json.Unmarshal(nodeOut, &nodes); err != nil {
		return nil, fmt.Errorf("parsing nodes: %w", err)
	}
	snap.Nodes = nodes.Items

	if snap.ConfigMaps, err = getRefs(ctx, r, "configmaps"); err != nil {
		return nil, err
	}
	if snap.Secrets, err = getRefs(ctx, r, "secrets"); err != nil {
		return nil, err
	}
	return snap, nil
}

// HasConfigMap reports whether a ConfigMap exists in the snapshot.
func (s *Snapshot) HasConfigMap(namespace, name string) bool {
	for _, ref := range s.ConfigMaps {
		if ref.Namespace == namespace && ref.Name == name {
			return true
		}
	}
	return false
}

// HasSecret reports whether a Secret exists in the snapshot.
func (s *Snapshot) HasSecret(namespace, name string) bool {
	for _, ref := range s.Secrets {
		if ref.Namespace == namespace && ref.Name == name {
			return true
		}
	}
	return false
}
