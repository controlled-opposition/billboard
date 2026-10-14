package bb

import (
	"context"
	"time"
)

// budget paces requests so a recursive content walk stays gentle on the
// shared 10k/day site budget, and caps one process at max requests so a
// runaway loop cannot spend it. Sequential by design: the client issues one
// request at a time, and only do() touches this struct.
type budget struct {
	interval time.Duration
	max      int
	last     time.Time
	used     int
}

func (b *budget) wait(ctx context.Context) error {
	if b.used >= b.max {
		return ErrRequestCap
	}
	if d := time.Until(b.last.Add(b.interval)); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	b.last = time.Now()
	b.used++
	return nil
}

// RequestsUsed reports requests issued this process.
func (c *Client) RequestsUsed() int { return c.budget.used }
