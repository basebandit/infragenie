<!--
Write like you're briefing a colleague — plain prose, not box-ticking. Lead with
what a reviewer needs to judge this quickly. Delete any heading that doesn't apply;
don't leave it empty. No "Generated with …" footers.
-->

## Summary

<!-- What changed, in a sentence or two. This is the first thing the reviewer reads. -->

## Why

<!-- The problem, gap, or request behind it. Link the issue if there is one (Closes #). -->

## Testing

<!--
How you verified it — commands run, what you saw, anything you couldn't cover.
CI already runs: build · go test -race ./... · eval harness (TestFixtureCorpus) · lint · secret scan.
Add an eval fixture when you touch scanner adapters, grounding prompts, or reviewer logic
(see internal/eval/README.md). For golden-path/generate changes, prove the loop stays clean:

    infragenie generate service demo --goldenpath goldenpath.yml
    infragenie review   --goldenpath goldenpath.yml --diff HEAD~1   # → No findings.
-->

## Risk & rollback

<!-- Blast radius and how to revert. "Low — additive, revert the commit" is a fine answer. Note any breaking change or migration here. -->

---

- [ ] Scoped small and single-concern (≤ ~10 files)
- [ ] Docs / golden-path examples updated if behaviour changed
