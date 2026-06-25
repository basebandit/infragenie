#!/usr/bin/env bash
# Translate GitHub Action inputs (INPUT_*) into an `infragenie review` invocation.
# Findings are emitted as GitHub annotations; the exit code propagates so the
# step fails when findings reach the configured severity.
set -euo pipefail

goldenpath="${INPUT_GOLDENPATH:-goldenpath.yml}"
provider="${INPUT_PROVIDER:-}"
model="${INPUT_MODEL:-}"
fail_on="${INPUT_FAIL_ON:-high}"
budget_tokens="${INPUT_BUDGET_TOKENS:-50000}"
budget_usd="${INPUT_BUDGET_USD:-0.50}"
pr="${INPUT_PR:-}"
comment="${INPUT_COMMENT:-true}"

# Derive the PR reference from the event when not supplied explicitly.
if [[ -z "$pr" ]]; then
  if [[ "${GITHUB_REF:-}" =~ ^refs/pull/([0-9]+)/ ]]; then
    pr="${GITHUB_REPOSITORY}#${BASH_REMATCH[1]}"
  else
    echo "::error::no PR context found; set the 'pr' input (owner/repo#N) for non-PR events" >&2
    exit 3
  fi
fi

args=(review
  --goldenpath "$goldenpath"
  --pr "$pr"
  --format github
  --fail-on "$fail_on"
  --budget-tokens "$budget_tokens"
  --budget-usd "$budget_usd")

[[ -n "$provider" ]] && args+=(--provider "$provider")
[[ -n "$model" ]] && args+=(--model "$model")
[[ "$comment" == "true" ]] && args+=(--comment)

exec infragenie "${args[@]}"
