package llm

import (
	"context"
	"fmt"
)

type Provider string

const (
	ProviderOpenAI    Provider = "openAI"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderAzure     Provider = "azure"
	ProviderAWS       Provider = "aws"
	ProviderLocal     Provider = "local"
)

type LLMInterface interface {
	GenerateResponse(ctx context.Context, prompt string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GetModel() string
	GetProvider() Provider
	IsAvailable(ctx context.Context) bool
}

type Client struct {
	providers map[Provider]LLMInterface
	primary   Provider
	fallbacks []Provider
}

type Config struct {
	Provider    Provider `json:"provider"`
	APIKey      string   `json:"api_key"`
	Model       string   `json:"model"`
	BaseURL     string   `json:"base_url,omitempty"`
	Region      string   `json:"region,omitempty"`
	MaxTokens   int      `json:"max_tokens"`
	Temperature float32  `json:"temperature"`
}

func NewClient(configs []Config) (*Client, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("at least one LLM provider configuration is required")
	}

	client := &Client{
		providers: make(map[Provider]LLMInterface),
		primary:   configs[0].Provider,
	}

	// Initialize all configured providers
	for i, config := range configs {
		provider, err := createProvider(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", config.Provider, err)
		}

		client.providers[config.Provider] = provider

		// Set fallbacks (all providers except primary)
		if i > 0 {
			client.fallbacks = append(client.fallbacks, config.Provider)
		}
	}

	return client, nil
}

func createProvider(config Config) (LLMInterface, error) {
	switch config.Provider {
	case ProviderOpenAI:
		return NewOpenAIProvider(config)
	case ProviderAnthropic:
		return NewAnthropicProvider(config)
	case ProviderGoogle:
		return NewGoogleProvider(config)
	case ProviderAzure:
		return NewAzureProvider(config)
	case ProviderAWS:
		return NewAWSProvider(config)
	case ProviderLocal:
		return NewLocalProvider(config)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

func (c *Client) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	// Try primary provider first
	if provider := c.providers[c.primary]; provider != nil {
		if provider.IsAvailable(ctx) {
			response, err := provider.GenerateResponse(ctx, prompt)
			if err == nil {
				return response, nil
			}
			fmt.Printf("Primary provider %s failed: %v\n", c.primary, err)
		}
	}

	// Try fallback providers
	for _, fallbackProvider := range c.fallbacks {
		if provider := c.providers[fallbackProvider]; provider != nil {
			if provider.IsAvailable(ctx) {
				response, err := provider.GenerateResponse(ctx, prompt)
				if err == nil {
					fmt.Printf("Fallback to provider %s successful\n", fallbackProvider)
					return response, nil
				}
				fmt.Printf("Fallback provider %s failed: %v\n", fallbackProvider, err)
			}
		}
	}

	return "", fmt.Errorf("all LLM providers failed")
}

func (c *Client) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Try primary provider first
	if provider := c.providers[c.primary]; provider != nil {
		if provider.IsAvailable(ctx) {
			embedding, err := provider.GenerateEmbedding(ctx, text)
			if err == nil {
				return embedding, nil
			}
			fmt.Printf("Primary provider %s failed: %v\n", c.primary, err)
		}
	}

	// Try fallback providers
	for _, fallbackProvider := range c.fallbacks {
		if provider := c.providers[fallbackProvider]; provider != nil {
			if provider.IsAvailable(ctx) {
				embedding, err := provider.GenerateEmbedding(ctx, text)
				if err == nil {
					return embedding, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("all embedding providers failed")
}

func (c *Client) GetAvailableProviders(ctx context.Context) []Provider {
	var available []Provider
	for providerType, provider := range c.providers {
		if provider.IsAvailable(ctx) {
			available = append(available, providerType)
		}
	}
	return available
}
