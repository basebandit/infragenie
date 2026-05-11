package main

import (
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/internal/scanners/infra"
	"github.com/basebandit/infragenie/internal/scanners/lang"
)

// ScannerMeta describes a scanner for display purposes.
type ScannerMeta struct {
	Scanner     scanners.Scanner
	Category    string
	Description string
	InstallHint string
}

// registeredScanners is the canonical list of all supported scanners.
// Add new adapters here — review and scanners-list both derive from this.
func registeredScanners() []ScannerMeta {
	return []ScannerMeta{
		{
			Scanner:     infra.NewCheckov(),
			Category:    "infra",
			Description: "Terraform, K8s manifests, Helm, Dockerfiles",
			InstallHint: "pip install checkov",
		},
		{
			Scanner:     infra.NewHadolint(),
			Category:    "infra",
			Description: "Dockerfile best practices",
			InstallHint: "wget -qO /usr/local/bin/hadolint https://github.com/hadolint/hadolint/releases/latest/download/hadolint-Linux-x86_64 && chmod +x /usr/local/bin/hadolint",
		},
		{
			Scanner:     lang.NewSemgrep(),
			Category:    "lang",
			Description: "Go, Python, JS/TS, Java, Ruby, Rust and more — community ruleset",
			InstallHint: "pip install semgrep",
		},
		{
			Scanner:     lang.NewGosec(),
			Category:    "lang",
			Description: "Go source security issues",
			InstallHint: "go install github.com/securego/gosec/v2/cmd/gosec@latest",
		},
	}
}

// allScanners returns just the Scanner interface slice for engine use.
func allScanners() []scanners.Scanner {
	meta := registeredScanners()
	out := make([]scanners.Scanner, len(meta))
	for i, m := range meta {
		out[i] = m.Scanner
	}
	return out
}
