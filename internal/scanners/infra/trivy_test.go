package infra

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const trivySample = `{
  "Results": [
    {
      "Target": "deploy.yaml",
      "Class": "config",
      "Type": "kubernetes",
      "Misconfigurations": [
        {"ID":"KSV001","Title":"Process can elevate its own privileges","Severity":"HIGH","PrimaryURL":"https://avd.aquasec.com/KSV001","CauseMetadata":{"StartLine":15}},
        {"ID":"KSV012","Title":"Runs as root user","Severity":"CRITICAL","CauseMetadata":{"StartLine":9}}
      ]
    },
    {
      "Target": "clean.yaml",
      "Class": "config"
    }
  ]
}`

func TestParseTrivyConfig(t *testing.T) {
	fs, err := parseTrivyConfig([]byte(trivySample))
	require.NoError(t, err)
	require.Len(t, fs, 2)

	require.Equal(t, "trivy.KSV001", fs[0].RuleID)
	require.Equal(t, models.SeverityHigh, fs[0].Severity)
	require.Equal(t, "deploy.yaml", fs[0].File)
	require.Equal(t, 15, fs[0].Line)
	require.Equal(t, []string{"https://avd.aquasec.com/KSV001"}, fs[0].References)

	require.Equal(t, "trivy.KSV012", fs[1].RuleID)
	require.Equal(t, models.SeverityCritical, fs[1].Severity)
	require.Equal(t, 9, fs[1].Line)
}
