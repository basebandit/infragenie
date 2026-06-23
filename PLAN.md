# InfraGenie v0 Plan

**Golden Path platform for engineering teams.** A platform team encodes their internal standard once in a `goldenpath.yml` (chart shape, required labels, CI steps, security posture, observability defaults, reference services, stacks/languages covered). InfraGenie does two things with it:

1. **Generates** new services that conform to the Golden Path.
2. **Reviews** PRs against the same standard, with LLM-grounded explanations and suggested fixes.

One engine, multiple surfaces (CLI, GitHub Action, MCP server). CLI-first so solo engineers and platform teams alike can adopt in minutes.

**Polyglot by design.** Center of gravity is infrastructure code review (Helm, Kubernetes, Terraform, Dockerfile, CI workflows) — language-agnostic. Stack profiles and language-aware scanner adapters extend coverage to Go, Python, Node, Java, Rust, and beyond. Service templates are community-driven and span any common runtime.

**Solo-first via community Golden Paths.** A solo engineer doesn't need to author their own `goldenpath.yml` to get value. They `extends:` a curated community Golden Path (`infragenie-community/go-grpc`, `python-fastapi`, `node-express`, `rust-axum`, `java-spring`) and immediately get scaffolding + ongoing enforcement. As they grow into a team, they fork it and own it. Solo and org adoption flow through the same surface.

## The problem (agentic-coding era)

The standard "the right way to deploy here" lives in Slack threads, stale onboarding docs, and senior engineers' heads. Services drift. PRs ship without probes, with unpinned tags, with no NetworkPolicy.

AI coding agents make this acute. Agents now produce services, manifests, and pipelines faster than humans can review them line by line. Without an **executable** standard, agent output drifts from org norms exactly like human output — only at machine speed and volume. The reviewer becomes the bottleneck, or governance quietly erodes.

InfraGenie's bet: give humans and agents the **same paved road**. They **generate** scaffolds that conform by construction and **review** changes against the identical `goldenpath.yml`, through the identical engine, over CLI / GitHub Action / MCP. The standard is code; conformance is automated; velocity rises without governance falling. This is the real problem the tool exists to solve.

## Design principles

Mapped to how the engine already works — principles, not aspirations.

1. **Developer AND agent first.** Every surface (CLI, Action, MCP) feeds one engine with structured, machine-readable I/O. An agent calling `generate_service`/`review_*` over MCP has the same power as a human at the CLI. No tribal knowledge required.
2. **Paved road.** `generate` produces correct-by-construction services; `review` keeps them correct. The easy path is the correct path.
3. **Guardrails over gates.** Findings carry trust levels (T1–T3), confidence, and evidence. Low-trust findings are filtered, not forced; `fail_on` is opt-in per severity. The tool guides; it rarely blocks blindly.
4. **Platform as product.** Versioned `goldenpath.yml` schema, `extends:` for reuse, a cold-start ladder for adoption, FinOps telemetry for cost transparency, and a community Golden Path set. Success is measured by adoption and time-to-conformant-service, not feature count.

## Personas

| Persona | Drives it via | Core loop |
|---|---|---|
| Platform/SRE engineer | CLI + canonical `goldenpath.yml` | author standard → enforce in CI |
| Solo / small-team engineer | CLI + `extends:` community path | adopt path → generate → review |
| **AI agent** | MCP (`generate_service`, `review_*`) | generate conformant service → self-review → iterate |

All three converge on the same engine and the same standard.

## Goal

**A platform engineer at a mid-to-large org can:**
1. Author a `goldenpath.yml` from scratch, from a wizard, from intent (LLM-drafted), or by distilling existing services.
2. Run `infragenie generate service my-api` and get a PR with a chart + CI + observability config that matches.
3. Run `infragenie review` (locally or in CI) on any infra PR and get scanner findings enriched with grounded explanations and diff-shaped suggestions.
4. Inspect cost and usage of every LLM call via Prometheus / OTLP.
5. Drive the same engine from any MCP-aware agent (Claude Code, Cursor, etc.).

**A solo engineer can:**
1. Run `infragenie init --extends community/go-grpc` in a new repo and get a working override file that pins a community Golden Path.
2. Run `infragenie generate service my-api --template community/go-grpc` and get a sane chart + CI + Dockerfile + telemetry baseline.
3. Run `infragenie review` and get the same grounded findings the org user gets.
4. Outgrow the community path later: fork it, customize it, and migrate to canonical mode without changing tools.

## Non-goals (v0)

