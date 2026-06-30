# InfraGenie

**Codify your engineering standards once. Enforce them everywhere.**

InfraGenie turns your team's infrastructure standards into a single executable
file, `goldenpath.yml`, and uses it two ways:

- **Review** changes (local diffs or GitHub PRs) against the standard, with
  scanner findings enriched by repo-aware explanations and suggested fixes.
- **Generate** new services that conform to the standard from the first commit.

One engine, several surfaces: CLI, GitHub Action, and an MCP server for AI agents.

## Why

Every team has an unwritten "right way to deploy here" scattered across Slack,
stale docs, and senior engineers' memory. Services drift; PRs ship without probes,
with unpinned images, or no NetworkPolicy. AI coding agents make that drift faster
and harder to police. InfraGenie makes the standard explicit, version-controlled,
and enforced, so humans and agents follow the same paved road.

## Install

Pre-built binaries (Linux, macOS, Windows; amd64/arm64) are on the
[releases page](https://github.com/basebandit/infragenie/releases), each with
checksums, cosign signatures, and an SBOM.

```bash
go install github.com/basebandit/infragenie/cmd/infragenie@latest
```

Review shells out to scanners when they are present and skips them otherwise, so
none are required to start. The GitHub Action bundles them all.

## Quick start

```bash
# 1. Create a goldenpath.yml for this repo (auto-detects the stack)
infragenie init

# 2. Scaffold a service that conforms to it
infragenie generate service payments-api

# 3. Review a change against it
infragenie review --goldenpath goldenpath.yml --diff HEAD~1
infragenie review --goldenpath goldenpath.yml --pr owner/repo#42 --comment
```

Run `infragenie <command> --help` for the full flag set. Onboarding an existing
service? See [docs/onboarding/](docs/onboarding/) for a step-by-step guide (any
language, with or without an LLM key).

## goldenpath.yml

```yaml
version: 1
name: platform-baseline

required_labels: [app.kubernetes.io/name, team]

security:
  forbid_image_tag_latest: true
  require_non_root: true
  require_read_only_rootfs: true
  require_network_policy: true
  forbid_privileged: true

observability:
  require_prometheus_annotations: true

ci_required_steps:
  - name: tests
    matches: ["go test", "pytest", "npm test"]
```

Service repos extend a shared baseline, from a local path or a remote ref:

```yaml
version: 1
extends: github.com/basebandit/platform/goldenpath.yml@v1
ignore:
  - path: examples/**
    rules: ["*"]
```

## Surfaces

### CLI

Commands: `init`, `generate`, `review`, `scanners`, `mcp`, `serve`.
Output formats: `text`, `json`, `github` (PR annotations), `sarif` (code scanning).
`review --comment` posts and updates a single summary comment on the PR.

### GitHub Action

```yaml
- uses: actions/checkout@v4
- uses: basebandit/infragenie@v0.2.0
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  with:
    goldenpath: goldenpath.yml
    fail-on: high
    comment: "true"   # needs pull-requests: write
```

The image bundles `infragenie` and every scanner. A complete workflow is in
`examples/github-action/infragenie-review.yml`.

### MCP (AI agents)

```json
{ "mcpServers": { "infragenie": { "command": "infragenie", "args": ["mcp"] } } }
```

Tools: `review_diff`, `review_pr`, `generate_service`. An agent can scaffold a
conformant service and review its own changes against the same standard.

### Webhook server

`infragenie serve` receives GitHub `pull_request` webhooks, reviews each PR, and
posts the summary comment. Configure `WEBHOOK_SECRET` and `GITHUB_TOKEN`.

## How review works

Three layers, each finding tagged with a trust level:

- **T1, scanners** (deterministic): wrap best-of-breed tools; no LLM cost.
- **T2, grounding** (optional LLM): repo-aware explanation and suggested fix per
  finding, under a per-run token/USD budget.
- **T3, reviewers** (deterministic): Golden Path conformance, reliability, and
  conventions. Confidence-gated and evidence-required before surfacing.

Helm charts and Kustomize overlays are rendered before review, so templated
manifests are checked as real objects rather than skipped.

## Generate

`generate` renders a new service deterministically from the resolved
`goldenpath.yml`, so labels, securityContext, NetworkPolicy, Prometheus
annotations, resource limits, probes, and CI steps are correct by construction.
Generate then review yields zero Golden Path findings. Templates: `k8s-service`
(plain manifests) and `helm-service` (Helm chart).

## Scanners

| Scanner | Covers |
|---|---|
| checkov | Terraform, Kubernetes, Helm, Dockerfiles |
| trivy (config) | IaC misconfiguration across the above |
| kube-score | Kubernetes manifest reliability and security |
| kubeconform | Kubernetes schema validation |
| hadolint | Dockerfile best practices |
| tflint | Terraform linting |
| gosec | Go security issues |
| govulncheck | Go known vulnerabilities (called symbols) |
| semgrep | Multi-language SAST |

Stack-aware: only scanners relevant to the changed files run, and missing binaries
are skipped with a note rather than failing the run.

## Telemetry

Every LLM call is metered (provider, model, tokens, USD estimate, latency, cache
hit). Per-run token and USD budgets halt grounding before overspend. Prometheus
metrics are exported, with an example Grafana dashboard at
`examples/grafana/dashboard.json`; OTLP tracing is enabled when
`OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Development

Requires Go 1.25.

```bash
make build   # -> bin/infragenie
make test    # go test -race ./...
make lint
```

Architecture and contributor notes live in `CLAUDE.md`.

## License

MIT
