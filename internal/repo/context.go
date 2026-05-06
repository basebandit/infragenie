// Package repo holds the working-tree context the engine reasons over.
// Phase B: minimal stub. Phase C populates language detection and stack inference.
package repo

type Context struct {
	Root      string
	Files     []string
	Languages []string
	Stacks    []string
	Summary   string
}
