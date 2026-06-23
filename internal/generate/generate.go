// Package generate scaffolds new services from a resolved Golden Path. Output
// is deterministic (no LLM): every file is rendered from the GoldenPath so the
// result conforms to the same policy `infragenie review` enforces — generate
// then review yields zero Golden Path findings.
package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/basebandit/infragenie/pkg/models"
)

// Params controls a single generate run.
type Params struct {
	Name       string             // service name (DNS-1123 label)
	Template   string             // built-in template set; defaults to DefaultTemplate
	OutDir     string             // parent directory; files land in OutDir/Name
	GoldenPath *models.GoldenPath // resolved Golden Path; nil falls back to a sane default
	Force      bool               // overwrite existing files
}

// Result reports what a run produced.
type Result struct {
	Template string
	Dir      string
	Files    []string // absolute paths written, in render order
}

// Generator renders template sets to disk.
type Generator struct{}

func New() *Generator { return &Generator{} }

// Run renders the selected template set into OutDir/Name.
func (g *Generator) Run(p Params) (*Result, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, fmt.Errorf("service name is required")
	}
	tmplName := p.Template
	if tmplName == "" {
		tmplName = DefaultTemplate
	}
	set, ok := lookup(tmplName)
	if !ok {
		return nil, fmt.Errorf("unknown template %q (known: %s)", tmplName, strings.Join(TemplateNames(), ", "))
	}

	gp := p.GoldenPath
	if gp == nil {
		gp = defaultGoldenPath()
	}

	data := buildData(name, gp)
	outDir := p.OutDir
	if outDir == "" {
		outDir = "services"
	}
	root := filepath.Join(outDir, name)

	res := &Result{Template: set.name, Dir: root}
	for _, ft := range set.files {
		rendered, err := renderFile(ft.src, data, set.leftDelim, set.rightDelim)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", ft.src, err)
		}
		dst := filepath.Join(root, ft.out)
		if !p.Force {
			if _, err := os.Stat(dst); err == nil {
				return nil, fmt.Errorf("%s already exists — use --force to overwrite", dst)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, rendered, 0o644); err != nil {
			return nil, err
		}
		abs, _ := filepath.Abs(dst)
		res.Files = append(res.Files, abs)
	}
	return res, nil
}

// templateData is the render context exposed to every template.
type templateData struct {
	Service     serviceData
	GoldenPath  *models.GoldenPath
	Labels      map[string]string // resolved label key→value, includes required labels
	Annotations map[string]string // satisfies runtime_rule patterns that look like annotations
}

type serviceData struct {
	Name  string
	Image string
	Tag   string
	Port  string
}

func buildData(name string, gp *models.GoldenPath) templateData {
	tag := "0.1.0"
	d := templateData{
		Service: serviceData{
			Name:  name,
			Image: "ghcr.io/basebandit/" + name,
			Tag:   tag,
			Port:  "8080",
		},
		GoldenPath:  gp,
		Labels:      buildLabels(name, tag, gp.RequiredLabels),
		Annotations: buildAnnotations(gp.RuntimeRules),
	}
	return d
}

// buildAnnotations derives metadata annotations from GoldenPath runtime_rules.
// A rule whose `pattern` looks like an annotation key (ends with ":") becomes an
// annotation with a TODO value, so generated manifests satisfy the conventions
// reviewer's runtime-rule checks out of the box.
func buildAnnotations(rules map[string]map[string]any) map[string]string {
	out := map[string]string{}
	for _, params := range rules {
		p, ok := params["pattern"].(string)
		if !ok {
			continue
		}
		key := strings.TrimSuffix(strings.TrimSpace(p), ":")
		if key == "" || strings.ContainsAny(key, " \t\n") {
			continue
		}
		out[key] = "TODO"
	}
	return out
}

// buildLabels resolves a value for every required label and always includes
// app.kubernetes.io/name so selectors and the conventions reviewer are happy.
func buildLabels(name, tag string, required []string) map[string]string {
	labels := map[string]string{"app.kubernetes.io/name": name}
	for _, key := range required {
		labels[key] = labelValue(key, name, tag)
	}
	return labels
}

func labelValue(key, name, tag string) string {
	k := strings.ToLower(key)
	switch {
	case strings.Contains(k, "name"):
		return name
	case strings.Contains(k, "version"):
		return tag
	case strings.Contains(k, "component"):
		return "service"
	case strings.Contains(k, "part-of"):
		return name
	case strings.Contains(k, "team"):
		return "TODO-team"
	case strings.Contains(k, "cost"):
		return "TODO-cost-centre"
	case strings.Contains(k, "data-classification"):
		return "internal"
	case strings.Contains(k, "compliance"):
		return "none"
	case strings.Contains(k, "env"):
		return "dev"
	default:
		return "TODO"
	}
}

func renderFile(src string, data templateData, leftDelim, rightDelim string) ([]byte, error) {
	raw, err := tmplFS.ReadFile("templates/" + src)
	if err != nil {
		return nil, err
	}
	t := template.New(filepath.Base(src))
	if leftDelim != "" && rightDelim != "" {
		t = t.Delims(leftDelim, rightDelim)
	}
	t, err = t.Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// defaultGoldenPath is used when no Golden Path is supplied. It mirrors the
// secure defaults the scaffold already bakes in, so output stays self-conformant.
func defaultGoldenPath() *models.GoldenPath {
	return &models.GoldenPath{
		Version: 1,
		Name:    "default",
		RequiredLabels: []string{
			"app.kubernetes.io/name",
			"app.kubernetes.io/version",
			"app.kubernetes.io/component",
		},
		Security: models.Security{
			RequireNonRoot:        true,
			RequireReadOnlyRootFS: true,
			RequireNetworkPolicy:  true,
			ForbidImageTagLatest:  true,
		},
		Observability: models.Observability{RequirePrometheusAnnotations: true},
		CIRequired: []models.CIStep{
			{Name: "tests", Matches: []string{"go test", "pytest", "npm test"}},
			{Name: "vulnerability-scan", Matches: []string{"trivy", "grype"}},
		},
	}
}