- No auto-apply to clusters. No `kubectl apply`. No gateway admin calls.
- No Postgres, Redis, NATS, worker queue, or HTTP API server.
- No web UI, dashboard, or hosted SaaS.
- No multi-repo conventions index.
- No GitHub App webhook server. v0 talks to GitHub via personal token when `--pr` is used.
- No replacement for Backstage. Golden Paths sit alongside an IDP catalog, they don't replace one.
- No replacement for Checkov / kube-score / etc. We *wrap* them.
- No full application SAST. We wrap Semgrep / Snyk / equivalents when present, but we don't compete with them on application-code analysis.
- No language style/lint (`gofmt`, `black`, `prettier`). Those are pre-commit territory.
- No performance profiling, dead-code analysis, or coverage tooling. Out of scope.

## Architecture

```
                  ┌────────────────────────────────────────┐
   git diff ──┐   │              Engine                    │
   PR diff  ──┼──►│                                        │
   repo idx ──┘   │  Layer 1: Scanners (deterministic)     │
                  │   checkov, kube-score, kubeconform,    │
                  │   tflint, hadolint, trivy config       │
                  │                                        │
                  │  Layer 2: LLM grounding pass           │
                  │   per finding → repo-aware explanation │
                  │   + suggested fix as diff              │
                  │                                        │
                  │  Layer 3: LLM-only reviewers (opt-in)  │
                  │   GoldenPath conformance,              │
                  │   reliability heuristics,              │
                  │   conventions drift                    │
                  └────────────────────────────────────────┘
                           │                │
                           ▼                ▼
                       []Finding       Telemetry
                           │           (tokens, cost,
                           ▼            traces, metrics)
                  ┌────────────────┐
                  │   Reporter     │ → stdout / md / GitHub PR / MCP
                  └────────────────┘
```

The engine is the only thing that knows how to review. CLI, GitHub Action, and the MCP server are surfaces that feed it input and render its output. Nothing else.

## Three review layers — discipline, not features

- **Layer 1 is the cost ceiling.** Free, fast, deterministic. Scanner findings ground the rest.
- **Layer 2 is the value-add.** For each Layer 1 finding, an LLM call produces an explanation grounded in the actual repo and a suggested fix as a diff. Token spend scales with finding count, not file count — predictable.
- **Layer 3 is opt-in.** For things rules can't catch: Golden Path conformance ("this Deployment lacks the labels declared in `goldenpath.yml`"), conventions drift, intent mismatches. Off by default. Cost bounded by user opt-in via `--deep` or per-reviewer flags.

This makes us complementary to the existing scanner ecosystem and keeps cost-per-PR in cents for typical diffs.

## Target package layout

```
infragenie/
├── cmd/infragenie/        # CLI entrypoint (cobra)
├── internal/
│   ├── diff/              # local git diff + GitHub PR diffs → unified Diff
│   ├── repo/              # walk repo, identify file types + languages,
│   │                      #   build context (manifest detection: go.mod,
│   │                      #   package.json, pyproject.toml, Cargo.toml, etc.)
│   ├── scanners/
│   │   ├── infra/         # IaC scanners: checkov, kube-score, kubeconform,
│   │   │                  #   tflint, hadolint, trivy config
│   │   └── lang/          # language-aware adapters (selective):
│   │                      #   gosec, govulncheck (Go);
│   │                      #   bandit, pip-audit (Python);
│   │                      #   npm audit, eslint-security (Node);
│   │                      #   semgrep (multi-lang); cargo audit (Rust);
│   │                      #   owasp dep-check (Java).
│   │                      #   Each shells out, parses JSON, returns []Finding
│   ├── grounding/         # Layer 2: per-finding LLM enrichment
│   ├── reviewers/         # Layer 3: LLM-only reviewers
│   │   ├── reviewer.go    # interface + shared helpers
│   │   ├── goldenpath.go  # conformance against goldenpath.yml
│   │   ├── reliability.go
│   │   └── conventions.go
│   ├── engine/            # orchestrates layers, dedupes findings, applies
│   │                      # suppressions, enforces token/cost budgets
│   ├── llm/               # KEEP: provider abstraction
│   │                      #   + openai.go, anthropic.go, ollama.go, bedrock.go
│   ├── telemetry/         # token + cost ledger, OTLP exporter, prom metrics
│   ├── mcp/               # MCP server (Model Context Protocol)
│   ├── generate/          # `infragenie generate` — render goldenpath templates
│   ├── goldenpath/        # goldenpath.yml loader + validator + migrate
│                          #   handles canonical mode and `extends:` override mode
│   ├── eval/              # fixture corpus + precision/recall harness for L3
│   └── report/            # stdout / markdown / github-comment / mcp renderers
├── pkg/models/
│   ├── finding.go         # universal output type
│   ├── diff.go
│   └── goldenpath.go
├── deploy/
│   └── action/            # GitHub Action wrapper (Dockerfile + action.yml)
├── examples/
│   └── goldenpath/        # reference goldenpath.yml + templates
└── PLAN.md
```

