package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

// TrivyConfig wraps `trivy config`, Trivy's IaC misconfiguration scanner for
// Kubernetes, Terraform, Dockerfiles, and more.
type TrivyConfig struct{}

func NewTrivyConfig() *TrivyConfig     { return &TrivyConfig{} }
func (t *TrivyConfig) Name() string    { return "trivy-config" }
func (t *TrivyConfig) Available() bool { return scanners.BinaryAvailable("trivy") }
func (t *TrivyConfig) Stacks() []string {
	return []string{"kubernetes", "helm", "terraform", "docker", "dockerfile"}
}

func (t *TrivyConfig) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	root := "."
	if in.RepoCtx != nil && in.RepoCtx.Root != "" {
		root = in.RepoCtx.Root
	}
	out, err := scanners.RunJSON(ctx, "trivy", "config", "--quiet", "--format", "json", root)
	if err != nil {
		return nil, err
	}
	return parseTrivyConfig(out)
}

func parseTrivyConfig(b []byte) ([]models.Finding, error) {
	var doc struct {
		Results []struct {
			Target            string `json:"Target"`
			Misconfigurations []struct {
				ID            string `json:"ID"`
				Title         string `json:"Title"`
				Severity      string `json:"Severity"`
				PrimaryURL    string `json:"PrimaryURL"`
				CauseMetadata struct {
					StartLine int `json:"StartLine"`
				} `json:"CauseMetadata"`
			} `json:"Misconfigurations"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("trivy-config: %w", err)
	}
	var out []models.Finding
	for _, r := range doc.Results {
		for _, m := range r.Misconfigurations {
			fd := models.Finding{
				RuleID:   "trivy." + m.ID,
				Origin:   "trivy-config",
				Severity: scanners.MapSeverity(m.Severity),
				File:     r.Target,
				Line:     m.CauseMetadata.StartLine,
				Title:    m.Title,
			}
			if m.PrimaryURL != "" {
				fd.References = []string{m.PrimaryURL}
			}
			out = append(out, fd)
		}
	}
	return out, nil
}
