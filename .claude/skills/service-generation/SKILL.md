---
name: service-generation
description: Use when working on `infragenie generate` — how deterministic scaffolding works, the reviewer conformance contract a template must satisfy, and how to add a new template set.
---

# Service generation

`internal/generate` scaffolds services from the resolved `goldenpath.yml`. **Deterministic — no LLM.** Output conforms by construction: generate then review against the same golden path yields zero Golden Path findings.

## How a run works

`Generator.Run(Params{Name, Template, OutDir, GoldenPath, Force})`:
1. Resolve the template set (`templates.go`; default `k8s-service`).
2. Build render data: `Service{Name,Image,Tag,Port}`, `GoldenPath`, `Labels` (resolved from `required_labels` + always `app.kubernetes.io/name`), `Annotations` (from `runtime_rules` patterns ending in `:`).
3. Render each embedded `text/template` to `OutDir/Name/<out>` via `os.WriteFile`; refuse overwrite unless `Force`.

CLI: `infragenie generate service <name>` (`-t/-p/-g/--force/--list-templates`). MCP: `generate_service`.

## The conformance contract (do not break)

`internal/reviewers/goldenpath.go` (+ reliability, conventions) match **per-file** via strings / single-doc YAML. A template's manifests must therefore:

- carry every `required_labels` entry in `metadata.labels`;
- in the **Deployment file**: `runAsNonRoot`/`runAsUser`, `readOnlyRootFilesystem: true`, Prometheus annotations, and a **co-located** `kind: NetworkPolicy` (the NetworkPolicy check is per-file);
- no `:latest`;
- reliability: `limits:` + `requests:`, `livenessProbe:` + `readinessProbe:`, `replicas: 2+` (never `replicas: 1`);
- conventions: contain `app.kubernetes.io/` and any `runtime_rules` pattern (auto-injected as annotations).

`internal/generate/generate_test.go` renders a strict golden path, builds an added-file diff, and asserts the three reviewers return zero findings. **Keep it green.**

## Why plain manifests, not Helm

A Helm `Chart.yaml` has `apiVersion:` but no `metadata.labels`, so `checkRequiredLabels` falsely flags it. The shipped `k8s-service` emits plain manifests. A `helm-service` template needs `goldenpath.go` to skip `Chart.yaml` (or non-workload kinds) first — that's roadmap rung 2.

## Add a template set

1. Embedded files under `internal/generate/templates/<name>/` (`.tmpl`). `templates.go` embeds `templates`.
2. Register a `templateSet` in `builtins` with `name`, `desc`, and `files []fileTmpl{src, out}` (out may include subdirs, e.g. `.github/workflows/ci.yml`).
3. Satisfy the contract above; add a conformance case to `generate_test.go`.
4. Keep label/security/observability values driven by `GoldenPath` so output conforms to whatever path is passed.
