package main

import (
	"context"
	"fmt"
	"os"

	"github.com/basebandit/infragenie/internal/diff"
	"github.com/basebandit/infragenie/internal/engine"
	"github.com/basebandit/infragenie/internal/goldenpath"
	"github.com/basebandit/infragenie/internal/grounding"
	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/internal/reporter"
	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/internal/reviewers"
	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/basebandit/infragenie/internal/scanners/infra"
	"github.com/basebandit/infragenie/internal/scanners/lang"
	"github.com/basebandit/infragenie/internal/telemetry"
	"github.com/basebandit/infragenie/pkg/config"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/spf13/cobra"
)

func reviewCmd(appCfg **config.AppConfig) *cobra.Command {
	var (
		gpPath       string
		diffRef      string
		prTarget     string
		githubToken  string
		providerName string
		model        string
		format       string
		failOn       string
		budgetTokens int
		budgetUSD    float64
		noGround     bool
		fix          bool
		fixAuto      bool
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review a diff or GitHub PR against your Golden Path",
		Example: `  infragenie review --goldenpath goldenpath.yml --diff HEAD~1
  infragenie review --goldenpath goldenpath.yml --pr owner/repo#42
  infragenie review --goldenpath goldenpath.yml --pr owner/repo#42 --format github`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			// ── load golden path ──────────────────────────────────────────────
			var gp *models.GoldenPath
			if gpPath != "" {
				loaded, err := goldenpath.New(".").Load(gpPath)
				if err != nil {
					return fmt.Errorf("goldenpath: %w", err)
				}
				gp = loaded
			}

			// ── load diff ─────────────────────────────────────────────────────
			var d *models.Diff
			switch {
			case prTarget != "":
				tok := githubToken
				if tok == "" {
					tok = os.Getenv("GITHUB_TOKEN")
				}
				pr, err := diff.ParsePRURL(prTarget)
				if err != nil {
					return fmt.Errorf("pr: %w", err)
				}
				d, err = diff.GitHubPR(ctx, pr, tok)
				if err != nil {
					return fmt.Errorf("pr fetch: %w", err)
				}
			case diffRef != "":
				var err error
				d, err = diff.Local(ctx, diff.LocalOptions{Base: diffRef})
				if err != nil {
					return fmt.Errorf("local diff: %w", err)
				}
			default:
				return fmt.Errorf("one of --diff or --pr is required")
			}

			// ── repo context ──────────────────────────────────────────────────
			rc, _ := repo.Build(".")

			// ── scanners ──────────────────────────────────────────────────────
			allScanners := []scanners.Scanner{
				infra.NewCheckov(),
				infra.NewHadolint(),
				lang.NewGosec(),
			}
			selected := scanners.Select(allScanners, gp, rc)

			// ── grounder ──────────────────────────────────────────────────────
			var grounder grounding.Grounder
			if !noGround && providerName != "" {
				cfg := *appCfg
				apiKey := cfg.APIKey(providerName)
				resolvedModel := model
				if resolvedModel == "" {
					resolvedModel = cfg.Model(providerName)
				}
				client, err := llm.NewClient([]llm.Config{
					{Provider: llm.Provider(providerName), APIKey: apiKey, Model: resolvedModel},
				})
				if err == nil {
					grounder = grounding.NewLLMGrounder(client, resolvedModel)
				}
			}

			// ── budget ────────────────────────────────────────────────────────
			var budget *telemetry.Counter
			if budgetTokens > 0 || budgetUSD > 0 {
				budget = telemetry.NewCounter(telemetry.Budget{
					Tokens: budgetTokens,
					USD:    budgetUSD,
				})
			}

			// ── reviewers ─────────────────────────────────────────────────────
			rev := []reviewers.Reviewer{
				reviewers.GoldenPathReviewer{},
				reviewers.ReliabilityReviewer{},
				reviewers.ConventionsReviewer{},
			}

			// ── engine ────────────────────────────────────────────────────────
			eng := engine.New(engine.Config{
				GoldenPath: gp,
				Scanners:   selected,
				Grounder:   grounder,
				Reviewers:  rev,
				Budget:     budget,
			})

			result, err := eng.Run(ctx, engine.Input{Diff: d, RepoCtx: rc})
			if err != nil {
				return err
			}

			// ── output ────────────────────────────────────────────────────────
			if err := reporter.Write(os.Stdout, result.Findings, result.Skipped,
				reporter.Format(format)); err != nil {
				return err
			}

			// ── human-in-the-loop fix suggestions ─────────────────────────────
			if (fix || fixAuto) && len(result.Findings) > 0 {
				if err := runFix(ctx, result.Findings, providerName, model, fixAuto); err != nil {
					fmt.Fprintf(os.Stderr, "fix: %v\n", err)
				}
			}

			os.Exit(reporter.ExitCode(result.Findings, failOnSeverity(failOn, gp)))
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpPath, "goldenpath", "g", "", "path to goldenpath.yml")
	cmd.Flags().StringVarP(&diffRef, "diff", "d", "", "local git ref to diff against (e.g. HEAD~1)")
	cmd.Flags().StringVar(&prTarget, "pr", "", "GitHub PR: owner/repo#N or full URL")
	cmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub token (default: $GITHUB_TOKEN)")
	cmd.Flags().StringVar(&providerName, "provider", "", "LLM provider for grounding: anthropic, openai, local")
	cmd.Flags().StringVar(&model, "model", "", "LLM model (default chosen per provider)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json, github")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "exit 1 when any finding is at or above this severity (critical, high, medium, low)")
	cmd.Flags().IntVar(&budgetTokens, "budget-tokens", 0, "max tokens to spend on grounding (0 = unlimited)")
	cmd.Flags().Float64Var(&budgetUSD, "budget-usd", 0, "max USD to spend on grounding (0 = unlimited)")
	cmd.Flags().BoolVar(&noGround, "no-ground", false, "skip LLM grounding pass")
	cmd.Flags().BoolVar(&fix, "fix", false, "interactively apply LLM-suggested fixes for auto-fixable findings (requires --provider)")
	cmd.Flags().BoolVar(&fixAuto, "fix-auto", false, "apply all LLM-suggested fixes without prompting — for CI pipelines (requires --provider)")

	return cmd
}


func failOnSeverity(flag string, gp *models.GoldenPath) models.Severity {
	if flag != "" {
		return models.Severity(flag)
	}
	if gp != nil && gp.FailOn != "" {
		return gp.FailOn
	}
	if gp != nil && gp.Defaults.FailOn != "" {
		return gp.Defaults.FailOn
	}
	return models.SeverityHigh
}
