// Package infra holds Layer-1 adapters for IaC scanners (checkov,
// kube-score, kubeconform, tflint, hadolint, trivy config).
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Checkov struct{}

func NewCheckov() *Checkov                                  { return &Checkov{} }
func (c *Checkov) Name() string                             { return "checkov" }
func (c *Checkov) Available() bool                          { return scanners.BinaryAvailable("checkov") }
func (c *Checkov) Stacks() []string {
	return []string{"kubernetes", "helm", "terraform", "docker", "dockerfile", "serverless"}
}

func (c *Checkov) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	root := "."
	if in.RepoCtx != nil && in.RepoCtx.Root != "" {
		root = in.RepoCtx.Root
	}
	out, err := scanners.RunJSON(ctx, "checkov", "--directory", root, "--output", "json", "--quiet")
	if err != nil {
		return nil, err
	}
	return parseCheckov(out)
}

func parseCheckov(b []byte) ([]models.Finding, error) {
	var doc struct {
		Results struct {
			FailedChecks []struct {
				CheckID       string  `json:"check_id"`
				CheckName     string  `json:"check_name"`
				FilePath      string  `json:"file_path"`
				FileLineRange []int   `json:"file_line_range"`
				Severity      *string `json:"severity"`
				Guideline     string  `json:"guideline"`
			} `json:"failed_checks"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("checkov: %w", err)
	}
	out := make([]models.Finding, 0, len(doc.Results.FailedChecks))
	for _, f := range doc.Results.FailedChecks {
		sev := models.SeverityMedium
		if f.Severity != nil {
			sev = scanners.MapSeverity(*f.Severity)
		}
		line := 0
		if len(f.FileLineRange) > 0 {
			line = f.FileLineRange[0]
		}
		fd := models.Finding{
			RuleID:   "checkov." + f.CheckID,
			Origin:   "checkov",
			Severity: sev,
			File:     strings.TrimPrefix(f.FilePath, "/"),
			Line:     line,
			Title:    f.CheckName,
		}
		if f.Guideline != "" {
			fd.References = []string{f.Guideline}
		}
		out = append(out, fd)
	}
	return out, nil
}
