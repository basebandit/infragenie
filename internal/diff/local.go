package diff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
