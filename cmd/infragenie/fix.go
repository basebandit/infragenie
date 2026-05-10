package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	difflib "github.com/pmezard/go-difflib/difflib"

	"github.com/basebandit/infragenie/internal/fixer"
	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/pkg/models"
)

// runFix is called after review findings are surfaced when --fix or --fix-auto is set.
// autoApprove skips the TTY check and applies all suggestions without prompting.
func runFix(ctx context.Context, findings []models.Finding, providerName, apiKey, model string, autoApprove bool) error {
	if !autoApprove && !isTerminal() {
		return fmt.Errorf("--fix requires an interactive terminal; use --fix-auto for CI pipelines")
	}
	if providerName == "" {
		return fmt.Errorf("--fix/--fix-auto requires --provider (e.g. --provider openai)")
	}

	client, err := llm.NewClient([]llm.Config{
		{Provider: llm.Provider(providerName), APIKey: apiKey, Model: model},
	})
	if err != nil {
		return fmt.Errorf("fix: %w", err)
	}
	fx := fixer.New(client)

	// Group fixable findings by file — one LLM call per file.
	byFile := map[string][]models.Finding{}
	for _, f := range findings {
		if fixer.Fixable(f) {
			byFile[f.File] = append(byFile[f.File], f)
		}
	}
	if len(byFile) == 0 {
		fmt.Fprintln(os.Stderr, "\nNo auto-fixable findings.")
		return nil
	}

	sc := bufio.NewScanner(os.Stdin)
	skipAll := false

	for path, ff := range byFile {
		if skipAll {
			break
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nskip %s: cannot read file: %v\n", path, err)
			continue
		}
		original := string(raw)

		fmt.Printf("\n── Generating fix for %s (%d finding(s))... ", path, len(ff))
		suggested, err := fx.Suggest(ctx, ff, path, original)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nskip %s: %v\n", path, err)
			continue
		}
		fmt.Println("done")

		udiff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(original),
			B:        difflib.SplitLines(suggested),
			FromFile: "a/" + path,
			ToFile:   "b/" + path,
			Context:  3,
		})
		if udiff == "" {
			fmt.Printf("  (no changes produced for %s)\n", path)
			continue
		}
		fmt.Println(udiff)

		if autoApprove {
			if err := os.WriteFile(path, []byte(suggested), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			fmt.Printf("  auto-applied → %s\n", path)
			continue
		}

		fmt.Print("Apply? [y]es / [n]o / [s]kip all  > ")
		if !sc.Scan() {
			break
		}
		switch strings.ToLower(strings.TrimSpace(sc.Text())) {
		case "y", "yes":
			if err := os.WriteFile(path, []byte(suggested), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			fmt.Printf("  applied → %s\n", path)
		case "s", "skip all":
			skipAll = true
		default:
			fmt.Printf("  skipped %s\n", path)
		}
	}
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
