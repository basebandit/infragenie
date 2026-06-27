// Package webhook implements a GitHub webhook receiver: it verifies the payload
// signature, parses pull_request events, and hands them to a processor (which
// reviews the PR and posts a comment).
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxBody caps the webhook payload we read.
const maxBody = 8 << 20 // 8 MiB

// PullRequest is the subset of a pull_request event the engine needs.
type PullRequest struct {
	Owner  string
	Repo   string
	Number int
	Action string
}

// Processor handles a pull_request event, e.g. review it and post a comment.
type Processor func(ctx context.Context, pr PullRequest) error

// VerifySignature checks GitHub's X-Hub-Signature-256 (HMAC-SHA256 of the body
// keyed by the webhook secret) in constant time.
func VerifySignature(payload []byte, signatureHeader, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	want := strings.TrimPrefix(signatureHeader, prefix)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(want))
}

// ParsePullRequest extracts PR coordinates and the action from a pull_request
// event payload.
func ParsePullRequest(payload []byte) (PullRequest, error) {
	var e struct {
		Action     string `json:"action"`
		Number     int    `json:"number"`
		Repository struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &e); err != nil {
		return PullRequest{}, err
	}
	num := e.Number
	if num == 0 {
		num = e.PullRequest.Number
	}
	if e.Repository.Owner.Login == "" || e.Repository.Name == "" || num == 0 {
		return PullRequest{}, fmt.Errorf("incomplete pull_request event")
	}
	return PullRequest{
		Owner:  e.Repository.Owner.Login,
		Repo:   e.Repository.Name,
		Number: num,
		Action: e.Action,
	}, nil
}

// Handler is the HTTP handler for GitHub webhooks.
type Handler struct {
	Secret  string
	Process Processor
	// Actions are the pull_request actions that trigger processing.
	Actions map[string]bool
}

// NewHandler builds a Handler that processes opened/synchronize/reopened PRs.
func NewHandler(secret string, p Processor) *Handler {
	return &Handler{
		Secret:  secret,
		Process: p,
		Actions: map[string]bool{"opened": true, "synchronize": true, "reopened": true},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if h.Secret != "" && !VerifySignature(body, r.Header.Get("X-Hub-Signature-256"), h.Secret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-GitHub-Event") != "pull_request" {
		w.WriteHeader(http.StatusNoContent) // ignore other events
		return
	}
	pr, err := ParsePullRequest(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.Actions[pr.Action] {
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "ignored action %q", pr.Action)
		return
	}
	if err := h.Process(r.Context(), pr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "reviewed %s/%s#%d", pr.Owner, pr.Repo, pr.Number)
}
