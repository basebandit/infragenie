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

// ── OpenAI ────────────────────────────────────────────────────────────────────

type openAIProvider struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float32
}

func newOpenAIProvider(cfg Config) (*openAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai API key required")
	}
	c := openai.NewClient(cfg.APIKey)
	if cfg.BaseURL != "" {
		cc := openai.DefaultConfig(cfg.APIKey)
		cc.BaseURL = cfg.BaseURL
		c = openai.NewClientWithConfig(cc)
	}
	return &openAIProvider{
		client:      c,
		model:       stringOr(cfg.Model, "gpt-4o-mini"),
		maxTokens:   intOr(cfg.MaxTokens, 4000),
		temperature: floatOr(cfg.Temperature, 0.1),
	}, nil
}

func (p *openAIProvider) Generate(ctx context.Context, system, user string) (string, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
	})
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *openAIProvider) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := p.client.ListModels(ctx)
	return err == nil
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

type anthropicProvider struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float32
	http        *http.Client
}

func newAnthropicProvider(cfg Config) (*anthropicProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic API key required")
	}
	return &anthropicProvider{
		apiKey:      cfg.APIKey,
		model:       stringOr(cfg.Model, "claude-3-5-haiku-20241022"),
		maxTokens:   intOr(cfg.MaxTokens, 4000),
		temperature: floatOr(cfg.Temperature, 0.1),
		http:        &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type anthropicSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *anthropicCacheCtrl `json:"cache_control,omitempty"`
}

type anthropicCacheCtrl struct {
	Type string `json:"type"`
}

type anthropicReq struct {
	Model       string                  `json:"model"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature float32                 `json:"temperature"`
	System      []anthropicSystemBlock  `json:"system,omitempty"`
	Messages    []anthropicMessage      `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResp struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (p *anthropicProvider) Generate(ctx context.Context, system, user string) (string, error) {
	// Mark the system prompt for caching — Anthropic caches prefixes ≥1024 tokens.
	// Safe to always set; ignored silently when below threshold.
	sysBlocks := []anthropicSystemBlock{{
		Type:         "text",
		Text:         system,
		CacheControl: &anthropicCacheCtrl{Type: "ephemeral"},
	}}
	body, _ := json.Marshal(anthropicReq{
		Model:       p.model,
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
		System:      sysBlocks,
		Messages:    []anthropicMessage{{Role: "user", Content: user}},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: %d %s", resp.StatusCode, raw)
	}
	var out anthropicResp
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Content) == 0 {
		return "", fmt.Errorf("anthropic: bad response")
	}
	return out.Content[0].Text, nil
}

func (p *anthropicProvider) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body, _ := json.Marshal(anthropicReq{
		Model: p.model, MaxTokens: 1,
		Messages: []anthropicMessage{{Role: "user", Content: "ping"}},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	resp, err := p.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ── Google (Gemini) ───────────────────────────────────────────────────────────

type googleProvider struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float32
	http        *http.Client
}

func newGoogleProvider(cfg Config) (*googleProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Google API key required")
	}
	return &googleProvider{
		apiKey:      cfg.APIKey,
		model:       stringOr(cfg.Model, "gemini-1.5-flash"),
		maxTokens:   intOr(cfg.MaxTokens, 4000),
		temperature: floatOr(cfg.Temperature, 0.1),
		http:        &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type googleReq struct {
	Contents         []googleContent `json:"contents"`
	GenerationConfig struct {
		MaxOutputTokens int     `json:"maxOutputTokens"`
		Temperature     float32 `json:"temperature"`
	} `json:"generationConfig"`
}
type googleContent struct {
	Parts []struct{ Text string `json:"text"` } `json:"parts"`
	Role  string                                 `json:"role,omitempty"`
}
type googleResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct{ Text string `json:"text"` } `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (p *googleProvider) Generate(ctx context.Context, system, user string) (string, error) {
	r := googleReq{
		Contents: []googleContent{
			{Parts: []struct{ Text string `json:"text"` }{{system + "\n\n" + user}}},
		},
	}
	r.GenerationConfig.MaxOutputTokens = p.maxTokens
	r.GenerationConfig.Temperature = p.temperature

	body, _ := json.Marshal(r)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("google: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google: %d %s", resp.StatusCode, raw)
	}
	var out googleResp
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("google: bad response")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}

func (p *googleProvider) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.apiKey)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := p.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ── Local (Ollama / LocalAI) ──────────────────────────────────────────────────

type localProvider struct {
	baseURL     string
	model       string
	maxTokens   int
	temperature float32
	http        *http.Client
}

func newLocalProvider(cfg Config) (*localProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("local provider requires base_url")
	}
	return &localProvider{
		baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
		model:       stringOr(cfg.Model, "llama3"),
		maxTokens:   intOr(cfg.MaxTokens, 4000),
		temperature: floatOr(cfg.Temperature, 0.1),
		http:        &http.Client{Timeout: 120 * time.Second},
	}, nil
}

type localReq struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream"`
}
type localResp struct{ Response string `json:"response"` }

func (p *localProvider) Generate(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(localReq{
		Model:       p.model,
		Prompt:      system + "\n\nUser: " + user + "\n\nAssistant:",
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("local: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local: %d %s", resp.StatusCode, raw)
	}
	var out localResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("local: bad response")
	}
	return out.Response, nil
}

func (p *localProvider) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	resp, err := p.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ── Azure OpenAI ──────────────────────────────────────────────────────────────

type azureProvider struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float32
}

func newAzureProvider(cfg Config) (*azureProvider, error) {
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("azure requires api_key and base_url")
	}
	cc := openai.DefaultAzureConfig(cfg.APIKey, cfg.BaseURL)
	return &azureProvider{
		client:      openai.NewClientWithConfig(cc),
		model:       stringOr(cfg.Model, "gpt-4"),
		maxTokens:   intOr(cfg.MaxTokens, 4000),
		temperature: floatOr(cfg.Temperature, 0.1),
	}, nil
}

func (p *azureProvider) Generate(ctx context.Context, system, user string) (string, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
	})
	if err != nil {
		return "", fmt.Errorf("azure: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("azure: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *azureProvider) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := p.client.ListModels(ctx)
	return err == nil
}

// ── AWS Bedrock ───────────────────────────────────────────────────────────────

type awsProvider struct {
	region string
	model  string
}

func newAWSProvider(cfg Config) (*awsProvider, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS provider requires region")
	}
	return &awsProvider{
		region: cfg.Region,
		model:  stringOr(cfg.Model, "anthropic.claude-3-5-haiku-20241022-v1:0"),
	}, nil
}

func (p *awsProvider) Generate(_ context.Context, _, _ string) (string, error) {
	return "", fmt.Errorf("aws: bedrock not implemented yet")
}

func (p *awsProvider) Available(_ context.Context) bool { return false }

// ── helpers ───────────────────────────────────────────────────────────────────

func stringOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func intOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func floatOr(v, def float32) float32 {
	if v > 0 {
		return v
	}
	return def
}
