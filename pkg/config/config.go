package config

import (
	"os"
	"strconv"
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
	}
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