Lambda + Helm + EKS deploys are sketched in `## Deferred (v1+)` below. Not in v0.

## Core types

```go
// pkg/models/finding.go
type Severity string
const (
    SeverityCritical Severity = "critical"  // exit 2
    SeverityHigh     Severity = "high"      // exit 1 by default
    SeverityMedium   Severity = "medium"
    SeverityLow      Severity = "low"
    SeverityInfo     Severity = "info"
)

type Source string
const (
    SourceScanner    Source = "scanner"     // Layer 1
    SourceGrounding  Source = "grounding"   // Layer 2 enrichment of a scanner finding
    SourceReviewer   Source = "reviewer"    // Layer 3
)

type Finding struct {
    RuleID      string   `json:"rule_id"`
    Source      Source   `json:"source"`
    Origin      string   `json:"origin"`
    Severity    Severity `json:"severity"`
    File        string   `json:"file"`
    Line        int      `json:"line"`
    Title       string   `json:"title"`
    Explanation string   `json:"explanation"`
    Suggestion  string   `json:"suggestion"`
    References  []string `json:"references,omitempty"`

    // Trust gating (Layer 3 must populate; Layer 1 sets Confidence=1.0)
    Confidence  float64  `json:"confidence"`            // 0.0–1.0
    Evidence    string   `json:"evidence,omitempty"`    // literal quote from source
    EvidenceLoc string   `json:"evidence_loc,omitempty"`// file:line of the quote
    TrustLevel  string   `json:"trust_level"`           // T1 | T2 | T3
}

// pkg/models/goldenpath.go
type GoldenPath struct {
    Version       int                  `yaml:"version"`
    Name          string               `yaml:"name"`
    RequiredLabels []string            `yaml:"required_labels"`
    ChartShape    ChartShape           `yaml:"chart_shape"`
    CIRequired    []string             `yaml:"ci_required_steps"`
    Security      SecurityStandard     `yaml:"security"`
    Observability ObservabilityDefaults `yaml:"observability"`
    Templates     map[string]string    `yaml:"templates"`     // service-type → template path
    References    []string             `yaml:"reference_services"`
}
```

```go
// internal/reviewers/reviewer.go
type ReviewInput struct {
    Diff       *models.Diff
    RepoCtx    *repo.Context
    GoldenPath *models.GoldenPath  // optional
    Config     *config.Config
}

type Reviewer interface {
    Name() string
    Review(ctx context.Context, in ReviewInput) ([]models.Finding, error)
}
```

## v0 review layers in detail

### Layer 1 — Scanners (in `internal/scanners/`)
One adapter per scanner. Each:
- Detects the binary on `PATH`; skips with a structured warning if missing.
- Shells out, reads JSON output, maps to `Finding{Source: SourceScanner, Origin: "<scanner>"}`.
- Maps the scanner's severity to ours.
- Filters to files touched by the diff.

**Infra scanners shipped in v0** (in `internal/scanners/infra/`): `checkov`, `kube-score`, `kubeconform`, `tflint`, `hadolint`, `trivy config`.

**Language-aware scanners shipped in v0** (in `internal/scanners/lang/`): `gosec`, `govulncheck` (Go); `bandit`, `pip-audit` (Python); `npm audit` (Node); `semgrep` (multi-language SAST — broad coverage); `cargo audit` (Rust). All optional — engine reports which scanners ran and which were skipped (binary missing, stack not detected, or disabled in `goldenpath.yml`). The line we hold: language-aware *infra checks* (lockfiles, vuln scans in CI, Dockerfile per-runtime hygiene), not full application SAST.

**Stack-driven activation.** Scanners only run when relevant. `goldenpath.yml` declares `stacks:`; the engine cross-references with auto-detected languages from manifest files (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `Gemfile`, `composer.json`, `*.csproj`) and runs the intersection.

### Layer 2 — Grounding (in `internal/grounding/`)
For each Layer 1 finding:
- Build a small prompt: the finding + the file's full new contents + relevant sibling context (e.g. nearest `Chart.yaml`).
- Call LLM with **structured output** — schema is `{explanation: string, suggestion: string}`.
- Attach the result to the finding, set `Source = SourceGrounding`.
- Cache key: `sha256(rule_id, file_content_hash, model, schema_version)`. Repeat PRs hit cache.

Cap at N concurrent calls (configurable, default 4) and an overall token budget (configurable, default 100k input).

### Layer 3 — LLM reviewers (in `internal/reviewers/`)
- **`goldenpath.go`** — checks the diff against `goldenpath.yml`. Emits findings for missing labels, chart-shape divergence, missing CI steps, security/observability gaps. **This is the headline feature.**
- **`reliability.go`** — heuristics rules can't catch: weak probes, missing PDB on multi-replica deployments, single-replica prod-tier services.
- **`conventions.go`** — drift from sibling services. Off by default until v1+ confidence.

