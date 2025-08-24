package config

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/basebandit/infragenie/internal/llm"
)

type Config struct {
	Port           int
	DatabaseURL    string
	RedisURL       string
	KubeConfig     string
	KongAdminURL   string
	KongAdminToken string
	OpenAIAPIKey   string
	OpenAIModel    string
	LogLevel       string
	NatsURL        string
	PrometheusURL  string
	LLMProviders   []llm.Config
}

func Load() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))

	return &Config{
		Port:           port,
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://user:pass@localhost/infragenie?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		KubeConfig:     getEnv("KUBE_CONFIG", ""),
		KongAdminURL:   getEnv("KONG_ADMIN_URL", "http://localhost:8001"),
		KongAdminToken: getEnv("KONG_ADMIN_TOKEN", ""),
		OpenAIAPIKey:   getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:    getEnv("OPENAI_MODEL", "gpt-4-turbo-preview"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		NatsURL:        getEnv("NATS_URL", "nats://localhost:4222"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		LLMProviders:   loadLLMProviders(),
	}
}

func loadLLMProviders() []llm.Config {
	var providers []llm.Config

	// load from JSON configuration if available
	if configJSON := getEnv("LLM_PROVIDERS_JSON", ""); configJSON != "" {
		var jsonProviders []llm.Config
		if err := json.Unmarshal([]byte(configJSON), &jsonProviders); err == nil {
			providers = append(providers, jsonProviders...)
		}
	}

	// Load individual provider configurations from environment variables
	if openaiKey := getEnv("OPENAI_API_KEY", ""); openaiKey != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderOpenAI,
			APIKey:      openaiKey,
			Model:       getEnv("OPENAI_MODEL", "gpt-4-turbo-preview"),
			BaseURL:     getEnv("OPENAI_BASE_URL", ""),
			MaxTokens:   getEnvInt("OPENAI_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("OPENAI_TEMPERATURE", 0.1),
		})
	}

	if anthropicKey := getEnv("ANTHROPIC_API_KEY", ""); anthropicKey != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderAnthropic,
			APIKey:      anthropicKey,
			Model:       getEnv("ANTHROPIC_MODEL", "claude-3-sonnet-20240229"),
			MaxTokens:   getEnvInt("ANTHROPIC_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("ANTHROPIC_TEMPERATURE", 0.1),
		})
	}

	if googleKey := getEnv("GOOGLE_API_KEY", ""); googleKey != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderGoogle,
			APIKey:      googleKey,
			Model:       getEnv("GOOGLE_MODEL", "gemini-pro"),
			MaxTokens:   getEnvInt("GOOGLE_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("GOOGLE_TEMPERATURE", 0.1),
		})
	}

	if azureKey := getEnv("AZURE_OPENAI_API_KEY", ""); azureKey != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderAzure,
			APIKey:      azureKey,
			Model:       getEnv("AZURE_OPENAI_MODEL", "gpt-4"),
			BaseURL:     getEnv("AZURE_OPENAI_ENDPOINT", ""),
			MaxTokens:   getEnvInt("AZURE_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("AZURE_TEMPERATURE", 0.1),
		})
	}

	if awsRegion := getEnv("AWS_REGION", ""); awsRegion != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderAWS,
			Region:      awsRegion,
			Model:       getEnv("AWS_BEDROCK_MODEL", "anthropic.claude-v2"),
			MaxTokens:   getEnvInt("AWS_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("AWS_TEMPERATURE", 0.1),
		})
	}

	if localURL := getEnv("LOCAL_LLM_URL", ""); localURL != "" {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderLocal,
			BaseURL:     localURL,
			Model:       getEnv("LOCAL_LLM_MODEL", "llama2"),
			MaxTokens:   getEnvInt("LOCAL_MAX_TOKENS", 4000),
			Temperature: getEnvFloat("LOCAL_TEMPERATURE", 0.1),
		})
	}

	// If no providers configured, default to OpenAI with placeholder
	if len(providers) == 0 {
		providers = append(providers, llm.Config{
			Provider:    llm.ProviderOpenAI,
			APIKey:      "your-openai-key-here",
			Model:       "gpt-4-turbo-preview",
			MaxTokens:   4000,
			Temperature: 0.1,
		})
	}

	return providers
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 32); err == nil {
			return float32(floatValue)
		}
	}
	return defaultValue
}
