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

// CrashLoop is C3: a container stuck in CrashLoopBackOff.
//
// Verification reads the previous container's logs and extracts the actual
// terminal error. Reporting "it is crashing" is what every other tool does and
// is explicitly not good enough (spec §4.3).
type CrashLoop struct{}

func (CrashLoop) Name() string { return "crash_looping" }

const logTailLines = 50

// ExplainExitCode turns a container exit code into a human explanation.
func ExplainExitCode(code int32) string {
	switch code {
	case 0:
		return "exit code 0 (clean exit)"
	case 1:
		return "exit code 1 (general application error)"
	case 126:
		return "exit code 126 (command found but not executable)"
	case 127:
		return "exit code 127 (command not found — check the image entrypoint)"
	case 137:
		return "exit code 137 (SIGKILL — usually an OOM kill or a failed liveness probe)"
	case 139:
		return "exit code 139 (SIGSEGV — segmentation fault)"
	case 143:
		return "exit code 143 (SIGTERM — terminated)"
	default:
		return fmt.Sprintf("exit code %d", code)
	}
}

// errorPrefixes are ordered by how strongly they indicate a terminal failure.
var errorPrefixes = []string{
	"panic:",
	"fatal error:",
	"Exception in thread",
	"Unhandled exception",
	"FATAL",
	"ERROR:",
	"Error:",
}

// ExtractError returns the most meaningful terminal error line from logs, or "".
func ExtractError(logs string) string {
	lines := strings.Split(strings.TrimSpace(logs), "\n")

	// A Python traceback's cause is its final line, so handle it first.
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Traceback (most recent call last):") {
			for i := len(lines) - 1; i >= 0; i-- {
				candidate := strings.TrimSpace(lines[i])
				if candidate != "" && !strings.HasPrefix(candidate, "File ") && !strings.HasPrefix(candidate, "Traceback") {
					return candidate
				}
			}
		}
	}

	// Otherwise take the last line matching the strongest available prefix.
	for _, prefix := range errorPrefixes {
		for i := len(lines) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(lines[i])
			if strings.HasPrefix(candidate, prefix) {
				return candidate
			}
		}
	}
	return ""
}

func (c CrashLoop) Detect(s *kube.Snapshot) []Suspicion {
	var out []Suspicion
	for _, p := range s.Pods {
		// Completed pods are never crashlooping, whatever their container states say.
		if p.Status.Phase == corev1.PodSucceeded {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting == nil || cs.State.Waiting.Reason != "CrashLoopBackOff" {
				continue
			}
			out = append(out, Suspicion{
				Namespace: p.Namespace,
				Name:      p.Name,
				Container: cs.Name,
				Data:      map[string]string{"restarts": fmt.Sprint(cs.RestartCount)},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (c CrashLoop) Verify(ctx context.Context, s *kube.Snapshot, lf kube.LogFetcher, sus Suspicion) (*model.Finding, error) {
	// Re-confirm the container is still crashlooping.
	var exitCode int32
	confirmed := false
	for _, p := range s.Pods {
		if p.Namespace != sus.Namespace || p.Name != sus.Name {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name != sus.Container {
				continue
			}
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				confirmed = true
				if cs.LastTerminationState.Terminated != nil {
					exitCode = cs.LastTerminationState.Terminated.ExitCode
				}
			}
		}
	}
	if !confirmed {
		return nil, nil
	}

	explanation := fmt.Sprintf("Container %q has restarted %s times and is in CrashLoopBackOff (%s).",
		sus.Container, sus.Data["restarts"], ExplainExitCode(exitCode))

	evidence := []model.Evidence{{
		Kind: "Pod", Name: sus.Namespace + "/" + sus.Name,
		Detail: fmt.Sprintf("container %s, %s restarts, %s", sus.Container, sus.Data["restarts"], ExplainExitCode(exitCode)),
	}}

	method := "exit_code_analysis"

	// Logs are best-effort: unavailable logs downgrade the detail, never the finding.
	if lf != nil {
		if logs, err := lf.PreviousLogs(ctx, sus.Namespace, sus.Name, sus.Container, logTailLines); err == nil {
			if msg := ExtractError(logs); msg != "" {
				explanation = fmt.Sprintf("Container %q is in CrashLoopBackOff after %s restarts (%s). Last error: %s",
					sus.Container, sus.Data["restarts"], ExplainExitCode(exitCode), msg)
				evidence = append(evidence, model.Evidence{
					Kind: "Log", Name: sus.Namespace + "/" + sus.Name,
					Detail: msg,
				})
				method = "previous_container_log_extraction"
			}
		}
	}

	return &model.Finding{
		ID:                 fmt.Sprintf("c3-%s-%s-%s", sus.Namespace, sus.Name, sus.Container),
		Rule:               c.Name(),
		Severity:           model.SeverityHigh,
		Verified:           true,
		VerificationMethod: method,
		Title:              fmt.Sprintf("Pod %s/%s is crash looping", sus.Namespace, sus.Name),
		Explanation:        explanation,
		Evidence:           evidence,
		Affected:           []string{sus.Namespace + "/" + sus.Name},
		BlastRadius:        1,
		SuggestedAction:    "Fix the error above, or roll back to the previous working image.",
		Reproduce: fmt.Sprintf("kubectl logs -n %s %s -c %s --previous --tail=%d",
			sus.Namespace, sus.Name, sus.Container, logTailLines),
	}, nil
}
