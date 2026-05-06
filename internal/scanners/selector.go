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

// Select returns scanners whose declared stacks intersect with the
// resolved Golden Path's stacks ∪ the auto-detected repo context.
// Scanners that don't implement StackAware are always included.
// If no relevance signal is available, all scanners are included.
func Select(all []Scanner, gp *models.GoldenPath, rc *repo.Context) []Scanner {
	rel := relevance(gp, rc)
	if len(rel) == 0 {
		return all
	}
	out := make([]Scanner, 0, len(all))
	for _, s := range all {
		sa, ok := s.(StackAware)
		if !ok {
			out = append(out, s)
			continue
		}
		for _, st := range sa.Stacks() {
			if rel[st] {
				out = append(out, s)
				break
			}
		}
	}
	return out
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
