## Summary

<!-- What does this PR do and why? One paragraph max. Link the issue if one exists. -->

Closes #

## Type of change

<!-- Check all that apply -->

- [ ] Bug fix (non-breaking)
- [ ] New feature (non-breaking)
- [ ] Breaking change (requires migration note below)
- [ ] Refactor (behaviour unchanged)
- [ ] Documentation
- [ ] CI / tooling

## Changes

<!-- Bullet list of notable changes. Focus on the non-obvious. -->

-

## Testing

<!--
CI runs automatically: build · go test -race ./... · eval harness (TestFixtureCorpus) · lint · secret scan.
The checklist below is about AUTHOR responsibility — did you write the right tests, not just did they pass.
-->

**Author checklist — check what you did:**

- [ ] Unit tests added or updated for new/changed behaviour
  - _Skip if this is docs-only or a refactor with no logic change_
- [ ] Eval harness fixture added or updated
  - _Required when changing scanner adapters, grounding prompts, or reviewer logic_
  - _See `internal/eval/README.md` for how to write a fixture_
- [ ] Manually ran `infragenie review` against a real diff and output looked correct

```bash
# Adjust for your change — delete block if not applicable
make build
infragenie review --goldenpath goldenpath.yml --diff HEAD~1

# For config / env changes
cp examples/config.yml ~/.config/infragenie/config.yml
export INFRAGENIE_PROVIDER=openai
infragenie review --goldenpath goldenpath.yml --diff HEAD~1
```

- [ ] Integration tested locally if the change touches LLM calls, scanner exec, or external APIs
  - _Describe non-obvious setup below_

## Breaking changes

<!-- If breaking, what must callers / operators change? Delete section if not applicable. -->

## Security

<!-- Any auth, secrets, input validation, or supply-chain impact? If none, delete section. -->

## Checklist

> CI enforced automatically — no need to check these manually:
> `make build` · `go test -race ./...` · eval harness (`TestFixtureCorpus`) · `golangci-lint` · secret scan · conventional commit messages

- [ ] Public API / CLI flags documented in README or relevant doc
- [ ] No unintentional behaviour change in existing commands
