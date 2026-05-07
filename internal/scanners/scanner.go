// Package scanners is the Layer-1 adapter contract.
// Implementations live in scanners/infra and scanners/lang (Phase D).
package scanners

import (
	"context"

	"github.com/basebandit/infragenie/internal/repo"
	"github.com/basebandit/infragenie/pkg/models"
)

type ScanInput struct {
	Diff    *models.Diff
	RepoCtx *repo.Context
}

type Scanner interface {
	Name() string
	Available() bool
	Scan(ctx context.Context, in ScanInput) ([]models.Finding, error)
}
