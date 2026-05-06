// Package reviewers is the Layer-3 contract: opt-in LLM reviewers that flag
// issues rule-based scanners can't (Golden Path conformance, drift, intent).
package reviewers

import (
	"context"

	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/pkg/models"
)

type ReviewInput struct {
	Diff       *models.Diff
	RepoCtx    *repo.Context
	GoldenPath *models.GoldenPath
}

type Reviewer interface {
	Name() string
	Review(ctx context.Context, in ReviewInput) ([]models.Finding, error)
}
