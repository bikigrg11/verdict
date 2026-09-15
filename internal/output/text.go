// Package output renders findings for humans.
package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/bikigrg11/verdict/internal/model"
)

// Text writes a human-readable report. An empty findings slice is a success state.
func Text(w io.Writer, findings []model.Finding, context string) error {
	if len(findings) == 0 {
		_, err := fmt.Fprintf(w, "\n  No problems found in %s.\n\n", context)
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n  %d problem(s) in %s\n\n", len(findings), context)

	for _, f := range findings {
		fmt.Fprintf(&b, "  [%s] %s\n", strings.ToUpper(string(f.Severity)), f.Title)
		fmt.Fprintf(&b, "      %s\n", f.Explanation)

		for _, e := range f.Evidence {
			fmt.Fprintf(&b, "      evidence: %s %s — %s\n", e.Kind, e.Name, e.Detail)
		}
		if f.BlastRadius > 1 {
			fmt.Fprintf(&b, "      affects:  %d objects\n", f.BlastRadius)
		}
		if f.SuggestedAction != "" {
			fmt.Fprintf(&b, "      fix:      %s\n", f.SuggestedAction)
		}
		if f.Reproduce != "" {
			fmt.Fprintf(&b, "      verify:   %s\n", f.Reproduce)
		}
		b.WriteString("\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}