Each reviewer is one structured-output LLM call per logical chunk. JSON schema enforced server-side.

## Trust gradient

Every finding carries an explicit trust level. Output format and exit-code policy honor it. We never automate above the level the user opted into.

| Level | Source | Behavior |
|---|---|---|
| **T1** | Layer 1 scanners | Deterministic. Always shown. Counts toward `fail_on`. |
| **T2** | Layer 2 grounding | Advisory enrichment of T1. Never blocks alone. |
| **T3** | Layer 3 reviewers | Confidence-gated (default ≥0.85), evidence-required. Below threshold: logged, not surfaced. |
| **T4** | reserved | Future auto-fix / auto-PR. Off in v0. |

`--strict-evidence` (default on for `--pr` mode) drops any T3 finding without `Evidence` + `EvidenceLoc`.

## Cost discipline

- Default models: `gpt-4o-mini` / `claude-3-5-haiku` / `llama3.1:8b`. Frontier models opt-in via `goldenpath.yml`.
- Layer 2 grounding only runs on findings ≥ `min_ground_severity` (default `medium`); below that, sampled at 1-in-N (default 3).
- Per-PR cost printed at end of every run; structured cost line in JSON output for CI ingestion.
- Pre-flight estimate halts the run if it would exceed `--budget-usd` / `--budget-tokens`.
- Local mode defaults to Ollama when no cloud key is set.

## Feedback loop

- `infragenie review --feedback` walks findings interactively; user marks accept / suppress / false-positive with reason.
- Suppressions write to `goldenpath.yml` under `ignore:` with the reason as a comment.
- PR-mode comments include 👍/👎 reaction reasons that update the same suppression list on next run.
- Suppression reasons are evaluation data for tuning Layer 3 prompts and rule catalogs.

## Reviewer prompt skeleton

Same shape across all Layer 3 reviewers. Differences live in the rule catalog and few-shot examples in code, not in the prompt template.

```
SYSTEM: You are a {role} reviewer for IaC changes. You produce findings as a JSON
array matching the provided schema. You only flag issues you can localize to a
specific file and line. You never invent files, rules, or evidence.

USER:
## Repo summary
{repo_ctx.summary}

## Golden Path
{goldenpath_yaml or "(none configured)"}

## Diff
{unified_diff}

## Full new contents of touched files
{file_contents}

## Rules in scope
{rule_catalog}

Return findings as JSON: {schema}
```

## Cold-start ladder

The biggest adoption risk is users without an existing Golden Path *and* without a corpus of services to distill one from. Five rungs cover every cold-start case, ordered from "no taste, no repos" to "have repos, want to distill":

| Rung | Command | When to use |
|---|---|---|
| 1. **Community starter** | `infragenie goldenpath init --from community/<stack>` | First repo, adopt curated defaults wholesale |
| 2. **Interactive wizard** | `infragenie goldenpath init --interactive` | First repo, opinions but unsure how to express them |
| 3. **Describe-it** | `infragenie goldenpath init --describe "Go services on EKS, mTLS via Istio, Prom metrics"` | First repo, clear intent — LLM drafts a `goldenpath.yml` |
| 4. **Bootstrap from repos** | `infragenie goldenpath bootstrap <glob>` | Existing services to distill conventions from |
| 5. **Extends a community path** | `extends: github.com/infragenie-community/<path>` in a service repo's `goldenpath.yml` | Solo dev or small team that wants someone else's Golden Path with light overrides |

**Community Golden Paths** live in a sibling repo (`infragenie-community/golden-paths`) and are versioned. v0 ships with a small curated set: `go-grpc`, `python-fastapi`, `node-express`, `rust-axum`, `java-spring`. The set grows by community contribution. The `bootstrap` command can also output to that repo for upstreaming back.

The community-starter rung doubles as the **solo-dev gateway**: solo devs adopt a community Golden Path with one command and immediately get scaffolding + ongoing enforcement. As they grow into a team, they fork the community path and end up with their own canonical `goldenpath.yml`. Solo and org adoption converge on the same surface.

## `goldenpath.yml` schema

One filename, two modes. Modeled on `tsconfig.json` / `.eslintrc` / `Cargo.toml`: same name everywhere, the schema differentiates based on whether `extends:` is set.

- **Canonical mode** — lives in a platform-team-owned repo. Declares the standard.
- **Override mode** — lives in each service repo. Has `extends:` pointing at the canonical, plus per-repo suppressions, severity overrides, and budgets.

