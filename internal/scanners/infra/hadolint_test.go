package infra

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const hadolintSample = `[
  {"file":"Dockerfile","line":1,"column":1,"level":"warning","code":"DL3007","message":"Using latest is prone to errors"},
  {"file":"svc/Dockerfile","line":7,"column":1,"level":"error","code":"DL3025","message":"Use arguments JSON notation"}
]`

func TestParseHadolint(t *testing.T) {
	fs, err := parseHadolint([]byte(hadolintSample))
	require.NoError(t, err)
	require.Len(t, fs, 2)

	require.Equal(t, "hadolint.DL3007", fs[0].RuleID)
	require.Equal(t, models.SeverityMedium, fs[0].Severity)
	require.Equal(t, "Dockerfile", fs[0].File)
	require.Equal(t, 1, fs[0].Line)

	require.Equal(t, "hadolint.DL3025", fs[1].RuleID)
	require.Equal(t, models.SeverityHigh, fs[1].Severity)
}
