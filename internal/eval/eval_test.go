package eval

import (
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestMatchPerfect(t *testing.T) {
	exp := []Expected{{RuleID: "r", File: "a", Line: 10}}
	act := []models.Finding{{RuleID: "r", File: "a", Line: 10}}
	m := Match(exp, act, 0)
	require.Equal(t, Metrics{TP: 1}, m)
	require.Equal(t, 1.0, m.Precision())
	require.Equal(t, 1.0, m.Recall())
	require.Equal(t, 1.0, m.F1())
}

func TestMatchFalsePositive(t *testing.T) {
	m := Match(nil, []models.Finding{{RuleID: "r", File: "a", Line: 10}}, 0)
	require.Equal(t, Metrics{FP: 1}, m)
	require.Equal(t, 0.0, m.Precision())
}

func TestMatchFalseNegative(t *testing.T) {
	m := Match([]Expected{{RuleID: "r", File: "a", Line: 10}}, nil, 0)
	require.Equal(t, Metrics{FN: 1}, m)
	require.Equal(t, 0.0, m.Recall())
}

func TestMatchLineTolerance(t *testing.T) {
	exp := []Expected{{RuleID: "r", File: "a", Line: 10}}
	act := []models.Finding{{RuleID: "r", File: "a", Line: 12}}
	require.Equal(t, Metrics{TP: 1}, Match(exp, act, 3))
	require.Equal(t, Metrics{FN: 1, FP: 1}, Match(exp, act, 0))
}

func TestSum(t *testing.T) {
	require.Equal(t, Metrics{TP: 4, FP: 1, FN: 2},
		Sum(Metrics{TP: 3, FP: 1}, Metrics{TP: 1, FN: 2}))
}

// TestFixtureCorpus loads every fixture under testdata/ and gates per-fixture
// thresholds. Adding a fixture extends the regression corpus; loosening a
// threshold is an explicit, reviewable change.
func TestFixtureCorpus(t *testing.T) {
	fxs, err := LoadFixtures("testdata")
	require.NoError(t, err)
	require.NotEmpty(t, fxs, "ship at least one fixture so CI has a regression baseline")

	var agg Metrics
	for _, fx := range fxs {
		m, pass := fx.Score()
		agg = Sum(agg, m)
		require.Truef(t, pass,
			"fixture %q below thresholds: precision=%.2f recall=%.2f (min %.2f/%.2f)",
			fx.Name, m.Precision(), m.Recall(),
			fx.Thresholds.MinPrecision, fx.Thresholds.MinRecall)
	}
	t.Logf("corpus: tp=%d fp=%d fn=%d precision=%.2f recall=%.2f f1=%.2f",
		agg.TP, agg.FP, agg.FN, agg.Precision(), agg.Recall(), agg.F1())
}
