# InfraGenie

**Codify your engineering standards once. Enforce them everywhere.**

InfraGenie is a Golden Path platform for platform, DevOps, and SRE teams. It lets you capture your organisation's infrastructure standards in a single `goldenpath.yml` file and then does two things with it:

1. **Reviews pull requests** — scans every diff against your standards and surfaces violations with LLM-grounded explanations and suggested fixes, not just rule IDs.
2. **Generates new services** — scaffolds new services that are correct from day one: right labels, right resource limits, right CI steps, right security posture.

---

## The problem

Every team ends up with an informal "the right way to deploy here" — but that knowledge lives in Slack threads, onboarding docs no one reads, and the heads of the engineers who have been there longest. New services drift. PRs slip through with missing probes, unpinned image tags, or no network policy. Runbooks describe what *was* true, not what *is*.

InfraGenie makes those standards explicit, version-controlled, and automatically enforced.

---

## How it works

Reviews run through three layers:

```
Diff
 │
 ├─► Layer 1 — Deterministic scanners (checkov, hadolint, gosec, …)
 │         High-confidence, T1 trust. No LLM cost.
 │
 ├─► Layer 2 — LLM grounding pass
 │         Takes Layer 1 findings and enriches them with repo-aware
 │         explanations and unified-diff fix suggestions. Cached.
 │         Token budget enforced per run.
 │
 └─► Layer 3 — Golden Path reviewers
           Opt-in LLM reviewers that check Golden Path conformance,
           reliability patterns, and naming conventions. T3 trust —
           confidence-gated and evidence-required before surfacing.
```

Every finding carries a **trust level** (T1/T2/T3), a **confidence score**, and an **evidence snippet** pointing to the exact line. High-noise findings are filtered before they ever reach you.

---

## Golden Path configuration

```yaml
# goldenpath.yml
version: 1
name: platform-baseline

required_labels:
  - app.kubernetes.io/name
  - app.kubernetes.io/version
  - team

security:
  forbid_image_tag_latest: true
  require_non_root: true
  require_read_only_rootfs: true
  require_network_policy: true

observability:
  require_prometheus_annotations: true

ci_required_steps:
  - name: trivy-scan
    matches: ["aquasec/trivy", "trivy image"]
  - name: unit-tests
    matches: ["go test", "pytest", "npm test"]

budget:
  tokens: 50000
  usd: 0.50
```

Teams that want to build on top of a shared baseline can use `extends:`:

```yaml
# goldenpath.yml (payments service)
version: 1
extends: ../platform/goldenpath.yml

security:
  require_network_policy: true   # stricter than baseline

ignore:
  - path: examples/**
    rules: ["*"]
```

---

## Quick start

```bash
# Review a local diff against your golden path
infragenie review --goldenpath goldenpath.yml --diff HEAD~1

# Review a GitHub PR
infragenie review --goldenpath goldenpath.yml --pr owner/repo#42

# Generate a new service from a template
infragenie generate --goldenpath goldenpath.yml --template go-service --name payments
```

> CLI surface is landing in Phase G. The engine, scanners, reviewers, grounding layer, and telemetry are complete and tested.

---

## Architecture

```
infragenie/
├── internal/
│   ├── engine/          # Orchestrates all three review layers
│   ├── goldenpath/      # goldenpath.yml loader, validation, extends resolution
│   ├── diff/            # Unified diff parser; local and GitHub PR sources
│   ├── repo/            # Repo context: language, platform, CI detection
│   ├── scanners/
│   │   ├── infra/       # checkov, hadolint adapters
│   │   └── lang/        # gosec adapter
│   ├── grounding/       # LLM grounding pass with sha256 cache
│   ├── reviewers/       # Golden Path, reliability, conventions reviewers
│   ├── llm/             # Multi-provider LLM client (Anthropic, OpenAI, Ollama, …)
│   ├── telemetry/       # Prometheus metrics, spend ledger, OTLP tracing, budget gate
│   └── eval/            # Eval harness with precision/recall gates
└── pkg/
    └── models/          # Shared types: Finding, Diff, GoldenPath, Severity
```

---

## Scanners

InfraGenie wraps best-of-breed scanners and adds the LLM grounding layer on top. It does not reinvent static analysis — it makes existing tools actionable.

| Scanner | What it covers |
|---------|---------------|
| checkov | Terraform, K8s manifests, Helm, Dockerfiles |
| hadolint | Dockerfile best practices |
| gosec | Go source security issues |

Scanners are stack-aware: if your repo has no Dockerfiles, hadolint is skipped automatically.

---

## Telemetry and FinOps

Every LLM call is recorded: provider, model, prompt tokens, completion tokens, USD estimate, cache hit, latency. The spend ledger is queryable at runtime and exportable as JSON.

A token/USD budget can be set per run. The engine performs a pre-flight reservation before each grounding call and halts mid-run if the budget is exhausted — no surprise bills.

Prometheus metrics are exposed on `/metrics`. A Grafana dashboard is included at `examples/grafana/dashboard.json`.

---

## Development

**Requirements:** Go 1.21+, and any scanners you want to use (`checkov`, `hadolint`, `gosec` on `$PATH`).

```bash
git clone https://github.com/basebandit/infragenie
cd infragenie
go build ./...
go test ./...
```

**Environment variables:**

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic API key for grounding/reviewers |
| `OPENAI_API_KEY` | OpenAI API key (alternative) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Enable OTLP trace export |

---

## Roadmap

| Phase | Status | Description |
|-------|--------|-------------|
| A | done | Core types, engine skeleton, golden path loader |
| B | done | Diff parser, local + GitHub PR sources, repo context |
| C | done | Scanner exec helpers, checkov/hadolint/gosec adapters |
| D | done | LLM grounder with cache, eval harness |
| E | done | Telemetry ledger, Prometheus metrics, OTLP tracer, budget gate |
| F | done | Layer-3 reviewers: golden path, reliability, conventions |
| G | in progress | CLI (cobra), reporters, MCP server, GitHub Action |
| H | planned | Community golden paths, goreleaser, signed releases |

---

## License

MIT
