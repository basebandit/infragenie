package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Kubeconform struct{}

func NewKubeconform() *Kubeconform      { return &Kubeconform{} }
func (k *Kubeconform) Name() string     { return "kubeconform" }
func (k *Kubeconform) Available() bool  { return scanners.BinaryAvailable("kubeconform") }
func (k *Kubeconform) Stacks() []string { return []string{"kubernetes", "helm"} }

func (k *Kubeconform) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	files := k8sManifestsFromDiff(in)
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{"-output", "json", "-summary"}, files...)
	out, err := scanners.RunJSON(ctx, "kubeconform", args...)
	if err != nil {
		return nil, err
	}
	return parseKubeconform(out)
}

func parseKubeconform(b []byte) ([]models.Finding, error) {
	var doc struct {
		Resources []struct {
			Filename string `json:"filename"`
			Kind     string `json:"kind"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			Msg      string `json:"msg"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("kubeconform: %w", err)
	}
	var out []models.Finding
	for _, r := range doc.Resources {
		var sev models.Severity
		switch r.Status {
		case "statusInvalid":
			sev = models.SeverityHigh
		case "statusError":
			sev = models.SeverityMedium
		default:
			continue // valid or skipped
		}
		title := r.Msg
		if title == "" {
			title = fmt.Sprintf("%s %q failed schema validation", r.Kind, r.Name)
		}
		out = append(out, models.Finding{
			RuleID:   "kubeconform.schema-validation",
			Origin:   "kubeconform",
			Severity: sev,
			File:     r.Filename,
			Title:    title,
		})
	}
	return out, nil
}
