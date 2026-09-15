// Command verdict reports verified problems in a Kubernetes cluster.
//
// It does not report problems. It confirms them.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bikigrg11/verdict/internal/kube"
	"github.com/bikigrg11/verdict/internal/model"
	"github.com/bikigrg11/verdict/internal/output"
	"github.com/bikigrg11/verdict/internal/rules"
)

const fetchTimeout = 30 * time.Second

// allRules is the v0 check set. Adding to this list is a scope change.
func allRules() []rules.Rule {
	return []rules.Rule{
		rules.MissingRef{},
		rules.NoEndpoints{},
		rules.CrashLoop{},
		rules.Unschedulable{},
	}
}

// report runs every rule and writes the result. Returns the process exit code.
func report(ctx context.Context, snap *kube.Snapshot, lf kube.LogFetcher, w io.Writer, asJSON bool) (int, error) {
	findings, err := rules.Run(ctx, snap, lf, allRules())
	if err != nil {
		return 2, err
	}

	if asJSON {
		if findings == nil {
			findings = []model.Finding{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(findings); err != nil {
			return 2, err
		}
	} else if err := output.Text(w, findings, snap.Context); err != nil {
		return 2, err
	}

	if len(findings) > 0 {
		return 1, nil
	}
	return 0, nil
}

func main() {
	kubeContext := flag.String("context", "", "kubeconfig context to use (default: current context)")
	asJSON := flag.Bool("json", false, "emit findings as JSON")
	binary := flag.String("kubectl", "kubectl", "path to the kubectl binary")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	k := kube.Kubectl{Binary: *binary, Context: *kubeContext}

	name := *kubeContext
	if name == "" {
		if out, err := k.Run(ctx, "config", "current-context"); err == nil {
			name = string(bytes.TrimSpace(out))
		} else {
			name = "current context"
		}
	}

	snap, err := kube.Fetch(ctx, k)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verdict: %v\n", err)
		os.Exit(2)
	}
	snap.Context = name

	code, err := report(ctx, snap, k, os.Stdout, *asJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verdict: %v\n", err)
	}
	os.Exit(code)
}
