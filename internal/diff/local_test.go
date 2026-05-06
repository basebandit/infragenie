package diff

import (
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
