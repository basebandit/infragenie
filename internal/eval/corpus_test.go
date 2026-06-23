package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLiveCorpus runs the deterministic reviewers against real manifests and
// gates per-case precision/recall. Unlike the recorded-fixture test, this
// exercises the reviewers live, so it both prevents drift and produces the
// published precision/recall numbers in the logged summary.
func TestLiveCorpus(t *testing.T) {
	cases, err := LoadCorpus("testdata/corpus")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "ship at least one corpus case")

	ctx := context.Background()
	var agg Metrics
	for _, c := range cases {
		m, pass, err := c.Score(ctx)
		require.NoErrorf(t, err, "case %q", c.Name)
		agg = Sum(agg, m)
		require.Truef(t, pass,
			"case %q below thresholds: precision=%.2f recall=%.2f (min %.2f/%.2f) tp=%d fp=%d fn=%d",
			c.Name, m.Precision(), m.Recall(),
			c.Thresholds.MinPrecision, c.Thresholds.MinRecall, m.TP, m.FP, m.FN)
	}
	t.Logf("LIVE CORPUS: cases=%d tp=%d fp=%d fn=%d precision=%.3f recall=%.3f f1=%.3f",
		len(cases), agg.TP, agg.FP, agg.FN, agg.Precision(), agg.Recall(), agg.F1())
}
