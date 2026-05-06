package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Hadolint struct{}

func NewHadolint() *Hadolint                                { return &Hadolint{} }
func (h *Hadolint) Name() string                            { return "hadolint" }
func (h *Hadolint) Available() bool                         { return scanners.BinaryAvailable("hadolint") }
func (h *Hadolint) Stacks() []string                        { return []string{"docker", "dockerfile"} }

func (h *Hadolint) Scan(ctx context.Context, in scanners.ScanInput) ([]models.Finding, error) {
	files := dockerfilesFromDiff(in)
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{"-f", "json"}, files...)
	out, err := scanners.RunJSON(ctx, "hadolint", args...)
	if err != nil {
		return nil, err
	}
	return parseHadolint(out)
}

func dockerfilesFromDiff(in scanners.ScanInput) []string {
	if in.Diff == nil {
		return nil
	}
	var out []string
	for _, f := range in.Diff.Files {
		if f.Status == "deleted" {
			continue
		}
		base := f.Path
		if i := lastSlash(base); i >= 0 {
			base = base[i+1:]
		}
		if base == "Dockerfile" || hasSuffix(base, ".dockerfile") {
			out = append(out, f.Path)
		}
	}
	return out
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func parseHadolint(b []byte) ([]models.Finding, error) {
	var arr []struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Level   string `json:"level"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, fmt.Errorf("hadolint: %w", err)
	}
	out := make([]models.Finding, 0, len(arr))
	for _, e := range arr {
		out = append(out, models.Finding{
			RuleID:   "hadolint." + e.Code,
			Origin:   "hadolint",
			Severity: scanners.MapSeverity(e.Level),
			File:     e.File,
			Line:     e.Line,
			Title:    e.Message,
		})
	}
	return out, nil
}
