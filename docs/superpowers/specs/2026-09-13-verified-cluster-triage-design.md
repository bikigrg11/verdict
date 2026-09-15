# Verified Cluster Triage — Design

**Status:** draft, pending review
**Date:** 2026-09-13
**Supersedes:** kubescope (read-only Kubernetes MCP server) — deleted, rationale in §3

---

## 1. The one-line pitch

> **It doesn't report problems. It confirms them.**
> A dashboard whose entire screen is the list of things wrong with your cluster — verified,
> ranked, and explained. An empty screen means healthy.

No pod counts. No resource trees. No inventory. If something is on the screen, it is
actually broken, and the tool checked.

---

## 2. Why this project exists (the evidence)

Research conducted 2026-09-13 against the Kubernetes issue trackers, practitioner surveys,
and the existing tool ecosystem.

### 2.1 Kubernetes refuses to explain itself, and has for a decade

Four of the eleven most-discussed open issues in `kubernetes/kubernetes` are the same
complaint — the cluster knows something and will not tell you:

| Issue | Title | Open since | Comments |
|---|---|---|---|
| [#22368](https://github.com/kubernetes/kubernetes/issues/22368) | Facilitate ConfigMap rollouts / management | 2016 | 300 |
| [#50375](https://github.com/kubernetes/kubernetes/issues/50375) | Clear restart counter | 2017 | 190 |
| [#49387](https://github.com/kubernetes/kubernetes/issues/49387) | `kubectl get` should filter on advanced pod status | 2017 | 100 |
| [#69676](https://github.com/kubernetes/kubernetes/issues/69676) | Log something about OOMKilled containers | 2018 | 95 |
| [#81723](https://github.com/kubernetes/kubernetes/issues/81723) | Give the proper reason for killing pods | 2019 | 88 |

**None are fixed.** Everything with momentum in the 2026 tracker is deep internals — DRA
authorization, declarative validation, scheduler batching, CPU manager/NUMA, watch-cache
scale. No user-facing diagnosis work is underway. This gap is permanently third-party
territory.

### 2.2 Practitioners rank it their #1 problem

- **Debugging/observability is the top Kubernetes networking challenge at 61%**, ahead of
  egress control (35%) and service-to-service (32%) — survey of 232 IT professionals.
- **~80% of incidents stem from operational complexity**, not infrastructure failure.
- Teams run **6.28 tools on average** for networking alone. The problem is not a shortage
  of tools; it is that none of them answer the question.

### 2.3 Demand is proven, and the incumbent is unreliable

`k8sgpt` has **8,175 stars**, which settles whether people want automated diagnosis. Its own
issue tracker documents the failure mode: *"incorrectly reports a successfully completed pod
as an issue with exit code 255"*, *"ReplicaSet analyzer ignores ReplicaFailure"*, *"Security
analyzer resolves ClusterRole RoleBindings as namespaced Roles."* It also requires a paid API
key and returns different answers on different runs.

**That unreliability is the opening.** See §4.

---

## 3. What we are deliberately not building

### 3.1 kubescope (deleted)

A ~180-line Python read-only Kubernetes MCP server. Deleted 2026-09-13 because read-only
access plus guardrails is table stakes among existing Kubernetes MCP servers — it had no
defensible claim.

Salvaged: the deterministic failure-chain idea in `src/diagnose.py`
(scheduling → image → runtime → config → probes → node, returning a verdict with no LLM
involved). That logic becomes the rules engine described in §6. The original source is
archived at `kubescope-original-20260913.tar.gz`.

### 3.2 A `kubectl` plugin as the primary surface

Investigated and rejected as the *lead* surface. The "why is my pod broken" CLI space is
actively fragmenting: krew already carries `why-fail`, `why-pending`, `why-blocked`, `doctor`
and `config-doctor`, plus `kubectl-why`, `kubectl-why-fail`, `kubectl-why-pending` in the
wild. All are tiny (2–17 stars) and most were created within the last four months. Entering
as the sixth narrow plugin is a losing position.

A CLI may ship later as a secondary interface to the same engine.

### 3.3 Another resource browser

k9s (34,574 stars), Portainer (38,496), Headlamp (7,263) and Lens all organize around
inventory. Finding the problem remains the user's job. We are explicitly not competing here.

---

## 4. Competitive landscape and the gap

| Tool | Stars | What it actually does | Why it isn't this |
|---|---|---|---|
| k9s | 34,574 | Terminal resource browser | Inventory-first; you hunt |
| Portainer | 38,496 | Management UI | Inventory-first |
| Headlamp | 7,263 | Official Dashboard successor (SIG UI) | Inventory-first, but **plugin-extensible** — see §10 |
| k8sgpt | 8,175 | AI-based analysis | Nondeterministic, documented false positives, needs API key |
| Reloader | 10,407 | Auto-restarts pods on ConfigMap change | **Prevention**, not detection; must be pre-installed and annotated |
| Polaris | 3,386 | Best-practice manifest validation | Static YAML advice, not live breakage |
| Robusta | 3,094 | Prometheus alert enrichment | Requires a full monitoring stack first |
| Kubevious | 1,707 | Config validation + browser | Validation-flavored, hasn't landed after years |
| Zora | 314 | Compliance scanning | Static scanning |
| kubernetes/dashboard | — | **Archived 2026-01-21** | Dead, no maintainers |
| [ROZOOM](https://github.com/ceh13-community/rozoom) | 3 | Desktop app (Tauri/SvelteKit), cluster scoring | Laptop-side — **structurally cannot verify**; see §4.2 |

**The gap:** every tool either shows you everything (browsers), or tells you about problems
it has not checked (analyzers/scanners). Nothing verifies.

Searches for ConfigMap *drift detection* returned nothing at all.

### 4.2 ROZOOM — the nearest neighbour, and why it cannot cross this line

`ceh13-community/rozoom` (Apache 2.0, TypeScript, Tauri + SvelteKit desktop app) is the
closest thing found. Verified 2026-09-13: created 2026-03-30, actively developed (v0.23.1,
20+ releases since August), **3 stars, 0 forks**.

Its repository tagline is *"The Swiss Army Knife for Kubernetes. One app, every cluster,
zero dependencies"* — a **general-purpose** cluster app. The problems-first scoring described
in its August 2026 launch article is one feature, not the product's identity. It is closer to
a Lens alternative than to this project.

**The structural point.** ROZOOM advertises "no agent, no operator, nothing installed on the
cluster" as a selling point: it talks to the API server from the user's laptop. That choice
makes verification impossible, because from outside the cluster you cannot

- call a pod's readiness probe on its pod IP,
- resolve a cluster-internal Service DNS name,
- test pod-to-pod reachability, or
- confirm a refused connection rather than inferring one.

Pod IPs and cluster DNS are not routable from a laptop. ROZOOM can therefore score and infer
but never confirm — the same category as k8sgpt and Polaris.

This is the strongest available evidence for the §5 architecture: the in-cluster agent is not
a preference, it is the precondition for the entire thesis, and the nearest competitor has
explicitly chosen not to cross that line.

### 4.3 The install-cost burden this creates

ROZOOM's strongest selling point is one this project deliberately gives up:

> "Nothing runs on your cluster. No agent, no operator, no DaemonSet, and nothing to
> `helm install` before you can connect. Disconnect, and nothing is left behind."

That is real friction on our side and must not be waved away. Requiring a `helm install`
loses outright to a zero-install competitor **unless verification visibly delivers something
a laptop tool cannot produce.**

**Therefore, a hard rule on output:** every finding must carry evidence that is impossible to
obtain from outside the cluster — an actual probe response body and status code, a real DNS
resolution result, an observed connection refusal, the real log line from the crashed
container. A finding whose evidence a laptop could have inferred does not justify the install
and must be treated as a design failure, not a shipped feature.

Concretely: "readiness probe is failing" is a ROZOOM-grade finding and is not good enough.
"Probe GET http://10.1.4.7:8080/healthz returned 500 — body: `db: connection refused`" is the
bar.

### 4.4 Positioning lesson taken from ROZOOM

ROZOOM describes itself as a Swiss Army knife, a linter, an IDE, a clarity tool, and a scorer
across five different artifacts, and has 3 stars after six months of competent engineering
and 20+ well-packaged releases. Its problem is legibility, not quality.

**Consequence for this project:** one sentence, used everywhere without variation — repo
tagline, README headline, UI title, and any post. The sentence is §1: *"It doesn't report
problems. It confirms them."* Breadth is what killed the nearest neighbour; scope discipline
(§10) is a marketing requirement as much as an engineering one.

### 4.1 The commercial market (researched 2026-09-13)

This is a funded category, which is validation rather than discouragement.

| Company | Funding / price | What it is |
|---|---|---|
| **Komodor** | **$72M raised** ($4M seed, $21M A/Accel, $42M B/Tiger Global), 124 staff, **~$30/node/mo** | Founded explicitly to "redefine Kubernetes troubleshooting" — the closest commercial analogue |
| **Causely** | VC-backed | Causal inference for root-cause attribution; **shipped an MCP server Nov 2025** |
| **Groundcover** | $30–50/host/mo | eBPF observability with an AI analyst mode |
| **Fairwinds Insights** | Custom | Commercial layer over their own OSS (Polaris, Goldilocks) |
| **PerfectScale** | Tiered | Rightsizing and governance |
| **Datadog / Dynatrace** | Premium | Enterprise AI root-cause analysis |

**What this explains.** §4 asked why no good free problems-only tool exists. The answer is
that everyone who built one well monetized it. The gap is not evidence the idea is weak — it
is evidence the idea is worth money. A 100-node cluster pays roughly **$36,000/year** to
Komodor simply to learn what is broken.

**The risk, stated plainly.** This project cannot out-feature 124 employees and $72M, and
should not try. Komodor has already published "Komodor AI SRE vs. OSS AI SRE Agent," so they
treat open-source agents as a competitive threat and are watching the space.

**Where a free tool still wins.** Every commercial product above requires installing an
agent, shipping telemetry to a SaaS backend, creating an account, and paying per node — a
procurement decision. Nobody serves the user who wants to point something at a cluster right
now: free, no account, no telemetry egress, no per-node bill, answer in seconds. That is
homelabs and learners, consultants handed an unfamiliar cluster, teams with three clusters
rather than three hundred, and **any environment whose compliance rules forbid shipping
telemetry off-premises**.

That audience built k9s (34,574 stars), Reloader (10,407) and stern (4,857). It is large, and
it is **disjoint from Komodor's buyer** — this project takes no revenue from them and is
therefore not a target. ROZOOM (§4.2) leads with "no sign-in, no cloud account, Apache 2.0,"
which independently confirms that this is the natural free-tool positioning — even though its
product is a different one.

**Consequence for the design:** no account, no egress, no per-node metering, no SaaS
dependency — ever. §8 constraint 8 (no egress) is therefore a *positioning* requirement, not
only a safety one.

---

## 5. Core concept: detect → verify → rank

The product is three stages, and stage two is the entire differentiator.

```
   API watch            active confirmation          blast-radius
   (suspicion)    →     (fact)                 →     ordering
   "probe may be         "called it: connection       "affects 1 Service,
    failing"              refused on :8080"            12 pods"
```

**Nothing reaches the screen unverified.** A check that cannot be confirmed is marked
`unverified`, ranked below every verified finding, and hidden behind an off-by-default
toggle. It is never presented as fact. This is the trust mechanism, and trust is the
product.

### Why the agent must run inside the cluster

A CLI on a laptop can only read the API server and infer. An in-cluster agent can go look.
That is the architectural justification for the agent form factor, and it is not available
to any laptop-side tool.

---

## 6. Check catalog

Each check is a `(detect, verify)` pair. Verification is mandatory.

| # | Check | Detection signal | **Verification** |
|---|---|---|---|
| C1 | Missing ConfigMap/Secret reference | `CreateContainerConfigError`, or spec reference scan | `GET` the named object. Exists or does not. Binary and certain. |
| C2 | Service has no endpoints | Empty EndpointSlice | List pods matching the selector; report which labels actually differ |
| C3 | CrashLooping | `CrashLoopBackOff` | Read previous container logs, extract terminal error and exit code — report *the actual error*, not "it's crashing" |
| C4 | Readiness probe misconfigured | Container not ready, probe defined | **Agent calls the probe endpoint itself** and reports the real status code or connection error |
| C5 | Unschedulable | `PodScheduled=False` | Compute against real node allocatable and taints: "3 nodes — 2 lack the GPU label, 1 has 3.2Gi free, needs 8Gi" |
| C6 | Image pull failing | `ImagePullBackOff` / `ErrImagePull` | Distinguish not-found vs unauthorized vs registry-unreachable |
| C7 | Stale config (env-injected) | CM/Secret consumed via `env`/`envFrom` changed after pod start | Agent observed the change event *after* `pod.status.startTime`; env vars never refresh, so this is fact, not inference |
| C8 | Stale config (`subPath` mount) | CM mounted with `subPath` and changed | Same; `subPath` mounts never refresh — a trap almost nobody knows about |
| C9 | OOMKilled | `lastState.terminated.reason` | Compare real memory usage against the limit via metrics API; **if metrics are unavailable, say so — do not assert** |

C7/C8 address issue #22368 directly — 300 comments, nine years open, and per §4 nothing
detects it today.

### Grouping is part of correctness

Forty broken pods caused by one missing ConfigMap is **one finding with a blast radius of
40**, not forty rows. Alert lists that fail to collapse causes are how dashboards become
noise people ignore.

### Severity

- **Critical** — serving traffic is broken (no endpoints, all replicas down)
- **High** — workload degraded (crashloop, OOM, unschedulable)
- **Medium** — latent (stale config, `subPath` trap)

Ordered within severity by blast radius.

---

## 7. Architecture

```
┌─────────────────────── in-cluster (one Deployment) ───────────────────────┐
│                                                                            │
│  Watchers ──────→ State store ──────→ Rules engine ──────→ Verifier pool   │
│  (informers on    (current state +    (the salvaged        (rate-limited    │
│   Pods, CMs,       change history      diagnose chain       active probes)  │
│   Secrets, Svc,    for C7/C8)          as pure functions)        │          │
│   EndpointSlice,                                                 ↓          │
│   Events, Nodes)                                          Findings store    │
│                                                                  │          │
│                                              ┌───────────────────┤          │
│                                       HTTP API              (later) MCP     │
└──────────────────────────────────────────────┼───────────────────┼─────────┘
                                               ↓                   ↓
                                        Web UI (SPA)         AI agents
```

**Components, each independently testable:**

- **Watchers** — informers only; no polling loops. Record change timestamps for config
  objects, which is what makes C7/C8 provable rather than guessed.
- **State store** — current cluster state plus a bounded change history for config objects.
- **Rules engine** — pure functions: `state → suspicion[]`. No I/O, no network. Trivially
  unit-testable against fixtures. This is where the old `diagnose.py` logic lands.
- **Verifier pool** — the only component that touches anything actively. Bounded, rate
  limited, times out.
- **Findings store** — dedupes, groups by cause, computes blast radius, tracks
  `first_seen`/`last_seen`.
- **HTTP API** — serves findings as JSON; the UI is a pure consumer.
- **UI** — single screen. Static SPA served by the agent.

Separating the rules engine (pure) from the verifier (impure) is the key boundary: it means
correctness can be tested without a cluster, and safety can be reviewed in one small file.

### Finding data model

```jsonc
{
  "id": "c2-demo-api-svc",
  "rule": "service_no_endpoints",
  "severity": "critical",
  "verified": true,
  "verified_at": "2026-09-13T21:04:11Z",
  "verification_method": "selector_match_scan",
  "title": "Service demo/api has no backing pods",
  "explanation": "Selector app=api,tier=web matches 0 pods. 3 pods carry app=api but tier=backend.",
  "evidence": [
    { "kind": "Service", "name": "demo/api", "detail": "selector app=api,tier=web" },
    { "kind": "Pod", "name": "demo/api-7d9f", "detail": "labels app=api,tier=backend" }
  ],
  "affected": ["demo/api", "demo/api-7d9f", "..."],
  "blast_radius": 3,
  "suggested_action": "Reconcile the Service selector with the pod template labels.",
  "first_seen": "2026-09-13T20:51:02Z",
  "last_seen":  "2026-09-13T21:04:11Z"
}
```

`verified`, `verification_method` and `evidence` are the fields that make the claim in §1
checkable rather than asserted.

---

## 8. Safety constraints (non-negotiable)

A diagnostic tool that worsens an outage is a tool nobody installs twice.

1. **Read-only RBAC.** `get`/`list`/`watch` only. No write verb appears in the ClusterRole.
2. **No `exec` into containers** in v1. HTTP `GET` and TCP connect only.
3. **Rate limited.** Global cap on probes per second; per-target exponential backoff.
4. **Timeouts.** 2s per probe, hard.
5. **Circuit breaker.** If API-server latency rises or probes start timing out in bulk, stop
   probing and degrade to detection-only, marking findings `unverified`.
6. **Idempotent and side-effect free.** Probing never mutates cluster state.
7. **Bounded footprint.** The agent ships with its own CPU/memory limits set.
8. **No egress.** No telemetry, no SaaS callback, no API keys.

---

## 9. Testing strategy

Two suites. The second one is the product.

### 9.1 Fixture suite (does it detect?)

A reproducible catalog of deliberately-broken clusters on kind/k3d — one manifest set per
check in §6. Each fixture must produce **exactly one** finding, with the right verdict and
the right blast radius.

This catalog is independently useful to anyone learning Kubernetes and is a candidate to
publish as its own repository.

### 9.2 False-positive suite (does it stay quiet?)

Given that the entire pitch is "if it's here, it's real," this matters more than 9.1.
A healthy cluster plus every edge case that naively looks broken must yield **zero findings**:

- Completed Jobs and successful init containers
- Deployments intentionally scaled to 0
- Pods mid-rollout and mid-termination
- Pods pending for a few seconds during normal scheduling
- `subPath` mounts of ConfigMaps that genuinely have not changed
- Services with intentionally empty selectors (headless, ExternalName)
- Restart counts that are historical rather than current

Any regression here is a release blocker. k8sgpt's documented false positives are the
cautionary example and, frankly, the benchmark to beat.

---

## 9a. v0 — the laptop version (build this first)

Decided 2026-09-14. Ship the smallest useful thing before any agent exists.

### The realization that makes this viable

"Laptop-side" and "cannot verify" are not the same thing. Re-examining §6:

| Check | Laptop-verifiable? | Method |
|---|---|---|
| C1 missing ConfigMap/Secret | **yes, certain** | `GET` the referenced object |
| C2 Service has no endpoints | **yes, certain** | list pods, diff the labels, name the mismatch |
| C3 crashlooping | **yes, certain** | `logs --previous` works remotely; extract the real error |
| C5 unschedulable | **yes, certain** | node allocatable vs requests is arithmetic |
| C6 image pull | mostly | parse the reason |
| C4 probe failing | **no** | requires reaching the pod IP |
| network reachability | **no** | requires being inside the cluster network |

Only C4 and network checks actually need §7's agent. v0 is therefore not a degraded thesis —
it is the majority of the thesis, minus one check.

### v0 scope

- Reads the local kubeconfig; current context by default, `--context` to override
- Checks **C1, C2, C3, C5** — each verified, never inferred
- Output ranked by severity, grouped by cause, each finding carrying evidence **and a
  copy-pasteable command that reproduces it** (see "transparency" below)
- No install, nothing in the cluster, no UI

Target: 400–600 lines. The rules engine written here is consumed unchanged by the UI (v1) and
the agent (v2).

### Sequencing

`v0 CLI → v1 UI on the same engine → v2 in-cluster agent (unlocks C4 + network)`

§3.2 rejected a CLI as the *published identity* and that still stands — five competing
`kubectl-why` plugins make it a bad launch surface. v0 is the engine's first consumer and a
development artifact, not the announcement. The public launch is the UI.

### Borrowed from ROZOOM (Apache 2.0, ideas not code)

1. **Shell out to `kubectl` rather than implementing kubeconfig auth.** Their
   `kubectl-proxy` / `kubectlRawFront` layer bundles kubectl and shells out. Kubeconfig auth
   is a swamp — OIDC, exec plugins, EKS/GKE/AKS helpers, client certs, Vault — and `kubectl`
   has already solved all of it. Shelling out with `-o json` inherits every auth method for
   free. This is the single biggest simplification available to v0.
2. **Graceful degradation** (their principle #4): missing `metrics-server` hides a section
   rather than raising an error. This resolves open question 4 — C9 degrades, never errors.
3. **Transparency** (their principle #7): they surface every command they run, with a copy
   button. Adapted here it is stronger than in their product: **every finding ships the exact
   command that proves it**, so the user can confirm any claim by hand. Verification the user
   can re-run themselves is the most credible kind.

**Not borrowed:** fleet-first scope, the Swiss Army knife ambition, bundling binaries, and
desktop-app packaging (see §4.4).

### Why their breadth is our opening

`check-pod-issues.ts` is 8.5 KB and does what the deleted kubescope did: read pod status,
extract `waiting.reason`, count restarts, compare against constants
(`RESTART_THRESHOLD = 5`, `PENDING_THRESHOLD_MINUTES = 10`), report the message verbatim. It
never crosses an object boundary and never reads a log. They have 35+ checks at that depth.
Four checks that resolve references and read crashed-container logs will out-answer all 35 on
the cases that matter.

---

## 9b. Language: Go

Decided 2026-09-14, reversing an earlier same-day call for Python.

**Rationale.**

1. **Career signal, which is the project's stated purpose.** The Kubernetes ecosystem is Go
   throughout — client-go, controller-runtime, kubectl, krew, operator-sdk. A Python
   Kubernetes tool reads as "used the API"; a Go one reads as "works in this ecosystem."
2. **Distribution.** Go emits a single static binary: no runtime, no venv, no pip, no Python
   version negotiation. For a tool intended to be installed by strangers this is decisive.
   Python CLI distribution is a known adoption tax.
3. **v2 requires it regardless.** `controller-runtime` is Go and the Python client has no
   equivalent. Writing v0 in Python would force an engine rewrite at v2, breaking the
   "nothing is thrown away" property the whole sequence depends on.
4. **krew plugins are binaries**, should the CLI ever ship publicly.

**What made this affordable.** The §9a decision to shell out to `kubectl` removes most of the
Go learning curve — no client-go, no informers, no auth providers, no rest configs. v0 is
`os/exec` + `encoding/json` + pure rule functions + printing. No concurrency, no networking
beyond the subprocess.

**Middle path on types:** import `k8s.io/api/core/v1` for the *type definitions only*, and
unmarshal kubectl's JSON into real `v1.Pod` / `v1.Service` structs. Real field names and
autocomplete, none of client-go's auth machinery.

**Forward compatibility:** at v2 the exec-and-parse layer is replaced by client-go informers
and **the rule functions are untouched**, because §7 already defines them as pure
`state → findings`. That boundary is what makes the language choice safe this early.

**Accepted cost:** roughly a week of extra time if Go is new, mostly `go.mod` and the type
system rather than anything conceptual. Mitigation, and it is non-negotiable: **v0 scope stays
at four checks.** Learning a language while expanding scope is the documented way this
project fails.

---

## 10. Milestones

Scope discipline is the main risk to this project. Milestones are deliberately small.

| M | Deliverable | Done when |
|---|---|---|
| **M0** | Skeleton | Agent runs in kind with read-only RBAC, serves an empty findings API, UI renders "All clear" |
| **M1** | Three checks, verified | C1, C2, C3 implemented with verification + fixtures. **This is the first thing worth showing anyone.** |
| **M2** | The screen | Grouping, blast radius, severity ordering, per-finding evidence view |
| **M3** | No false positives | Suite 9.2 green |
| **M4** | Active probing | C4 — the flagship verification |
| **M5** | Stale config | C7, C8 — needs the change-history machinery |
| **M6** | Packaging | Helm chart, one-command install, README with screenshots |

C9 (OOMKilled) is intentionally unscheduled pending open question 4 — it is the only check
whose verification depends on an optional cluster component, and it ships with M4 or gets
dropped from v1 depending on that decision.

Ship M1–M3 before starting M4. A narrow tool that is never wrong beats a broad one that
cries wolf.

### Deferred (explicitly not in v1)

- MCP endpoint for AI agents (the kubescope work returns here as an *interface*, not the
  product)
- Headlamp plugin — Headlamp is the official Dashboard successor with only ~12 public
  plugins, all tiny and recent; a real land-grab window, but only after the engine is proven
- Go rewrite for ecosystem signal
- Multi-cluster
- Historical timeline / incident replay

---

## 11. Open questions

1. ~~**Name.**~~ **Decided 2026-09-14: `verdict`.** One word, it is what the tool emits, it
   ties to the existing verdict enum, and both `verdict` and `kubectl verdict` read well.
   Project directory is still `kubescope` and needs renaming.
2. ~~**Language.**~~ **Decided 2026-09-14: Go.** (Reversal — Python was chosen earlier the
   same day for speed, then reconsidered; see §9b.) UI language deferred to v1.
3. **Standalone UI vs Headlamp plugin** as the eventual primary surface.
4. ~~**Metrics API dependency** for C9.~~ **Resolved by §9a:** degrade gracefully — hide the
   check when `metrics-server` is absent rather than erroring or asserting.

---

## 12. Success criteria

- A Kubernetes user installs it in one command and immediately sees something true that they
  did not already know.
- The false-positive suite is green, and the README says so with numbers.
- The claim in §1 survives contact with a real cluster.