The loader resolves `extends:` at parse time (Git URL with optional `@<ref>`, or a local path). Service-repo files only specify what differs.

### Canonical mode (platform-team repo)

```yaml
# goldenpath.yml — canonical Golden Path
version: 1
name: basebandit-service-v2

# Stacks / runtimes / platforms this Golden Path covers.
# Engine activates only relevant rules and scanners based on this + auto-detection.
stacks:
  runtimes: [go, python, node]      # languages
  platforms: [helm, kubernetes]     # deployment platforms
  cloud: [aws]                      # cloud targets
  ci: [github-actions]              # CI systems

required_labels:
  - app
  - team
  - env
  - cost-center
  - data-classification

chart_shape:
  charts_dir: charts/
  required_files:
    - Chart.yaml
    - values.yaml
    - templates/deployment.yaml
    - templates/service.yaml
    - templates/networkpolicy.yaml

ci_required_steps:
  - name: lint
    matches: ["helm lint", "yamllint"]
  - name: scan
    matches: ["trivy", "checkov"]
  - name: test
    matches: ["go test", "pytest", "jest"]

# Per-runtime infra checks (language-aware, not SAST).
runtime_rules:
  go:
    require_govulncheck_in_ci: true
    require_go_mod_pinned: true
  python:
    require_lockfile: true            # poetry.lock or hashed requirements.txt
    require_pip_audit_in_ci: true
  node:
    require_lockfile: true            # package-lock.json or pnpm-lock.yaml
    require_npm_audit_in_ci: true

security:
  require_network_policy: true
  require_non_root: true
  require_read_only_rootfs: true
  forbid_image_tag_latest: true

observability:
  require_prometheus_annotations: true
  require_structured_logs: true

# Service generation templates — used by `infragenie generate`.
# Local paths or community refs (github.com/infragenie-community/<name>@<ref>).
templates:
  go-grpc:        github.com/infragenie-community/go-grpc@v1
  python-fastapi: github.com/infragenie-community/python-fastapi@v1
  node-express:   github.com/infragenie-community/node-express@v1
  rust-axum:      github.com/infragenie-community/rust-axum@v1
  java-spring:    github.com/infragenie-community/java-spring@v1

reference_services:
  - charts/payments-api  # the canonical "good" example

# Engine defaults — service repos can override these
defaults:
  reviewers:
    scanners: true       # Layer 1
    grounding: true      # Layer 2
    goldenpath: true     # Layer 3
    reliability: true    # Layer 3
    conventions: false   # Layer 3 — opt-in
  fail_on: high
  budget:
    tokens: 200000
    usd: 0.50
```

### Override mode (service repo)

```yaml
# goldenpath.yml — service overrides. Extends the platform Golden Path.
# See <extends ref> for the canonical definition.
extends: github.com/basebandit/platform/goldenpath.yml@v2

# Optional engine override — defaults inherited from canonical
fail_on: high

# Per-rule overrides
rules:
  checkov.CKV_K8S_10: { severity: medium }
  goldenpath.required-label.cost-center: { enabled: false }   # per-repo exception

# Path-scoped suppressions
ignore:
  - path: "examples/**"
    rules: ["*"]
```

`infragenie goldenpath validate` reports which mode it parsed and surfaces the resolved (post-extends) shape.

### Versioning

- `goldenpath.yml` declares `version:` (SemVer). `extends:` should pin an immutable ref (`@v2.1.0`), not `@main`.
- `infragenie goldenpath migrate` bumps `extends:` refs across a repo, shows a diff preview, and lists newly-enforced rules so platform teams can roll forward intentionally.
- Breaking schema changes increment major version; loader rejects unknown major with a clear error.

## CLI surface

```
infragenie review [path]              # review staged + unstaged
infragenie review --staged
infragenie review --base main         # HEAD vs base branch
infragenie review --pr <url|number>   # GitHub PR via $GITHUB_TOKEN
infragenie review --format pretty|md|json|sarif
infragenie review --layer 1,2,3       # selective layers
infragenie review --deep              # enable Layer 3
infragenie review --budget-tokens 100000 --budget-usd 0.50
infragenie review --strict-evidence              # drop T3 findings without evidence
infragenie review --scanners auto|bundled|none   # scanner sourcing strategy
infragenie review --feedback                     # interactive accept/suppress
infragenie review --min-confidence 0.9           # tighter T3 gate

infragenie goldenpath migrate                    # bump extends refs with diff preview

infragenie generate service <name> --template go-grpc --path services/
                                      # render Golden Path templates → PR

infragenie mcp serve                  # run as MCP server on stdio (default)
                                      #   tools: review, explain_finding,
                                      #          suggest_fix, generate_service
infragenie mcp serve --http :7000     # MCP over HTTP (for remote agents)

# Cold-start ladder — get a goldenpath.yml from any starting point
infragenie goldenpath init --from community/go-grpc
infragenie goldenpath init --interactive
infragenie goldenpath init --describe "Go gRPC services on EKS, mTLS via Istio, Prom metrics"
infragenie goldenpath bootstrap "services/*" --output goldenpath.yml
infragenie goldenpath validate        # lint + show resolved (post-extends) shape

infragenie init                       # service-repo init: writes a `goldenpath.yml` in
                                      #   override mode that `extends:` a community path
infragenie version
```

