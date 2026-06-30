package diff

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), args)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Run(), args)
}

func TestLocalUnstagedDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644))
	gitCmd(t, dir, "add", "a.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "init")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\nworld\n"), 0o644))

	d, err := Local(context.Background(), LocalOptions{Root: dir})
	require.NoError(t, err)
	require.Len(t, d.Files, 1)
	require.Equal(t, "a.txt", d.Files[0].Path)
	require.Equal(t, "hello\nworld\n", d.Files[0].NewContent)
}

func TestTreeScan(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "k8s"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "k8s", "deploy.yaml"), []byte("apiVersion: apps/v1\nkind: Deployment\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644))
	// skipped: vendored dir and a binary file
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "x.js"), []byte("ignored"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0x00, 0x01, 0x02}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.txt"), bytes.Repeat([]byte("a"), maxTreeFile+1), 0o644))

	d, err := Tree(context.Background(), dir)
	require.NoError(t, err)

	got := map[string]string{}
	for _, f := range d.Files {
		got[f.Path] = f.NewContent
		require.Equal(t, "added", f.Status)
	}
	require.Contains(t, got, filepath.Join("k8s", "deploy.yaml"))
	require.Contains(t, got, "Dockerfile")
	require.Contains(t, got["k8s/deploy.yaml"], "kind: Deployment")
	require.NotContains(t, got, filepath.Join("node_modules", "x.js"), "vendored dirs skipped")
	require.NotContains(t, got, "bin.dat", "binary files skipped")
	require.NotContains(t, got, "big.txt", "oversized files skipped")
}

func TestTreeScan_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x: 1\n"), 0o644))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Tree(ctx, dir)
	require.Error(t, err)
}
