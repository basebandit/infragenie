package generate

import (
	"embed"
	"sort"
)

//go:embed templates
var tmplFS embed.FS

// DefaultTemplate is used when no template is named.
const DefaultTemplate = "k8s-service"

// fileTmpl maps an embedded template source to its output path within the
// generated service directory.
type fileTmpl struct {
	src string // path under templates/
	out string // path relative to the service dir
}

type templateSet struct {
	name  string
	desc  string
	files []fileTmpl
}

// builtins is the registry of shipped template sets.
var builtins = map[string]templateSet{
	"k8s-service": {
		name: "k8s-service",
		desc: "Plain Kubernetes service: Deployment + NetworkPolicy + Service, Dockerfile, CI. Conformant to the Golden Path by construction.",
		files: []fileTmpl{
			{"k8s-service/deployment.yaml.tmpl", "deployment.yaml"},
			{"k8s-service/service.yaml.tmpl", "service.yaml"},
			{"k8s-service/Dockerfile.tmpl", "Dockerfile"},
			{"k8s-service/ci.yml.tmpl", ".github/workflows/ci.yml"},
			{"k8s-service/README.md.tmpl", "README.md"},
		},
	},
}

func lookup(name string) (templateSet, bool) {
	s, ok := builtins[name]
	return s, ok
}

// TemplateNames returns the sorted list of built-in template names.
func TemplateNames() []string {
	names := make([]string, 0, len(builtins))
	for n := range builtins {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Describe returns the one-line description for a template, or "" if unknown.
func Describe(name string) string {
	if s, ok := builtins[name]; ok {
		return s.desc
	}
	return ""
}
