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

<!-- How was this tested? Check what applies and describe anything non-standard. -->

- [ ] Unit tests added / updated (`go test ./...` passes)
- [ ] Eval harness fixture added / updated (`go test ./internal/eval/...` passes)
- [ ] Integration tested locally (describe setup below if needed)
- [ ] Manual smoke test — `infragenie review --diff HEAD~1` ran cleanly

```bash
# Smoke check (adjust for your change — delete if not applicable)
# Config / env resolution
cp examples/config.yml ~/.config/infragenie/config.yml
infragenie review --goldenpath goldenpath.yml --diff HEAD~1

# Env var override
export INFRAGENIE_PROVIDER=openai
infragenie review --goldenpath goldenpath.yml --diff HEAD~1

# Confirm no secrets leaked into git
git status  # .env.local must not appear
```

## Breaking changes

<!-- If breaking, what must callers / operators change? Delete section if not applicable. -->

## Security

<!-- Any auth, secrets, input validation, or supply-chain impact? If none, delete section. -->

## Checklist

> CI enforced automatically — no need to check these manually:
> `make build` · `make test -race` · `golangci-lint` · secret scan · conventional commit messages

- [ ] Public API / CLI flags documented in README or relevant doc
- [ ] No unintentional behaviour change in existing commands
