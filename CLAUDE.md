# CLAUDE.md

Instructions for AI agents in this repo. Be terse, verify before claiming done, keep diffs small.

## What this is

InfraGenie: a Golden Path engine. Codify infra standards in `goldenpath.yml`, then **review** diffs/PRs against them and **generate** services that conform. Surfaces: CLI (`cmd/infragenie`), GitHub Action (`deploy/action`), MCP (`internal/mcp`), webhook (`cmd/infragenie/serve.go` + `internal/webhook`). Go 1.25.

## Verify (run before saying it works)

```
make build
make test     # go test -race ./...
make lint
```

Never assert success without running these. If tests fail, say so with the output.

## Invariants — do not break

- **Generated output passes its own review.** `generate` then `review` against the same golden path yields zero Golden Path findings. Locked by `internal/generate/generate_test.go`; keep it green.
- **Reviewers are deterministic and per-document.** `internal/reviewers` parses each YAML doc structurally (kind, labels, securityContext). Don't substring-guess. Helm/Kustomize are rendered first (`internal/render`); undecodable templates are skipped, not guessed.
- **LLM calls use native structured output. No regex extraction from markdown.**
- **Scanners detect-or-skip.** Missing binary is a structured skip, never an error.
- The live eval corpus (`internal/eval`) gates precision/recall. Don't regress it.

## Conventions

- Tests use `testify` (`require`/`assert`). Comments: plain and minimal, no `// ── … ──` banner lines.
- Use `basebandit` as the placeholder org/repo in docs and examples.
- Commits: Conventional Commits, one-line subject, no `Co-Authored-By` trailer, no em dashes.
- PRs: one concern, about 10 files or fewer. Body follows the template headers (Summary/Context/Testing/Risk), prose, no "Generated with Claude Code" footer. See the `writing-prs` skill.
- Branches: `git fetch && git rebase origin/main` (never stale local main). Don't merge main into a branch; it breaks the commitlint check.

## Extending

- Scanner: `internal/scanners/{infra,lang}`, register in `cmd/infragenie/registry.go`.
- Reviewer: implement `reviewers.Reviewer`, add to the slices in `cmd/infragenie/review.go` and `internal/mcp/server.go`.
- LLM provider: `internal/llm` (native structured output).
- Generate template: `internal/generate/templates.go` + embedded files; must satisfy the conformance test.

## Skills

Use the project skills when relevant: `writing-prs`, `extending-infragenie`, `authoring-golden-paths`, `service-generation`, `agent-integration`.
