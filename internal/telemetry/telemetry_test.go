package telemetry

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestLedgerRecordsTotals(t *testing.T) {
	l := NewLedger()
	l.Record(Entry{Provider: "openai", Model: "gpt-4o-mini", PromptTokens: 1000, CompletionTokens: 200, USDEstimate: 0.01, Latency: 500 * time.Millisecond})
	l.Record(Entry{Provider: "openai", Model: "gpt-4o-mini", CacheHit: true})
	l.Record(Entry{Provider: "openai", Model: "gpt-4o-mini", Err: "timeout"})

	tot := l.Totals()
	require.Equal(t, 3, tot.Calls)
	require.Equal(t, 1000, tot.PromptTokens)
	require.Equal(t, 1, tot.CacheHits)
	require.Equal(t, 1, tot.Errors)
	require.InDelta(t, 0.01, tot.USDEstimate, 1e-9)
}

func TestLedgerSummaryAndJSON(t *testing.T) {
	l := NewLedger()
	l.Record(Entry{Provider: "openai", Model: "x", PromptTokens: 100, CompletionTokens: 50, USDEstimate: 0.002})
	require.Contains(t, l.Summary(), "$0.0020")

	var buf bytes.Buffer
	require.NoError(t, l.WriteJSON(&buf))
	out := buf.String()
	require.Contains(t, out, `"calls": 1`)
	require.Contains(t, out, `"prompt_tokens": 100`)
}

func TestBudgetReserveAndAdd(t *testing.T) {
	c := NewCounter(Budget{Tokens: 1000, USD: 0.05})
	require.NoError(t, c.Reserve(500, 0.02))
	c.Add(500, 0.02)
	require.NoError(t, c.Reserve(400, 0.02))
	c.Add(400, 0.02)
	err := c.Reserve(200, 0.02)
	require.ErrorIs(t, err, ErrBudgetExceeded)
}

func TestBudgetZeroLimitDisablesGate(t *testing.T) {
	c := NewCounter(Budget{})
	require.NoError(t, c.Reserve(1<<30, 1e9))
}

func TestMetricsObserve(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	m.Observe(Entry{Provider: "openai", Model: "gpt-4o-mini", Reviewer: "grounding", PromptTokens: 100, CompletionTokens: 50, USDEstimate: 0.001, Latency: 250 * time.Millisecond})
	m.Observe(Entry{Provider: "openai", Model: "gpt-4o-mini", Reviewer: "grounding", CacheHit: true})

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, mfs)

	names := []string{}
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	require.Contains(t, strings.Join(names, ","), "infragenie_llm_calls_total")
	require.Contains(t, strings.Join(names, ","), "infragenie_llm_tokens_total")
}

func TestRecordError(t *testing.T) {
	l := NewLedger()
	l.Record(Entry{Err: errors.New("boom").Error()})
	require.Equal(t, 1, l.Totals().Errors)
}
