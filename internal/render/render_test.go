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

func TestRenderedPath(t *testing.T) {
	require.True(t, strings.HasSuffix(RenderedPath("charts/app"), "<helm-rendered>.yaml"))
}
