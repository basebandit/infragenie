package engine

import (
	"context"
	"testing"

	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

type stubScanner struct {
	name      string
	available bool
	findings  []models.Finding
}

func (s stubScanner) Name() string    { return s.name }
func (s stubScanner) Available() bool { return s.available }
func (s stubScanner) Scan(_ context.Context, _ scanners.ScanInput) ([]models.Finding, error) {
	return s.findings, nil
}

type stubReviewer struct {
	name     string
	findings []models.Finding
}

func (r stubReviewer) Name() string { return r.name }
func (r stubReviewer) Review(_ context.Context, _ reviewers.ReviewInput) ([]models.Finding, error) {
	return r.findings, nil
}

func TestRunMarksScannerFindingsT1(t *testing.T) {
	e := New(Config{
		Scanners: []scanners.Scanner{stubScanner{
			name: "checkov", available: true,
			findings: []models.Finding{{RuleID: "x", Severity: models.SeverityHigh, File: "a.yml"}},
		}},
	})
	res, err := e.Run(context.Background(), Input{Diff: &models.Diff{}})
	require.NoError(t, err)
	require.Len(t, res.Findings, 1)
	require.Equal(t, models.SourceScanner, res.Findings[0].Source)
	require.Equal(t, "T1", res.Findings[0].TrustLevel)
	require.Equal(t, 1.0, res.Findings[0].Confidence)
	require.Equal(t, "checkov", res.Findings[0].Origin)
}

func TestRunSkipsUnavailableScanner(t *testing.T) {
	e := New(Config{
		Scanners: []scanners.Scanner{stubScanner{name: "tflint", available: false}},
	})
	res, err := e.Run(context.Background(), Input{Diff: &models.Diff{}})
	require.NoError(t, err)
	require.Empty(t, res.Findings)
	require.Contains(t, res.Skipped[0], "tflint")
}

func TestFilterT3ByConfidenceAndEvidence(t *testing.T) {
	e := New(Config{
		MinConfidence:  0.9,
		StrictEvidence: true,
		Reviewers: []reviewers.Reviewer{stubReviewer{
			name: "goldenpath",
			findings: []models.Finding{
				{RuleID: "low", TrustLevel: "T3", Confidence: 0.5},
				{RuleID: "noev", TrustLevel: "T3", Confidence: 0.95},
				{RuleID: "good", TrustLevel: "T3", Confidence: 0.95, Evidence: "labels: [app]", EvidenceLoc: "x.yaml:3"},
			},
		}},
	})
	res, err := e.Run(context.Background(), Input{Diff: &models.Diff{}})
	require.NoError(t, err)
	require.Len(t, res.Findings, 1)
	require.Equal(t, "good", res.Findings[0].RuleID)
}

func TestApplySuppressions(t *testing.T) {
	e := New(Config{
		GoldenPath: &models.GoldenPath{
			Ignore: []models.IgnoreRule{
				{Path: "examples/**", Rules: []string{"*"}},
				{Path: "charts/legacy", Rules: []string{"goldenpath.*"}},
			},
		},
		Scanners: []scanners.Scanner{stubScanner{
			name: "checkov", available: true,
			findings: []models.Finding{
				{RuleID: "x", File: "examples/foo.yml", Severity: models.SeverityHigh},
				{RuleID: "goldenpath.required-label", File: "charts/legacy/deploy.yaml", Severity: models.SeverityHigh},
				{RuleID: "y", File: "charts/payments/deploy.yaml", Severity: models.SeverityHigh},
			},
		}},
	})
	res, err := e.Run(context.Background(), Input{Diff: &models.Diff{}})
	require.NoError(t, err)
	require.Len(t, res.Findings, 1)
	require.Equal(t, "y", res.Findings[0].RuleID)
}

func TestRequiresDiff(t *testing.T) {
	_, err := New(Config{}).Run(context.Background(), Input{})
	require.ErrorContains(t, err, "diff is required")
}
