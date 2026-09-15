// Package kube fetches cluster state by shelling out to kubectl.
//
// Shelling out rather than using client-go is deliberate: kubectl already
// implements every kubeconfig auth path (OIDC, exec plugins, cloud helpers,
// client certs, Vault), and inheriting that costs nothing.
package kube

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Runner executes a kubectl invocation and returns its stdout.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// Kubectl is the real Runner, backed by the kubectl binary on PATH.
type Kubectl struct {
	Binary  string // defaults to "kubectl"
	Context string // optional --context override
}

func (k Kubectl) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := k.Binary
	if bin == "" {
		bin = "kubectl"
	}
	full := args
	if k.Context != "" {
		full = append([]string{"--context", k.Context}, args...)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl %v: %w: %s", full, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// PreviousLogs returns the logs of the previous (crashed) instance of a container.
func (k Kubectl) PreviousLogs(ctx context.Context, namespace, pod, container string, tailLines int) (string, error) {
	out, err := k.Run(ctx, "logs", "-n", namespace, pod,
		"-c", container, "--previous", fmt.Sprintf("--tail=%d", tailLines))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// LogFetcher is the narrow slice of kubectl that rules are allowed to reach for.
type LogFetcher interface {
	PreviousLogs(ctx context.Context, namespace, pod, container string, tailLines int) (string, error)
}
