package models

type GoldenPath struct {
	Version int    `yaml:"version"`
	Name    string `yaml:"name,omitempty"`
	Extends string `yaml:"extends,omitempty"`

	Scanners       ScannerConfig             `yaml:"scanners,omitempty"`
	Stacks         Stacks                    `yaml:"stacks,omitempty"`
	RequiredLabels []string                  `yaml:"required_labels,omitempty"`
	ChartShape     ChartShape                `yaml:"chart_shape,omitempty"`
	CIRequired     []CIStep                  `yaml:"ci_required_steps,omitempty"`
	RuntimeRules   map[string]map[string]any `yaml:"runtime_rules,omitempty"`
	Security       Security                  `yaml:"security,omitempty"`
	Observability  Observability             `yaml:"observability,omitempty"`
	Templates      map[string]string         `yaml:"templates,omitempty"`
	References     []string                  `yaml:"reference_services,omitempty"`
	Defaults       Defaults                  `yaml:"defaults,omitempty"`

	// Override-mode fields
	FailOn Severity                `yaml:"fail_on,omitempty"`
	Budget Budget                  `yaml:"budget,omitempty"`
	Rules  map[string]RuleOverride `yaml:"rules,omitempty"`
	Ignore []IgnoreRule            `yaml:"ignore,omitempty"`
}

// ScannerConfig controls which Layer-1 scanners run for this golden path.
// Enable is an allowlist; if set, only listed scanners run (after stack-detection).
// Disable is a blocklist applied after all other selection logic.
type ScannerConfig struct {
	Enable  []string `yaml:"enable,omitempty"`
	Disable []string `yaml:"disable,omitempty"`
}

type Stacks struct {
	Runtimes  []string `yaml:"runtimes,omitempty"`
	Platforms []string `yaml:"platforms,omitempty"`
	Cloud     []string `yaml:"cloud,omitempty"`
	CI        []string `yaml:"ci,omitempty"`
}

type ChartShape struct {
	ChartsDir     string   `yaml:"charts_dir,omitempty"`
	RequiredFiles []string `yaml:"required_files,omitempty"`
}

type CIStep struct {
	Name    string   `yaml:"name"`
	Matches []string `yaml:"matches"`
}

type Security struct {
	RequireNetworkPolicy  bool `yaml:"require_network_policy,omitempty"`
	RequireNonRoot        bool `yaml:"require_non_root,omitempty"`
	RequireReadOnlyRootFS bool `yaml:"require_read_only_rootfs,omitempty"`
	ForbidImageTagLatest  bool `yaml:"forbid_image_tag_latest,omitempty"`
}

type Observability struct {
	RequirePrometheusAnnotations bool `yaml:"require_prometheus_annotations,omitempty"`
	RequireStructuredLogs        bool `yaml:"require_structured_logs,omitempty"`
}

type Defaults struct {
	Reviewers map[string]bool `yaml:"reviewers,omitempty"`
	FailOn    Severity        `yaml:"fail_on,omitempty"`
	Budget    Budget          `yaml:"budget,omitempty"`
}

type Budget struct {
	Tokens int     `yaml:"tokens,omitempty"`
	USD    float64 `yaml:"usd,omitempty"`
}

type RuleOverride struct {
	Severity Severity `yaml:"severity,omitempty"`
	Enabled  *bool    `yaml:"enabled,omitempty"`
}

type IgnoreRule struct {
	Path  string   `yaml:"path"`
	Rules []string `yaml:"rules"`
}

func (g *GoldenPath) IsOverride() bool { return g.Extends != "" }
