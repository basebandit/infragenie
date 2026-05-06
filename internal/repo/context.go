// Package repo walks a working tree and builds the engine's read-only
// context: file list, detected languages, deployment platforms, and CI
// system, plus a one-line summary suitable for prompt headers.
package repo

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type Context struct {
	Root      string
	Files     []string
	Languages []string
	Stacks    []string
	CISystems []string
	Summary   string
}

var skipDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true,
	"node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true,
	".terraform": true, ".venv": true, "venv": true,
	"__pycache__": true,
}

var manifestLang = map[string]string{
	"go.mod":           "go",
	"package.json":     "node",
	"pyproject.toml":   "python",
	"requirements.txt": "python",
	"Pipfile":          "python",
	"Cargo.toml":       "rust",
	"pom.xml":          "java",
	"build.gradle":     "java",
	"build.gradle.kts": "java",
	"Gemfile":          "ruby",
	"composer.json":    "php",
	"mix.exs":          "elixir",
	"deno.json":        "deno",
	"deno.jsonc":       "deno",
}

// Build walks root and produces a Context. Best-effort: walk errors do not
// abort; missing manifests just leave fields empty.
func Build(root string) (*Context, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	c := &Context{Root: abs}
	langs, stacks, cis := map[string]bool{}, map[string]bool{}, map[string]bool{}

	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != abs && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		c.Files = append(c.Files, rel)

		base := d.Name()
		if lang, ok := manifestLang[base]; ok {
			langs[lang] = true
		}
		if hasAnySuffix(base, ".csproj", ".fsproj", ".vbproj") {
			langs["dotnet"] = true
		}

		switch {
		case base == "Chart.yaml":
			stacks["helm"] = true
		case base == "kustomization.yaml" || base == "kustomization.yml":
			stacks["kustomize"] = true
		case strings.HasSuffix(base, ".tf"):
			stacks["terraform"] = true
		case base == "Dockerfile" || strings.HasSuffix(base, ".dockerfile"):
			stacks["docker"] = true
		case base == "serverless.yml" || base == "serverless.yaml":
			stacks["serverless"] = true
		case base == "template.yaml" || base == "template.yml":
			stacks["sam"] = true
		}

		switch {
		case strings.HasPrefix(rel, ".github/workflows/"):
			cis["github-actions"] = true
		case rel == ".gitlab-ci.yml":
			cis["gitlab-ci"] = true
		case rel == "Jenkinsfile":
			cis["jenkins"] = true
		case rel == ".circleci/config.yml":
			cis["circleci"] = true
		case rel == "azure-pipelines.yml":
			cis["azure-pipelines"] = true
		case strings.HasPrefix(rel, ".buildkite/"):
			cis["buildkite"] = true
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	c.Languages = sortedKeys(langs)
	c.Stacks = sortedKeys(stacks)
	c.CISystems = sortedKeys(cis)
	c.Summary = fmt.Sprintf("Languages: %v; Platforms: %v; CI: %v; Files: %d",
		c.Languages, c.Stacks, c.CISystems, len(c.Files))
	return c, nil
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
