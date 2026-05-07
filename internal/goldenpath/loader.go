// Package goldenpath loads, validates, and resolves goldenpath.yml files in
// canonical mode and override mode (`extends:`).
package goldenpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
	"gopkg.in/yaml.v3"
)

const SchemaMajorVersion = 1

type Loader struct {
	BaseDir string
}

func New(baseDir string) *Loader { return &Loader{BaseDir: baseDir} }

// Load reads a goldenpath.yml at path. If `extends:` is set, recursively
// resolves the parent and merges. Returns the resolved (post-extends) value.
func (l *Loader) Load(path string) (*models.GoldenPath, error) {
	gp, err := l.loadOne(path)
	if err != nil {
		return nil, err
	}
	if !gp.IsOverride() {
		return gp, nil
	}
	return l.resolve(gp, path)
}

func (l *Loader) loadOne(path string) (*models.GoldenPath, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var gp models.GoldenPath
	if err := yaml.Unmarshal(b, &gp); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if gp.Version != SchemaMajorVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (expected %d)", path, gp.Version, SchemaMajorVersion)
	}
	return &gp, nil
}

func (l *Loader) resolve(child *models.GoldenPath, childPath string) (*models.GoldenPath, error) {
	if isRemoteRef(child.Extends) {
		return nil, fmt.Errorf("remote extends not supported in v0: %s", child.Extends)
	}
	parentPath := child.Extends
	if !filepath.IsAbs(parentPath) {
		parentPath = filepath.Join(filepath.Dir(childPath), parentPath)
	}
	parent, err := l.Load(parentPath)
	if err != nil {
		return nil, fmt.Errorf("resolve extends %s: %w", child.Extends, err)
	}
	return merge(parent, child), nil
}

func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "github.com/") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "git@")
}

// merge layers child overrides on top of parent. Scalars override when set;
// `rules:` and `ignore:` accumulate. Parent's `defaults:` becomes the
// effective `fail_on` / `budget` when child doesn't set them.
func merge(parent, child *models.GoldenPath) *models.GoldenPath {
	out := *parent
	out.Extends = child.Extends

	if child.FailOn != "" {
		out.FailOn = child.FailOn
	} else {
		out.FailOn = parent.Defaults.FailOn
	}
	if child.Budget.Tokens > 0 || child.Budget.USD > 0 {
		out.Budget = child.Budget
	} else {
		out.Budget = parent.Defaults.Budget
	}

	if len(child.Rules) > 0 {
		if out.Rules == nil {
			out.Rules = map[string]models.RuleOverride{}
		}
		for k, v := range child.Rules {
			out.Rules[k] = v
		}
	}
	out.Ignore = append(append([]models.IgnoreRule{}, parent.Ignore...), child.Ignore...)
	return &out
}

// Validate runs structural checks on a (resolved) GoldenPath.
func Validate(gp *models.GoldenPath) error {
	if gp == nil {
		return fmt.Errorf("nil goldenpath")
	}
	if gp.Version != SchemaMajorVersion {
		return fmt.Errorf("version: %d (expected %d)", gp.Version, SchemaMajorVersion)
	}
	if gp.Extends == "" && gp.Name == "" {
		return fmt.Errorf("canonical mode: name is required")
	}
	return nil
}
