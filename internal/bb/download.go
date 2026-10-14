package bb

import (
	"context"
	"io"
)

// Planned is one attachment bound to a destination path. Plans are pure
// values so `--dry-run` prints exactly what a real run would write.
type Planned struct {
	Attachment Attachment
	Course     Course
	RelPath    string // relative to the download dir: "<Course.Code>/<node Path...>/<Name>"
}

// Plan maps walked nodes to destinations. Pure and deterministic:
//   - every path segment is sanitized ('/', NUL, leading dots, OS-reserved
//     chars replaced; length capped), so instructor-authored titles cannot
//     escape the download dir;
//   - the same attachment (same RID) reached from two nodes is planned once;
//   - two different RIDs with the same RelPath get " (<RID>)" before the
//     extension, so names never depend on walk order.
func Plan(course Course, nodes []ContentNode) []Planned { panic("not implemented") }

// FetchStatus is the per-file outcome, in the order an agent cares about.
type FetchStatus string

const (
	FetchDownloaded FetchStatus = "downloaded"
	FetchSkipped    FetchStatus = "skipped" // file exists with the expected size
	FetchFailed     FetchStatus = "failed"
)

type FetchResult struct {
	Planned
	Path   string // absolute path on disk
	Status FetchStatus
	Bytes  int64
	Err    error // non-nil iff Status == FetchFailed
}

// Fetch downloads one planned file into dir. Idempotent: if dir/RelPath
// exists and its size equals Attachment.Size (when known), it is skipped
// without a request. Otherwise it streams to a temp file in the same
// directory and renames into place, so a crash never leaves a truncated file
// under the final name. --force (overwrite) is the caller zeroing that check.
//
// Fetch never returns an error: failures are FetchResult values so one bad
// file does not abort the rest (the CLI turns any failure into exit 6).
func (c *Client) Fetch(ctx context.Context, p Planned, dir string, overwrite bool) FetchResult {
	panic("not implemented")
}

// open GETs the signed bbcswebdav href through do() (session attached; the
// signature may make the cookie unnecessary, untested) and returns the body.
// A 302 to an HTML login page is detected and mapped to auth.ErrNeedsHuman
// rather than saved as a PDF.
func (c *Client) open(ctx context.Context, a Attachment) (io.ReadCloser, error) {
	panic("not implemented")
}
