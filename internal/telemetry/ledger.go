// Package telemetry provides per-LLM-call accounting (Ledger), Prometheus
// metrics, OTLP tracing, and budget enforcement.
package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

type Entry struct {
	Provider         string        `json:"provider"`
	Model            string        `json:"model"`
	Reviewer         string        `json:"reviewer,omitempty"`
	RuleID           string        `json:"rule_id,omitempty"`
	Repo             string        `json:"repo,omitempty"`
	PR               string        `json:"pr,omitempty"`
	Team             string        `json:"team,omitempty"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	USDEstimate      float64       `json:"usd_estimate"`
	CacheHit         bool          `json:"cache_hit"`
	Latency          time.Duration `json:"latency_ns"`
	Err              string        `json:"error,omitempty"`
	Time             time.Time     `json:"time"`
}

type Totals struct {
	Calls            int     `json:"calls"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	USDEstimate      float64 `json:"usd_estimate"`
	CacheHits        int     `json:"cache_hits"`
	Errors           int     `json:"errors"`
}

type Ledger struct {
	mu      sync.Mutex
	entries []Entry
	totals  Totals
}

func NewLedger() *Ledger { return &Ledger{} }

func (l *Ledger) Record(e Entry) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	l.totals.Calls++
	l.totals.PromptTokens += e.PromptTokens
	l.totals.CompletionTokens += e.CompletionTokens
	l.totals.USDEstimate += e.USDEstimate
	if e.CacheHit {
		l.totals.CacheHits++
	}
	if e.Err != "" {
		l.totals.Errors++
	}
}

func (l *Ledger) Totals() Totals {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.totals
}

func (l *Ledger) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Summary is a one-line human-readable snapshot for end-of-run output.
func (l *Ledger) Summary() string {
	t := l.Totals()
	return fmt.Sprintf(
		"Spent $%.4f, %d tokens (%d in / %d out), %d cache hits, %d errors over %d calls",
		t.USDEstimate, t.PromptTokens+t.CompletionTokens,
		t.PromptTokens, t.CompletionTokens, t.CacheHits, t.Errors, t.Calls)
}

// WriteJSON dumps totals + entries to w (intended for CI ingestion).
func (l *Ledger) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Totals  Totals  `json:"totals"`
		Entries []Entry `json:"entries"`
	}{Totals: l.Totals(), Entries: l.Entries()})
}
