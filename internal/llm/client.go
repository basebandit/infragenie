// Package llm provides a multi-provider LLM client that implements
// grounding.LLM. Configure a primary provider and optional fallbacks; the
// client tries each in order and returns on the first success.
package llm

import (
	"context"
	"fmt"
)

type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderAzure     Provider = "azure"
	ProviderAWS       Provider = "aws"
	ProviderLocal     Provider = "local"
)

// provider is the internal contract each backend must satisfy.
type provider interface {
	Generate(ctx context.Context, system, user string) (string, error)
	Available(ctx context.Context) bool
}

// Client implements grounding.LLM with automatic provider fallback.
type Client struct {
	primary   Provider
	fallbacks []Provider
	providers map[Provider]provider
}

type Config struct {
	Provider    Provider
	APIKey      string
	Model       string
	BaseURL     string
	Region      string
	MaxTokens   int
	Temperature float32
}

func NewClient(configs []Config) (*Client, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("at least one provider config required")
	}
	c := &Client{
		primary:   configs[0].Provider,
		providers: make(map[Provider]provider),
	}
	for i, cfg := range configs {
		p, err := newProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", cfg.Provider, err)
		}
		c.providers[cfg.Provider] = p
		if i > 0 {
			c.fallbacks = append(c.fallbacks, cfg.Provider)
		}
	}
	return c, nil
}

func newProvider(cfg Config) (provider, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		return newOpenAIProvider(cfg)
	case ProviderAnthropic:
		return newAnthropicProvider(cfg)
	case ProviderGoogle:
		return newGoogleProvider(cfg)
	case ProviderAzure:
		return newAzureProvider(cfg)
	case ProviderAWS:
		return newAWSProvider(cfg)
	case ProviderLocal:
		return newLocalProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// Generate implements grounding.LLM. Tries primary then fallbacks.
func (c *Client) Generate(ctx context.Context, system, user string) (string, error) {
	if p := c.providers[c.primary]; p != nil && p.Available(ctx) {
		if resp, err := p.Generate(ctx, system, user); err == nil {
			return resp, nil
		}
	}
	for _, name := range c.fallbacks {
		if p := c.providers[name]; p != nil && p.Available(ctx) {
			if resp, err := p.Generate(ctx, system, user); err == nil {
				return resp, nil
			}
		}
	}
	return "", fmt.Errorf("all LLM providers failed")
}