Exit codes:
- `0` — no findings ≥ `fail_on`
- `1` — findings ≥ `fail_on`
- `2` — `critical` findings (always)
- `>2` — engine error

## MCP server (`internal/mcp/`)

Exposes the engine as an MCP server. Tools:

| Tool | Input | Output |
|---|---|---|
| `review` | `{path?, base?, pr?, layers?}` | `{findings: Finding[], cost: CostSummary}` |
| `explain_finding` | `{rule_id, file, line}` | `{explanation, suggestion}` |
| `suggest_fix` | `{finding}` | `{diff: string}` |
| `generate_service` | `{name, template, path?}` | `{files_created: string[], pr_url?}` |
| `validate_goldenpath` | `{path}` | `{ok, errors[]}` |

Transport: stdio by default (Claude Code, Cursor); optional HTTP for remote agents.

This is the JD-credibility lever: small surface, big signal. Implementation is `mcp-go` or hand-rolled JSON-RPC — the protocol is small.

## Telemetry / FinOps (`internal/telemetry/`)

Every LLM call goes through a metered client wrapper. Records:
- `provider`, `model`, `prompt_tokens`, `completion_tokens`, `usd_estimate`
- `repo`, `pr`, `team` (from env / config)
- `reviewer`, `rule_id`, `cache_hit`
- Latency, error class

Surfaces:
- **Local CLI:** human summary at end of run (`Spent $0.07, 14k tokens, 6 cache hits`).
- **JSON output:** full ledger entries for CI ingestion.
- **Prometheus** (when `--metrics-addr` set or running as MCP server): counters + histograms with the dimensions above.
- **OTLP traces** (when `OTEL_EXPORTER_OTLP_ENDPOINT` set): one span per LLM call, parented to the review run.

Reference Grafana dashboard JSON shipped under `examples/grafana/`.

Budget enforcement: pre-flight estimate + mid-run halt when crossing `--budget-usd` or `--budget-tokens`.

## LLM strategy

- Provider abstraction in `internal/llm/`: OpenAI, Anthropic, AWS Bedrock, Ollama (via OpenAI-compatible endpoint).
- All calls use **native structured output**:
  - OpenAI: `response_format: json_schema`.
  - Anthropic: tool use with a `record_findings` tool whose input is the `Finding[]` schema.
  - Bedrock: routed to the underlying model's structured-output mechanism (Claude on Bedrock = tool use).
  - Ollama: `format: json` + schema in prompt (best-effort; flag in docs).
- No regex extraction from markdown. Anywhere.
- Default models: `gpt-4o-mini` cloud, `claude-3-5-haiku-20241022` cloud-alt, `llama3.1:8b` Ollama. Override via `goldenpath.yml`.
- Bedrock auth via standard AWS SDK chain (IAM role, profile, env). Ship credibility for the AWS-native deployment story.

## Pruned dependencies

**Keep direct:**
- `github.com/spf13/cobra`
- `github.com/sashabaranov/go-openai` (OpenAI + Ollama)
- `github.com/google/uuid`
- `github.com/stretchr/testify`
- `gopkg.in/yaml.v3` (promote from indirect)

**Drop direct (v0):**
- `github.com/gin-gonic/gin`
- `github.com/go-redis/redis/v8`
- `gorm.io/gorm`, `gorm.io/driver/postgres`
- `k8s.io/apimachinery`, `k8s.io/client-go` (use `sigs.k8s.io/yaml` only, indirect)

**Add direct:**
- `github.com/google/go-github/v66` — GitHub PR fetch (only when `--pr` used).
- `github.com/anthropics/anthropic-sdk-go` — Anthropic provider with native tool use.
- `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` — Bedrock provider.
- `github.com/prometheus/client_golang` — telemetry metrics.
- `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk` (+ OTLP exporter) — tracing.
- `github.com/mark3labs/mcp-go` (or equivalent) — MCP server.

## File-level cut/keep list

