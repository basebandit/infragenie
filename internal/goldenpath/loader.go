// Package goldenpath loads, validates, and resolves goldenpath.yml files in
// canonical mode and override mode (`extends:`). Parents may be local paths or
// remote refs (https URLs or github.com/owner/repo/path@ref).
package goldenpath

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/basebandit/infragenie/pkg/models"
	"gopkg.in/yaml.v3"
)

const SchemaMajorVersion = 1

// maxRemoteSize caps how much we read from a remote extends target.
const maxRemoteSize = 1 << 20 // 1 MiB

type Loader struct {
	BaseDir string
	// HTTPClient fetches remote extends targets; nil uses a 15s-timeout client.
	HTTPClient *http.Client
}

func New(baseDir string) *Loader { return &Loader{BaseDir: baseDir} }

// Load reads a goldenpath.yml at path. If `extends:` is set, recursively
// resolves the parent (local or remote) and merges. Returns the resolved value.
func (l *Loader) Load(path string) (*models.GoldenPath, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return l.resolve(b, path)
}

func (l *Loader) resolve(b []byte, src string) (*models.GoldenPath, error) {
	gp, err := parse(b, src)
	if err != nil {
		return nil, err
	}
	if !gp.IsOverride() {
		return gp, nil
	}
	pb, parentSrc, err := l.loadParent(gp.Extends, src)
	if err != nil {
		return nil, fmt.Errorf("resolve extends %s: %w", gp.Extends, err)
	}
	parent, err := l.resolve(pb, parentSrc)
	if err != nil {
		return nil, err
	}
	return merge(parent, gp), nil
}

func parse(b []byte, src string) (*models.GoldenPath, error) {
	var gp models.GoldenPath
	if err := yaml.Unmarshal(b, &gp); err != nil {
		return nil, fmt.Errorf("parse %s: %w", src, err)
	}
	if gp.Version != SchemaMajorVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (expected %d)", src, gp.Version, SchemaMajorVersion)
	}
	return &gp, nil
}

// loadParent returns the bytes and a display source for an extends target,
// resolving it locally or fetching it remotely.
func (l *Loader) loadParent(ref, childSrc string) ([]byte, string, error) {
	if isRemoteRef(ref) {
		url, err := remoteURL(ref)
		if err != nil {
			return nil, "", err
		}
		b, err := l.fetch(url)
		return b, ref, err
	}
	parentPath := ref
	if !filepath.IsAbs(parentPath) {
		parentPath = filepath.Join(filepath.Dir(childSrc), parentPath)
	}
	b, err := os.ReadFile(parentPath)
	if err != nil {
		return nil, "", err
	}
	return b, parentPath, nil
}

func (l *Loader) fetch(url string) ([]byte, error) {
	client := l.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxRemoteSize))
}

func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "github.com/") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "git@")
}

// remoteURL turns an extends ref into a fetchable URL. https/http pass through;
// github.com/owner/repo/path/to/file.yml@ref becomes a raw.githubusercontent URL
// (ref defaults to main). SSH (git@) is unsupported.
func remoteURL(ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "https://"), strings.HasPrefix(ref, "http://"):
		return ref, nil
	case strings.HasPrefix(ref, "github.com/"):
		rest := strings.TrimPrefix(ref, "github.com/")
		gitRef := "main"
		if i := strings.LastIndex(rest, "@"); i >= 0 {
			gitRef, rest = rest[i+1:], rest[:i]
		}
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid github ref %q: want github.com/owner/repo/path@ref", ref)
		}
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", parts[0], parts[1], gitRef, parts[2]), nil
	default:
		return "", fmt.Errorf("unsupported remote extends %q (use https:// or github.com/owner/repo/path@ref)", ref)
	}
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
		maps.Copy(out.Rules, child.Rules)
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
