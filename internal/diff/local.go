package diff

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/basebandit/infragenie/pkg/models"
)

type LocalOptions struct {
	Root   string
	Base   string // optional base ref (e.g., "main"); empty = working diff
	Staged bool   // only staged changes
}

// Local invokes `git diff` per opts and returns a parsed Diff with
// best-effort NewContent loaded from the working tree for non-deleted files.
func Local(ctx context.Context, opts LocalOptions) (*models.Diff, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	args := []string{"diff", "--no-color", "-U3"}
	if opts.Staged {
		args = append(args, "--staged")
	}
	if opts.Base != "" {
		args = append(args, opts.Base+"...HEAD")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = opts.Root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
				return nil, fmt.Errorf("git diff: %w: %s", err, msg)
			}
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}
	files, err := Parse(out)
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Status == "deleted" || files[i].Path == "" {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(opts.Root, files[i].Path)); err == nil {
			files[i].NewContent = string(b)
		}
	}
	return &models.Diff{Source: models.DiffSourceLocal, Files: files}, nil
}

// maxTreeFile caps the size of a file read during a Tree scan.
const maxTreeFile = 1 << 20 // 1 MiB

var treeSkipDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true,
	"node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true,
	".terraform": true, ".venv": true, "venv": true,
	"__pycache__": true, ".pytest_cache": true, ".ruff_cache": true,
}

// Tree builds a Diff from the current files under root, with every text file
// marked as added and its contents loaded. It is the "review what's here now"
// mode: no git history required. Binary and oversized files are skipped.
func Tree(root string) (*models.Diff, error) {
	if root == "" {
		root = "."
	}
	var files []models.FileDiff
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && treeSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info, ierr := d.Info(); ierr == nil && info.Size() > maxTreeFile {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			return nil // skip binary
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		files = append(files, models.FileDiff{Path: rel, Status: "added", NewContent: string(b)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &models.Diff{Source: models.DiffSourceLocal, Files: files}, nil
}
