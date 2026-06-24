// Package prcomment renders an InfraGenie review summary and posts it as a
// single, deduplicated comment on a GitHub pull request: subsequent runs update
// the same comment instead of piling new ones up.
package prcomment

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/google/go-github/v66/github"
)

// marker is a hidden HTML comment used to find InfraGenie's own comment on
// re-runs. It never renders in the GitHub UI.
const marker = "<!-- infragenie:review-summary -->"

// Render builds the markdown comment body for a set of findings.
func Render(findings []models.Finding) string {
	var b strings.Builder
	b.WriteString(marker + "\n")
	b.WriteString("## InfraGenie review\n\n")
	if len(findings) == 0 {
		b.WriteString("No findings. ✅\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d finding(s):\n\n", len(findings))
	b.WriteString("| Severity | Rule | Location | Title |\n|---|---|---|---|\n")
	for _, f := range findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n",
			f.Severity, f.RuleID, loc, oneLine(f.Title))
	}
	return b.String()
}

func oneLine(s string) string { return strings.ReplaceAll(s, "\n", " ") }

// Upsert creates the InfraGenie summary comment on a PR, or updates the existing
// one if a previous run already posted it (matched by the hidden marker).
func Upsert(ctx context.Context, client *github.Client, owner, repo string, number int, body string) error {
	opt := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, number, opt)
		if err != nil {
			return fmt.Errorf("list comments: %w", err)
		}
		for _, c := range comments {
			if strings.Contains(c.GetBody(), marker) {
				_, _, err := client.Issues.EditComment(ctx, owner, repo, c.GetID(),
					&github.IssueComment{Body: &body})
				return err
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	_, _, err := client.Issues.CreateComment(ctx, owner, repo, number,
		&github.IssueComment{Body: &body})
	return err
}
