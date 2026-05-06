// Package lang holds Layer-1 adapters for language-aware scanners
// (gosec, govulncheck, bandit, pip-audit, npm audit, semgrep, cargo audit).
package lang

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Gosec struct{}

func NewGosec() *Gosec                                      { return &Gosec{} }
func (g *Gosec) Name() string                               { return "gosec" }
func (g *Gosec) Available() bool                            { return scanners.BinaryAvailable("gosec") }
func (g *Gosec) Stacks() []string                           { return []string{"go"} }

func (g *Gosec) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	root := "./..."
	if in.RepoCtx != nil && in.RepoCtx.Root != "" {
		root = in.RepoCtx.Root + "/..."
	}
	out, err := scanners.RunJSON(ctx, "gosec", "-fmt", "json", "-quiet", root)
	if err != nil {
		return nil, err
	}
	return parseGosec(out)
}

func parseGosec(b []byte) ([]models.Finding, error) {
	var doc struct {
		Issues []struct {
			Severity   string `json:"severity"`
			Confidence string `json:"confidence"`
			RuleID     string `json:"rule_id"`
			Details    string `json:"details"`
			File       string `json:"file"`
			Line       string `json:"line"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("gosec: %w", err)
	}
	out := make([]models.Finding, 0, len(doc.Issues))
	for _, i := range doc.Issues {
		out = append(out, models.Finding{
			RuleID:   "gosec." + i.RuleID,
			Origin:   "gosec",
			Severity: scanners.MapSeverity(i.Severity),
			File:     i.File,
			Line:     parseFirstInt(i.Line),
			Title:    i.Details,
		})
	}
	return out, nil
}

// parseFirstInt accepts gosec line strings like "10" or "10-12" and
// returns the leading integer.
func parseFirstInt(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			n, _ := strconv.Atoi(s[:i])
			return n
		}
	}
	n, _ := strconv.Atoi(s)
	return n
}
