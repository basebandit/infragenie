// Package render turns templated infrastructure (Helm charts, kustomize overlays)
// into the concrete Kubernetes manifests they produce, so the review engine can
// check the real objects instead of skipping un-rendered templates.
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

// KustomizeAvailable reports whether kustomize can run, via the kustomize binary
// or `kubectl kustomize`.
func KustomizeAvailable() bool {
	if _, err := exec.LookPath("kustomize"); err == nil {
		return true
	}
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// RenderKustomization builds a kustomize directory into concrete manifests,
// preferring the kustomize binary and falling back to `kubectl kustomize`.
func RenderKustomization(ctx context.Context, dir string) (string, error) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		out, err := exec.CommandContext(ctx, "kustomize", "build", dir).Output()
		if err != nil {
			return "", fmt.Errorf("kustomize build %s: %w", dir, err)
		}
		return string(out), nil
	}
	out, err := exec.CommandContext(ctx, "kubectl", "kustomize", dir).Output()
	if err != nil {
		return "", fmt.Errorf("kubectl kustomize %s: %w", dir, err)
	}
	return string(out), nil
}

// DiscoverChangedCharts returns the Helm chart directories (those containing a
// Chart.yaml) that own at least one of changedPaths.
func DiscoverChangedCharts(changedPaths []string) []string {
	return discoverChanged(changedPaths, []string{"Chart.yaml"})
}

// DiscoverChangedKustomizations returns the kustomize directories that own at
// least one of changedPaths.
func DiscoverChangedKustomizations(changedPaths []string) []string {
	return discoverChanged(changedPaths, []string{"kustomization.yaml", "kustomization.yml", "Kustomization"})
}

// discoverChanged walks up from each changed file to the nearest enclosing
// directory that contains one of markers, so editing a single file renders the
// whole unit (chart or overlay) it belongs to.
func discoverChanged(changedPaths, markers []string) []string {
	set := map[string]bool{}
	for _, p := range changedPaths {
		dir := filepath.Dir(filepath.Clean(p))
		for {
			if anyFileExists(dir, markers) {
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

func anyFileExists(dir string, names []string) bool {
	for _, n := range names {
		if fileExists(filepath.Join(dir, n)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// RenderedPath is the synthetic file path findings on rendered output are
// attributed to. label distinguishes the renderer (e.g. helm-rendered).
func RenderedPath(dir, label string) string {
	return filepath.Join(strings.TrimPrefix(filepath.Clean(dir), "./"), "<"+label+">.yaml")
}
