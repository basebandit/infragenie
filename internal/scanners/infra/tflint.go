package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type TFLint struct{}

func NewTFLint() *TFLint           { return &TFLint{} }
func (t *TFLint) Name() string     { return "tflint" }
func (t *TFLint) Available() bool  { return scanners.BinaryAvailable("tflint") }
func (t *TFLint) Stacks() []string { return []string{"terraform"} }

func (t *TFLint) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	root := "."
	if in.RepoCtx != nil && in.RepoCtx.Root != "" {
		root = in.RepoCtx.Root
	}
	out, err := scanners.RunJSON(ctx, "tflint", "--chdir", root, "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseTFLint(out)
}

func parseTFLint(b []byte) ([]models.Finding, error) {
	var doc struct {
		Issues []struct {
			Rule struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
				Link     string `json:"link"`
			} `json:"rule"`
			Message string `json:"message"`
			Range   struct {
				Filename string `json:"filename"`
				Start    struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("tflint: %w", err)
	}
	out := make([]models.Finding, 0, len(doc.Issues))
	for _, i := range doc.Issues {
		fd := models.Finding{
			RuleID:   "tflint." + i.Rule.Name,
			Origin:   "tflint",
			Severity: scanners.MapSeverity(i.Rule.Severity),
			File:     i.Range.Filename,
			Line:     i.Range.Start.Line,
			Title:    i.Message,
		}
		if i.Rule.Link != "" {
			fd.References = []string{i.Rule.Link}
		}
		out = append(out, fd)
	}
	return out, nil
}
