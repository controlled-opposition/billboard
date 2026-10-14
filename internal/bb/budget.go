package bb

import (
	"context"
	"time"
)

// budget paces requests so a recursive content walk stays gentle on the
// shared 10k/day site budget. Sequential by design: the client issues one
// request at a time, so there is no shared mutable state to guard beyond
// this struct, which only do() touches.
type budget struct {
	interval time.Duration // min spacing between requests, default 250ms
	last     time.Time
	used     int
}

func (b *budget) wait(ctx context.Context) error { panic("not implemented") }

// Used reports requests issued this process, for --verbose/stderr diagnostics.
func (c *Client) RequestsUsed() int { panic("not implemented") }
