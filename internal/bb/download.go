package bb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Planned is one attachment bound to a destination path. Plans are pure
// values so `--dry-run` prints exactly what a real run would write.
type Planned struct {
	Attachment Attachment
	Course     Course
	RelPath    string // relative to the download dir: "<Course.Code>/<node Path...>/<Name>"
}

// Plan maps walked nodes to destinations. Pure and deterministic:
//   - every path segment is sanitized, so instructor-authored titles cannot
//     escape the download dir;
//   - the same file reached from two nodes is planned once;
//   - two different files whose RelPaths match ignoring case get " (<RID>)" before the
//     extension, so names never depend on walk order.
func Plan(course Course, nodes []ContentNode) []Planned {
	seen := map[string]bool{}
	var out []Planned
	for _, n := range nodes {
		for _, a := range n.Attachments {
			if seen[a.path] {
				continue
			}
			seen[a.path] = true
			segs := []string{segment(course.Code)}
			for _, t := range n.Path {
				segs = append(segs, segment(t))
			}
			segs = append(segs, segment(a.Name))
			out = append(out, Planned{Attachment: a, Course: course, RelPath: filepath.Join(segs...)})
		}
	}
	// Keyed case-folded: macOS and Windows file systems ignore case by default.
	count := map[string]int{}
	for _, p := range out {
		count[strings.ToLower(p.RelPath)]++
	}
	for i, p := range out {
		if count[strings.ToLower(p.RelPath)] > 1 {
			ext := filepath.Ext(p.RelPath)
			out[i].RelPath = strings.TrimSuffix(p.RelPath, ext) + " (" + p.Attachment.RID + ")" + ext
		}
	}
	return out
}

// segment makes one title safe as a single path component.
func segment(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimLeft(strings.TrimSpace(s), ".")
	const maxBytes = 200 // under the usual 255-byte name limit, leaving room for " (rid)" and ".part"
	if len(s) > maxBytes {
		ext := filepath.Ext(s)
		if len(ext) > 16 { // a dot inside a long title, not a file extension
			ext = ""
		}
		stem := strings.TrimSuffix(s, ext)
		for len(stem)+len(ext) > maxBytes {
			_, size := utf8.DecodeLastRuneInString(stem)
			stem = stem[:len(stem)-size]
		}
		s = stem + ext
	}
	if s == "" {
		return "_"
	}
	return s
}

// FetchStatus is the per-file outcome.
type FetchStatus string

const (
	FetchPlanned    FetchStatus = "planned" // --dry-run: would download
	FetchDownloaded FetchStatus = "downloaded"
	FetchSkipped    FetchStatus = "skipped" // already on disk with the expected size and mtime
	FetchFailed     FetchStatus = "failed"
)

// FetchStatuses is every FetchStatus in display order; help text is built from it.
var FetchStatuses = []FetchStatus{FetchPlanned, FetchDownloaded, FetchSkipped, FetchFailed}

type FetchResult struct {
	Planned
	Path   string // absolute path on disk
	Status FetchStatus
	Bytes  int64 // size on disk when downloaded or skipped
	Err    error // non-nil iff Status == FetchFailed
}

// Check reports what Fetch would do without a request: FetchSkipped when
// dir/RelPath is current, else FetchPlanned. A file is current when its
// size equals the attachment's size and its mtime equals the attachment's
// Modified; an unknown value is not compared, and with both unknown nothing
// can prove the file current.
func (p Planned) Check(dir string) FetchResult {
	r := FetchResult{Planned: p, Path: filepath.Join(dir, p.RelPath), Status: FetchPlanned}
	a := p.Attachment
	st, err := os.Stat(r.Path)
	if err != nil || !st.Mode().IsRegular() || (a.Size == 0 && a.Modified.IsZero()) {
		return r
	}
	// Seconds only: some filesystems store coarser mtimes than Blackboard's milliseconds.
	if (a.Size == 0 || st.Size() == a.Size) &&
		(a.Modified.IsZero() || st.ModTime().Truncate(time.Second).Equal(a.Modified.Truncate(time.Second))) {
		r.Status, r.Bytes = FetchSkipped, st.Size()
	}
	return r
}

// Fetch downloads one planned file into dir unless Check says it is
// current. It streams to "<path>.part" and renames into place, so a crash
// never leaves a truncated file under the final name and the next run
// overwrites the leftover. The mtime is set after the rename, so a rerun
// skips the file without a request.
//
// Fetch never returns an error: failures are FetchResult values so one bad
// file does not abort the rest (the CLI turns any failure into exit 6).
func (c *Client) Fetch(ctx context.Context, p Planned, dir string) FetchResult {
	r := p.Check(dir)
	if r.Status == FetchSkipped {
		return r
	}
	n, err := c.download(ctx, p.Attachment, r.Path)
	if err != nil {
		r.Status, r.Err = FetchFailed, err
		return r
	}
	r.Status, r.Bytes = FetchDownloaded, n
	return r
}

func (c *Client) download(ctx context.Context, a Attachment, dest string) (int64, error) {
	body, err := c.open(ctx, a)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	part := dest + ".part"
	f, err := os.Create(part)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && a.Size > 0 && n != a.Size {
		err = fmt.Errorf("received %d bytes, expected %d", n, a.Size)
	}
	if err == nil {
		err = os.Rename(part, dest)
	}
	if err != nil {
		os.Remove(part)
		return 0, err
	}
	if !a.Modified.IsZero() {
		if err := os.Chtimes(dest, time.Time{}, a.Modified); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// open GETs the attachment's unsigned bbcswebdav path through do(), so the
// session, renewal, pacing, and the request cap all apply. An HTML page
// where a non-HTML file was expected is an error, never saved as the file.
func (c *Client) open(ctx context.Context, a Attachment) (io.ReadCloser, error) {
	u, err := c.resolve(a.path)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") && !strings.HasPrefix(a.MimeType, "text/html") {
		resp.Body.Close()
		return nil, fmt.Errorf("Blackboard returned an HTML page instead of %s", a.Name)
	}
	return resp.Body, nil
}
