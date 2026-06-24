package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type KubeScore struct{}

func NewKubeScore() *KubeScore        { return &KubeScore{} }
func (k *KubeScore) Name() string     { return "kube-score" }
func (k *KubeScore) Available() bool  { return scanners.BinaryAvailable("kube-score") }
func (k *KubeScore) Stacks() []string { return []string{"kubernetes", "helm"} }

func (k *KubeScore) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	files := k8sManifestsFromDiff(in)
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{"score"}, files...)
	args = append(args, "--output-format", "json")
	out, err := scanners.RunJSON(ctx, "kube-score", args...)
	if err != nil {
		return nil, err
	}
	return parseKubeScore(out)
}

func parseKubeScore(b []byte) ([]models.Finding, error) {
	var docs []struct {
		FileName string `json:"file_name"`
		FileRow  int    `json:"file_row"`
		Checks   []struct {
			Check struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"check"`
			Grade    int  `json:"grade"`
			Skipped  bool `json:"skipped"`
			Comments []struct {
				Summary     string `json:"summary"`
				Description string `json:"description"`
			} `json:"comments"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(b, &docs); err != nil {
		return nil, fmt.Errorf("kube-score: %w", err)
	}
	var out []models.Finding
	for _, d := range docs {
		for _, c := range d.Checks {
			if c.Skipped || c.Grade >= 10 {
				continue // 10 = all OK
			}
			title := c.Check.Name
			if len(c.Comments) > 0 && c.Comments[0].Summary != "" {
				title = c.Check.Name + ": " + c.Comments[0].Summary
			}
			out = append(out, models.Finding{
				RuleID:   "kube-score." + c.Check.ID,
				Origin:   "kube-score",
				Severity: kubeScoreGrade(c.Grade),
				File:     d.FileName,
				Line:     d.FileRow,
				Title:    title,
			})
		}
	}
	return out, nil
}

// kubeScoreGrade maps kube-score grades (1 critical, 5 warning, 7 almost-ok,
// 10 ok) to our severity taxonomy.
func kubeScoreGrade(grade int) models.Severity {
	switch {
	case grade >= 7:
		return models.SeverityMedium
	case grade >= 5:
		return models.SeverityHigh
	default:
		return models.SeverityCritical
	}
}

// k8sManifestsFromDiff returns added/modified Kubernetes manifest paths from the
// diff, excluding Helm chart metadata, kustomize files, and CI workflows.
func k8sManifestsFromDiff(in scanners.ScanInput) []string {
	if in.Diff == nil {
		return nil
	}
	var out []string
	for _, f := range in.Diff.Files {
		if f.Status == "deleted" {
			continue
		}
		if !strings.HasSuffix(f.Path, ".yaml") && !strings.HasSuffix(f.Path, ".yml") {
			continue
		}
		if strings.Contains(f.Path, ".github/") {
			continue
		}
		base := f.Path
		if i := strings.LastIndexByte(base, '/'); i >= 0 {
			base = base[i+1:]
		}
		switch {
		case base == "Chart.yaml", base == "values.yaml", strings.HasPrefix(base, "values-"):
			continue
		case base == "kustomization.yaml", base == "kustomization.yml":
			continue
		}
		out = append(out, f.Path)
	}
	return out
}
