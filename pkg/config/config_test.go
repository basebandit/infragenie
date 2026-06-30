package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Load

func TestLoad_NoFile(t *testing.T) {
	// No config file anywhere — should return empty config, not error.
	t.Chdir(t.TempDir())
	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoad_ExplicitPath(t *testing.T) {
	yml := `
providers:
  default: anthropic
  openai:
    model: gpt-4o
defaults:
  format: json
  fail_on: medium
  budget:
    tokens: 10000
    usd: 1.5
`
	path := writeTemp(t, yml)
	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "anthropic", cfg.Providers.Default)
	require.NotNil(t, cfg.Providers.OpenAI)
	require.Equal(t, "gpt-4o", cfg.Providers.OpenAI.Model)
	require.Equal(t, "json", cfg.Defaults.Format)
	require.Equal(t, "medium", cfg.Defaults.FailOn)
	require.Equal(t, 10000, cfg.Defaults.Budget.Tokens)
	require.InDelta(t, 1.5, cfg.Defaults.Budget.USD, 0.001)
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "::invalid: yaml: [")
	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config: parse")
}

func TestLoad_NonexistentExplicitPath(t *testing.T) {
	_, err := Load("/tmp/does-not-exist-infragenie-config.yml")
	require.Error(t, err)
}

func TestLoad_ProjectFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	yml := "providers:\n  default: google\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".infragenie.yml"), []byte(yml), 0o644))

	cfg, err := Load("")
	require.NoError(t, err)
	require.Equal(t, "google", cfg.Providers.Default)
}

// APIKey

func TestAPIKey_EnvVarWins(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	cfg := &AppConfig{
		Providers: ProvidersConfig{OpenAI: &ProviderEntry{APIKey: "file-key"}},
	}
	require.Equal(t, "env-key", cfg.APIKey("openai"))
}

func TestAPIKey_FallsBackToConfig(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	cfg := &AppConfig{
		Providers: ProvidersConfig{OpenAI: &ProviderEntry{APIKey: "file-key"}},
	}
	require.Equal(t, "file-key", cfg.APIKey("openai"))
}

func TestAPIKey_EmptyWhenNeitherSet(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	cfg := &AppConfig{}
	require.Equal(t, "", cfg.APIKey("openai"))
}

func TestAPIKey_AllProviders(t *testing.T) {
	cases := []struct{ provider, envVar, envVal string }{
		{"openai", "OPENAI_API_KEY", "sk-openai"},
		{"anthropic", "ANTHROPIC_API_KEY", "sk-ant"},
		{"google", "GOOGLE_API_KEY", "goog-key"},
		{"azure", "AZURE_OPENAI_API_KEY", "az-key"},
		{"unknown", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			if tc.envVar != "" {
				t.Setenv(tc.envVar, tc.envVal)
			}
			cfg := &AppConfig{}
			require.Equal(t, tc.envVal, cfg.APIKey(tc.provider))
		})
	}
}

// Model

func TestModel_EnvVarWins(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "gpt-4o")
	cfg := &AppConfig{
		Providers: ProvidersConfig{OpenAI: &ProviderEntry{Model: "gpt-3.5-turbo"}},
	}
	require.Equal(t, "gpt-4o", cfg.Model("openai"))
}

func TestModel_FallsBackToConfig(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "")
	cfg := &AppConfig{
		Providers: ProvidersConfig{OpenAI: &ProviderEntry{Model: "gpt-3.5-turbo"}},
	}
	require.Equal(t, "gpt-3.5-turbo", cfg.Model("openai"))
}

func TestModel_EmptyWhenNeitherSet(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "")
	cfg := &AppConfig{}
	require.Equal(t, "", cfg.Model("anthropic"))
}

func TestModel_AllProviders(t *testing.T) {
	cases := []struct{ provider, envVar, model string }{
		{"openai", "OPENAI_MODEL", "gpt-4o"},
		{"anthropic", "ANTHROPIC_MODEL", "claude-3-5-haiku-20241022"},
		{"google", "GOOGLE_MODEL", "gemini-1.5-flash"},
		{"azure", "AZURE_OPENAI_MODEL", "gpt-4"},
		{"local", "LOCAL_LLM_MODEL", "llama3"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			t.Setenv(tc.envVar, tc.model)
			require.Equal(t, tc.model, (&AppConfig{}).Model(tc.provider))
		})
	}
}

// BaseURL

func TestBaseURL_LocalEnvVar(t *testing.T) {
	t.Setenv("LOCAL_LLM_URL", "http://localhost:11434")
	cfg := &AppConfig{
		Providers: ProvidersConfig{Local: &LocalProviderEntry{BaseURL: "http://other:9999"}},
	}
	require.Equal(t, "http://localhost:11434", cfg.BaseURL("local"))
}

