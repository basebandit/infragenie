// Package diff parses unified diff output and produces models.Diff values
// from local working trees and GitHub PRs.
package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
)

// Parse converts unified diff bytes into []FileDiff. It handles add/delete/
// modify/rename and skips binary entries.
func Parse(b []byte) ([]models.FileDiff, error) {
	var (
		files []models.FileDiff
		cur   *models.FileDiff
		hunk  *models.Hunk
	)
	flush := func() {
		if cur != nil {
			files = append(files, *cur)
			cur = nil
			hunk = nil
		}
	}

	s := bufio.NewScanner(bytes.NewReader(b))
	s.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for s.Scan() {
		line := s.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			cur = &models.FileDiff{Status: "modified"}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "new file mode"):
			cur.Status = "added"
		case strings.HasPrefix(line, "deleted file mode"):
			cur.Status = "deleted"
		case strings.HasPrefix(line, "rename from "):
			cur.Status = "renamed"
			cur.OldPath = strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			cur.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "--- "):
			p := strings.TrimPrefix(line, "--- ")
			if p == "/dev/null" {
				cur.Status = "added"
			} else if cur.OldPath == "" {
				cur.OldPath = strings.TrimPrefix(p, "a/")
			}
		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if p == "/dev/null" {
				cur.Status = "deleted"
			} else if cur.Path == "" {
				cur.Path = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(line, "@@ "):
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			cur.Hunks = append(cur.Hunks, h)
			hunk = &cur.Hunks[len(cur.Hunks)-1]
		case strings.HasPrefix(line, "Binary files "):
			continue
		default:
			if hunk != nil && len(line) > 0 {
				switch line[0] {
				case '+', '-', ' ':
					hunk.Lines = append(hunk.Lines, line)
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	flush()
	return files, nil
}

func parseHunkHeader(line string) (models.Hunk, error) {
	rest := strings.TrimPrefix(line, "@@ ")
	end := strings.Index(rest, " @@")
	if end < 0 {
		return models.Hunk{}, fmt.Errorf("bad hunk header: %s", line)
	}
	parts := strings.Fields(rest[:end])
	if len(parts) != 2 {
		return models.Hunk{}, fmt.Errorf("bad hunk header: %s", line)
	}
	olds, oldln, err := splitRange(parts[0][1:])
	if err != nil {
		return models.Hunk{}, err
	}
	news, newln, err := splitRange(parts[1][1:])
	if err != nil {
		return models.Hunk{}, err
	}
	return models.Hunk{OldStart: olds, OldLines: oldln, NewStart: news, NewLines: newln}, nil
}

func splitRange(s string) (start, count int, err error) {
	if i := strings.Index(s, ","); i >= 0 {
		start, err = strconv.Atoi(s[:i])
		if err != nil {
			return
		}
		count, err = strconv.Atoi(s[i+1:])
		return
	}
	start, err = strconv.Atoi(s)
	count = 1
	return
}
