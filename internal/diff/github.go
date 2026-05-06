package diff

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/google/go-github/v66/github"
)

type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// ParsePRURL accepts a GitHub PR URL or owner/repo#N or owner/repo/N.
func ParsePRURL(s string) (PRRef, error) {
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) < 4 || parts[2] != "pull" {
			return PRRef{}, fmt.Errorf("not a PR url: %s", s)
		}
		n, err := strconv.Atoi(parts[3])
		if err != nil {
			return PRRef{}, fmt.Errorf("bad PR number: %s", parts[3])
		}
		return PRRef{Owner: parts[0], Repo: parts[1], Number: n}, nil
	}
	// owner/repo#N or owner/repo/N
	sep := strings.LastIndexAny(s, "#/")
	if sep < 0 {
		return PRRef{}, fmt.Errorf("cannot parse PR ref: %s", s)
	}
	left, right := s[:sep], s[sep+1:]
	or := strings.Split(left, "/")
	if len(or) != 2 {
		return PRRef{}, fmt.Errorf("cannot parse PR ref: %s", s)
	}
	n, err := strconv.Atoi(right)
	if err != nil {
		return PRRef{}, fmt.Errorf("bad PR number: %s", right)
	}
	return PRRef{Owner: or[0], Repo: or[1], Number: n}, nil
}

// GitHubPR fetches a PR's file list and patches via the GitHub API and
// returns a Diff. NewContent is left empty in v0; grounding (Phase D) can
// re-fetch when needed.
func GitHubPR(ctx context.Context, ref PRRef, token string) (*models.Diff, error) {
	client := github.NewClient(nil)
	if token != "" {
		client = client.WithAuthToken(token)
	}

	var all []*github.CommitFile
	opt := &github.ListOptions{PerPage: 100}
	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, ref.Owner, ref.Repo, ref.Number, opt)
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}
		all = append(all, files...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	out := make([]models.FileDiff, 0, len(all))
	for _, f := range all {
		fd := models.FileDiff{
			Path:    f.GetFilename(),
			OldPath: f.GetPreviousFilename(),
			Status:  normalizeStatus(f.GetStatus()),
		}
		if patch := f.GetPatch(); patch != "" {
			wrapped := fmt.Sprintf("--- a/%s\n+++ b/%s\n%s\n", fd.Path, fd.Path, patch)
			if parsed, err := Parse([]byte(wrapped)); err == nil && len(parsed) > 0 {
				fd.Hunks = parsed[0].Hunks
			}
		}
		out = append(out, fd)
	}
	return &models.Diff{Source: models.DiffSourcePR, Files: out}, nil
}

func normalizeStatus(s string) string {
	if s == "removed" {
		return "deleted"
	}
	return s
}
