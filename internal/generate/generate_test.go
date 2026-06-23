package generate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/basebandit/infragenie/internal/generate"
	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testGoldenPath exercises labels, all security rules, observability, and CI steps.
func testGoldenPath() *models.GoldenPath {
	return &models.GoldenPath{
		Version: 1,
		Name:    "test-golden-path",
		RequiredLabels: []string{
			"app.kubernetes.io/name",
			"app.kubernetes.io/version",
			"app.kubernetes.io/component",
			"team",
			"cost-centre",
		},
		Security: models.Security{
			RequireNonRoot:        true,
			RequireReadOnlyRootFS: true,
			RequireNetworkPolicy:  true,
			ForbidImageTagLatest:  true,
		},
		Observability: models.Observability{RequirePrometheusAnnotations: true},
		CIRequired: []models.CIStep{
			{Name: "tests", Matches: []string{"go test"}},
			{Name: "vulnerability-scan", Matches: []string{"trivy"}},
		},
		RuntimeRules: map[string]map[string]any{
			"require-owner-annotation": {"pattern": "owner:"},
		},
	}
}

// TestGenerateConformsToGoldenPath is the headline contract: a service rendered
// from a Golden Path passes that same Golden Path's deterministic reviewers with
// zero findings.
func TestGenerateConformsToGoldenPath(t *testing.T) {
	gp := testGoldenPath()
	dir := t.TempDir()

	res, err := generate.New().Run(generate.Params{
		Name:       "demo",
		Template:   "k8s-service",
		OutDir:     dir,
		GoldenPath: gp,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Files)

	d := diffFromFiles(t, dir, res.Files)

	revs := []reviewers.Reviewer{
		reviewers.GoldenPathReviewer{},
		reviewers.ReliabilityReviewer{},
		reviewers.ConventionsReviewer{},
	}
	for _, r := range revs {
		findings, err := r.Review(context.Background(), reviewers.ReviewInput{Diff: d, GoldenPath: gp})
		require.NoError(t, err)
		assert.Emptyf(t, findings, "reviewer %q produced findings: %+v", r.Name(), findings)
	}
}

func TestGenerateRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	p := generate.Params{Name: "demo", OutDir: dir, GoldenPath: testGoldenPath()}

	_, err := generate.New().Run(p)
	require.NoError(t, err)

	_, err = generate.New().Run(p)
	require.Error(t, err, "second run without --force must fail")

	p.Force = true
	_, err = generate.New().Run(p)
	require.NoError(t, err, "second run with Force must succeed")
}

func TestGenerateUnknownTemplate(t *testing.T) {
	_, err := generate.New().Run(generate.Params{Name: "demo", Template: "nope", OutDir: t.TempDir()})
	require.Error(t, err)
}

func TestGenerateNilGoldenPath(t *testing.T) {
	res, err := generate.New().Run(generate.Params{Name: "demo", OutDir: t.TempDir()})
	require.NoError(t, err)
	require.NotEmpty(t, res.Files)
}

// diffFromFiles reads generated files and builds an added-file diff, mirroring
// how the engine sees a PR that introduces a new service.
func diffFromFiles(t *testing.T, root string, files []string) *models.Diff {
	t.Helper()
	var fds []models.FileDiff
	for _, abs := range files {
		b, err := os.ReadFile(abs)
		require.NoError(t, err)
		rel, err := filepath.Rel(root, abs)
		require.NoError(t, err)
		fds = append(fds, models.FileDiff{
			Path:       rel,
			Status:     "added",
			NewContent: string(b),
		})
	}
	return &models.Diff{Source: models.DiffSourceLocal, Files: fds}
}
