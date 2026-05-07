package reporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

var sampleFindings = []models.Finding{
	{
		RuleID:      "checkov.CKV_K8S_14",
		Source:      models.SourceScanner,
		Severity:    models.SeverityHigh,
		File:        "charts/payments/deployment.yaml",
		Line:        12,
		Title:       "image uses :latest tag",
		Explanation: "Unpinned image breaks reproducibility.",
		Suggestion:  "-image: payments:latest\n+image: payments:1.2.3",
		Evidence:    "image: payments:latest",
		EvidenceLoc: "charts/payments/deployment.yaml:12",
		TrustLevel:  "T1",
		Confidence:  1.0,
	},
	{
		RuleID:     "reliability.single-replica",
		Source:     models.SourceReviewer,
		Severity:   models.SeverityMedium,
		File:       "charts/payments/deployment.yaml",
		Line:       5,
		Title:      "single replica — no high availability",
		TrustLevel: "T3",
		Confidence: 0.95,
	},
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, sampleFindings, nil, FormatText))
	out := buf.String()
	require.Contains(t, out, "HIGH")
	require.Contains(t, out, "checkov.CKV_K8S_14")
	require.Contains(t, out, "charts/payments/deployment.yaml:12")
	require.Contains(t, out, "total: 2 finding(s)")
}

func TestWriteText_NoFindings(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, nil, nil, FormatText))
	require.Contains(t, buf.String(), "No findings.")
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, sampleFindings, []string{"tflint"}, FormatJSON))
	out := buf.String()
	require.Contains(t, out, `"total": 2`)
	require.Contains(t, out, `"skipped"`)
	require.Contains(t, out, `"tflint"`)
	require.Contains(t, out, `"rule_id": "checkov.CKV_K8S_14"`)
}

func TestWriteGitHub(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, sampleFindings, nil, FormatGitHub))
	out := buf.String()
	require.Contains(t, out, "::error file=charts/payments/deployment.yaml,line=12")
	require.Contains(t, out, "::warning file=charts/payments/deployment.yaml")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
}

func TestExitCode(t *testing.T) {
	require.Equal(t, 1, ExitCode(sampleFindings, models.SeverityHigh))
	require.Equal(t, 0, ExitCode(sampleFindings, models.SeverityCritical))
	require.Equal(t, 1, ExitCode(sampleFindings, models.SeverityMedium))
	require.Equal(t, 0, ExitCode(nil, models.SeverityLow))
}
