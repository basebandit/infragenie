package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
}

func TestBuildPolyglot(t *testing.T) {
	d := t.TempDir()
	write(t, d, "go.mod", "module x")
	write(t, d, "services/api/package.json", "{}")
	write(t, d, "services/etl/pyproject.toml", "[project]")
	write(t, d, "infra/main.tf", "")
	write(t, d, "charts/svc/Chart.yaml", "name: svc")
	write(t, d, ".github/workflows/ci.yml", "")
	write(t, d, "Dockerfile", "FROM scratch")
	write(t, d, "node_modules/lodash/index.js", "// skipped")
	write(t, d, "vendor/x/y.go", "// skipped")

	c, err := Build(d)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"go", "node", "python"}, c.Languages)
	require.ElementsMatch(t, []string{"docker", "helm", "terraform"}, c.Stacks)
	require.Equal(t, []string{"github-actions"}, c.CISystems)
	for _, f := range c.Files {
		require.NotContains(t, f, "node_modules", "should skip node_modules")
		require.NotContains(t, f, "vendor"+string(filepath.Separator), "should skip vendor")
	}
}

func TestBuildEmpty(t *testing.T) {
	c, err := Build(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, c.Languages)
	require.Empty(t, c.Stacks)
	require.Empty(t, c.CISystems)
}
