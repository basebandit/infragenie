package goldenpath

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func TestLoadCanonical(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "goldenpath.yml", `
version: 1
name: test
required_labels: [app, team]
defaults:
  fail_on: high
`)
	gp, err := New(dir).Load(p)
	require.NoError(t, err)
	require.Equal(t, "test", gp.Name)
	require.Equal(t, []string{"app", "team"}, gp.RequiredLabels)
	require.Equal(t, models.SeverityHigh, gp.Defaults.FailOn)
	require.NoError(t, Validate(gp))
}

func TestLoadOverrideExtends(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "parent.yml", `
version: 1
name: parent
required_labels: [app]
defaults:
  fail_on: high
  budget:
    tokens: 100000
`)
	child := writeFile(t, dir, "goldenpath.yml", `
version: 1
extends: parent.yml
fail_on: medium
ignore:
  - path: examples/**
    rules: ["*"]
`)
	gp, err := New(dir).Load(child)
	require.NoError(t, err)
	require.Equal(t, "parent", gp.Name)
	require.Equal(t, []string{"app"}, gp.RequiredLabels)
	require.Equal(t, models.SeverityMedium, gp.FailOn)
	require.Equal(t, 100000, gp.Budget.Tokens)
	require.Len(t, gp.Ignore, 1)
}

func TestRemoteExtends(t *testing.T) {
	parent := "version: 1\nname: remote-parent\nrequired_labels: [app]\ndefaults:\n  fail_on: high\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, parent)
	}))
	defer srv.Close()

	dir := t.TempDir()
	child := writeFile(t, dir, "goldenpath.yml", "version: 1\nextends: "+srv.URL+"\nfail_on: medium\n")
	gp, err := New(dir).Load(child)
	require.NoError(t, err)
	require.Equal(t, "remote-parent", gp.Name)
	require.Equal(t, []string{"app"}, gp.RequiredLabels)
	require.Equal(t, models.SeverityMedium, gp.FailOn)
}

func TestRemoteURL(t *testing.T) {
	u, err := remoteURL("github.com/owner/repo/path/to/goldenpath.yml@v1.2.0")
	require.NoError(t, err)
	require.Equal(t, "https://raw.githubusercontent.com/owner/repo/v1.2.0/path/to/goldenpath.yml", u)

	u, err = remoteURL("github.com/owner/repo/goldenpath.yml")
	require.NoError(t, err)
	require.Equal(t, "https://raw.githubusercontent.com/owner/repo/main/goldenpath.yml", u)

	u, err = remoteURL("https://example.com/gp.yml")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/gp.yml", u)

	_, err = remoteURL("git@github.com:owner/repo.git")
	require.Error(t, err)
}

func TestVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "goldenpath.yml", `version: 99`)
	_, err := New(dir).Load(p)
	require.ErrorContains(t, err, "unsupported version")
}

func TestValidateCanonicalRequiresName(t *testing.T) {
	require.ErrorContains(t, Validate(&models.GoldenPath{Version: 1}), "name is required")
}
