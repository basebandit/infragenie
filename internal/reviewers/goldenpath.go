// Package reviewers — GoldenPathReviewer checks diff files against the
// structured rules declared in a resolved GoldenPath.
package reviewers

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
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
		// Manifest rules run per YAML document so multi-doc files, and each
		// document's own kind and labels, are evaluated correctly.
		if isK8sManifest(f.Path) {
			docs := parseManifestDocs(f.NewContent)
			fileHasNetworkPolicy := anyKind(docs, "NetworkPolicy")
			for _, doc := range docs {
				findings = append(findings, checkRequiredLabels(f, doc, gp.RequiredLabels)...)
				findings = append(findings, checkSecurity(f, doc, gp.Security, fileHasNetworkPolicy)...)
				findings = append(findings, checkObservability(f, doc, gp.Observability)...)
			}
		}
		findings = append(findings, checkCISteps(f, gp.CIRequired)...)
		findings = append(findings, checkChartShape(f, gp.ChartShape)...)
	}
	return findings, nil
}

// checkRequiredLabels verifies that a manifest document's metadata.labels
// contains all labels declared in the GoldenPath required_labels list.
// Undecodable documents (un-rendered templates) are skipped — we don't guess.
func checkRequiredLabels(f models.FileDiff, doc manifestDoc, required []string) []models.Finding {
	if len(required) == 0 || !doc.Decoded {
		return nil
	}
	var findings []models.Finding
	for _, lbl := range required {
		if _, ok := doc.Labels[lbl]; !ok {
			findings = append(findings, models.Finding{
				RuleID:      "goldenpath.required-label",
				Severity:    models.SeverityMedium,
				File:        f.Path,
				Title:       fmt.Sprintf("missing required label %q", lbl),
				Explanation: fmt.Sprintf("GoldenPath requires label %q on all K8s manifests.", lbl),
				Suggestion:  fmt.Sprintf("Add `%s: <value>` under metadata.labels.", lbl),
				Confidence:  0.97,
				Evidence:    labelsEvidence(doc.Labels),
				EvidenceLoc: fmt.Sprintf("%s:%s/metadata.labels", f.Path, doc.Kind),
			})
		}
	}
	return findings
}

// checkSecurity enforces the GoldenPath Security block against one manifest
// document. Workload rules apply only to pod-bearing kinds; NetworkPolicy is
// satisfied when a NetworkPolicy document exists anywhere in the same file.
func checkSecurity(f models.FileDiff, doc manifestDoc, sec models.Security, fileHasNetworkPolicy bool) []models.Finding {
	content := doc.Raw
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

	// Remaining rules need a known workload kind.
	if !doc.Decoded || !isWorkloadKind(doc.Kind) {
		return findings
	}

	if sec.RequireNonRoot && !containsAny(content, "runAsNonRoot: true", "runAsUser:") {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-non-root",
			Severity:    models.SeverityHigh,
			File:        f.Path,
			Title:       "container may run as root",
			Explanation: "GoldenPath requires containers to run as non-root. No securityContext.runAsNonRoot or runAsUser found.",
			Suggestion:  "Add `securityContext: { runAsNonRoot: true }` to the container spec.",
			Confidence:  0.90,
			Evidence:    "no runAsNonRoot or runAsUser in securityContext",
			EvidenceLoc: fmt.Sprintf("%s:%s", f.Path, doc.Kind),
		})
	}

	if sec.RequireReadOnlyRootFS && !strings.Contains(content, "readOnlyRootFilesystem: true") {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-readonly-rootfs",
			Severity:    models.SeverityMedium,
			File:        f.Path,
			Title:       "container rootfs is not read-only",
			Explanation: "GoldenPath requires readOnlyRootFilesystem: true to limit blast radius of container compromise.",
			Suggestion:  "Add `readOnlyRootFilesystem: true` to the container securityContext.",
			Confidence:  0.90,
			Evidence:    "readOnlyRootFilesystem not set",
			EvidenceLoc: fmt.Sprintf("%s:%s", f.Path, doc.Kind),
		})
	}

	if sec.RequireNetworkPolicy && !fileHasNetworkPolicy {
		findings = append(findings, models.Finding{
			RuleID:      "goldenpath.security.require-network-policy",
			Severity:    models.SeverityHigh,
			File:        f.Path,
			Title:       fmt.Sprintf("no NetworkPolicy found for %s", doc.Kind),
			Explanation: "GoldenPath requires a NetworkPolicy to restrict pod-to-pod traffic.",
			Suggestion:  "Add a NetworkPolicy manifest selecting this workload's pods.",
			Confidence:  0.80,
			Evidence:    "no NetworkPolicy document in this file",
			EvidenceLoc: fmt.Sprintf("%s:%s", f.Path, doc.Kind),
		})
	}

	return findings
}

// checkObservability enforces observability requirements on a manifest
// document. Scrape annotations only make sense for long-running workloads.
func checkObservability(f models.FileDiff, doc manifestDoc, obs models.Observability) []models.Finding {
	if !doc.Decoded || !isLongRunningKind(doc.Kind) {
		return nil
	}
	content := doc.Raw
	var findings []models.Finding

	if obs.RequirePrometheusAnnotations {
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

func isCIFile(path string) bool {
	return strings.Contains(path, ".github/workflows") ||
		strings.HasSuffix(path, ".gitlab-ci.yml") ||
		strings.Contains(path, ".circleci") ||
		strings.Contains(path, "Jenkinsfile") ||
		strings.Contains(path, "azure-pipelines")
}

// isDeployment reports a long-running, pod-bearing workload (probes, replicas,
// and metrics scraping all make sense here).
func isDeployment(content string) bool {
	return strings.Contains(content, "kind: Deployment") ||
		strings.Contains(content, "kind: StatefulSet") ||
		strings.Contains(content, "kind: DaemonSet")
}

// isBatchWorkload reports a run-to-completion workload. These carry pod specs
// too, so security and resource rules apply, but probes/replicas/scrape do not.
func isBatchWorkload(content string) bool {
	return strings.Contains(content, "kind: CronJob") ||
		strings.Contains(content, "kind: Job")
}

// isWorkload reports any pod-bearing workload (long-running or batch).
func isWorkload(content string) bool {
	return isDeployment(content) || isBatchWorkload(content)
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
