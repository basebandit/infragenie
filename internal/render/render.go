// Package render turns templated infrastructure (Helm charts today) into the
// concrete Kubernetes manifests they produce, so the review engine can check the
// real objects instead of skipping un-rendered templates.
package render

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// HelmAvailable reports whether the `helm` binary is on PATH.
func HelmAvailable() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// RenderHelmChart runs `helm template` on a chart directory and returns the
// rendered multi-document YAML. The release name is fixed; it only affects
// generated resource names, which the reviewers don't key on.
func RenderHelmChart(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "helm", "template", "release", dir)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("helm template %s: %w", dir, err)
	}
	return string(out), nil
}

// DiscoverChangedCharts returns the Helm chart directories (those containing a
// Chart.yaml) that own at least one of changedPaths. Walking up from each
// changed file finds the nearest enclosing chart, so editing a single template
// renders the whole chart it belongs to.
func DiscoverChangedCharts(changedPaths []string) []string {
	set := map[string]bool{}
	for _, p := range changedPaths {
		dir := filepath.Dir(filepath.Clean(p))
		for {
			if fileExists(filepath.Join(dir, "Chart.yaml")) {
				set[dir] = true
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir || dir == "." || dir == string(filepath.Separator) {
				break
			}
			dir = parent
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// chartName returns a stable, readable identifier for a rendered chart, used as
// the synthetic file path the reviewers attribute findings to.
func RenderedPath(chartDir string) string {
	return filepath.Join(strings.TrimPrefix(filepath.Clean(chartDir), "./"), "<helm-rendered>.yaml")
}
