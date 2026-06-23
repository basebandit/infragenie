// Package reviewers — GoldenPathReviewer checks diff files against the
// structured rules declared in a resolved GoldenPath.
package reviewers

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
	"gopkg.in/yaml.v3"
)

// GoldenPathReviewer is a deterministic Layer-3 reviewer that validates
// K8s/Helm/CI manifests against the resolved GoldenPath policy.
type GoldenPathReviewer struct{}

func (GoldenPathReviewer) Name() string { return "goldenpath" }

func (GoldenPathReviewer) Review(_ context.Context, in ReviewInput) ([]models.Finding, error) {
	if in.GoldenPath == nil {
		return nil, nil
	}
	gp := in.GoldenPath
	var findings []models.Finding
	for _, f := range in.Diff.Files {
		if f.Status == "deleted" || f.NewContent == "" {
			continue
		}
		findings = append(findings, checkRequiredLabels(f, gp.RequiredLabels)...)
		findings = append(findings, checkSecurity(f, gp.Security)...)
		findings = append(findings, checkObservability(f, gp.Observability)...)
		findings = append(findings, checkCISteps(f, gp.CIRequired)...)
		findings = append(findings, checkChartShape(f, gp.ChartShape)...)
	}
	return findings, nil
}

// checkRequiredLabels verifies that K8s manifest metadata.labels contains all
// labels declared in the GoldenPath required_labels list.
func checkRequiredLabels(f models.FileDiff, required []string) []models.Finding {
	if len(required) == 0 || !isK8sManifest(f.Path) || !hasK8sAPIVersion(f.NewContent) {
		return nil
	}
	var m struct {
		Metadata struct {
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(f.NewContent), &m); err != nil {
		return nil
	}
	var findings []models.Finding
	for _, lbl := range required {
		if _, ok := m.Metadata.Labels[lbl]; !ok {
			findings = append(findings, models.Finding{
				RuleID:      "goldenpath.required-label",
				Severity:    models.SeverityMedium,
				File:        f.Path,
				Title:       fmt.Sprintf("missing required label %q", lbl),
				Explanation: fmt.Sprintf("GoldenPath requires label %q on all K8s manifests.", lbl),
				Suggestion:  fmt.Sprintf("Add `%s: <value>` under metadata.labels.", lbl),
				Confidence:  0.97,
				Evidence:    labelsEvidence(m.Metadata.Labels),
				EvidenceLoc: f.Path + ":metadata.labels",
			})
		}
	}
	return findings
}

// checkSecurity enforces security rules from the GoldenPath Security block.
func checkSecurity(f models.FileDiff, sec models.Security) []models.Finding {
	if !isK8sManifest(f.Path) || !hasK8sAPIVersion(f.NewContent) {
		return nil
	}
	content := f.NewContent
	var findings []models.Finding

	if sec.ForbidImageTagLatest && strings.Contains(content, ":latest") {
		line := lineOf(content, ":latest")
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.no-latest-tag",
			Severity:    models.SeverityHigh,
			File:        f.Path,
			Line:        line,
			Title:       "image uses :latest tag",
			Explanation: "GoldenPath forbids :latest — image is not pinned, breaking reproducibility.",
			Suggestion:  "Pin to a specific digest or semver tag (e.g. `image:1.2.3`).",
			Confidence:  0.99,
			Evidence:    extractLine(content, line),
			EvidenceLoc: fmt.Sprintf("%s:%d", f.Path, line),
		})
	}

	if sec.RequireNonRoot && isDeployment(content) && !containsAny(content, "runAsNonRoot: true", "runAsUser:") {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-non-root",
			Severity:    models.SeverityHigh,
			File:        f.Path,
			Title:       "container may run as root",
			Explanation: "GoldenPath requires containers to run as non-root. No securityContext.runAsNonRoot or runAsUser found.",
			Suggestion:  "Add `securityContext: { runAsNonRoot: true }` to the container spec.",
			Confidence:  0.90,
			Evidence:    "no runAsNonRoot or runAsUser in securityContext",
			EvidenceLoc: f.Path,
		})
	}

	if sec.RequireReadOnlyRootFS && isDeployment(content) && !strings.Contains(content, "readOnlyRootFilesystem: true") {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-readonly-rootfs",
			Severity:    models.SeverityMedium,
			File:        f.Path,
			Title:       "container rootfs is not read-only",
			Explanation: "GoldenPath requires readOnlyRootFilesystem: true to limit blast radius of container compromise.",
			Suggestion:  "Add `readOnlyRootFilesystem: true` to the container securityContext.",
			Confidence:  0.90,
			Evidence:    "readOnlyRootFilesystem not set",
			EvidenceLoc: f.Path,
		})
	}

	if sec.RequireNetworkPolicy && isDeployment(content) && !strings.Contains(content, "kind: NetworkPolicy") {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-network-policy",
			Severity:    models.SeverityHigh,
			File:        f.Path,
			Title:       "no NetworkPolicy found for deployment",
			Explanation: "GoldenPath requires a NetworkPolicy to restrict pod-to-pod traffic.",
			Suggestion:  "Add a NetworkPolicy manifest selecting this deployment's pods.",
			Confidence:  0.80,
			Evidence:    "kind: NetworkPolicy not found in diff file",
			EvidenceLoc: f.Path,
		})
	}

	return findings
}

