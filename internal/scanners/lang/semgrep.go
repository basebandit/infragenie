package lang

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

// Semgrep runs `semgrep --config auto` against the repo root.
// --config auto pulls the community ruleset and auto-detects language.
type Semgrep struct{}

func NewSemgrep() *Semgrep  { return &Semgrep{} }
func (s *Semgrep) Name() string  { return "semgrep" }
func (s *Semgrep) Available() bool { return scanners.BinaryAvailable("semgrep") }

// Stacks lists every language semgrep covers out of the box.
// The scanner is skipped on pure-infra repos (kubernetes/terraform only).
func (s *Semgrep) Stacks() []string {
	return []string{
		"go", "python", "javascript", "typescript",
		"java", "ruby", "rust", "kotlin", "scala", "php", "c", "cpp",
	}
}

func (s *Semgrep) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	root := "."
	if in.RepoCtx != nil && in.RepoCtx.Root != "" {
		root = in.RepoCtx.Root
	}
	out, err := scanners.RunJSON(ctx, "semgrep", "--config", "auto", "--json", "--quiet", root)
	if err != nil {
		return nil, err
	}
	return parseSemgrep(out)
}

func parseSemgrep(b []byte) ([]models.Finding, error) {
	var doc struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
				Metadata struct {
					Confidence string `json:"confidence"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("semgrep: %w", err)
	}
	out := make([]models.Finding, 0, len(doc.Results))
	for _, r := range doc.Results {
		out = append(out, models.Finding{
			RuleID:   "semgrep." + r.CheckID,
			Origin:   "semgrep",
			Severity: scanners.MapSeverity(r.Extra.Severity),
			File:     r.Path,
			Line:     r.Start.Line,
			Title:    r.Extra.Message,
		})
	}
	return out, nil
}
