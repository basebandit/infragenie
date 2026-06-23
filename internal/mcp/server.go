// Package mcp exposes InfraGenie as an MCP server so LLM assistants can
// call review_diff and review_pr as tools.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/basebandit/infragenie/internal/diff"
	"github.com/basebandit/infragenie/internal/engine"
	"github.com/basebandit/infragenie/internal/generate"
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
	s.AddTool(generateServiceTool(), handleGenerateService)

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

func generateServiceTool() mcplib.Tool {
	return mcplib.NewTool("generate_service",
		mcplib.WithDescription(
			"Scaffold a new service that conforms to a Golden Path. Files are rendered "+
				"deterministically from goldenpath.yml, so the result passes review with zero "+
				"Golden Path findings. Returns the list of files created."),
		mcplib.WithString("name",
			mcplib.Required(),
			mcplib.Description("Service name (DNS-1123 label, e.g. payments-api).")),
		mcplib.WithString("template",
			mcplib.Description("Template set to render (default: k8s-service).")),
		mcplib.WithString("path",
			mcplib.Description("Parent directory for the generated service (default: services).")),
		mcplib.WithString("goldenpath_path",
			mcplib.Description("Path to goldenpath.yml on disk (optional; secure defaults used if absent).")),
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

func handleGenerateService(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcplib.NewToolResultError("name argument required"), nil
	}
	gpPath := req.GetString("goldenpath_path", "")

	var gp *models.GoldenPath
	if gpPath != "" {
		loaded, lerr := goldenpath.New(".").Load(gpPath)
		if lerr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("goldenpath: %v", lerr)), nil
		}
		gp = loaded
	}

	res, err := generate.New().Run(generate.Params{
		Name:       name,
		Template:   req.GetString("template", ""),
		OutDir:     req.GetString("path", ""),
		GoldenPath: gp,
	})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	payload, err := json.Marshal(struct {
		Template     string   `json:"template"`
		Dir          string   `json:"dir"`
		FilesCreated []string `json:"files_created"`
	}{Template: res.Template, Dir: res.Dir, FilesCreated: res.Files})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(payload)), nil
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
