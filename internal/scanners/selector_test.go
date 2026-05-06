package scanners

import (
	"context"
	"testing"

	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/stretchr/testify/require"
)

type fakeScanner struct {
	name   string
	stacks []string
}

func (f fakeScanner) Name() string                                                  { return f.name }
func (f fakeScanner) Available() bool                                               { return true }
func (f fakeScanner) Scan(_ context.Context, _ ScanInput) ([]models.Finding, error) { return nil, nil }
func (f fakeScanner) Stacks() []string                                              { return f.stacks }

type universalScanner struct{ name string }

func (u universalScanner) Name() string                                                  { return u.name }
func (u universalScanner) Available() bool                                               { return true }
func (u universalScanner) Scan(_ context.Context, _ ScanInput) ([]models.Finding, error) { return nil, nil }

func TestSelectByDetectedLanguage(t *testing.T) {
	all := []Scanner{
		fakeScanner{name: "gosec", stacks: []string{"go"}},
		fakeScanner{name: "bandit", stacks: []string{"python"}},
		universalScanner{name: "trivy"},
	}
	rc := &repo.Context{Languages: []string{"go"}}
	out := Select(all, nil, rc)

	names := []string{}
	for _, s := range out {
		names = append(names, s.Name())
	}
	require.ElementsMatch(t, []string{"gosec", "trivy"}, names)
}

func TestSelectByGoldenPathStacks(t *testing.T) {
	all := []Scanner{
		fakeScanner{name: "checkov", stacks: []string{"helm", "terraform"}},
		fakeScanner{name: "tflint", stacks: []string{"terraform"}},
		fakeScanner{name: "gosec", stacks: []string{"go"}},
	}
	gp := &models.GoldenPath{Stacks: models.Stacks{Platforms: []string{"helm"}}}
	out := Select(all, gp, nil)

	names := []string{}
	for _, s := range out {
		names = append(names, s.Name())
	}
	require.ElementsMatch(t, []string{"checkov"}, names)
}

func TestSelectNoSignalReturnsAll(t *testing.T) {
	all := []Scanner{fakeScanner{name: "gosec", stacks: []string{"go"}}}
	require.Len(t, Select(all, nil, nil), 1)
}
