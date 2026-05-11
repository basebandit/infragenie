package scanners

import (
	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/pkg/models"
)

// StackAware is an optional capability scanners implement to opt into
// stack-driven activation.
type StackAware interface {
	Stacks() []string
}

// Select returns the scanners that should run for this invocation.
//
// Resolution order (highest to lowest priority):
//  1. cliOnly — if non-empty, run exactly these scanners (by name)
//  2. gp.Scanners.Enable — allowlist from goldenpath.yml; only these run
//  3. Stack-aware auto-detection against golden path + repo context
//  4. gp.Scanners.Disable — blocklist applied after all of the above
func Select(all []Scanner, gp *models.GoldenPath, rc *repo.Context, cliOnly []string) []Scanner {
	// Step 1: CLI --scanner flag is the highest override.
	if len(cliOnly) > 0 {
		return filterByNames(all, toSet(cliOnly))
	}

	var selected []Scanner

	// Step 2: goldenpath.yml scanners.enable is an explicit allowlist.
	if gp != nil && len(gp.Scanners.Enable) > 0 {
		selected = filterByNames(all, toSet(gp.Scanners.Enable))
	} else {
		// Step 3: stack-aware auto-detection.
		rel := relevance(gp, rc)
		if len(rel) == 0 {
			selected = all
		} else {
			for _, s := range all {
				sa, ok := s.(StackAware)
				if !ok {
					selected = append(selected, s)
					continue
				}
				for _, st := range sa.Stacks() {
					if rel[st] {
						selected = append(selected, s)
						break
					}
				}
			}
		}
	}

	// Step 4: apply goldenpath.yml scanners.disable blocklist.
	if gp != nil && len(gp.Scanners.Disable) > 0 {
		blocked := toSet(gp.Scanners.Disable)
		out := selected[:0]
		for _, s := range selected {
			if !blocked[s.Name()] {
				out = append(out, s)
			}
		}
		return out
	}
	return selected
}

func filterByNames(all []Scanner, names map[string]bool) []Scanner {
	out := make([]Scanner, 0, len(names))
	for _, s := range all {
		if names[s.Name()] {
			out = append(out, s)
		}
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func relevance(gp *models.GoldenPath, rc *repo.Context) map[string]bool {
	rel := map[string]bool{}
	add := func(xs []string) {
		for _, x := range xs {
			rel[x] = true
		}
	}
	if gp != nil {
		add(gp.Stacks.Runtimes)
		add(gp.Stacks.Platforms)
	}
	if rc != nil {
		add(rc.Languages)
		add(rc.Stacks)
	}
	return rel
}
