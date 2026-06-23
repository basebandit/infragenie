package render

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverChangedCharts(t *testing.T) {
	dir := t.TempDir()
	// Lay out a chart at <dir>/charts/app with a template, and an unrelated file.
	chart := filepath.Join(dir, "charts", "app")
	require.NoError(t, os.MkdirAll(filepath.Join(chart, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chart, "Chart.yaml"), []byte("apiVersion: v2\nname: app\n"), 0o644))

	changed := []string{
		filepath.Join(chart, "templates", "deployment.yaml"), // inside the chart
		filepath.Join(dir, "README.md"),                      // not in any chart
	}
	got := DiscoverChangedCharts(changed)
	require.Equal(t, []string{chart}, got)
}

func TestDiscoverChangedCharts_NoChart(t *testing.T) {
	require.Empty(t, DiscoverChangedCharts([]string{"main.go", "internal/x/y.go"}))
}

func TestRenderHelmChart(t *testing.T) {
	if !HelmAvailable() {
		t.Skip("helm not installed")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "cm.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}-cm\n"), 0o644))

	out, err := RenderHelmChart(context.Background(), dir)
	require.NoError(t, err)
	require.Contains(t, out, "kind: ConfigMap")
	require.Contains(t, out, "release-cm") // {{ .Release.Name }} rendered
	require.NotContains(t, out, "{{")      // no template left
}

func TestDiscoverChangedKustomizations(t *testing.T) {
	dir := t.TempDir()
	overlay := filepath.Join(dir, "overlays", "prod")
	require.NoError(t, os.MkdirAll(overlay, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overlay, "kustomization.yaml"),
		[]byte("resources:\n  - ../../base\n"), 0o644))

	changed := []string{
		filepath.Join(overlay, "patch.yaml"),
		filepath.Join(dir, "README.md"),
	}
	require.Equal(t, []string{overlay}, DiscoverChangedKustomizations(changed))
	require.Empty(t, DiscoverChangedKustomizations([]string{"main.go"}))
}

func TestRenderKustomization(t *testing.T) {
	if !KustomizeAvailable() {
		t.Skip("kustomize/kubectl not installed")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kustomization.yaml"),
		[]byte("resources:\n  - cm.yaml\ncommonLabels:\n  team: platform\n"), 0o644))

	out, err := RenderKustomization(context.Background(), dir)
	require.NoError(t, err)
	require.Contains(t, out, "kind: ConfigMap")
	require.Contains(t, out, "team: platform") // overlay applied
}

func TestRenderedPath(t *testing.T) {
	require.True(t, strings.HasSuffix(RenderedPath("charts/app", "helm-rendered"), "<helm-rendered>.yaml"))
}
