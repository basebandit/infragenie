package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/basebandit/infragenie/internal/diff"
	"github.com/basebandit/infragenie/internal/engine"
	"github.com/basebandit/infragenie/internal/goldenpath"
	"github.com/basebandit/infragenie/internal/prcomment"
	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/internal/webhook"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/google/go-github/v66/github"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the GitHub webhook server (reviews PRs and posts a comment)",
		Long: `serve listens for GitHub pull_request webhooks, reviews each PR against
your Golden Path, and posts a single deduplicated summary comment.

Environment:
  WEBHOOK_SECRET         shared secret for X-Hub-Signature-256 verification
  GITHUB_TOKEN           token used to fetch the PR diff and post comments
  INFRAGENIE_GOLDENPATH  path to goldenpath.yml (default: goldenpath.yml if present)`,
		RunE: func(_ *cobra.Command, _ []string) error {
			secret := os.Getenv("WEBHOOK_SECRET")
			token := os.Getenv("GITHUB_TOKEN")
			gpPath := os.Getenv("INFRAGENIE_GOLDENPATH")
			if gpPath == "" {
				if _, err := os.Stat("goldenpath.yml"); err == nil {
					gpPath = "goldenpath.yml"
				}
			}
			if secret == "" {
				fmt.Fprintln(os.Stderr, "warning: WEBHOOK_SECRET not set; signature verification disabled")
			}

			h := webhook.NewHandler(secret, func(ctx context.Context, pr webhook.PullRequest) error {
				return reviewAndComment(ctx, pr, token, gpPath)
			})
			mux := http.NewServeMux()
			mux.Handle("/webhook", h)
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

			srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
			fmt.Printf("infragenie webhook server listening on %s\n", addr)
			return srv.ListenAndServe()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	return cmd
}

// reviewAndComment reviews a PR and upserts the summary comment. It runs the
// deterministic layers only (no grounding), keeping the webhook free of LLM keys.
func reviewAndComment(ctx context.Context, pr webhook.PullRequest, token, gpPath string) error {
	d, err := diff.GitHubPR(ctx, diff.PRRef{Owner: pr.Owner, Repo: pr.Repo, Number: pr.Number}, token)
	if err != nil {
		return fmt.Errorf("fetch pr: %w", err)
	}
	var gp *models.GoldenPath
	if gpPath != "" {
		if loaded, lerr := goldenpath.New(".").Load(gpPath); lerr == nil {
			gp = loaded
		}
	}
	eng := engine.New(engine.Config{
		GoldenPath: gp,
		Scanners:   scanners.Select(allScanners(), gp, nil, nil),
		Reviewers: []reviewers.Reviewer{
			reviewers.GoldenPathReviewer{},
			reviewers.ReliabilityReviewer{},
			reviewers.ConventionsReviewer{},
		},
	})
	res, err := eng.Run(ctx, engine.Input{Diff: d})
	if err != nil {
		return err
	}
	client := github.NewClient(nil)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	return prcomment.Upsert(ctx, client, pr.Owner, pr.Repo, pr.Number, prcomment.Render(res.Findings))
}
