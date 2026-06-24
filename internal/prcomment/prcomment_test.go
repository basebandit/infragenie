package prcomment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	require.Contains(t, Render(nil), "No findings")
	require.Contains(t, Render(nil), marker)

	body := Render([]models.Finding{
		{Severity: models.SeverityHigh, RuleID: "goldenpath.security.no-latest-tag", File: "deploy.yaml", Line: 12, Title: "image uses :latest"},
	})
	require.Contains(t, body, marker)
	require.Contains(t, body, "1 finding(s)")
	require.Contains(t, body, "`goldenpath.security.no-latest-tag`")
	require.Contains(t, body, "deploy.yaml:12")
}

// testClient points a go-github client at a test server.
func testClient(t *testing.T, srv *httptest.Server) *github.Client {
	t.Helper()
	c := github.NewClient(srv.Client())
	u, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	c.BaseURL = u
	return c
}

func TestUpsertCreatesWhenAbsent(t *testing.T) {
	var created bool
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/issues/7/comments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		case http.MethodPost:
			created = true
			_, _ = w.Write([]byte(`{"id":1}`))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Upsert(context.Background(), testClient(t, srv), "o", "r", 7, "body")
	require.NoError(t, err)
	require.True(t, created, "should create a new comment when none exists")
}

func TestUpsertUpdatesExisting(t *testing.T) {
	var edited bool
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/issues/7/comments", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":42,"body":"` + marker + `\nold"}]`))
	})
	mux.HandleFunc("/repos/o/r/issues/comments/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			edited = true
		}
		_, _ = w.Write([]byte(`{"id":42}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Upsert(context.Background(), testClient(t, srv), "o", "r", 7, "new body")
	require.NoError(t, err)
	require.True(t, edited, "should edit the existing marked comment")
}

func TestUpsertIgnoresOtherComments(t *testing.T) {
	var created bool
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/issues/7/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"id":5,"body":"a human comment"}]`))
			return
		}
		created = true
		_, _ = w.Write([]byte(`{"id":9}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	require.NoError(t, Upsert(context.Background(), testClient(t, srv), "o", "r", 7, strings.Repeat("x", 3)))
	require.True(t, created, "unmarked comments must not be treated as ours")
}
