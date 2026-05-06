// Package grounding is the Layer-2 contract: enrich a Layer-1 finding with a
// repo-aware explanation and a suggested fix as a diff snippet.
// Implementation lands in Phase D.
package grounding

import (
	"context"

	"github.com/basebandit/infragenie/pkg/models"
)

type Grounder interface {
	Ground(ctx context.Context, finding models.Finding, fileContent string) (explanation, suggestion string, err error)
}
