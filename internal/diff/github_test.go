package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePRURL(t *testing.T) {
	cases := []struct {
		in    string
		owner string
		repo  string
		num   int
		err   bool
	}{
		{"https://github.com/foo/bar/pull/42", "foo", "bar", 42, false},
		{"https://github.com/foo/bar/pull/42/files", "foo", "bar", 42, false},
		{"foo/bar#42", "foo", "bar", 42, false},
		{"foo/bar/42", "foo", "bar", 42, false},
		{"foo/bar", "", "", 0, true},
		{"https://github.com/foo/bar/issues/42", "", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, err := ParsePRURL(c.in)
			if c.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.owner, r.Owner)
			require.Equal(t, c.repo, r.Repo)
			require.Equal(t, c.num, r.Number)
		})
	}
}
