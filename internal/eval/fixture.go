package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/basebandit/infragenie/pkg/models"
)

// Fixture is a JSON-backed regression case: an expected finding set and the
// recorded actual finding set from a known-good run. Drift in the engine's
// behavior shows up as recall or precision drops on subsequent runs.
type Fixture struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Thresholds  Thresholds       `json:"thresholds"`
	Expected    []Expected       `json:"expected"`
	Actual      []models.Finding `json:"actual"`
}

type Thresholds struct {
	MinPrecision  float64 `json:"min_precision" yaml:"min_precision"`
	MinRecall     float64 `json:"min_recall" yaml:"min_recall"`
	LineTolerance int     `json:"line_tolerance" yaml:"line_tolerance"`
}

func defaultThresholds() Thresholds {
	return Thresholds{MinPrecision: 1.0, MinRecall: 1.0, LineTolerance: 1}
}

func LoadFixture(path string) (*Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx Fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if fx.Thresholds == (Thresholds{}) {
		fx.Thresholds = defaultThresholds()
	}
	return &fx, nil
}

func LoadFixtures(dir string) ([]*Fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	out := make([]*Fixture, 0, len(names))
	for _, n := range names {
		fx, err := LoadFixture(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		out = append(out, fx)
	}
	return out, nil
}

// Score evaluates the fixture against its thresholds. Returns the metrics
// and whether they pass.
func (f *Fixture) Score() (Metrics, bool) {
	m := Match(f.Expected, f.Actual, f.Thresholds.LineTolerance)
	pass := m.Precision() >= f.Thresholds.MinPrecision &&
		m.Recall() >= f.Thresholds.MinRecall
	return m, pass
}
