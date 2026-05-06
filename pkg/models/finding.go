// Package models holds shared data types used across the engine, scanners, reviewers, and reporters.
package models

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type Source string

const (
	SourceScanner   Source = "scanner"
	SourceGrounding Source = "grounding"
	SourceReviewer  Source = "reviewer"
)

type Finding struct {
	RuleID      string   `json:"rule_id"`
	Source      Source   `json:"source"`
	Origin      string   `json:"origin"`
	Severity    Severity `json:"severity"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Title       string   `json:"title"`
	Explanation string   `json:"explanation"`
	Suggestion  string   `json:"suggestion"`
	References  []string `json:"references,omitempty"`

	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence,omitempty"`
	EvidenceLoc string  `json:"evidence_loc,omitempty"`
	TrustLevel  string  `json:"trust_level"`
}

func SeverityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	case SeverityInfo:
		return 0
	}
	return -1
}
