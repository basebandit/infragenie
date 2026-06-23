---
name: extending-infragenie
description: Use when adding a scanner adapter, a Layer-3 reviewer, an LLM provider, an output format, or a generate template to InfraGenie — the four core extension points and where to wire each.
---

# Extending InfraGenie

InfraGenie is one engine (`internal/engine`) with pluggable parts. Adding capability means implementing an interface and registering it. Match surrounding code; keep findings deterministic where possible.

## Add a Layer-1 scanner adapter

1. New file in `internal/scanners/infra/` (IaC) or `internal/scanners/lang/` (language-aware).
2. Implement `scanners.Scanner` (`internal/scanners/scanner.go`): `Name()`, `Available()` (detect binary on `PATH`), `Scan(ctx, ScanInput) ([]models.Finding, error)`.
3. **Detect-or-skip:** `Available()` returns false when the binary is missing — the engine reports it as skipped, never errors.
4. Shell out via the helper in `internal/scanners/exec.go`; parse JSON; map the tool's severity to `models.Severity`; filter to diff-touched files.
5. Register in `cmd/infragenie/registry.go` (`registeredScanners()`), and it flows into `mcp` and `review` automatically via `allScanners()`.
6. Stack-awareness is handled by `scanners.Select` — declare relevance through repo/golden-path stacks, not bespoke gating.

The engine sets `Source=SourceScanner`, `TrustLevel="T1"`, `Confidence=1.0` for you.

## Add a Layer-3 reviewer

1. New file in `internal/reviewers/`. Implement `reviewers.Reviewer` (`reviewer.go`): `Name()`, `Review(ctx, ReviewInput) ([]models.Finding, error)`.
2. Prefer **deterministic** checks (string/YAML matching like `goldenpath.go`); set `Confidence`, `Evidence`, `EvidenceLoc`. The engine defaults `TrustLevel="T3"` — findings below `MinConfidence` (0.85) or (in strict mode) without evidence are filtered.
3. Register in BOTH `cmd/infragenie/review.go` and `internal/mcp/server.go` (`runEngine` reviewer slice) so CLI and MCP stay in lockstep.

## Add an LLM provider

1. Extend `internal/llm` (`client.go`/`providers.go`). Add the `llm.Provider` constant and client construction.
2. **Native structured output only** — no regex extraction from markdown. OpenAI `json_schema`; Anthropic tool use; Bedrock via the model's mechanism; Ollama `format: json`.
3. Surface config in `pkg/config/config.go` (`ProvidersConfig`) so `--provider` resolves keys/models/base-URL.

## Add an output format

1. Add a `reporter.Format` constant and a `write*` func in `internal/reporter/reporter.go`; switch in `Write`.
2. Keep exit-code policy in `reporter.ExitCode` (severity-gated).

## Add a generate template

See the `service-generation` skill. Add a set in `internal/generate/templates.go` + embedded files; it MUST satisfy the reviewer conformance contract and be asserted in `generate_test.go`.

## Always

- `make build && make test` green; don't regress the `internal/eval` precision/recall gate.
- Conventional one-line commits, no `Co-Authored-By` trailer. PRs single-concern, ≤10 files.
