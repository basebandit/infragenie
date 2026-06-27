# CLAUDE.md

How to work on InfraGenie. Merges with your default behavior; explicit user instructions win.

## 1. State assumptions; ask when unsure

Don't guess at intent. If a requirement is ambiguous, say what you're assuming and proceed, or ask first. Don't hide confusion behind plausible-looking code.

## 2. Keep it simple

Build what was asked, nothing more. No speculative flexibility, configuration, or abstraction that wasn't requested. Before finishing, ask: would a senior engineer call this overcomplicated? If so, cut it back.

## 3. Edit narrowly

Match the surrounding code's style and patterns. Touch only what the task needs. Remove dead code your change creates; leave pre-existing dead code alone. Keep comments plain and minimal, no `// ── … ──` banner lines.

## 4. Verify, don't assert

Turn the task into something testable. For a bug, write a test that reproduces it, then make it pass. Before claiming done, run:

```
make build
make test     # go test -race ./...
make lint
```

If anything fails, report it with the output. No success claims without evidence.

## Invariants (don't break these)

- `generate` output must pass its own `review` (zero Golden Path findings). Locked by `internal/generate/generate_test.go`.
- Reviewers are deterministic and parse each YAML document structurally; never substring-guess. Helm/Kustomize render before review.
- LLM calls use native structured output; no regex parsing of model text.
- Scanners detect-or-skip; a missing binary is never an error.
- Don't regress the `internal/eval` precision/recall corpus.

## Git

- Conventional Commits, one-line subject, no `Co-Authored-By`, no em dashes.
- One concern per PR (about 10 files or fewer). PR body uses the repo template; no "Generated with Claude Code" footer.
- `git fetch && git rebase origin/main` before pushing; never merge main into a branch.

Use the project skills when they fit: `writing-prs`, `extending-infragenie`, `authoring-golden-paths`, `service-generation`, `agent-integration`.
