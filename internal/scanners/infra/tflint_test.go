package infra

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const tflintSample = `{
  "issues": [
    {
      "rule": {"name":"terraform_unused_declarations","severity":"warning","link":"https://example.com/r1"},
      "message":"variable \"region\" is declared but not used",
      "range": {"filename":"main.tf","start":{"line":3,"column":1}}
    },
    {
      "rule": {"name":"aws_instance_invalid_type","severity":"error","link":""},
      "message":"invalid instance type",
      "range": {"filename":"ec2.tf","start":{"line":12,"column":3}}
    }
  ],
  "errors": []
}`

func TestParseTFLint(t *testing.T) {
	fs, err := parseTFLint([]byte(tflintSample))
	require.NoError(t, err)
	require.Len(t, fs, 2)

	require.Equal(t, "tflint.terraform_unused_declarations", fs[0].RuleID)
	require.Equal(t, models.SeverityMedium, fs[0].Severity) // warning
	require.Equal(t, "main.tf", fs[0].File)
	require.Equal(t, 3, fs[0].Line)
	require.Equal(t, []string{"https://example.com/r1"}, fs[0].References)

	require.Equal(t, "tflint.aws_instance_invalid_type", fs[1].RuleID)
	require.Equal(t, models.SeverityHigh, fs[1].Severity) // error
	require.Nil(t, fs[1].References)
}
