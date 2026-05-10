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

```
# paste relevant test output or command run here (optional)
```

## Breaking changes

<!-- If breaking, what must callers / operators change? Delete section if not applicable. -->

## Security

<!-- Any auth, secrets, input validation, or supply-chain impact? If none, delete section. -->

## Checklist

- [ ] `make build` passes
- [ ] `make test` passes (race detector on)
- [ ] `make lint` passes (or failures are pre-existing and noted)
- [ ] No secrets, credentials, or `.env.local` committed
- [ ] Public API / CLI flags documented in README or relevant doc
- [ ] Commit messages are one-liners in imperative mood (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`)
