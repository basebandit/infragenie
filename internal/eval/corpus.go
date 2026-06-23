package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/pkg/models"
	"gopkg.in/yaml.v3"
)

// CorpusCase is a live evaluation case: real manifests, the Golden Path to review
// them against, and the findings we expect. Unlike Fixture (which replays a
// recorded run), the corpus runs the reviewers live, so it catches reviewer drift
// and yields real precision/recall numbers for the deterministic reviewers.
type CorpusCase struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Thresholds  Thresholds        `yaml:"thresholds"`
	GoldenPath  models.GoldenPath `yaml:"goldenpath"`
	Manifests   map[string]string `yaml:"manifests"`
	Expected    []Expected        `yaml:"expected"`
}

// LoadCorpus reads every .yaml case under dir.
func LoadCorpus(dir string) ([]*CorpusCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && (filepath.Ext(e.Name()) == ".yaml" || filepath.Ext(e.Name()) == ".yml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	out := make([]*CorpusCase, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		var c CorpusCase
		if err := yaml.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		if c.Thresholds == (Thresholds{}) {
			// Match on rule_id + file; line is not the discriminator for the corpus.
			c.Thresholds = Thresholds{MinPrecision: 1.0, MinRecall: 1.0, LineTolerance: 1 << 30}
		}
		out = append(out, &c)
	}
	return out, nil
}

// Run reviews the case's manifests with the deterministic Layer-3 reviewers.
func (c *CorpusCase) Run(ctx context.Context) ([]models.Finding, error) {
	names := make([]string, 0, len(c.Manifests))
	for n := range c.Manifests {
		names = append(names, n)
	}
	sort.Strings(names)

	files := make([]models.FileDiff, 0, len(names))
	for _, n := range names {
		files = append(files, models.FileDiff{Path: n, Status: "added", NewContent: c.Manifests[n]})
	}

	gp := c.GoldenPath
	in := reviewers.ReviewInput{Diff: &models.Diff{Source: models.DiffSourceLocal, Files: files}, GoldenPath: &gp}

	var all []models.Finding
	for _, r := range []reviewers.Reviewer{
		reviewers.GoldenPathReviewer{},
		reviewers.ReliabilityReviewer{},
		reviewers.ConventionsReviewer{},
	} {
		fs, err := r.Review(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", c.Name, r.Name(), err)
		}
		all = append(all, fs...)
	}
	return all, nil
}

// Score runs the case and compares against its expected findings.
func (c *CorpusCase) Score(ctx context.Context) (Metrics, bool, error) {
	actual, err := c.Run(ctx)
	if err != nil {
		return Metrics{}, false, err
	}
	m := Match(c.Expected, actual, c.Thresholds.LineTolerance)
	pass := m.Precision() >= c.Thresholds.MinPrecision && m.Recall() >= c.Thresholds.MinRecall
	return m, pass, nil
}
