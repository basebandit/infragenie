package lang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Govulncheck struct{}

func NewGovulncheck() *Govulncheck      { return &Govulncheck{} }
func (g *Govulncheck) Name() string     { return "govulncheck" }
func (g *Govulncheck) Available() bool  { return scanners.BinaryAvailable("govulncheck") }
func (g *Govulncheck) Stacks() []string { return []string{"go"} }

func (g *Govulncheck) Scan(ctx context.Context, _ scanners.ScanInput) ([]models.Finding, error) {
	out, err := scanners.RunJSON(ctx, "govulncheck", "-json", "./...")
	if err != nil {
		return nil, err
	}
	return parseGovulncheck(out)
}

// parseGovulncheck reads govulncheck's streamed JSON (a sequence of concatenated
// objects). It collects vulnerability summaries and emits one finding per OSV
// that has a call site in the analysed code (trace[0] carries a position).
func parseGovulncheck(b []byte) ([]models.Finding, error) {
	type position struct {
		Filename string `json:"filename"`
		Line     int    `json:"line"`
	}
	type frame struct {
		Module   string    `json:"module"`
		Package  string    `json:"package"`
		Function string    `json:"function"`
		Position *position `json:"position"`
	}
	type osv struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	}
	type finding struct {
		OSV          string  `json:"osv"`
		FixedVersion string  `json:"fixed_version"`
		Trace        []frame `json:"trace"`
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	summaries := map[string]string{}
	var findings []finding
	for dec.More() {
		var msg struct {
			OSV     *osv     `json:"osv"`
			Finding *finding `json:"finding"`
		}
		if err := dec.Decode(&msg); err != nil {
			return nil, fmt.Errorf("govulncheck: %w", err)
		}
		if msg.OSV != nil {
			summaries[msg.OSV.ID] = msg.OSV.Summary
		}
		if msg.Finding != nil {
			findings = append(findings, *msg.Finding)
		}
	}

	seen := map[string]bool{}
	var out []models.Finding
	for _, f := range findings {
		if len(f.Trace) == 0 || f.Trace[0].Position == nil || seen[f.OSV] {
			continue
		}
		seen[f.OSV] = true
		title := summaries[f.OSV]
		if title == "" {
			title = f.OSV
		}
		fd := models.Finding{
			RuleID:   "govulncheck." + f.OSV,
			Origin:   "govulncheck",
			Severity: models.SeverityHigh,
			File:     f.Trace[0].Position.Filename,
			Line:     f.Trace[0].Position.Line,
			Title:    title,
		}
		if f.FixedVersion != "" {
			fd.Suggestion = "Upgrade to " + f.FixedVersion + " or later."
		}
		out = append(out, fd)
	}
	return out, nil
}
