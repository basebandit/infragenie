package reviewers

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

// ConventionsReviewer checks naming and annotation conventions that are
// typically enforced by platform teams but hard to catch with static scanners.
// Rules are driven by the GoldenPath runtime_rules map when present.
type ConventionsReviewer struct{}

func (ConventionsReviewer) Name() string { return "conventions" }

func (ConventionsReviewer) Review(_ context.Context, in ReviewInput) ([]models.Finding, error) {
	var findings []models.Finding
	for _, f := range in.Diff.Files {
		if f.Status == "deleted" || f.NewContent == "" {
			continue
		}
		if !isK8sManifest(f.Path) {
			continue
		}
		findings = append(findings, checkAppKubernetesLabels(f)...)
		// runtime_rules only apply to files that actually contain a Kubernetes
		// object, so service descriptors and generic YAML aren't flagged. Gate on
		// rule presence first so the no-rules path never parses YAML.
		if in.GoldenPath != nil && len(in.GoldenPath.RuntimeRules) > 0 && hasKubernetesDoc(f.NewContent) {
			findings = append(findings, checkRuntimeRules(f, in.GoldenPath)...)
		}
	}
	return findings, nil
}

// hasKubernetesDoc reports whether content holds at least one decodable
// Kubernetes object (a document with a kind). A cheap substring check skips the
// YAML parse for content that obviously has no kind.
func hasKubernetesDoc(content string) bool {
	if !strings.Contains(content, "kind:") {
		return false
	}
	for _, d := range parseManifestDocs(content) {
		if d.Decoded {
			return true
		}
	}
	return false
}

// checkAppKubernetesLabels flags manifests that use app: <name> but are missing
// the recommended app.kubernetes.io/ well-known labels.
func checkAppKubernetesLabels(f models.FileDiff) []models.Finding {
	content := f.NewContent
	if !isDeployment(content) {
		return nil
	}
	hasWellKnown := strings.Contains(content, "app.kubernetes.io/")
	hasLegacyApp := strings.Contains(content, "\n    app:")
	if hasWellKnown || !hasLegacyApp {
		return nil
	}
	line := lineOf(content, "\n    app:")
	return []models.Finding{{
		RuleID:      "conventions.app-kubernetes-io-labels",
		Severity:    models.SeverityLow,
		File:        f.Path,
		Line:        line,
		Title:       "prefer app.kubernetes.io/ well-known labels",
		Explanation: "The legacy `app` label is not standardised. app.kubernetes.io/name, /version, /component are part of the K8s label convention and enable tooling (dashboards, selectors) to work out of the box.",
		Suggestion:  "Replace or supplement `app: <name>` with `app.kubernetes.io/name: <name>` and `app.kubernetes.io/version: <ver>`.",
		Confidence:  0.85,
		Evidence:    extractLine(content, line),
		EvidenceLoc: fmt.Sprintf("%s:%d", f.Path, line),
	}}
}

// checkRuntimeRules evaluates the GoldenPath runtime_rules map.
// Each key is a rule name, value is a map with at least "pattern" (substring
// to require) and optionally "message" (human description).
func checkRuntimeRules(f models.FileDiff, gp *models.GoldenPath) []models.Finding {
	if gp == nil || len(gp.RuntimeRules) == 0 {
		return nil
	}
	content := f.NewContent
	var findings []models.Finding
	for ruleID, params := range gp.RuntimeRules {
		pattern, ok := stringVal(params, "pattern")
		if !ok || strings.Contains(content, pattern) {
			continue
		}
		msg, _ := stringVal(params, "message")
		if msg == "" {
			msg = fmt.Sprintf("runtime rule %q: pattern %q not found", ruleID, pattern)
		}
		suggestion, _ := stringVal(params, "suggestion")
		findings = append(findings, models.Finding{
			RuleID:      "conventions.runtime-rule." + ruleID,
			Severity:    models.SeverityLow,
			File:        f.Path,
			Title:       msg,
			Explanation: fmt.Sprintf("GoldenPath runtime rule %q requires pattern %q in %s.", ruleID, pattern, f.Path),
			Suggestion:  suggestion,
			Confidence:  0.88,
			Evidence:    fmt.Sprintf("pattern %q not found", pattern),
			EvidenceLoc: f.Path,
		})
	}
	return findings
}

func stringVal(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
