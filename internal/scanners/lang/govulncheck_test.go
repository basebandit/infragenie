package lang

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

// govulncheck streams concatenated JSON objects, not a single array.
const govulncheckSample = `{"osv":{"id":"GO-2024-0001","summary":"Denial of service in example/pkg"}}
{"progress":{"message":"Scanning..."}}
{"finding":{"osv":"GO-2024-0001","fixed_version":"v1.5.0","trace":[
  {"module":"example.com/m","package":"example.com/m/p","function":"Vuln","position":{"filename":"internal/svc/handler.go","line":42}},
  {"module":"example.com/m"}
]}}
{"finding":{"osv":"GO-2024-0001","trace":[{"module":"example.com/m"}]}}
{"osv":{"id":"GO-2024-0002","summary":"Imported but not called"}}
{"finding":{"osv":"GO-2024-0002","trace":[{"module":"x"}]}}`

func TestParseGovulncheck(t *testing.T) {
	fs, err := parseGovulncheck([]byte(govulncheckSample))
	require.NoError(t, err)
	require.Len(t, fs, 1) // only the called vuln with a positioned trace

	require.Equal(t, "govulncheck.GO-2024-0001", fs[0].RuleID)
	require.Equal(t, "govulncheck", fs[0].Origin)
	require.Equal(t, models.SeverityHigh, fs[0].Severity)
	require.Equal(t, "internal/svc/handler.go", fs[0].File)
	require.Equal(t, 42, fs[0].Line)
	require.Contains(t, fs[0].Title, "Denial of service")
	require.Contains(t, fs[0].Suggestion, "v1.5.0")
}

func TestParseGovulncheckEmpty(t *testing.T) {
	fs, err := parseGovulncheck([]byte(""))
	require.NoError(t, err)
	require.Empty(t, fs)
}
