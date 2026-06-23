# CLAUDE.md

Guidance for AI agents and humans working in this repo. Read before editing.

## What InfraGenie is

A Golden Path platform. A team codifies its infra standards once in
`goldenpath.yml`, then InfraGenie does two things with it over **one engine**:

- **review** — scan a diff/PR against the standard (deterministic scanners +
  LLM-grounded explanations + Golden Path reviewers).
- **generate** — scaffold new services that conform to the same standard by
  construction.

Three surfaces feed the one engine: **CLI**, **GitHub Action** (`deploy/action/`),
**MCP server** (`internal/mcp`). Designed for humans and AI agents equally — the
agentic-era bet is that agents both generate and get reviewed against the same
executable standard.

## Architecture

Review runs in three layers with an explicit **trust gradient**:

- **T1 — scanners** (`internal/scanners/{infra,lang}`): deterministic, always
  shown, count toward `fail_on`. Each adapter shells out to a binary and **skips
  cleanly** (structured warning) when the binary is absent.
- **T2 — grounding** (`internal/grounding`): per-finding LLM enrichment of T1.
  Advisory; never blocks alone. Cached.
- **T3 — reviewers** (`internal/reviewers`): Golden Path conformance, reliability,
  conventions. Confidence-gated (default ≥0.85) and evidence-required.

`internal/engine` orchestrates the layers, applies suppressions, and enforces
token/USD budgets (`internal/telemetry`).

## Package map

| Path | Role |
|---|---|
| `cmd/infragenie` | Cobra CLI: `init`, `generate`, `review`, `mcp`, `scanners`, `version` |
| `internal/engine` | Layer orchestration, trust gating, suppressions, budget |
| `internal/diff` | Unified diff: local git + GitHub PR sources |
| `internal/repo` | Repo context: language / platform / CI detection |
| `internal/scanners` | Layer-1 adapters + stack-aware selection |
| `internal/grounding` | Layer-2 LLM enrichment |
| `internal/reviewers` | Layer-3 deterministic Golden Path reviewers |
| `internal/goldenpath` | `goldenpath.yml` loader + `extends:` resolution |
| `internal/generate` | Deterministic service scaffolding from a golden path |
| `internal/llm` | Multi-provider client (Anthropic, OpenAI, Ollama/local) |
| `internal/telemetry` | Spend ledger, Prometheus, OTLP, budget counter |
| `internal/reporter` | Output: text / json / github annotations |
| `internal/mcp` | MCP server tools: `review_diff`, `review_pr`, `generate_service` |
| `internal/eval` | Fixture corpus + precision/recall gate |
| `pkg/models` | Shared types: `Finding`, `Diff`, `GoldenPath`, `Severity` |
| `pkg/config` | App config load (env > file > default) |

## Conventions

- Go 1.25. Tests use `testify` (`require`/`assert`).
- **LLM calls use native structured output. No regex extraction from markdown.**
- Reviewers are **deterministic** where possible; every finding carries
  `Confidence`, `Evidence`, `EvidenceLoc`, `TrustLevel`.
- Scanners must detect their binary and skip (not error) when missing.
- Generated artifacts must be **conformant by construction**: `generate` then
  `review` against the same golden path yields zero Golden Path findings. The
  contract is locked by `internal/generate/generate_test.go` — keep it green.

## The Golden Path reviewer contract (read before touching `internal/generate`)

`internal/reviewers/goldenpath.go` is deterministic and **per-file** string/YAML
matching. Generated manifests must therefore:

- carry every `required_labels` entry in `metadata.labels` (validated by single-doc
  YAML unmarshal — skipped on parse error);
- include `runAsNonRoot`/`runAsUser`, `readOnlyRootFilesystem: true`, Prometheus
  annotations, and a **co-located** `kind: NetworkPolicy` *in the Deployment file*
  (the NetworkPolicy check is per-file);
- avoid `:latest`; satisfy `reliability` (resource limits + requests, liveness +
  readiness probes, `replicas: 2+`) and `conventions` (`app.kubernetes.io/` labels,
  `runtime_rules` patterns → injected as annotations).

Helm chart metadata (`Chart.yaml`, `values*.yaml`) carries Helm's `apiVersion:`
but is not a K8s object, so `isK8sManifest` (in `goldenpath.go`) excludes it from
manifest checks. Two templates ship: `k8s-service` (plain manifests) and
`helm-service` (a Helm chart). Helm templates render with generate-time `[[ ]]`
delimiters so Helm's own `{{ }}` passes through — see `internal/generate/templates.go`.

## Build / test

```bash
make build        # -> bin/infragenie
make test         # go test -race ./...
make lint         # golangci-lint
go test ./internal/generate/...   # the generate→review conformance contract
```

The `eval` harness gates review precision/recall — don't regress it.

## How to extend

- **Scanner adapter** → `internal/scanners/{infra,lang}`, register in
  `cmd/infragenie/registry.go`. Detect-or-skip; map severity to `models.Severity`.
- **Layer-3 reviewer** → implement `reviewers.Reviewer`, add to the slice in
  `cmd/infragenie/review.go` and `internal/mcp/server.go`.
- **LLM provider** → `internal/llm`. Use native structured output.
- **Generate template** → add a set in `internal/generate/templates.go` + embed
  files under `internal/generate/templates/<name>/`; it must satisfy the reviewer
  contract above (assert it in `generate_test.go`).

## Commits

Conventional Commits, one-line subject. **No `Co-Authored-By` trailer.** Keep PRs
small and single-concern (≤10 files).

## Naming

Use `basebandit` as the placeholder org/repo in all examples and docs.
