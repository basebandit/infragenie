package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	require.True(t, VerifySignature(body, sign(body, "s3cret"), "s3cret"))
	require.False(t, VerifySignature(body, sign(body, "wrong"), "s3cret"))
	require.False(t, VerifySignature(body, "sha256=deadbeef", "s3cret"))
	require.False(t, VerifySignature(body, "not-prefixed", "s3cret"))
}

func TestParsePullRequest(t *testing.T) {
	payload := []byte(`{"action":"opened","number":42,"repository":{"name":"infragenie","owner":{"login":"basebandit"}}}`)
	pr, err := ParsePullRequest(payload)
	require.NoError(t, err)
	require.Equal(t, PullRequest{Owner: "basebandit", Repo: "infragenie", Number: 42, Action: "opened"}, pr)

	_, err = ParsePullRequest([]byte(`{"action":"opened"}`))
	require.Error(t, err, "incomplete event must error")
}

const prPayload = `{"action":"opened","number":7,"repository":{"name":"r","owner":{"login":"o"}}}`

func TestHandlerProcessesValidEvent(t *testing.T) {
	var got PullRequest
	h := NewHandler("sec", func(_ context.Context, pr PullRequest) error {
		got = pr
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(prPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", sign([]byte(prPayload), "sec"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, PullRequest{Owner: "o", Repo: "r", Number: 7, Action: "opened"}, got)
}

func TestHandlerRejectsBadSignature(t *testing.T) {
	called := false
	h := NewHandler("sec", func(context.Context, PullRequest) error { called = true; return nil })

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(prPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", sign([]byte(prPayload), "wrong"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, called)
}

func TestHandlerIgnoresNonPREvent(t *testing.T) {
	called := false
	h := NewHandler("sec", func(context.Context, PullRequest) error { called = true; return nil })

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", sign([]byte(`{}`), "sec"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	require.False(t, called)
}

func TestHandlerIgnoresUninterestingAction(t *testing.T) {
	called := false
	body := `{"action":"labeled","number":7,"repository":{"name":"r","owner":{"login":"o"}}}`
	h := NewHandler("sec", func(context.Context, PullRequest) error { called = true; return nil })

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", sign([]byte(body), "sec"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.False(t, called, "labeled action should not trigger a review")
}