// checkObservability enforces observability requirements.
func checkObservability(f models.FileDiff, obs models.Observability) []models.Finding {
	if !isK8sManifest(f.Path) {
		return nil
	}
	content := f.NewContent
	var findings []models.Finding

	if obs.RequirePrometheusAnnotations && isDeployment(content) {
		hasPrometheus := strings.Contains(content, "prometheus.io/scrape") ||
			strings.Contains(content, "prometheus.io/port")
		if !hasPrometheus {
			findings = append(findings, models.Finding{
				RuleID:      "goldenpath.observability.prometheus-annotations",
				Severity:    models.SeverityMedium,
				File:        f.Path,
				Title:       "missing Prometheus scrape annotations",
				Explanation: "GoldenPath requires prometheus.io/scrape and prometheus.io/port annotations for metrics collection.",
				Suggestion:  "Add `prometheus.io/scrape: \"true\"` and `prometheus.io/port: \"<port>\"` to pod annotations.",
				Confidence:  0.90,
				Evidence:    "prometheus.io/scrape not found in annotations",
				EvidenceLoc: f.Path + ":metadata.annotations",
			})
		}
	}

	return findings
}

// checkCISteps verifies that CI config files contain required workflow steps.
func checkCISteps(f models.FileDiff, steps []models.CIStep) []models.Finding {
	if len(steps) == 0 || !isCIFile(f.Path) {
		return nil
	}
	content := f.NewContent
	var findings []models.Finding
	for _, step := range steps {
		found := false
		for _, match := range step.Matches {
			if strings.Contains(content, match) {
				found = true
				break
			}
		}
		if !found {
			findings = append(findings, models.Finding{
				RuleID:      "goldenpath.ci.required-step",
				Severity:    models.SeverityMedium,
				File:        f.Path,
				Title:       fmt.Sprintf("CI step %q not found", step.Name),
				Explanation: fmt.Sprintf("GoldenPath requires CI step %q (one of: %s).", step.Name, strings.Join(step.Matches, ", ")),
				Suggestion:  fmt.Sprintf("Add a step matching one of: %s", strings.Join(step.Matches, ", ")),
				Confidence:  0.93,
				Evidence:    fmt.Sprintf("none of %v found in %s", step.Matches, f.Path),
				EvidenceLoc: f.Path,
			})
		}
	}
	return findings
}

// checkChartShape verifies Helm chart directory structure.
func checkChartShape(f models.FileDiff, shape models.ChartShape) []models.Finding {
	if shape.ChartsDir == "" || len(shape.RequiredFiles) == 0 {
		return nil
	}
	if !strings.HasPrefix(f.Path, shape.ChartsDir) {
		return nil
	}
	var findings []models.Finding
	for _, required := range shape.RequiredFiles {
		full := shape.ChartsDir + "/" + required
		if f.Path == full && f.Status == "deleted" {
			findings = append(findings, models.Finding{
				RuleID:      "goldenpath.chart.required-file",
				Severity:    models.SeverityMedium,
				File:        f.Path,
				Title:       fmt.Sprintf("required chart file %q deleted", required),
				Explanation: fmt.Sprintf("GoldenPath requires %s to exist in %s.", required, shape.ChartsDir),
				Suggestion:  fmt.Sprintf("Restore %s or update the chart_shape policy.", full),
				Confidence:  0.98,
				Evidence:    "file status: deleted",
				EvidenceLoc: f.Path,
			})
		}
	}
	return findings
}

func isK8sManifest(path string) bool {
	return (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) &&
		!strings.Contains(path, ".github/") &&
		!strings.Contains(path, ".gitlab-ci") &&
		!strings.Contains(path, "azure-pipelines") &&
		!isHelmChartMeta(path)
}

// isHelmChartMeta reports whether path is Helm chart metadata rather than a
// Kubernetes API object. Chart.yaml carries `apiVersion:` (Helm's, not K8s') and
// values files have no manifest shape, so neither should be checked for required
// labels, security context, or other K8s manifest rules.
func isHelmChartMeta(path string) bool {
	base := path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		base = path[i+1:]
	}
	switch {
	case base == "Chart.yaml", base == "Chart.yml":
		return true
	case base == "values.yaml", base == "values.yml":
		return true
	case strings.HasPrefix(base, "values-") && (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")):
		return true
	}
	return false
}

func hasK8sAPIVersion(content string) bool {
	return strings.Contains(content, "apiVersion:")
}

func isCIFile(path string) bool {
	return strings.Contains(path, ".github/workflows") ||
		strings.HasSuffix(path, ".gitlab-ci.yml") ||
		strings.Contains(path, ".circleci") ||
		strings.Contains(path, "Jenkinsfile") ||
		strings.Contains(path, "azure-pipelines")
}

func isDeployment(content string) bool {
	return strings.Contains(content, "kind: Deployment") ||
		strings.Contains(content, "kind: StatefulSet") ||
		strings.Contains(content, "kind: DaemonSet")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func lineOf(content, substr string) int {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if strings.Contains(l, substr) {
			return i + 1
		}
	}
	return 0
}

func extractLine(content string, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}

func labelsEvidence(labels map[string]string) string {
	if len(labels) == 0 {
		return "labels: {}"
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	return "labels: [" + strings.Join(parts, ", ") + "]"
}
