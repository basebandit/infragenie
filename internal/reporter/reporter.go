// Package reporter formats engine findings for different output targets.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

type Format string

const (
	FormatText   Format = "text"
	FormatJSON   Format = "json"
	FormatGitHub Format = "github"
	FormatSARIF  Format = "sarif"
)

// Write renders findings to w in the requested format.
func Write(w io.Writer, findings []models.Finding, skipped []string, format Format) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, findings, skipped)
	case FormatGitHub:
		return writeGitHub(w, findings)
	case FormatSARIF:
		return writeSARIF(w, findings)
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

// ── SARIF 2.1.0 ─────────────────────────────────────────────────────────────────

// writeSARIF emits SARIF 2.1.0 so findings can be uploaded to GitHub code
// scanning (github/codeql-action/upload-sarif) and surfaced in the Security tab.
// Spec: https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html
func writeSARIF(w io.Writer, findings []models.Finding) error {
	rules := map[string]sarifRule{}
	results := make([]sarifResult, 0, len(findings))

	for _, f := range findings {
		if _, ok := rules[f.RuleID]; !ok {
			rules[f.RuleID] = sarifRule{
				ID:               f.RuleID,
				Name:             f.RuleID,
				ShortDescription: sarifText{Text: f.Title},
			}
		}
		loc := sarifLocation{}
		loc.PhysicalLocation.ArtifactLocation.URI = f.File
		if f.Line > 0 {
			loc.PhysicalLocation.Region = &sarifRegion{StartLine: f.Line}
		}
		results = append(results, sarifResult{
			RuleID:    f.RuleID,
			Level:     sarifLevel(f.Severity),
			Message:   sarifText{Text: sarifMessage(f)},
			Locations: []sarifLocation{loc},
		})
	}

	driverRules := make([]sarifRule, 0, len(rules))
	for _, r := range rules {
		driverRules = append(driverRules, r)
	}
	sort.Slice(driverRules, func(i, j int) bool { return driverRules[i].ID < driverRules[j].ID })

	report := sarifReport{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "infragenie",
				InformationURI: "https://github.com/basebandit/infragenie",
				Rules:          driverRules,
			}},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func sarifLevel(s models.Severity) string {
	switch s {
	case models.SeverityCritical, models.SeverityHigh:
		return "error"
	case models.SeverityMedium:
		return "warning"
	default:
		return "note"
	}
}

func sarifMessage(f models.Finding) string {
	msg := f.Title
	if f.Explanation != "" {
		msg += " — " + strings.ReplaceAll(f.Explanation, "\n", " ")
	}
	return msg
}

type sarifReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ShortDescription sarifText `json:"shortDescription"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region *sarifRegion `json:"region,omitempty"`
	} `json:"physicalLocation"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

type sarifText struct {
	Text string `json:"text"`
}
