package infra

import (
	"testing"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const kubeScoreSample = `[
  {
    "file_name": "deploy.yaml",
    "file_row": 1,
    "checks": [
      {"check": {"id":"container-resources","name":"Container Resources"}, "grade": 1, "skipped": false,
       "comments": [{"summary":"CPU limit is not set","description":"..."}]},
      {"check": {"id":"pod-probes","name":"Pod Probes"}, "grade": 7, "skipped": false, "comments": []},
      {"check": {"id":"stable-version","name":"Stable version"}, "grade": 10, "skipped": false, "comments": []},
      {"check": {"id":"skipped-one","name":"Skipped"}, "grade": 1, "skipped": true, "comments": []}
    ]
  }
]`

func TestParseKubeScore(t *testing.T) {
	fs, err := parseKubeScore([]byte(kubeScoreSample))
	require.NoError(t, err)
	require.Len(t, fs, 2) // grade 10 and skipped are dropped

	require.Equal(t, "kube-score.container-resources", fs[0].RuleID)
	require.Equal(t, models.SeverityCritical, fs[0].Severity) // grade 1
	require.Equal(t, "deploy.yaml", fs[0].File)
	require.Equal(t, 1, fs[0].Line)
	require.Contains(t, fs[0].Title, "CPU limit is not set")

	require.Equal(t, "kube-score.pod-probes", fs[1].RuleID)
	require.Equal(t, models.SeverityMedium, fs[1].Severity) // grade 7
}

func TestK8sManifestsFromDiff(t *testing.T) {
	in := scanners.ScanInput{Diff: &models.Diff{Files: []models.FileDiff{
		{Path: "k8s/deploy.yaml", Status: "added"},
		{Path: "charts/app/Chart.yaml", Status: "added"},
		{Path: "charts/app/values.yaml", Status: "added"},
		{Path: ".github/workflows/ci.yml", Status: "added"},
		{Path: "k8s/old.yaml", Status: "deleted"},
		{Path: "main.go", Status: "added"},
	}}}
	require.Equal(t, []string{"k8s/deploy.yaml"}, k8sManifestsFromDiff(in))
}
