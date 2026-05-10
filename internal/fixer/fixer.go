// Package fixer generates LLM-powered file fixes for Golden Path violations
// and surfaces them for human approval before applying.
package fixer

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

// LLM is the narrow interface required for fix generation.
type LLM interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

// Fixer generates corrected file content for Golden Path violations.
type Fixer struct {
	llm LLM
}

// New returns a Fixer backed by the given LLM.
func New(llm LLM) *Fixer {
	return &Fixer{llm: llm}
}

// Fixable reports whether a finding rule can be auto-fixed.
func Fixable(f models.Finding) bool {
	return f.RuleID == "goldenpath.ci.required-step" ||
		f.RuleID == "goldenpath.required-label"
}

// Suggest returns the corrected full file content fixing all supplied findings.
// All findings must belong to the same file. Caller shows a diff and handles approval.
func (fx *Fixer) Suggest(ctx context.Context, findings []models.Finding, path, content string) (string, error) {
	raw, err := fx.llm.Generate(ctx, fixSystemPrompt, buildPrompt(findings, path, content))
	if err != nil {
		return "", err
	}
	return stripFences(raw), nil
}

// fixSystemPrompt is cached by Anthropic after the first call (≥1024 tokens threshold).
// OpenAI caches it automatically as a prefix.
const fixSystemPrompt = `You are a platform engineer fixing Golden Path violations in infrastructure files.
Return the complete corrected file content — no diff, no explanation, no markdown fences.
Rules:
- Fix only the reported violations; change nothing else
- Preserve all existing content, formatting, comments, and indentation exactly
- For missing CI steps: append the step(s) under the correct job's steps list using the file's existing indentation style
- For missing Kubernetes labels: add them under metadata.labels
- Output the raw file content only, starting from line 1 of the file`

func buildPrompt(findings []models.Finding, path, content string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "File: %s\n\n```\n%s\n```\n\nFindings to fix:\n", path, content)
	for i, f := range findings {
		fmt.Fprintf(&sb, "%d. rule: %s\n   title: %s\n   suggestion: %s\n", i+1, f.RuleID, f.Title, f.Suggestion)
	}
	return sb.String()
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if nl := strings.Index(s, "\n"); nl >= 0 {
		s = s[nl+1:]
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}
