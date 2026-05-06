package scanners

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

// BinaryAvailable reports whether bin is on PATH.
func BinaryAvailable(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// RunJSON executes bin with args and returns stdout. Many scanners exit
// non-zero when findings exist; non-zero with non-empty stdout is treated
// as success.
func RunJSON(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); ok {
		if len(out) > 0 {
			return out, nil
		}
		return out, fmt.Errorf("%s: %w (stderr: %s)", bin, err, string(ee.Stderr))
	}
	return out, err
}

// MapSeverity normalizes scanner-reported severity strings to our taxonomy.
func MapSeverity(s string) models.Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CRITICAL":
		return models.SeverityCritical
	case "HIGH", "ERROR":
		return models.SeverityHigh
	case "MEDIUM", "MODERATE", "WARNING":
		return models.SeverityMedium
	case "LOW", "INFO":
		return models.SeverityLow
	case "INFORMATIONAL", "STYLE", "NOTE":
		return models.SeverityInfo
	}
	return models.SeverityMedium
}
