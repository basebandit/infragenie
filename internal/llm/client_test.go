package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	resp      string
	available bool
	calls     int
}

func (s *stubProvider) Generate(_ context.Context, _, _ string) (string, error) {
	s.calls++
	return s.resp, nil
}
func (s *stubProvider) Available(_ context.Context) bool { return s.available }

func TestGenerate_PrimarySuccess(t *testing.T) {
	p := &stubProvider{resp: "ok", available: true}
	c := &Client{
		primary:   ProviderLocal,
		providers: map[Provider]provider{ProviderLocal: p},
	}
	resp, err := c.Generate(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "ok", resp)
	require.Equal(t, 1, p.calls)
}

func TestGenerate_FallbackOnUnavailable(t *testing.T) {
	primary := &stubProvider{available: false}
	fallback := &stubProvider{resp: "fallback", available: true}
	c := &Client{
		primary:   ProviderOpenAI,
		fallbacks: []Provider{ProviderLocal},
		providers: map[Provider]provider{
			ProviderOpenAI: primary,
			ProviderLocal:  fallback,
		},
	}
	resp, err := c.Generate(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "fallback", resp)
	require.Equal(t, 0, primary.calls)
	require.Equal(t, 1, fallback.calls)
}

func TestGenerate_AllUnavailable(t *testing.T) {
	c := &Client{
		primary:   ProviderOpenAI,
		providers: map[Provider]provider{ProviderOpenAI: &stubProvider{available: false}},
	}
	_, err := c.Generate(context.Background(), "sys", "user")
	require.ErrorContains(t, err, "all LLM providers failed")
}

func TestNewClient_EmptyConfigs(t *testing.T) {
	_, err := NewClient([]Config{})
	require.Error(t, err)
}

func TestNewClient_UnsupportedProvider(t *testing.T) {
	_, err := NewClient([]Config{{Provider: "bogus"}})
	require.ErrorContains(t, err, "unsupported provider")
}

func TestNewClient_LocalProvider(t *testing.T) {
	c, err := NewClient([]Config{{Provider: ProviderLocal, BaseURL: "http://localhost:11434", Model: "llama3"}})
	require.NoError(t, err)
	require.Equal(t, ProviderLocal, c.primary)
}
