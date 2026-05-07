package grounding

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	calls    atomic.Int64
	response string
	err      error
}

func (s *stubLLM) Generate(_ context.Context, _, _ string) (string, error) {
	s.calls.Add(1)
	return s.response, s.err
}

func TestGroundReturnsParsedJSON(t *testing.T) {
	llm := &stubLLM{response: `{"explanation": "missing limits in deployment.yaml line 12", "suggestion": "+++ b/x\n@@\n+resources: {limits: {cpu: 100m}}"}`}
	g := NewLLMGrounder(llm, "test-model")

	ex, sg, err := g.Ground(context.Background(), models.Finding{RuleID: "x", File: "x.yml"}, "content")
	require.NoError(t, err)
	require.Contains(t, ex, "missing limits")
	require.Contains(t, sg, "@@")
}

func TestGroundCaches(t *testing.T) {
	llm := &stubLLM{response: `{"explanation":"e","suggestion":"s"}`}
	g := NewLLMGrounder(llm, "test-model")

	f := models.Finding{RuleID: "x", File: "x.yml"}
	for i := 0; i < 3; i++ {
		_, _, err := g.Ground(context.Background(), f, "same content")
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, llm.calls.Load(), "should hit cache after first call")
}

func TestGroundCacheSensitiveToContent(t *testing.T) {
	llm := &stubLLM{response: `{"explanation":"e","suggestion":"s"}`}
	g := NewLLMGrounder(llm, "test-model")
	f := models.Finding{RuleID: "x", File: "x.yml"}

	_, _, _ = g.Ground(context.Background(), f, "v1")
	_, _, _ = g.Ground(context.Background(), f, "v2")
	require.EqualValues(t, 2, llm.calls.Load(), "different content → different cache key")
}

func TestGroundLLMError(t *testing.T) {
	llm := &stubLLM{err: errors.New("boom")}
	g := NewLLMGrounder(llm, "test-model")
	_, _, err := g.Ground(context.Background(), models.Finding{}, "")
	require.ErrorContains(t, err, "boom")
}

func TestGroundBadJSON(t *testing.T) {
	llm := &stubLLM{response: `not json`}
	g := NewLLMGrounder(llm, "test-model")
	_, _, err := g.Ground(context.Background(), models.Finding{}, "")
	require.ErrorContains(t, err, "bad JSON")
}
