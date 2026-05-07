// Package reporter formats engine findings for different output targets.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

type Format string

const (
	FormatText   Format = "text"
	FormatJSON   Format = "json"
	FormatGitHub Format = "github"
)

// Write renders findings to w in the requested format.
func Write(w io.Writer, findings []models.Finding, skipped []string, format Format) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, findings, skipped)
	case FormatGitHub:
		return writeGitHub(w, findings)
	default:
		return writeText(w, findings, skipped)
	}
}

// ExitCode returns 1 when any finding meets or exceeds minSeverity.
func ExitCode(findings []models.Finding, minSeverity models.Severity) int {
	min := models.SeverityRank(minSeverity)
	for _, f := range findings {
		if models.SeverityRank(f.Severity) >= min {
			return 1
		}
	}
	return 0
}

// ── text ──────────────────────────────────────────────────────────────────────

func writeText(w io.Writer, findings []models.Finding, skipped []string) error {
	if len(findings) == 0 {
		fmt.Fprintln(w, "No findings.")
		return nil
	}
	for _, f := range findings {
		sev := severityTag(f.Severity)
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(w, "%s  [%s]  %s\n", sev, f.RuleID, loc)
		fmt.Fprintf(w, "       %s\n", f.Title)
		if f.Explanation != "" {
			fmt.Fprintf(w, "       %s\n", indent(f.Explanation, 7))
		}
		if f.Suggestion != "" {
			fmt.Fprintf(w, "  fix: %s\n", indent(f.Suggestion, 7))
		}
		if f.Evidence != "" {
			fmt.Fprintf(w, "  at:  %s (%s)\n", f.Evidence, f.EvidenceLoc)
		}
		fmt.Fprintln(w)
	}
	if len(skipped) > 0 {
		fmt.Fprintf(w, "skipped: %s\n", strings.Join(skipped, ", "))
	}
	fmt.Fprintf(w, "total: %d finding(s)\n", len(findings))
	return nil
}

func severityTag(s models.Severity) string {
	switch s {
	case models.SeverityCritical:
		return "CRIT "
	case models.SeverityHigh:
		return "HIGH "
	case models.SeverityMedium:
		return "MED  "
	case models.SeverityLow:
		return "LOW  "
	default:
		return "INFO "
	}
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return strings.ReplaceAll(s, "\n", "\n"+pad)
}

// ── JSON ──────────────────────────────────────────────────────────────────────

type jsonReport struct {
	Findings []models.Finding `json:"findings"`
	Skipped  []string         `json:"skipped,omitempty"`
	Total    int              `json:"total"`
}

func writeJSON(w io.Writer, findings []models.Finding, skipped []string) error {
	if findings == nil {
		findings = []models.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonReport{
		Findings: findings,
		Skipped:  skipped,
		Total:    len(findings),
	})
}

// ── GitHub Actions annotations ────────────────────────────────────────────────

// writeGitHub emits GitHub Actions workflow commands so findings appear as
// inline PR annotations. See:
// https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions
func writeGitHub(w io.Writer, findings []models.Finding) error {
	for _, f := range findings {
		level := ghLevel(f.Severity)
		msg := f.Title
		if f.Explanation != "" {
			msg += " — " + strings.ReplaceAll(f.Explanation, "\n", " ")
		}
		if f.Line > 0 {
			fmt.Fprintf(w, "::%s file=%s,line=%d,title=%s::%s\n",
				level, f.File, f.Line, f.RuleID, msg)
		} else {
			fmt.Fprintf(w, "::%s file=%s,title=%s::%s\n",
				level, f.File, f.RuleID, msg)
		}
	}
	return nil
}

func ghLevel(s models.Severity) string {
	switch s {
	case models.SeverityCritical, models.SeverityHigh:
		return "error"
	case models.SeverityMedium:
		return "warning"
	default:
		return "notice"
	}
}
