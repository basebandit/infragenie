package telemetry

import (
	"errors"
	"fmt"
	"sync"
)

var ErrBudgetExceeded = errors.New("budget exceeded")

type Budget struct {
	Tokens int
	USD    float64
}

func (b Budget) Zero() bool { return b.Tokens == 0 && b.USD == 0 }

// Counter tracks consumed budget against a configured limit. Callers invoke
// Reserve before an LLM call (with rough estimates) and Add after with
// actual usage. Reserve returns ErrBudgetExceeded when caps would be crossed.
type Counter struct {
	mu    sync.Mutex
	limit Budget
	spent Budget
}

func NewCounter(limit Budget) *Counter { return &Counter{limit: limit} }

func (c *Counter) Reserve(estTokens int, estUSD float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.limit.Zero() {
		return nil
	}
	afterTokens := c.spent.Tokens + estTokens
	afterUSD := c.spent.USD + estUSD
	if c.limit.Tokens > 0 && afterTokens > c.limit.Tokens {
		return fmt.Errorf("%w: tokens %d > limit %d", ErrBudgetExceeded, afterTokens, c.limit.Tokens)
	}
	if c.limit.USD > 0 && afterUSD > c.limit.USD {
		return fmt.Errorf("%w: $%.4f > limit $%.4f", ErrBudgetExceeded, afterUSD, c.limit.USD)
	}
	return nil
}

func (c *Counter) Add(tokens int, usd float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spent.Tokens += tokens
	c.spent.USD += usd
}

func (c *Counter) Spent() Budget {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.spent
}

func (c *Counter) Limit() Budget { return c.limit }
