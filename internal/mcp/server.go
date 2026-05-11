// Package mcp exposes InfraGenie as an MCP server so LLM assistants can
// call review_diff and review_pr as tools.
package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/basebandit/infragenie/internal/diff"
	"github.com/basebandit/infragenie/internal/engine"
	"github.com/basebandit/infragenie/internal/goldenpath"
	"github.com/basebandit/infragenie/internal/reporter"
	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/internal/scanners/infra"
	"github.com/basebandit/infragenie/internal/scanners/lang"
	"github.com/basebandit/infragenie/pkg/models"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer builds and returns a configured MCP server. Call
// server.NewStdioServer(s).Listen(ctx, os.Stdin, os.Stdout) to run it.
func NewServer(version string) *server.MCPServer {
	s := server.NewMCPServer(
		"infragenie",
		version,
		server.WithToolCapabilities(false),
	)

	s.AddTool(reviewDiffTool(), handleReviewDiff)
	s.AddTool(reviewPRTool(), handleReviewPR)

	return s
}

// ── tools ─────────────────────────────────────────────────────────────────────

func reviewDiffTool() mcplib.Tool {
	return mcplib.NewTool("review_diff",
		mcplib.WithDescription(
			"Review a unified diff against InfraGenie's scanner and reviewer rules. "+
				"Returns findings as JSON. Optionally provide a goldenpath_path to enforce "+
				"Golden Path policy."),
		mcplib.WithString("diff",
			mcplib.Required(),
			mcplib.Description("Unified diff content (output of `git diff`).")),
		mcplib.WithString("goldenpath_path",
			mcplib.Description("Path to goldenpath.yml on disk (optional).")),
		mcplib.WithString("format",
			mcplib.Description("Output format: json (default) or text.")),
	)
}

func reviewPRTool() mcplib.Tool {
	return mcplib.NewTool("review_pr",
		mcplib.WithDescription(
			"Fetch a GitHub PR diff and review it against InfraGenie rules. "+
				"Returns findings as JSON. Requires GITHUB_TOKEN in environment or "+
				"the github_token argument."),
		mcplib.WithString("pr",
			mcplib.Required(),
			mcplib.Description("GitHub PR reference: owner/repo#N or full PR URL.")),
		mcplib.WithString("goldenpath_path",
			mcplib.Description("Path to goldenpath.yml on disk (optional).")),
		mcplib.WithString("github_token",
			mcplib.Description("GitHub token (default: $GITHUB_TOKEN).")),
		mcplib.WithString("format",
			mcplib.Description("Output format: json (default) or text.")),
	)
}

// ── handlers ──────────────────────────────────────────────────────────────────

func handleReviewDiff(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	raw, err := req.RequireString("diff")
	if err != nil {
		return mcplib.NewToolResultError("diff argument required"), nil
	}
	gpPath := req.GetString("goldenpath_path", "")
	format := reporter.Format(req.GetString("format", "json"))

	files, err := diff.Parse([]byte(raw))
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("parse diff: %v", err)), nil
	}
	d := &models.Diff{Source: models.DiffSourceLocal, Files: files}

	out, err := runEngine(ctx, d, gpPath)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(renderFindings(out, format)), nil
}

func handleReviewPR(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	prRef, err := req.RequireString("pr")
	if err != nil {
		return mcplib.NewToolResultError("pr argument required"), nil
	}
	tok := req.GetString("github_token", os.Getenv("GITHUB_TOKEN"))
	gpPath := req.GetString("goldenpath_path", "")
	format := reporter.Format(req.GetString("format", "json"))

	pr, err := diff.ParsePRURL(prRef)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("invalid pr ref: %v", err)), nil
	}
	d, err := diff.GitHubPR(ctx, pr, tok)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("fetch pr: %v", err)), nil
	}

	out, err := runEngine(ctx, d, gpPath)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(renderFindings(out, format)), nil
}

// ── shared ────────────────────────────────────────────────────────────────────

func runEngine(ctx context.Context, d *models.Diff, gpPath string) (*engine.Result, error) {
	var gp *models.GoldenPath
	if gpPath != "" {
		loaded, err := goldenpath.New(".").Load(gpPath)
		if err != nil {
			return nil, fmt.Errorf("goldenpath: %w", err)
		}
		gp = loaded
	}

	all := []scanners.Scanner{infra.NewCheckov(), infra.NewHadolint(), lang.NewGosec()}
	rev := []reviewers.Reviewer{
		reviewers.GoldenPathReviewer{},
		reviewers.ReliabilityReviewer{},
		reviewers.ConventionsReviewer{},
	}

	eng := engine.New(engine.Config{
		GoldenPath: gp,
		Scanners:   scanners.Select(all, gp, nil, nil),
		Reviewers:  rev,
	})
	return eng.Run(ctx, engine.Input{Diff: d})
}

func renderFindings(result *engine.Result, format reporter.Format) string {
	var buf bytes.Buffer
	_ = reporter.Write(&buf, result.Findings, result.Skipped, format)
	return buf.String()
}
