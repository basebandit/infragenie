// Package config loads and resolves infragenie user configuration.
//
// Resolution order (highest to lowest precedence):
//  1. CLI flags
//  2. Environment variables
//  3. Config file (~/.config/infragenie/config.yml or .infragenie.yml)
//  4. Built-in defaults
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// AppConfig is the top-level user configuration for infragenie.
// It is loaded once at startup and passed into commands.
type AppConfig struct {
	Providers ProvidersConfig `yaml:"providers"`
	Defaults  Defaults        `yaml:"defaults"`
}

// ProvidersConfig holds per-provider settings. Default names the provider
// used when --provider is not specified on the CLI.
type ProvidersConfig struct {
	Default   string              `yaml:"default"`
	OpenAI    *ProviderEntry      `yaml:"openai,omitempty"`
	Anthropic *ProviderEntry      `yaml:"anthropic,omitempty"`
	Google    *ProviderEntry      `yaml:"google,omitempty"`
	Azure     *AzureProviderEntry `yaml:"azure,omitempty"`
	Local     *LocalProviderEntry `yaml:"local,omitempty"`
}

// ProviderEntry configures a cloud LLM provider.
// api_key may be omitted when the corresponding env var is set.
type ProviderEntry struct {
	APIKey      string  `yaml:"api_key,omitempty"`
	Model       string  `yaml:"model,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	Temperature float32 `yaml:"temperature,omitempty"`
}

// AzureProviderEntry extends ProviderEntry with an Azure-specific endpoint.
type AzureProviderEntry struct {
	ProviderEntry `yaml:",inline"`
	Endpoint      string `yaml:"endpoint,omitempty"`
}

// LocalProviderEntry configures a self-hosted / Ollama endpoint.
type LocalProviderEntry struct {
	ProviderEntry `yaml:",inline"`
	BaseURL       string `yaml:"base_url"`
}

// Defaults sets run-level defaults that CLI flags override.
type Defaults struct {
	Format string `yaml:"format,omitempty"`
	FailOn string `yaml:"fail_on,omitempty"`
	Budget Budget `yaml:"budget,omitempty"`
}

// Budget caps LLM spend per review run.
type Budget struct {
	Tokens int     `yaml:"tokens,omitempty"`
	USD    float64 `yaml:"usd,omitempty"`
}

// Load reads the config file from the first location that exists:
//  1. explicitPath — set by --config flag
//  2. .infragenie.yml in the current directory (project-level)
//  3. $XDG_CONFIG_HOME/infragenie/config.yml (user-level, default ~/.config/infragenie/config.yml)
//
// Missing file is not an error — callers fall back to env vars and CLI flags.
func Load(explicitPath string) (*AppConfig, error) {
	path, err := resolve(explicitPath)
	if err != nil {
		return &AppConfig{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	raw = expandEnvRefs(raw)
	var cfg AppConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &cfg, nil
}

var envRefPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvRefs replaces ${VAR} references in the config with the matching
// environment variable. This lets secrets be referenced from committed config
// (api_key: ${OPENAI_API_KEY}) without embedding the value. Unset vars expand
// to an empty string.
func expandEnvRefs(b []byte) []byte {
	return envRefPattern.ReplaceAllFunc(b, func(m []byte) []byte {
		name := envRefPattern.FindSubmatch(m)[1]
		return []byte(os.Getenv(string(name)))
	})
}

// APIKey returns the resolved API key for provider using the lookup order:
// env var (wins) → config file value.
// CLI callers should override this with a --api-key flag value when present.
func (c *AppConfig) APIKey(provider string) string {
	if v := os.Getenv(envVarForProvider(provider)); v != "" {
		return v
	}
	switch provider {
	case "openai":
		if c.Providers.OpenAI != nil {
			return c.Providers.OpenAI.APIKey
		}
	case "anthropic":
		if c.Providers.Anthropic != nil {
			return c.Providers.Anthropic.APIKey
		}
	case "google":
		if c.Providers.Google != nil {
			return c.Providers.Google.APIKey
		}
	case "azure":
		if c.Providers.Azure != nil {
			return c.Providers.Azure.APIKey
		}
	}
	return ""
}

// BaseURL returns the resolved base URL for providers that need one (local/azure).
// Lookup order: env var → config file → empty string.
func (c *AppConfig) BaseURL(provider string) string {
	switch provider {
	case "local":
		if v := os.Getenv("LOCAL_LLM_URL"); v != "" {
			return v
		}
		if c.Providers.Local != nil {
			return c.Providers.Local.BaseURL
		}
	case "azure":
		if v := os.Getenv("AZURE_OPENAI_ENDPOINT"); v != "" {
			return v
		}
		if c.Providers.Azure != nil {
			return c.Providers.Azure.Endpoint
		}
	}
	return ""
}

// Model returns the resolved model for provider.
// Lookup order: env var → config file → empty string (provider picks its default).
func (c *AppConfig) Model(provider string) string {
	if v := os.Getenv(modelEnvVar(provider)); v != "" {
		return v
	}
	switch provider {
	case "openai":
		if c.Providers.OpenAI != nil {
			return c.Providers.OpenAI.Model
		}
	case "anthropic":
		if c.Providers.Anthropic != nil {
			return c.Providers.Anthropic.Model
		}
	case "google":
		if c.Providers.Google != nil {
			return c.Providers.Google.Model
		}
	case "azure":
		if c.Providers.Azure != nil {
			return c.Providers.Azure.Model
		}
	case "local":
		if c.Providers.Local != nil {
			return c.Providers.Local.Model
		}
	}
	return ""
}

// DefaultProvider returns the provider to use when --provider is not set.
// Lookup order: INFRAGENIE_PROVIDER env var → config file → empty string.
func (c *AppConfig) DefaultProvider() string {
	if v := os.Getenv("INFRAGENIE_PROVIDER"); v != "" {
		return v
	}
	return c.Providers.Default
}

// DefaultFormat returns the output format default.
// Lookup order: INFRAGENIE_FORMAT env var → config file → empty string.
func (c *AppConfig) DefaultFormat() string {
	if v := os.Getenv("INFRAGENIE_FORMAT"); v != "" {
		return v
	}
	return c.Defaults.Format
}

// DefaultFailOn returns the fail-on severity default.
// Lookup order: INFRAGENIE_FAIL_ON env var → config file → empty string.
func (c *AppConfig) DefaultFailOn() string {
	if v := os.Getenv("INFRAGENIE_FAIL_ON"); v != "" {
		return v
	}
	return c.Defaults.FailOn
}

// DefaultBudgetTokens returns the token budget default (0 = unlimited).
// Lookup order: INFRAGENIE_BUDGET_TOKENS env var → config file → 0.
func (c *AppConfig) DefaultBudgetTokens() int {
	if v := os.Getenv("INFRAGENIE_BUDGET_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return c.Defaults.Budget.Tokens
}

// DefaultBudgetUSD returns the USD budget default (0 = unlimited).
// Lookup order: INFRAGENIE_BUDGET_USD env var → config file → 0.
func (c *AppConfig) DefaultBudgetUSD() float64 {
	if v := os.Getenv("INFRAGENIE_BUDGET_USD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return c.Defaults.Budget.USD
}

func modelEnvVar(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_MODEL"
	case "anthropic":
		return "ANTHROPIC_MODEL"
	case "google":
		return "GOOGLE_MODEL"
	case "azure":
		return "AZURE_OPENAI_MODEL"
	case "local":
		return "LOCAL_LLM_MODEL"
	default:
		return ""
	}
}

// DefaultPath returns the canonical user-level config path regardless of
// whether the file exists.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "infragenie", "config.yml")
}

func resolve(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if _, err := os.Stat(".infragenie.yml"); err == nil {
		return ".infragenie.yml", nil
	}
	p := DefaultPath()
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("no config file found")
}

func envVarForProvider(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "google":
		return "GOOGLE_API_KEY"
	case "azure":
		return "AZURE_OPENAI_API_KEY"
	default:
		return ""
	}
}
