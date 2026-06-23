package reviewers

import (
	"context"
	"strings"
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

const deployBase = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments
  labels:
    app: payments
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: payments
        image: payments:1.2.3
        resources:
          limits:
            cpu: "500m"
            memory: "256Mi"
          requests:
            cpu: "250m"
            memory: "128Mi"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
`

const cronJobSecure = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: etl
  labels:
    app.kubernetes.io/name: etl
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          securityContext:
            runAsNonRoot: true
          containers:
          - name: etl
            image: etl:1.0.0
            securityContext:
              readOnlyRootFilesystem: true
            resources:
              limits:
                cpu: "500m"
                memory: "256Mi"
              requests:
                cpu: "250m"
                memory: "128Mi"
`

const cronJobInsecure = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: etl
  labels:
    app.kubernetes.io/name: etl
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
          - name: etl
            image: etl:1.0.0
`

func makeInput(path, content string, gp *models.GoldenPath) ReviewInput {
	return ReviewInput{
		Diff: &models.Diff{Files: []models.FileDiff{
			{Path: path, Status: "added", NewContent: content},
		}},
		GoldenPath: gp,
	}
}

// ── GoldenPathReviewer ────────────────────────────────────────────────────────

func TestGoldenPath_RequiredLabelMissing(t *testing.T) {
	gp := &models.GoldenPath{RequiredLabels: []string{"team", "app"}}
	in := makeInput("charts/payments/deployment.yaml", deployBase, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	ruleIDs := ruleIDSet(fs)
	require.Contains(t, ruleIDs, "goldenpath.required-label")
	// "app" IS present in deployBase labels; only "team" should be missing
	found := findByRuleID(fs, "goldenpath.required-label")
	require.Len(t, found, 1)
	require.Contains(t, found[0].Title, "team")
}

func TestGoldenPath_NoFindingsOnCleanManifest(t *testing.T) {
	clean := deployBase + "    team: platform\n"
	gp := &models.GoldenPath{RequiredLabels: []string{"app"}}
	in := makeInput("deploy.yaml", clean, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "goldenpath.required-label"))
}

func TestGoldenPath_LatestTagFlagged(t *testing.T) {
	content := strings.ReplaceAll(deployBase, "payments:1.2.3", "payments:latest")
	gp := &models.GoldenPath{Security: models.Security{ForbidImageTagLatest: true}}
	in := makeInput("deploy.yaml", content, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.no-latest-tag"))
}

func TestGoldenPath_PrometheusMissing(t *testing.T) {
	gp := &models.GoldenPath{Observability: models.Observability{RequirePrometheusAnnotations: true}}
	in := makeInput("deploy.yaml", deployBase, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.observability.prometheus-annotations"))
}

func TestGoldenPath_CIStepMissing(t *testing.T) {
	ciContent := "steps:\n  - name: build\n    run: go build\n"
	gp := &models.GoldenPath{CIRequired: []models.CIStep{
		{Name: "trivy-scan", Matches: []string{"trivy", "aquasec/trivy"}},
	}}
	in := makeInput(".github/workflows/ci.yml", ciContent, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.ci.required-step"))
}

func TestGoldenPath_HelmChartMetaNotLabelChecked(t *testing.T) {
	// Chart.yaml carries Helm's apiVersion but is not a K8s object; it must not
	// be flagged for missing required labels. Same for values files.
	chart := "apiVersion: v2\nname: payments\nversion: 0.1.0\n"
	values := "apiVersion: ignored\nimage:\n  repository: payments\n  tag: 1.2.3\n"
	gp := &models.GoldenPath{RequiredLabels: []string{"team", "app"}}

	for _, path := range []string{"charts/payments/Chart.yaml", "charts/payments/values.yaml", "charts/payments/values-prod.yaml"} {
		content := chart
		if strings.Contains(path, "values") {
			content = values
		}
		in := makeInput(path, content, gp)
		fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
		require.NoError(t, err)
		require.Emptyf(t, findByRuleID(fs, "goldenpath.required-label"), "unexpected label finding for %s", path)
	}
}

func TestGoldenPath_NilGoldenPathIsNoop(t *testing.T) {
	in := makeInput("deploy.yaml", deployBase, nil)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, fs)
}

func TestGoldenPath_CronJobSecurityEnforced(t *testing.T) {
	gp := &models.GoldenPath{Security: models.Security{
		RequireNonRoot:        true,
		RequireReadOnlyRootFS: true,
	}}
	in := makeInput("cronjob.yaml", cronJobInsecure, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-non-root"))
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-readonly-rootfs"))
}

func TestGoldenPath_CronJobNetworkPolicyEnforced(t *testing.T) {
	// A batch workload with no co-located NetworkPolicy is flagged.
	gp := &models.GoldenPath{Security: models.Security{RequireNetworkPolicy: true}}
	in := makeInput("cronjob.yaml", cronJobSecure, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-network-policy"))

	// Co-locating a NetworkPolicy in the same file clears it.
	withNP := cronJobSecure + "---\nkind: NetworkPolicy\n"
	in = makeInput("cronjob.yaml", withNP, gp)
	fs, err = GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "goldenpath.security.require-network-policy"))
}

func TestGoldenPath_MultiDocSecondDocumentChecked(t *testing.T) {
	// A label missing on the SECOND document must be caught — the old whole-file
	// approach only ever decoded the first document.
	svc := "apiVersion: v1\nkind: Service\nmetadata:\n  name: x\n  labels:\n    team: platform\n"
	dep := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: x\n  labels:\n    app.kubernetes.io/name: x\n"
	gp := &models.GoldenPath{RequiredLabels: []string{"team"}}

	in := makeInput("manifests.yaml", svc+"---\n"+dep, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	// Service has team; Deployment does not → exactly one finding, for the Deployment.
	found := findByRuleID(fs, "goldenpath.required-label")
	require.Len(t, found, 1)
	require.Contains(t, found[0].Title, "team")
}

func TestGoldenPath_TemplatedDocumentSkipped(t *testing.T) {
	// An un-rendered Helm template can't be decoded; skip it rather than guess.
	tmpl := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{ .Values.name }}\n  labels: {{ .Values.labels }}\n"
	gp := &models.GoldenPath{RequiredLabels: []string{"team"}}
	in := makeInput("templates/deployment.yaml", tmpl, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "goldenpath.required-label"))
}

func TestGoldenPath_NetworkPolicyAcrossDocuments(t *testing.T) {
	// A co-located NetworkPolicy document satisfies the requirement.
	dep := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: x\nspec:\n  template:\n    spec:\n      securityContext:\n        runAsNonRoot: true\n"
	np := "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: x\n"
	gp := &models.GoldenPath{Security: models.Security{RequireNetworkPolicy: true}}

	in := makeInput("manifests.yaml", dep+"---\n"+np, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "goldenpath.security.require-network-policy"))

	in = makeInput("manifests.yaml", dep, gp)
	fs, err = GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-network-policy"))
}

func TestGoldenPath_StructuralSecurityContext(t *testing.T) {
	// Substring matching would PASS this: "runAsUser:" is present (but it's 0 =
	// root) and "readOnlyRootFilesystem: true" is present (but only on one of two
	// containers). Structural parsing must catch both.
	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: x
  labels:
    app.kubernetes.io/name: x
spec:
  template:
    spec:
      securityContext:
        runAsUser: 0
      containers:
      - name: a
        image: a:1.0.0
        securityContext:
          readOnlyRootFilesystem: true
      - name: b
        image: b:1.0.0
`
	gp := &models.GoldenPath{Security: models.Security{
		RequireNonRoot:        true,
		RequireReadOnlyRootFS: true,
	}}
	in := makeInput("deploy.yaml", manifest, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-non-root"), "runAsUser: 0 is root")
	require.NotEmpty(t, findByRuleID(fs, "goldenpath.security.require-readonly-rootfs"), "container b lacks readOnlyRootFilesystem")
}

func TestGoldenPath_StructuralSecurityContextClean(t *testing.T) {
	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: x
  labels:
    app.kubernetes.io/name: x
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
      - name: a
        image: a:1.0.0
        securityContext:
          readOnlyRootFilesystem: true
`
	gp := &models.GoldenPath{Security: models.Security{
		RequireNonRoot:        true,
		RequireReadOnlyRootFS: true,
	}}
	in := makeInput("deploy.yaml", manifest, gp)
	fs, err := GoldenPathReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "goldenpath.security.require-non-root"))
	require.Empty(t, findByRuleID(fs, "goldenpath.security.require-readonly-rootfs"))
}

// ── ReliabilityReviewer ───────────────────────────────────────────────────────

func TestReliability_CleanManifestNoFindings(t *testing.T) {
	in := makeInput("deploy.yaml", deployBase, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, fs)
}

func TestReliability_MissingResourceLimits(t *testing.T) {
	content := strings.ReplaceAll(deployBase, "          limits:\n            cpu: \"500m\"\n            memory: \"256Mi\"\n", "")
	in := makeInput("deploy.yaml", content, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "reliability.resource-limits"))
}

func TestReliability_MissingProbes(t *testing.T) {
	content := strings.ReplaceAll(deployBase, "        livenessProbe:\n          httpGet:\n            path: /healthz\n            port: 8080\n", "")
	in := makeInput("deploy.yaml", content, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "reliability.liveness-probe"))
}

func TestReliability_SingleReplica(t *testing.T) {
	content := strings.ReplaceAll(deployBase, "  replicas: 2", "  replicas: 1")
	in := makeInput("deploy.yaml", content, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "reliability.single-replica"))
}

func TestReliability_CronJobResourceLimitsFlagged(t *testing.T) {
	in := makeInput("cronjob.yaml", cronJobInsecure, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "reliability.resource-limits"))
}

func TestReliability_CronJobNoProbeOrReplicaFindings(t *testing.T) {
	// Batch workloads run to completion: probes and replica checks must not fire.
	in := makeInput("cronjob.yaml", cronJobSecure, nil)
	fs, err := ReliabilityReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "reliability.liveness-probe"))
	require.Empty(t, findByRuleID(fs, "reliability.readiness-probe"))
	require.Empty(t, findByRuleID(fs, "reliability.single-replica"))
	require.Empty(t, findByRuleID(fs, "reliability.resource-limits"))
}

// ── ConventionsReviewer ───────────────────────────────────────────────────────

func TestConventions_LegacyAppLabelFlagged(t *testing.T) {
	in := makeInput("deploy.yaml", deployBase, nil)
	fs, err := ConventionsReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "conventions.app-kubernetes-io-labels"))
}

func TestConventions_WellKnownLabelsClean(t *testing.T) {
	content := strings.ReplaceAll(deployBase, "    app: payments", "    app.kubernetes.io/name: payments")
	in := makeInput("deploy.yaml", content, nil)
	fs, err := ConventionsReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, findByRuleID(fs, "conventions.app-kubernetes-io-labels"))
}

func TestConventions_RuntimeRuleViolation(t *testing.T) {
	gp := &models.GoldenPath{RuntimeRules: map[string]map[string]any{
		"require-owner-annotation": {
			"pattern":    "owner:",
			"message":    "owner annotation required",
			"suggestion": "Add `owner: <team>` to metadata.annotations.",
		},
	}}
	in := makeInput("deploy.yaml", deployBase, gp)
	fs, err := ConventionsReviewer{}.Review(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, findByRuleID(fs, "conventions.runtime-rule.require-owner-annotation"))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ruleIDSet(fs []models.Finding) map[string]bool {
	m := map[string]bool{}
	for _, f := range fs {
		m[f.RuleID] = true
	}
	return m
}

func findByRuleID(fs []models.Finding, id string) []models.Finding {
	var out []models.Finding
	for _, f := range fs {
		if f.RuleID == id {
			out = append(out, f)
		}
	}
	return out
}
