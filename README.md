# verdict

**It doesn't report problems. It confirms them.**

`verdict` reads your current kubeconfig and prints what is actually wrong with the
cluster. Not an inventory. Not pod counts. If something is on the screen, it was
checked — and every finding carries the command that proves it.

An empty screen means healthy.

```
$ verdict

  1 problem(s) in k3d-devlab

  [CRITICAL] Service demo/c2-api has no backing pods
      Selector app=c2,tier=web matches 0 pods. Closest pod demo/c2-api-5475d95cfb-9ndqm
      differs on — tier: want "web", pod has "backend".
      evidence: Service demo/c2-api — selector app=c2,tier=web
      evidence: Pod demo/c2-api-5475d95cfb-9ndqm — labels app=c2,tier=backend
      fix:      Align the Service selector with pod demo/c2-api-5475d95cfb-9ndqm.
      verify:   kubectl get endpoints c2-api -n demo
```

## Why

Kubernetes usually knows why something is broken and won't tell you. The complaint
is old: ["Give the proper reason for killing pods"](https://github.com/kubernetes/kubernetes/issues/81723)
has been open since 2019, ["Log something about OOMKilled containers"](https://github.com/kubernetes/kubernetes/issues/69676)
since 2018. Neither is fixed, and nothing user-facing is being worked on upstream.

Existing tools either show you everything and let you hunt, or tell you about
problems they never checked. `verdict` verifies before it speaks — so it reports
the actual error from the crashed container, the exact label that doesn't match,
the specific node constraint that failed.

Being quiet when nothing is wrong is a feature, and the test suite that enforces
it matters more than the one that finds problems.

## Install

Requires `kubectl` on your PATH — `verdict` shells out to it, so every auth method
your kubeconfig already uses (OIDC, exec plugins, cloud helpers, Vault) just works.

```bash
go build -o verdict .
./verdict
```

## Usage

```
verdict [flags]

  --context string    kubeconfig context to use (default: current context)
  --json              emit findings as JSON
  --kubectl string    path to the kubectl binary (default "kubectl")
```

Exit codes: `0` nothing wrong, `1` problems found, `2` could not run.

## What it checks

| Check | What it confirms |
|---|---|
| Missing ConfigMap/Secret | The referenced object genuinely does not exist. Optional references are ignored. |
| Service with no backing pods | Scans for near-miss pods and names the label that differs |
| CrashLoopBackOff | Reads the crashed container's logs and reports the real error plus a decoded exit code |
| Unschedulable | Computes per node which constraint failed — label, taint, or the actual resource shortfall |

Pods failing for one shared cause are collapsed into a single finding with a blast
radius, not one row each.

## Safety

Read-only by construction. The only kubectl verbs it can issue are `get`, `logs`
and `config`, and a test asserts it. Secrets are decoded into a metadata-only
struct, so secret values never reach memory. Nothing is installed in your cluster
and nothing leaves your machine.

## Status

v0 — four checks, laptop-side. Next: a problems-only UI, then an in-cluster agent
that can actively call probe endpoints and test pod-to-pod reachability, which a
laptop tool structurally cannot do.

Design notes in [`docs/superpowers/specs/`](docs/superpowers/specs/).
