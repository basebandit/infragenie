# Onboarding a service to InfraGenie

This guide takes any existing service, in any language, from zero to reviewed
against a Golden Path. It works the same whether or not you have an LLM API key.

InfraGenie does two things with one file (`goldenpath.yml`):

- **Review** a change (local files, a git diff, or a GitHub PR) against your
  standard, with scanner findings and, optionally, LLM-grounded explanations and
  fixes.
- **Generate** new services that pass review from the first commit.

## 1. Install

```bash
go install github.com/basebandit/infragenie/cmd/infragenie@latest
# or download a signed binary (with SBOM) from the releases page
```

Scanners are external tools InfraGenie shells out to. They are optional: any that
are not installed are skipped with a note, never a hard failure. Install the ones
relevant to your stack, or skip this and use the GitHub Action (step 6), which
bundles them all.

| Stack | Scanners |
|---|---|
| Kubernetes / Helm | kube-score, kubeconform, checkov, trivy |
| Terraform | tflint, checkov, trivy |
| Dockerfile | hadolint, trivy |
| Go | gosec, govulncheck, semgrep |
| Python / JS / TS / Java / other | semgrep |

## 2. Create a goldenpath.yml

```bash
cd path/to/your/service
infragenie init
```

`init` detects your languages, platforms, and CI system from the repo and writes
a starting `goldenpath.yml`. Open it and adjust it to your standard:
`required_labels`, `security`, `observability`, `ci_required_steps`. Each rule
applies only where it makes sense, for example securityContext checks run on
Kubernetes workloads, while Dockerfile and CI checks run regardless of language.

## 3. Review without an API key

```bash
infragenie review --goldenpath goldenpath.yml
```

With no `--diff` or `--pr`, InfraGenie reviews the current files under `--path`
(default `.`), so you can check an existing service with no git history. This is
the deterministic path: Layer 1 scanners plus the Layer 3 Golden Path,
reliability, and conventions reviewers. No LLM, no key, no cost.

Other modes:

```bash
infragenie review -g goldenpath.yml --diff HEAD~1        # changes since a commit
infragenie review -g goldenpath.yml --pr owner/repo#42   # a GitHub PR (needs GITHUB_TOKEN)
infragenie review -g goldenpath.yml --format sarif       # upload to code scanning
```

## 4. Review with LLM grounding (optional)

Grounding (Layer 2) adds a repo-aware explanation and a suggested fix to each
finding. The deterministic findings are identical with or without it; grounding
only enriches them. Enable it by selecting a provider.

By environment variable (recommended):

```bash
export OPENAI_API_KEY=sk-...   # or ANTHROPIC_API_KEY / GOOGLE_API_KEY / AZURE_OPENAI_API_KEY
infragenie review -g goldenpath.yml --provider openai
```

Or reference the key from config without committing it, using `${VAR}`
interpolation (`.infragenie.yml` in the repo, or `~/.config/infragenie/config.yml`):

```yaml
providers:
  default: openai
  openai:
    api_key: ${OPENAI_API_KEY}   # the reference is committed, never the secret
    model: gpt-4o-mini
defaults:
  budget: {tokens: 50000, usd: 0.50}
```

Self-hosted models (Ollama) need no key:

```yaml
providers:
  default: local
  local:
    base_url: http://localhost:11434
    model: llama3
```

`--budget-tokens` and `--budget-usd` cap spend per run. To force the
deterministic-only path even when a provider is configured, pass `--no-ground`.

## 5. Tune the standard

In `goldenpath.yml`:

- `rules:` override a rule's severity or disable it per repo.
- `ignore:` skip paths, for example `examples/**`.
- `fail_on:` the severity that fails the run (drives the CI exit code).
- `extends:` inherit a shared baseline, from a local path or a remote ref
  (`github.com/org/repo/goldenpath.yml@v1`). Pin a tag for reproducibility.

## 6. Add to CI

The published Action bundles InfraGenie and every scanner:

```yaml
permissions:
  contents: read
  pull-requests: write
steps:
  - uses: actions/checkout@v4
  - uses: basebandit/infragenie@v0.2.0
    env:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}   # optional; omit for deterministic-only
    with:
      goldenpath: goldenpath.yml
      fail-on: high
      comment: "true"   # posts and updates one summary comment on the PR
```

Without the key it still runs every deterministic check; with it, findings carry
grounded explanations.

## Generate new services (optional)

```bash
infragenie generate service my-api --template k8s-service
infragenie generate service --list-templates   # k8s-service, helm-service, worker, data-pipeline
```

Generated services pass `review` against the same golden path by construction.

## Language coverage today

| Language | Detected from | Application scanners |
|---|---|---|
| Go | go.mod | gosec, govulncheck, semgrep |
| Python | pyproject.toml, requirements.txt | semgrep |
| JavaScript / TypeScript | package.json | semgrep |
| Java | pom.xml, build.gradle | semgrep |
| Rust / Ruby / PHP / others | Cargo.toml, Gemfile, ... | semgrep |
| Dockerfile / Kubernetes / Terraform | detected from files | hadolint, checkov, trivy, kube-score, kubeconform, tflint |

InfraGenie's center of gravity is infrastructure (Kubernetes, Helm, Terraform,
Dockerfile, CI). For application code, semgrep provides broad multi-language
coverage today, with Go additionally covered by gosec and govulncheck. Dedicated
per-language scanners (for example bandit, pip-audit, npm audit) are added over
time; see below.

## Extending coverage

InfraGenie is built to be extended. A new scanner, reviewer, LLM provider, or
generate template is a small Go adapter plus a registry entry, contributed by
pull request. It compiles into the binary; there is no separate plugin runtime.

- **Scanner** (wrap a tool such as bandit, pip-audit, npm audit, cargo audit):
  implement the `scanners.Scanner` interface under `internal/scanners/{infra,lang}`
  and register it in `cmd/infragenie/registry.go`. Detect-or-skip on the binary,
  parse its JSON, map results to findings.
- **Reviewer** (Layer 3 policy): implement `reviewers.Reviewer`.
- **LLM provider**: add it to `internal/llm`.
- **Generate template**: add a set under `internal/generate/templates`.

Adding a language's dedicated scanner is the recommended way to deepen coverage
beyond semgrep.

## Troubleshooting

- `scanner: X (unavailable)` — the binary is not installed; install it, or use the
  Action which bundles them. List status with `infragenie scanners list`.
- No findings — usually no scanners installed and your files already conform.
- `--path cannot be combined with --diff or --pr` — choose one input mode.
- LLM errors — grounding needs a valid provider and key; drop `--provider` (or
  pass `--no-ground`) to run the deterministic checks only.
