package infra

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const checkovSample = `{
  "results": {
    "failed_checks": [
      {
        "check_id": "CKV_K8S_10",
        "check_name": "CPU limits should be set",
        "file_path": "/k8s/deployment.yaml",
        "file_line_range": [12, 20],
        "severity": "MEDIUM",
        "guideline": "https://docs.checkov.io/CKV_K8S_10"
      },
      {
        "check_id": "CKV_K8S_38",
        "check_name": "Service account",
        "file_path": "/k8s/sa.yaml",
        "file_line_range": [1, 5],
        "severity": null,
        "guideline": ""
      }
    ]
  }
}`

func TestParseCheckov(t *testing.T) {
	fs, err := parseCheckov([]byte(checkovSample))
	require.NoError(t, err)
	require.Len(t, fs, 2)

	require.Equal(t, "checkov.CKV_K8S_10", fs[0].RuleID)
	require.Equal(t, "checkov", fs[0].Origin)
	require.Equal(t, models.SeverityMedium, fs[0].Severity)
	require.Equal(t, "k8s/deployment.yaml", fs[0].File)
	require.Equal(t, 12, fs[0].Line)
	require.Equal(t, []string{"https://docs.checkov.io/CKV_K8S_10"}, fs[0].References)

	require.Equal(t, models.SeverityMedium, fs[1].Severity) // null severity → default
	require.Empty(t, fs[1].References)
}
