package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	if config.BaseURL != "" {
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
type AnthropicProvider struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float32
	httpClient  *http.Client
}

func NewAnthropicProvider(config Config) (*AnthropicProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API Key is required")
	}

	model := config.Model
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	return &AnthropicProvider{
		apiKey:      config.APIKey,
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float32            `json:"temperature"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func (p *AnthropicProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	request := anthropicRequest{
		Model:       p.model,
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
		System:      SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Anthropic API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API error %d: %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Content[0].Text, nil
}

func (p *AnthropicProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Anthropic doesn't provide embeddings, fallback to OpenAI or return error
	return nil, fmt.Errorf("Anthropic provider does not support embeddings")
}

func (p *AnthropicProvider) GetModel() string      { return p.model }
func (p *AnthropicProvider) GetProvider() Provider { return ProviderAnthropic }

func (p *AnthropicProvider) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	testRequest := anthropicRequest{
		Model:     p.model,
		MaxTokens: 10,
		Messages:  []anthropicMessage{{Role: "user", Content: "Hello"}},
	}

	jsonData, _ := json.Marshal(testRequest)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// =============================================================================
// Google Provider (Gemini)
// =============================================================================
type GoogleProvider struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float32
	httpClient  *http.Client
}

func NewGoogleProvider(config Config) (*GoogleProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Google API key is required")
	}

	model := config.Model
	if model == "" {
		model = "gemini-pro"
	}

	return &GoogleProvider{
		apiKey:      config.APIKey,
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type googleRequest struct {
	Contents         []googleContent `json:"contents"`
	GenerationConfig struct {
		MaxOutputTokens int     `json:"maxOutputTokens"`
		Temperature     float32 `json:"temperature"`
	} `json:"generationConfig"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type googlePart struct {
	Text string `json:"text"`
}

type googleResponse struct {
	Candidates []struct {
		Content struct {
			Parts []googlePart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (p *GoogleProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	request := googleRequest{
		Contents: []googleContent{
			{Parts: []googlePart{{Text: SystemPrompt + "\n\n" + prompt}}},
		},
	}
	request.GenerationConfig.MaxOutputTokens = p.maxTokens
	request.GenerationConfig.Temperature = p.temperature

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Google API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Google API error %d: %s", resp.StatusCode, string(body))
	}

	var response googleResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in Google response")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}

func (p *GoogleProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Google embeddings API implementation would go here
	return nil, fmt.Errorf("Google provider embedding support not implemented yet")
}

func (p *GoogleProvider) GetModel() string      { return p.model }
func (p *GoogleProvider) GetProvider() Provider { return ProviderGoogle }

func (p *GoogleProvider) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Simple health check
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.apiKey)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// =============================================================================
// Local Provider (Ollama, LocalAI, etc.)
// =============================================================================
type LocalProvider struct {
	baseURL     string
	model       string
	maxTokens   int
	temperature float32
	httpClient  *http.Client
}

func NewLocalProvider(config Config) (*LocalProvider, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for local provider")
	}

	model := config.Model
	if model == "" {
		model = "llama2"
	}

	return &LocalProvider{
		baseURL:     strings.TrimSuffix(config.BaseURL, "/"),
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
		httpClient:  &http.Client{Timeout: 120 * time.Second}, // Local models might be slower
	}, nil
}

type localRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream"`
}

type localResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (p *LocalProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	request := localRequest{
		Model:       p.model,
		Prompt:      SystemPrompt + "\n\nUser: " + prompt + "\n\nAssistant:",
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("local API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local API error %d: %s", resp.StatusCode, string(body))
	}

	var response localResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

func (p *LocalProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Local embedding API implementation would depend on the specific local service
	return nil, fmt.Errorf("local provider embedding support not implemented yet")
}

func (p *LocalProvider) GetModel() string      { return p.model }
func (p *LocalProvider) GetProvider() Provider { return ProviderLocal }

func (p *LocalProvider) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// =============================================================================
// Azure OpenAI Provider
// =============================================================================
type AzureProvider struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float32
}

func NewAzureProvider(config Config) (*AzureProvider, error) {
	if config.APIKey == "" || config.BaseURL == "" {
		return nil, fmt.Errorf("Azure API key and base URL are required")
	}

	clientConfig := openai.DefaultAzureConfig(config.APIKey, config.BaseURL)
	client := openai.NewClientWithConfig(clientConfig)

	model := config.Model
	if model == "" {
		model = "gpt-4"
	}

	return &AzureProvider{
		client:      client,
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
	}, nil
}

func (p *AzureProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
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
		return "", fmt.Errorf("Azure OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from Azure OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

func (p *AzureProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: "text-embedding-ada-002", // Azure embedding model
	})

	if err != nil {
		return nil, fmt.Errorf("Azure embedding error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return resp.Data[0].Embedding, nil
}

func (p *AzureProvider) GetModel() string      { return p.model }
func (p *AzureProvider) GetProvider() Provider { return ProviderAzure }

func (p *AzureProvider) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := p.client.ListModels(ctx)
	return err == nil
}

// =============================================================================
// AWS Bedrock Provider
// =============================================================================
type AWSProvider struct {
	region      string
	model       string
	maxTokens   int
	temperature float32
	// AWS SDK client would be initialized here
}

func NewAWSProvider(config Config) (*AWSProvider, error) {
	if config.Region == "" {
		return nil, fmt.Errorf("AWS region is required")
	}

	model := config.Model
	if model == "" {
		model = "anthropic.claude-v2"
	}

	return &AWSProvider{
		region:      config.Region,
		model:       model,
		maxTokens:   getMaxTokens(config.MaxTokens, 4000),
		temperature: getTemperature(config.Temperature, 0.1),
	}, nil
}

func (p *AWSProvider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	// AWS Bedrock implementation would go here
	// This would use the AWS SDK to call Bedrock Runtime
	return "", fmt.Errorf("AWS Bedrock provider not implemented yet")
}

func (p *AWSProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("AWS Bedrock embedding support not implemented yet")
}

func (p *AWSProvider) GetModel() string      { return p.model }
func (p *AWSProvider) GetProvider() Provider { return ProviderAWS }

func (p *AWSProvider) IsAvailable(ctx context.Context) bool {
	// AWS Bedrock health check would go here
	return false
}

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
