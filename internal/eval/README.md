# Eval Harness

Regression harness for the engine. Catches silent quality regressions when prompts, scanners, or grounding logic change.

## How it works

Each fixture is a JSON file in `testdata/` pairing:
- **`expected`** — findings the engine *must* produce for a known input
- **`actual`** — findings recorded from a known-good run
- **`thresholds`** — minimum precision/recall the fixture must meet

`TestFixtureCorpus` loads every `testdata/*.json`, scores each against its thresholds, and fails CI if any fixture drops below them.

## Running

```bash
# run harness (shows corpus aggregate in -v output)
go test ./internal/eval/... -v -run TestFixtureCorpus

# run all eval tests
go test ./internal/eval/...
```

## Writing a fixture

Create `internal/eval/testdata/<name>.json`:

```json
{
  "name": "my-rule-smoke",
  "description": "Engine must flag missing resource limits in a Deployment.",
  "thresholds": {
    "min_precision": 1.0,
    "min_recall":    1.0,
    "line_tolerance": 2
  },
  "expected": [
    {"rule_id": "checkov.CKV_K8S_10", "file": "k8s/deployment.yaml", "line": 12, "severity": "medium"}
  ],
  "actual": [
    {
      "rule_id":    "checkov.CKV_K8S_10",
      "source":     "scanner",
      "origin":     "checkov",
      "severity":   "medium",
      "file":       "k8s/deployment.yaml",
      "line":       12,
      "title":      "CPU limits should be set",
      "confidence": 1.0,
      "trust_level": "T1"
    }
  ]
}
```

Then run the harness to confirm it passes.

### Populating `actual`

Run the engine against the sample input and capture its JSON output:

```bash
infragenie review --diff HEAD~1 --format json | jq '.findings' > /tmp/actual.json
```

Copy the relevant findings into `actual` in your fixture file.

## Fields

| Field | Description |
|-------|-------------|
| `thresholds.min_precision` | Fraction of reported findings that must be correct (0–1). `1.0` = no false positives tolerated. |
| `thresholds.min_recall` | Fraction of expected findings that must be found (0–1). `1.0` = no misses tolerated. |
| `thresholds.line_tolerance` | Findings match if their line numbers are within this many lines. Tolerates minor line-number drift across refactors. |

## Threshold guidance

- Start strict: `min_precision: 1.0, min_recall: 1.0, line_tolerance: 1`.
- Loosen only deliberately — a threshold change is a reviewable signal that detection quality has changed.
- Loosening recall means accepting that the engine will miss some findings; document why in the fixture's `description`.

## Metrics reference

- **Precision** = TP / (TP + FP) — quality of what the engine reports
- **Recall** = TP / (TP + FN) — coverage of what the engine should find
- **F1** = harmonic mean of both
