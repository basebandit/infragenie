package telemetry

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Calls   *prometheus.CounterVec
	Tokens  *prometheus.CounterVec
	USD     *prometheus.CounterVec
	Latency *prometheus.HistogramVec
}

// NewMetrics builds the LLM metric vectors and registers them on reg.
// Pass prometheus.DefaultRegisterer to use the global, or a fresh registry
// in tests.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Calls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "infragenie", Subsystem: "llm",
			Name: "calls_total", Help: "LLM calls",
		}, []string{"provider", "model", "reviewer", "cache_hit"}),
		Tokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "infragenie", Subsystem: "llm",
			Name: "tokens_total", Help: "LLM tokens consumed",
		}, []string{"provider", "model", "kind"}),
		USD: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "infragenie", Subsystem: "llm",
			Name: "usd_total", Help: "Estimated USD spent on LLM calls",
		}, []string{"provider", "model"}),
		Latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "infragenie", Subsystem: "llm",
			Name: "call_seconds", Help: "LLM call latency",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider", "model"}),
	}
	reg.MustRegister(m.Calls, m.Tokens, m.USD, m.Latency)
	return m
}

func (m *Metrics) Observe(e Entry) {
	cacheHit := "false"
	if e.CacheHit {
		cacheHit = "true"
	}
	m.Calls.WithLabelValues(e.Provider, e.Model, e.Reviewer, cacheHit).Inc()
	if e.CacheHit {
		return
	}
	m.Tokens.WithLabelValues(e.Provider, e.Model, "prompt").Add(float64(e.PromptTokens))
	m.Tokens.WithLabelValues(e.Provider, e.Model, "completion").Add(float64(e.CompletionTokens))
	m.USD.WithLabelValues(e.Provider, e.Model).Add(e.USDEstimate)
	m.Latency.WithLabelValues(e.Provider, e.Model).Observe(e.Latency.Seconds())
}
