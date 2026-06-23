package reviewers

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// manifestDoc is a single YAML document from a manifest file: its raw text plus
// the fields we reason about structurally. Decoded is false when the document
// isn't valid YAML with a kind — e.g. an un-rendered Helm/Kustomize template —
// in which case callers skip structural checks rather than guess from substrings.
type manifestDoc struct {
	Raw         string
	Decoded     bool
	Kind        string
	Labels      map[string]string
	Annotations map[string]string
}

// parseManifestDocs splits content into YAML documents and decodes each one's
// kind and metadata. A file is no longer treated as a single blob: every
// document is reviewed in its own right, with its own labels and kind.
func parseManifestDocs(content string) []manifestDoc {
	var out []manifestDoc
	for _, raw := range splitYAMLDocuments(content) {
		d := manifestDoc{Raw: raw}
		var probe struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Labels      map[string]string `yaml:"labels"`
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
		}
		if err := yaml.Unmarshal([]byte(raw), &probe); err == nil && probe.Kind != "" {
			d.Decoded = true
			d.Kind = probe.Kind
			d.Labels = probe.Metadata.Labels
			d.Annotations = probe.Metadata.Annotations
		}
		out = append(out, d)
	}
	return out
}

// splitYAMLDocuments splits a multi-document YAML string on `---` separators,
// preserving each document's raw text. A file with no separators yields one doc.
func splitYAMLDocuments(content string) []string {
	var docs []string
	var cur []string
	flush := func() {
		if strings.TrimSpace(strings.Join(cur, "\n")) != "" {
			docs = append(docs, strings.Join(cur, "\n"))
		}
		cur = nil
	}
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimRight(line, " \t") == "---" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	if len(docs) == 0 {
		return []string{content}
	}
	return docs
}

// anyKind reports whether any decoded document in docs has the given kind.
func anyKind(docs []manifestDoc, kind string) bool {
	for _, d := range docs {
		if d.Decoded && d.Kind == kind {
			return true
		}
	}
	return false
}

// isLongRunningKind reports a long-running, pod-bearing workload — probes,
// replicas, and metrics scraping all make sense here.
func isLongRunningKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Pod":
		return true
	}
	return false
}

// isBatchKind reports a run-to-completion workload (no probes/replicas/scrape).
func isBatchKind(kind string) bool {
	return kind == "Job" || kind == "CronJob"
}

// isWorkloadKind reports any pod-bearing workload kind.
func isWorkloadKind(kind string) bool {
	return isLongRunningKind(kind) || isBatchKind(kind)
}
