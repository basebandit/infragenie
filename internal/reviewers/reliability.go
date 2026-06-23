package reviewers

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

// ReliabilityReviewer flags common K8s reliability anti-patterns: missing
// resource limits/requests, missing probes, and single-replica deployments.
type ReliabilityReviewer struct{}

func (ReliabilityReviewer) Name() string { return "reliability" }

func (ReliabilityReviewer) Review(_ context.Context, in ReviewInput) ([]models.Finding, error) {
	var findings []models.Finding
	for _, f := range in.Diff.Files {
		if f.Status == "deleted" || f.NewContent == "" {
			continue
		}
		if !isK8sManifest(f.Path) || !isWorkload(f.NewContent) {
			continue
		}
		// Resource limits/requests matter for any workload, batch included.
		findings = append(findings, checkResourceLimits(f)...)
		// Probes and replicas only apply to long-running workloads — batch jobs
		// run to completion and have neither.
		if isDeployment(f.NewContent) {
			findings = append(findings, checkProbes(f)...)
			findings = append(findings, checkReplicas(f)...)
		}
	}
	return findings, nil
}

func checkResourceLimits(f models.FileDiff) []models.Finding {
	content := f.NewContent
	hasLimits := strings.Contains(content, "limits:")
	hasRequests := strings.Contains(content, "requests:")
	if hasLimits && hasRequests {
		return nil
	}
	msg := "missing resource limits and requests"
	if hasRequests {
		msg = "missing resource limits"
	} else if hasLimits {
		msg = "missing resource requests"
	}
	return []models.Finding{{
		RuleID:      "reliability.resource-limits",
		Severity:    models.SeverityHigh,
		File:        f.Path,
		Title:       msg,
		Explanation: "Without resource limits the container can starve neighbours; without requests the scheduler cannot place it correctly.",
		Suggestion:  "Add resources.limits (cpu, memory) and resources.requests (cpu, memory) to each container spec.",
		Confidence:  0.92,
		Evidence:    fmt.Sprintf("limits present: %v, requests present: %v", hasLimits, hasRequests),
		EvidenceLoc: f.Path,
	}}
}

func checkProbes(f models.FileDiff) []models.Finding {
	content := f.NewContent
	hasLiveness := strings.Contains(content, "livenessProbe:")
	hasReadiness := strings.Contains(content, "readinessProbe:")
	if hasLiveness && hasReadiness {
		return nil
	}
	var findings []models.Finding
	if !hasLiveness {
		findings = append(findings, models.Finding{
			RuleID:      "reliability.liveness-probe",
			Severity:    models.SeverityMedium,
			File:        f.Path,
			Title:       "missing livenessProbe",
			Explanation: "Without a liveness probe kubelet cannot restart a stuck container — degraded pods stay in rotation.",
			Suggestion:  "Add a livenessProbe (httpGet, exec, or tcpSocket) tuned to the app's startup time.",
			Confidence:  0.90,
			Evidence:    "livenessProbe not found in container spec",
			EvidenceLoc: f.Path,
		})
	}
	if !hasReadiness {
		findings = append(findings, models.Finding{
			RuleID:      "reliability.readiness-probe",
			Severity:    models.SeverityMedium,
			File:        f.Path,
			Title:       "missing readinessProbe",
			Explanation: "Without a readiness probe traffic is sent to pods before they are ready, causing request errors during rollouts.",
			Suggestion:  "Add a readinessProbe that returns success only when the app is ready to serve traffic.",
			Confidence:  0.90,
			Evidence:    "readinessProbe not found in container spec",
			EvidenceLoc: f.Path,
		})
	}
	return findings
}

func checkReplicas(f models.FileDiff) []models.Finding {
	content := f.NewContent
	// Only flag if explicitly set to 1; omitted replicas default to 1 in K8s
	// but we can't be certain without the full manifest — skip on absence.
	if !strings.Contains(content, "replicas: 1") {
		return nil
	}
	line := lineOf(content, "replicas: 1")
	return []models.Finding{{
		RuleID:      "reliability.single-replica",
		Severity:    models.SeverityMedium,
		File:        f.Path,
		Line:        line,
		Title:       "single replica — no high availability",
		Explanation: "replicas: 1 means a single pod failure takes the service offline with no redundancy.",
		Suggestion:  "Use replicas: 2+ for any production workload, or add an HPA with minReplicas >= 2.",
		Confidence:  0.95,
		Evidence:    extractLine(content, line),
		EvidenceLoc: fmt.Sprintf("%s:%d", f.Path, line),
	}}
}