**Cut entirely (v0):**
- `internal/database/postgres.go` + test
- `internal/database/redis.go` + test
- `internal/orchestrator/events.go`, `queue.go`, `manager.go` + test
- `internal/kong/admin.go` + test  *(gateway becomes a Layer-3 rule, not a client)*
- `internal/kubernetes/client.go` apply paths + test
- `internal/api/handlers.go`, `routes.go`, `middleware/`
- `internal/agents/deployment.go`, `optimization.go` + tests
- `cmd/server/`, `cmd/agent/`
- All root planning markdown except this file and `README.md`

**Rewrite/rename:**
- `internal/agents/base.go` → `internal/reviewers/reviewer.go`
- `internal/agents/security.go` → fold rules into `internal/scanners/` (delegated to `checkov`/`trivy`) + a thin Layer-3 reviewer if needed
- `internal/agents/monitoring.go` → fold useful prompts into `reliability.go`
- `internal/agents/infrastructure.go` → split: prompts → reviewers; deploy paths deleted
- `internal/agents/documentation.go` → defer to v1+
- `internal/llm/prompts.go` → rewrite around the layered architecture

**Keep mostly as-is:**
- `internal/llm/client.go`, `providers.go` and integration tests (extend with Bedrock + Anthropic native).
- `pkg/utils/logger.go`.
- `pkg/config/config.go` — adapt.

## v0 milestone checklist

**Status: v0 shipped.** Phases A–H are complete (review engine, scanners, grounding, telemetry, Layer-3 reviewers, surfaces, releases). Phase I closed the two remaining promises — `generate` service scaffolding (CLI + MCP `generate_service`) and the Docker-based GitHub Action with a signed GHCR image. Remaining work is the **Vision / roadmap** below, not v0.

Order matters: cut first, then build on cleared ground. Three demonstrable wins drive the order: **Golden Paths**, **FinOps/telemetry**, **MCP server**.

### Phase A — Cut
- [ ] Branch `v0-refactor`.
- [ ] Delete files in the "Cut entirely" list.
- [ ] Drop deps from `go.mod`; `go mod tidy`; `go build ./...` clean.

### Phase B — Core types + engine skeleton
- [ ] `pkg/models/{finding,diff,goldenpath}.go`.
- [ ] `internal/reviewers/reviewer.go`.
- [ ] `internal/engine/engine.go` — runs layers, merges findings, applies suppressions, enforces budgets.
- [ ] `internal/goldenpath/loader.go` — handles canonical mode and `extends:` resolution.

### Phase C — Diff + repo context
- [ ] `internal/diff/local.go` — `git diff` parser.
- [ ] `internal/diff/github.go` — PR fetch via `go-github`.
- [ ] `internal/repo/context.go` — file tree, language detection (manifest files), detected stacks, conventions hints.

### Phase D — Layer 1 + Layer 2
- [ ] `internal/scanners/infra/{checkov,kube-score,kubeconform,tflint,hadolint,trivy}.go` adapters.
- [ ] `internal/scanners/lang/{gosec,govulncheck,bandit,pip_audit,npm_audit,semgrep,cargo_audit}.go` adapters.
- [ ] Stack-driven activation: engine runs only scanners relevant to declared `stacks:` ∩ detected languages.
- [ ] `internal/grounding/grounding.go` with structured-output schema + cache.
- [ ] Golden tests against a fixture repo with planted issues across multiple languages.

### Phase D.5 — Eval harness
- [ ] `internal/eval/` with fixture diffs + expected findings.
- [ ] Precision/recall metrics per reviewer; CI gate on regression.

### Phase E — Telemetry
- [ ] `internal/telemetry/ledger.go` — per-call accounting.
- [ ] Prometheus metrics handler.
- [ ] OTLP tracer setup.
- [ ] Reference Grafana dashboard JSON in `examples/grafana/`.
- [ ] Budget enforcement wired into engine.

### Phase F — Layer 3 reviewers
- [ ] `internal/reviewers/goldenpath.go` — the headline feature.
- [ ] `internal/reviewers/reliability.go`.
- [ ] `internal/reviewers/conventions.go` (off by default).

### Phase G — Surfaces
- [ ] `internal/report/{pretty,markdown,json,sarif}.go`.
- [ ] `cmd/infragenie/main.go` with `review`, `generate`, `mcp serve`, `init`, `goldenpath {init,bootstrap,validate}`, `version`.
- [ ] `internal/generate/` — render templates from `goldenpath.yml`.
- [ ] `internal/mcp/server.go` — stdio + HTTP transports, all five tools.
- [ ] `deploy/action/` — Dockerfile + `action.yml` for GitHub Action.

