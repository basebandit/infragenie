---
name: authoring-golden-paths
description: Use when writing, extending, or validating a goldenpath.yml — the canonical vs extends (override) modes, the cold-start ladder, and what each field actually drives in the reviewers.
---

# Authoring Golden Paths

A `goldenpath.yml` is the executable standard. Loader: `internal/goldenpath/loader.go`; schema: `pkg/models/goldenpath.go` (`GoldenPath`). Schema major version is `1`; the loader rejects other majors.

## Two modes, one filename

- **Canonical** (platform-team repo): declares the standard. `name` required, no `extends`.
- **Override** (service repo): `extends:` a canonical (local path or `@ref`), then per-repo `rules:` / `ignore:` / severity / budget. `IsOverride()` is true when `extends` is set.

`extends:` resolves at load time and merges: scalars override, `rules`/`ignore` accumulate, parent `defaults.fail_on`/`budget` become effective when the child omits them. Parents can be local paths or remote refs:

- local: `extends: ../platform/goldenpath.yml`
- https: `extends: https://raw.githubusercontent.com/owner/repo/main/goldenpath.yml`
- github shorthand: `extends: github.com/owner/repo/path/goldenpath.yml@v1` (ref defaults to main)

SSH (`git@`) is not supported. Remote fetch has a 15s timeout and a 1 MiB cap. Pin a tag/sha rather than `main` for reproducibility.

## Cold-start ladder (get a file from any starting point)

| Command | When |
|---|---|
| `infragenie init` | auto-detect stack, write a baseline |
| `infragenie init --starter kubernetes\|fintech\|solo` | curated starter |
| edit `examples/golden-paths/*.yml` | adapt a reference |

## What each field drives (so you set them deliberately)

The deterministic reviewers (`internal/reviewers/`) read these per-file:

- `required_labels` → every K8s manifest's `metadata.labels` must contain them (validated by single-doc YAML unmarshal; skipped on parse error).
- `security.forbid_image_tag_latest` → no `:latest` anywhere in a manifest.
- `security.forbid_privileged` → no container sets `securityContext.privileged: true` (critical).
- `security.require_non_root` / `require_read_only_rootfs` → Deployment must contain `runAsNonRoot`/`runAsUser`, `readOnlyRootFilesystem: true`.
- `security.require_network_policy` → the **Deployment file itself** must contain `kind: NetworkPolicy` (the check is per-file — co-locate it).
- `observability.require_prometheus_annotations` → Deployment must contain `prometheus.io/scrape` or `/port`.
- `ci_required_steps[].matches` → CI file must contain at least one match substring per step.
- `runtime_rules[].pattern` → manifest must contain the substring (conventions reviewer); patterns ending in `:` are treated as required annotations by `generate`.
- `chart_shape.required_files` → flagged only when such a file is **deleted**.

`fail_on` sets the exit-code threshold; `budget.tokens`/`usd` cap grounding spend; `defaults.reviewers` toggles Layer-3 reviewers (conventions off by default).

## Validate

```
infragenie review --goldenpath goldenpath.yml --diff HEAD~1
```
`goldenpath.Validate` checks version + that a canonical file has a `name`. Pin `extends:` to immutable refs; bump deliberately.
