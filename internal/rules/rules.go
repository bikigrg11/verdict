// Package rules implements the detect-then-verify engine.
//
// Detect is pure: it reads the snapshot and returns suspicions, nothing more.
// Verify must confirm a suspicion before it becomes a Finding. A Verify that
// returns (nil, nil) means "looked, and it is fine" — that silence is the
// false-positive guard, and it is the whole product.
package rules

import (
	"context"

	"github.com/bikigrg11/verdict/internal/kube"
	"github.com/bikigrg11/verdict/internal/model"
)

// Suspicion is something that looks wrong and has not yet been confirmed.
type Suspicion struct {
	Namespace string
	Name      string
	Container string
	Data      map[string]string
}

// Rule is one check.
type Rule interface {
	Name() string
	Detect(s *kube.Snapshot) []Suspicion
	Verify(ctx context.Context, s *kube.Snapshot, lf kube.LogFetcher, sus Suspicion) (*model.Finding, error)
}

// Run executes every rule and returns the confirmed findings, sorted.
func Run(ctx context.Context, s *kube.Snapshot, lf kube.LogFetcher, rs []Rule) ([]model.Finding, error) {
	var findings []model.Finding
	for _, r := range rs {
		for _, sus := range r.Detect(s) {
			f, err := r.Verify(ctx, s, lf, sus)
			if err != nil {
				// A rule that cannot verify stays silent rather than guessing.
				continue
			}
			if f != nil {
				findings = append(findings, *f)
			}
		}
	}
	model.Sort(findings)
	return findings, nil
}
