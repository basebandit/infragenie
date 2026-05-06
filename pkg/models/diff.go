package models

type DiffSource string

const (
	DiffSourceLocal DiffSource = "local"
	DiffSourcePR    DiffSource = "pr"
)

type Diff struct {
	Source DiffSource
	Files  []FileDiff
}

type FileDiff struct {
	Path       string
	OldPath    string
	Status     string // added | modified | deleted | renamed
	Hunks      []Hunk
	NewContent string
}

type Hunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	Lines              []string
}
