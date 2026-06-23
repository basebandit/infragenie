package reporter

import (
	"bytes"
	"encoding/json"
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

func TestWriteSARIF(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, sampleFindings, nil, FormatSARIF))

	var report struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string                `json:"ruleId"`
				Level     string                `json:"level"`
				Message   struct{ Text string } `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region *struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &report), "SARIF must be valid JSON")

	require.Equal(t, "2.1.0", report.Version)
	require.Len(t, report.Runs, 1)
	run := report.Runs[0]
	require.Equal(t, "infragenie", run.Tool.Driver.Name)
	require.Len(t, run.Results, 2)
	require.Len(t, run.Tool.Driver.Rules, 2, "rules deduped per ruleId")

	r0 := run.Results[0]
	require.Equal(t, "checkov.CKV_K8S_14", r0.RuleID)
	require.Equal(t, "error", r0.Level) // high → error
	require.Equal(t, "charts/payments/deployment.yaml", r0.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	require.NotNil(t, r0.Locations[0].PhysicalLocation.Region)
	require.Equal(t, 12, r0.Locations[0].PhysicalLocation.Region.StartLine)
	require.Equal(t, "warning", run.Results[1].Level) // medium → warning
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
