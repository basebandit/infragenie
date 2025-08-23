package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

// =============================================================================
// OpenAI Provider
// =============================================================================
type OpenAIProvider struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float32
}

func NewOpenAIProvider(config Config) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(config.APIKey)
	if config.BaseURL != nil {
		// support for custom OpenAI-compatible endpoints
		clientConfig := openai.DefaultConfig(config.APIKey)
		clientConfig.BaseURL = config.BaseURL
		client = openai.NewClientWithConfig(clientConfig)
	}

	model := config.Model
	if model == "" {
		model = "gpt-4-turbo-preview"
	}

	return &OpenAIProvider{
		client:      client,
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
	}, nil
}

func (p *OpenAIProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: SystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
	})

	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	})

	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return resp.Data[0].Embedding, nil
}

func (p *OpenAIProvider) GetModel() string      { return p.model }
func (p *OpenAIProvider) GetProvider() Provider { return ProviderOpenAI }

func (p *OpenAIProvider) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := p.client.ListModels(ctx)
	return err == nil
}

// =============================================================================
// Anthropic Provider
// =============================================================================

// =============================================================================
// Utility Functions
// =============================================================================
func getMaxTokens(configured, defaultValue int) int {
	if configured > 0 {
		return configured
	}

	return defaultValue
}

func getTemperature(configured, defaultValue float32) float32 {
	if configured >= 0 {
		return configured
	}
	return defaultValue
}