func TestBaseURL_LocalConfigFallback(t *testing.T) {
	t.Setenv("LOCAL_LLM_URL", "")
	cfg := &AppConfig{
		Providers: ProvidersConfig{Local: &LocalProviderEntry{BaseURL: "http://localhost:11434"}},
	}
	require.Equal(t, "http://localhost:11434", cfg.BaseURL("local"))
}

func TestBaseURL_AzureEnvVar(t *testing.T) {
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://my.openai.azure.com")
	cfg := &AppConfig{}
	require.Equal(t, "https://my.openai.azure.com", cfg.BaseURL("azure"))
}

func TestBaseURL_EmptyForUnknown(t *testing.T) {
	cfg := &AppConfig{}
	require.Equal(t, "", cfg.BaseURL("openai"))
}

// Defaults

func TestDefaultProvider_EnvVar(t *testing.T) {
	t.Setenv("INFRAGENIE_PROVIDER", "anthropic")
	cfg := &AppConfig{Providers: ProvidersConfig{Default: "openai"}}
	require.Equal(t, "anthropic", cfg.DefaultProvider())
}

func TestDefaultProvider_ConfigFallback(t *testing.T) {
	t.Setenv("INFRAGENIE_PROVIDER", "")
	cfg := &AppConfig{Providers: ProvidersConfig{Default: "openai"}}
	require.Equal(t, "openai", cfg.DefaultProvider())
}

func TestDefaultFormat_EnvVar(t *testing.T) {
	t.Setenv("INFRAGENIE_FORMAT", "github")
	cfg := &AppConfig{Defaults: Defaults{Format: "text"}}
	require.Equal(t, "github", cfg.DefaultFormat())
}

func TestDefaultFailOn_EnvVar(t *testing.T) {
	t.Setenv("INFRAGENIE_FAIL_ON", "critical")
	cfg := &AppConfig{Defaults: Defaults{FailOn: "high"}}
	require.Equal(t, "critical", cfg.DefaultFailOn())
}

func TestDefaultFailOn_ConfigFallback(t *testing.T) {
	t.Setenv("INFRAGENIE_FAIL_ON", "")
	cfg := &AppConfig{Defaults: Defaults{FailOn: "high"}}
	require.Equal(t, "high", cfg.DefaultFailOn())
}

func TestDefaultBudgetTokens_EnvVar(t *testing.T) {
	t.Setenv("INFRAGENIE_BUDGET_TOKENS", "75000")
	cfg := &AppConfig{Defaults: Defaults{Budget: Budget{Tokens: 50000}}}
	require.Equal(t, 75000, cfg.DefaultBudgetTokens())
}

func TestDefaultBudgetTokens_InvalidEnvFallsBackToConfig(t *testing.T) {
	t.Setenv("INFRAGENIE_BUDGET_TOKENS", "not-a-number")
	cfg := &AppConfig{Defaults: Defaults{Budget: Budget{Tokens: 50000}}}
	require.Equal(t, 50000, cfg.DefaultBudgetTokens())
}

func TestDefaultBudgetUSD_EnvVar(t *testing.T) {
	t.Setenv("INFRAGENIE_BUDGET_USD", "2.50")
	cfg := &AppConfig{Defaults: Defaults{Budget: Budget{USD: 0.50}}}
	require.InDelta(t, 2.50, cfg.DefaultBudgetUSD(), 0.001)
}

func TestDefaultBudgetUSD_InvalidEnvFallsBackToConfig(t *testing.T) {
	t.Setenv("INFRAGENIE_BUDGET_USD", "bad")
	cfg := &AppConfig{Defaults: Defaults{Budget: Budget{USD: 0.50}}}
	require.InDelta(t, 0.50, cfg.DefaultBudgetUSD(), 0.001)
}

// DefaultPath

func TestDefaultPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	p := DefaultPath()
	require.Equal(t, "/custom/config/infragenie/config.yml", p)
}

func TestDefaultPath_HomeDirFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	p := DefaultPath()
	require.Contains(t, p, ".config/infragenie/config.yml")
}

// helpers

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoad_EnvInterpolation(t *testing.T) {
	t.Setenv("MY_OPENAI_SECRET", "sk-from-env")
	yml := `
providers:
  default: openai
  openai:
    api_key: ${MY_OPENAI_SECRET}
    model: gpt-4o-mini
`
	cfg, err := Load(writeTemp(t, yml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Providers.OpenAI)
	require.Equal(t, "sk-from-env", cfg.Providers.OpenAI.APIKey, "${VAR} should resolve from env")
}

func TestLoad_EnvInterpolation_Unset(t *testing.T) {
	yml := "providers:\n  openai:\n    api_key: ${DEFINITELY_UNSET_VAR_XYZ}\n"
	cfg, err := Load(writeTemp(t, yml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Providers.OpenAI)
	require.Equal(t, "", cfg.Providers.OpenAI.APIKey, "unset var expands to empty")
}
