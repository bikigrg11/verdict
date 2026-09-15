// Package model defines the Finding — the single thing verdict emits.
package model

import "sort"

type Severity string

const (
	SeverityCritical Severity = "critical" // serving traffic is broken
	SeverityHigh     Severity = "high"     // workload degraded
	SeverityMedium   Severity = "medium"   // latent
)

// rank orders severities most-serious first.
func (s Severity) rank() int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	default:
		return 3
	}
}

// Evidence is one concrete fact supporting a finding.
type Evidence struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

// Finding is a confirmed problem. Nothing reaches a Finding unverified.
//
// first_seen/last_seen from spec §7 are intentionally absent in v0: they need
// state across runs, and v0 is a stateless one-shot CLI. Reproduce is added
// per spec §9a.
type Finding struct {
	ID                 string     `json:"id"`
	Rule               string     `json:"rule"`
	Severity           Severity   `json:"severity"`
	Verified           bool       `json:"verified"`
	VerificationMethod string     `json:"verification_method"`
	Title              string     `json:"title"`
	Explanation        string     `json:"explanation"`
	Evidence           []Evidence `json:"evidence"`
	Affected           []string   `json:"affected"`
	BlastRadius        int        `json:"blast_radius"`
	SuggestedAction    string     `json:"suggested_action"`
	Reproduce          string     `json:"reproduce"`
}

// Sort orders findings by severity, then by blast radius descending.
func Sort(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity.rank() != b.Severity.rank() {
			return a.Severity.rank() < b.Severity.rank()
		}
		return a.BlastRadius > b.BlastRadius
	})
}
