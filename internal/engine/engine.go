// Package engine orchestrates the three review layers (scanners → grounding →
// reviewers), enforces trust gating, and applies suppressions from the
// resolved Golden Path.
package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/internal/grounding"
	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
)

type Config struct {
	GoldenPath *models.GoldenPath

	Scanners  []scanners.Scanner
	Grounder  grounding.Grounder
	Reviewers []reviewers.Reviewer

	MinGroundSeverity models.Severity // default: medium
	MinConfidence     float64         // default: 0.85
	StrictEvidence    bool
}

type Input struct {
	Diff    *models.Diff
	RepoCtx *repo.Context
}

type Result struct {
	Findings []models.Finding
	Skipped  []string
}

type Engine struct{ cfg Config }

func New(cfg Config) *Engine {
	if cfg.MinGroundSeverity == "" {
		cfg.MinGroundSeverity = models.SeverityMedium
	}
	if cfg.MinConfidence == 0 {
		cfg.MinConfidence = 0.85
	}
	return &Engine{cfg: cfg}
}

func (e *Engine) Run(ctx context.Context, in Input) (*Result, error) {
	if in.Diff == nil {
		return nil, errors.New("engine: diff is required")
	}
	var (
		all     []models.Finding
		skipped []string
	)

	all, skipped = e.runScanners(ctx, in, all, skipped)
	if e.cfg.Grounder != nil {
		all = e.applyGrounding(ctx, all, in)
	}
	all, skipped = e.runReviewers(ctx, in, all, skipped)

	all = e.filterTrust(all)
	all = e.applySuppressions(all)

	return &Result{Findings: all, Skipped: skipped}, nil
}

func (e *Engine) runScanners(ctx context.Context, in Input, all []models.Finding, skipped []string) ([]models.Finding, []string) {
	for _, s := range e.cfg.Scanners {
		if !s.Available() {
			skipped = append(skipped, fmt.Sprintf("scanner:%s (unavailable)", s.Name()))
			continue
		}
		fs, err := s.Scan(ctx, scanners.ScanInput{Diff: in.Diff, RepoCtx: in.RepoCtx})
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("scanner:%s (error: %v)", s.Name(), err))
			continue
		}
		for i := range fs {
			fs[i].Source = models.SourceScanner
			fs[i].TrustLevel = "T1"
			fs[i].Confidence = 1.0
			if fs[i].Origin == "" {
				fs[i].Origin = s.Name()
			}
		}
		all = append(all, fs...)
	}
	return all, skipped
}

func (e *Engine) runReviewers(ctx context.Context, in Input, all []models.Finding, skipped []string) ([]models.Finding, []string) {
	for _, r := range e.cfg.Reviewers {
		fs, err := r.Review(ctx, reviewers.ReviewInput{
			Diff:       in.Diff,
			RepoCtx:    in.RepoCtx,
			GoldenPath: e.cfg.GoldenPath,
		})
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("reviewer:%s (error: %v)", r.Name(), err))
			continue
		}
		for i := range fs {
			fs[i].Source = models.SourceReviewer
			if fs[i].TrustLevel == "" {
				fs[i].TrustLevel = "T3"
			}
			if fs[i].Origin == "" {
				fs[i].Origin = r.Name()
			}
		}
		all = append(all, fs...)
	}
	return all, skipped
}

func (e *Engine) applyGrounding(ctx context.Context, fs []models.Finding, in Input) []models.Finding {
	contents := contentMap(in.Diff)
	minRank := models.SeverityRank(e.cfg.MinGroundSeverity)
	for i := range fs {
		if fs[i].Source != models.SourceScanner {
			continue
		}
		if models.SeverityRank(fs[i].Severity) < minRank {
			continue
		}
		ex, sg, err := e.cfg.Grounder.Ground(ctx, fs[i], contents[fs[i].File])
		if err != nil {
			continue
		}
		fs[i].Explanation = ex
		fs[i].Suggestion = sg
		fs[i].Source = models.SourceGrounding
		fs[i].TrustLevel = "T2"
	}
	return fs
}

func contentMap(d *models.Diff) map[string]string {
	if d == nil {
		return nil
	}
	out := make(map[string]string, len(d.Files))
	for _, f := range d.Files {
		out[f.Path] = f.NewContent
	}
	return out
}

func (e *Engine) filterTrust(fs []models.Finding) []models.Finding {
	out := fs[:0]
	for _, f := range fs {
		if f.TrustLevel == "T3" {
			if f.Confidence < e.cfg.MinConfidence {
				continue
			}
			if e.cfg.StrictEvidence && f.Evidence == "" {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

func (e *Engine) applySuppressions(fs []models.Finding) []models.Finding {
	if e.cfg.GoldenPath == nil || len(e.cfg.GoldenPath.Ignore) == 0 {
		return fs
	}
	out := fs[:0]
	for _, f := range fs {
		if !suppressed(f, e.cfg.GoldenPath.Ignore) {
			out = append(out, f)
		}
	}
	return out
}

func suppressed(f models.Finding, rules []models.IgnoreRule) bool {
	for _, r := range rules {
		if !pathMatches(r.Path, f.File) {
			continue
		}
		for _, rule := range r.Rules {
			if rule == "*" || rule == f.RuleID {
				return true
			}
			if strings.HasSuffix(rule, "*") && strings.HasPrefix(f.RuleID, strings.TrimSuffix(rule, "*")) {
				return true
			}
		}
	}
	return false
}

// pathMatches: minimal `**` glob — supports `dir/**` and `*` only. Phase F+
// can switch to a real glob lib if needed.
func pathMatches(pattern, path string) bool {
	pattern = strings.TrimSuffix(pattern, "/**")
	pattern = strings.TrimSuffix(pattern, "/*")
	if pattern == "" || pattern == "*" {
		return true
	}
	return strings.HasPrefix(path, pattern)
}
