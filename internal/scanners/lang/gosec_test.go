package lang

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const gosecSample = `{
  "Issues": [
    {"severity":"HIGH","confidence":"HIGH","rule_id":"G101","details":"Hardcoded credentials","file":"/repo/main.go","line":"42"},
    {"severity":"MEDIUM","confidence":"HIGH","rule_id":"G304","details":"Potential file inclusion","file":"/repo/util.go","line":"10-12"}
  ]
}`

func TestParseGosec(t *testing.T) {
	fs, err := parseGosec([]byte(gosecSample))
	require.NoError(t, err)
	require.Len(t, fs, 2)

	require.Equal(t, "gosec.G101", fs[0].RuleID)
	require.Equal(t, models.SeverityHigh, fs[0].Severity)
	require.Equal(t, "/repo/main.go", fs[0].File)
	require.Equal(t, 42, fs[0].Line)

	require.Equal(t, 10, fs[1].Line, "should parse leading int from line range")
}