### Phase H — Polish
- [ ] `infragenie init` writes a sensible default `goldenpath.yml` in override mode.
- [ ] `examples/goldenpath/` reference Golden Path with five community templates (Go-gRPC, Python-FastAPI, Node-Express, Rust-Axum, Java-Spring).
- [ ] README rewritten around Golden Paths.
- [ ] Single-binary release via goreleaser.
- [ ] CI: build, test, lint, CodeQL, Trivy, SBOM (cyclonedx).
- [ ] Distribution security: cosign-signed binaries + container image, reproducible builds, published security policy, air-gapped mode (BYO-LLM via Bedrock private endpoint or Ollama).
- [ ] Container image bundling all scanners pinned; `--scanners=bundled` activates it in CI.
- [ ] One **public review run** against a popular Helm chart, posted as a markdown gist or blog post — the demo artifact.

### Phase I — Generate + Action (shipped)
- [x] `internal/generate/` — deterministic, golden-path-driven scaffolding. `k8s-service` template emits Deployment + co-located NetworkPolicy + Service + Dockerfile + CI, conformant by construction.
- [x] `generate service` CLI command (`--template`, `--path`, `--goldenpath`, `--force`, `--list-templates`).
- [x] MCP `generate_service` tool — agents create services over the same engine.
- [x] `deploy/action/` — Docker GitHub Action (Dockerfile bundling scanners + entrypoint + action.yml) and a cosign-signed GHCR image published from release CI.
- [x] Conformance contract test: `generate` then `review` over the same golden path yields zero findings.

## Success metrics

How we know the platform works — measurable, lifted from the principles.

- **Time-to-conformant-service:** a new service scaffolded and passing `review` in minutes, not days.
- **Agent autonomy:** an AI agent can `generate_service` then `review_*` over MCP and converge to zero Golden Path findings with no human in the loop.
- **Voluntary adoption:** teams `extends:` a Golden Path because it's faster than rolling their own, not because they're mandated to.
- **Governance + velocity together:** security/reliability findings caught per PR go up while cost-per-PR stays in cents (FinOps ledger) and review latency stays low.

## Vision / roadmap

v0 is the **paved-road kernel**: enforce a standard (`review`) and scaffold to it (`generate`). It is deliberately *not* a hosted IDP and *not* a Backstage replacement — it sits alongside a catalog, it doesn't become one. The kernel is the on-ramp to the broader "project creation + operations" ambition; each rung below extends the same engine and the same `goldenpath.yml`, and stays honest about cloud-mutation boundaries.

| Rung | Capability | Notes / boundary |
|---|---|---|
| 1 | `generate` plain-manifest `k8s-service` + `review` | shipped |
| 2 | `helm-service` Helm-chart template | shipped (reviewer now skips Helm `Chart.yaml`/values; generate-time `[[ ]]` delimiters) |
| 3 (next) | Multi-archetype templates: HTTP/gRPC API, worker, event-driven, data pipeline, AI service | each conformant-by-construction; full paved road (CI/CD, security, observability, tests, runbook) |
| 4 | `generate` emits IaC (Terraform/OpenTofu) for infra/env/IAM/secrets | **emit + PR only**; no `apply`/cloud mutation in this tool (honours the v0 non-goal) |
| 5 | Service-catalog *export* (Backstage `catalog-info.yaml`, etc.) | integrate with an IDP; still not a catalog of our own |
| 6 | Operational agents over MCP: troubleshoot, generate runbooks, drift PRs | HITL; reuses the trust gradient |

The line we hold across every rung: **the easy path is the correct path, the wrong path is hard, and humans and agents travel it identically.**

## Deferred (v1+)

Each is a one-paragraph design sketch in `docs/roadmap/` rather than v0 code:

- **GitHub App** — webhook receiver, posts inline PR comments, dedupes prior comments via SQLite cache.
- **AWS Lambda deployment** — same engine, EventBridge-triggered for serverless webhook ingestion.
- **Helm chart** — for self-hosted EKS deployments of the GitHub App.
- **HITL drift remediation** — detect drift between live cluster and git, draft a reconciliation PR.
- **Multi-repo conventions index** — pgvector across N service repos for "this differs from `payments-api` because…"
- **Auto-fix mode** — apply suggested diffs as a follow-up commit.
- **Documentation/runbook reviewer** — keep runbooks in sync with manifests + alerts.
- **Cost reviewer** — flag right-sizing opportunities from generated specs.

## Success criteria for v0

A platform engineer can:
1. Install `infragenie` (single binary or `go install`).
2. Author a `goldenpath.yml` matching their org's standard in under an hour.
3. Run `infragenie generate service my-api --template go-grpc` and get a PR that conforms.
4. Add `infragenie review` as a CI step; every PR comment surfaces grounded findings under a known token budget.
5. Drive `review` and `explain_finding` from Claude Code or Cursor over MCP.
6. See cost and usage in Grafana from the shipped dashboard.

If that loop works without surprise, v0 is done. The demo artifact is a public review run + a screenshot of the Grafana dashboard + a 2-minute MCP integration video.
