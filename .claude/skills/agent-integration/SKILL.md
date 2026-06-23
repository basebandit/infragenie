---
name: agent-integration
description: Use when an AI agent should drive InfraGenie over MCP — the tools, their structured I/O, and the generate→review loop an agent runs to ship a conformant service unattended.
---

# Agent integration (MCP)

InfraGenie exposes its engine as an MCP server (`internal/mcp/server.go`, built on `mark3labs/mcp-go`). Same engine as the CLI — an agent has the same power as a human.

## Run the server

```
infragenie mcp        # stdio transport
```

MCP client config:
```json
{ "mcpServers": { "infragenie": { "command": "infragenie", "args": ["mcp"] } } }
```

## Tools

| Tool | Required | Optional | Returns |
|---|---|---|---|
| `review_diff` | `diff` (unified diff) | `goldenpath_path`, `format` | findings (json/text) |
| `review_pr` | `pr` (`owner/repo#N`) | `goldenpath_path`, `github_token`, `format` | findings (json/text) |
| `generate_service` | `name` | `template`, `path`, `goldenpath_path` | `{template, dir, files_created}` |

Use `format: json` for machine parsing. `review_pr` needs `GITHUB_TOKEN` in env or the `github_token` arg.

## The autonomous loop

An agent ships a conformant service with no human in the loop:

1. `generate_service { name, goldenpath_path }` → files written, correct by construction.
2. `review_diff` (or `review_pr`) over the new files against the same `goldenpath_path`.
3. Expect **zero Golden Path findings** (the conformance contract). If any appear, the agent reads `Finding.Suggestion`/`Evidence`, edits, re-reviews.
4. Stop at zero, or when only sub-`fail_on` findings remain.

## Reading findings

Each `Finding` (`pkg/models/finding.go`) is structured: `RuleID`, `Severity`, `File`, `Line`, `Title`, `Explanation`, `Suggestion`, plus trust gating — `TrustLevel` (T1/T2/T3), `Confidence`, `Evidence`, `EvidenceLoc`. Agents should trust T1 absolutely, treat T3 as confidence-gated advice, and prefer findings with concrete `Evidence`/`EvidenceLoc` for auto-edits.

## Keep CLI and MCP in lockstep

When adding a reviewer or scanner, register it in BOTH `cmd/infragenie/review.go` and `internal/mcp/server.go`. A capability humans get at the CLI, agents must get over MCP.
