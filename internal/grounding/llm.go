package grounding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/basebandit/infragenie/pkg/models"
)

// LLM is the narrow interface the grounder needs. The real binding wraps
// internal/llm/Client (Phase F); tests use a stub.
type LLM interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

// LLMGrounder enriches a Layer-1 finding with a repo-aware explanation and
// a suggested fix as a unified diff snippet. Cached on
// (rule_id, file, content_hash, model).
type LLMGrounder struct {
	llm    LLM
	model  string
	cache  *cache
	system string
}

func NewLLMGrounder(llm LLM, model string) *LLMGrounder {
	return &LLMGrounder{
		llm:    llm,
		model:  model,
		cache:  newCache(),
		system: defaultSystemPrompt,
	}
}

const defaultSystemPrompt = `You are an infrastructure code reviewer.
You receive a finding from a static scanner and the contents of the file it points at.
Return a JSON object: {"explanation": "...", "suggestion": "..."}.
- explanation: why this matters in this repo, grounded in the actual file (cite labels, lines, references that exist).
- suggestion: a unified diff snippet that fixes the issue.
Do not invent files, line numbers, or rules. Output JSON only.`

func (g *LLMGrounder) Ground(ctx context.Context, f models.Finding, fileContent string) (string, string, error) {
	key := keyFor(f.RuleID, f.File, fileContent, g.model)
	if r, ok := g.cache.get(key); ok {
		return r.Explanation, r.Suggestion, nil
	}
	user := buildUserPrompt(f, fileContent)
	raw, err := g.llm.Generate(ctx, g.system, user)
	if err != nil {
		return "", "", err
	}
	var out struct {
		Explanation string `json:"explanation"`
		Suggestion  string `json:"suggestion"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", "", fmt.Errorf("grounding: bad JSON: %w", err)
	}
	g.cache.set(key, out)
	return out.Explanation, out.Suggestion, nil
}

func buildUserPrompt(f models.Finding, fileContent string) string {
	return fmt.Sprintf(
		"## Finding\nrule_id: %s\norigin: %s\nseverity: %s\nfile: %s\nline: %d\ntitle: %s\n\n## File contents\n```\n%s\n```\n",
		f.RuleID, f.Origin, f.Severity, f.File, f.Line, f.Title, fileContent,
	)
}

func keyFor(ruleID, file, content, model string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s", ruleID, file, content, model)
	return hex.EncodeToString(h.Sum(nil))
}

type cachedResult struct {
	Explanation string `json:"explanation"`
	Suggestion  string `json:"suggestion"`
}

type cache struct {
	mu sync.RWMutex
	m  map[string]cachedResult
}

func newCache() *cache { return &cache{m: map[string]cachedResult{}} }

func (c *cache) get(k string) (cachedResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.m[k]
	return r, ok
}

func (c *cache) set(k string, r cachedResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = r
}
