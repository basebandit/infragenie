package infra

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const kubeconformSample = `{
  "resources": [
    {"filename":"deploy.yaml","kind":"Deployment","name":"web","status":"statusValid","msg":""},
    {"filename":"bad.yaml","kind":"Deplyoment","name":"typo","status":"statusError","msg":"failed to parse kind"},
    {"filename":"invalid.yaml","kind":"Service","name":"svc","status":"statusInvalid","msg":"missing required field spec.ports"},
    {"filename":"skip.yaml","kind":"Custom","name":"c","status":"statusSkipped","msg":""}
  ],
  "summary": {"valid":1,"invalid":1,"errors":1,"skipped":1}
}`

func TestParseKubeconform(t *testing.T) {
	fs, err := parseKubeconform([]byte(kubeconformSample))
	require.NoError(t, err)
	require.Len(t, fs, 2) // valid and skipped are dropped

	require.Equal(t, "kubeconform.schema-validation", fs[0].RuleID)
	require.Equal(t, models.SeverityMedium, fs[0].Severity) // statusError
	require.Equal(t, "bad.yaml", fs[0].File)
	require.Contains(t, fs[0].Title, "failed to parse kind")

	require.Equal(t, models.SeverityHigh, fs[1].Severity) // statusInvalid
	require.Equal(t, "invalid.yaml", fs[1].File)
}
