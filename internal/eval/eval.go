// Package eval is the regression harness for engine output. Fixtures pair an
// expected set of findings with a recorded actual set; CI gates per-fixture
// precision and recall thresholds.
package eval

import "github.com/basebandit/infragenie/pkg/models"

type Expected struct {
	RuleID   string `json:"rule_id"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity,omitempty"`
}

type Metrics struct {
	TP int
	FP int
	FN int
}

func (m Metrics) Precision() float64 {
	if m.TP+m.FP == 0 {
		return 1.0
	}
	return float64(m.TP) / float64(m.TP+m.FP)
}

func (m Metrics) Recall() float64 {
	if m.TP+m.FN == 0 {
		return 1.0
	}
	return float64(m.TP) / float64(m.TP+m.FN)
}

func (m Metrics) F1() float64 {
	p, r := m.Precision(), m.Recall()
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

// Sum aggregates multiple Metrics into one.
func Sum(ms ...Metrics) Metrics {
	var out Metrics
	for _, m := range ms {
		out.TP += m.TP
		out.FP += m.FP
		out.FN += m.FN
	}
	return out
}

// Match scores actual findings against expected. Two findings match when
// they share rule_id and file and have lines within tolerance.
func Match(expected []Expected, actual []models.Finding, lineTolerance int) Metrics {
	used := make([]bool, len(actual))
	var m Metrics
	for _, e := range expected {
		if i := findMatch(e, actual, used, lineTolerance); i >= 0 {
			used[i] = true
			m.TP++
		} else {
			m.FN++
		}
	}
	for _, u := range used {
		if !u {
			m.FP++
		}
	}
	return m
}

func findMatch(e Expected, actual []models.Finding, used []bool, tol int) int {
	for i, a := range actual {
		if used[i] {
			continue
		}
		if a.RuleID != e.RuleID || a.File != e.File {
			continue
		}
		if abs(a.Line-e.Line) > tol {
			continue
		}
		return i
	}
	return -1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
