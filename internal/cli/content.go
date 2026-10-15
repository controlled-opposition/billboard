package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

var contentSchema = output.Schema[bb.ContentNode]{
	Fields: []output.Field[bb.ContentNode]{
		{Name: "id", Doc: "content id (accepted by `content list` and `content download`)", Value: func(n bb.ContentNode) any { return n.ID }},
		{Name: "kind", Doc: joinKinds(bb.ContentKinds), Value: func(n bb.ContentNode) any { return n.Kind }},
		{Name: "title", Doc: "title as shown in Ultra", Value: func(n bb.ContentNode) any { return n.Title }},
		{Name: "path", Doc: "titles from course root (TSV: joined with ' / ')", Value: func(n bb.ContentNode) any { return n.Path }},
		{Name: "depth", Doc: "0 = top level", Value: func(n bb.ContentNode) any { return n.Depth }},
		{Name: "parentId", Doc: "parent content id", Value: func(n bb.ContentNode) any { return n.Parent }},
		{Name: "available", Doc: "visible to you", Value: func(n bb.ContentNode) any { return n.Available }},
		{Name: "modified", Doc: "last modified, RFC 3339", Value: func(n bb.ContentNode) any { return n.Modified }},
		{Name: "url", Doc: "Ultra link", Value: func(n bb.ContentNode) any { return n.URL }},
		{Name: "files", Doc: "array of {name, size, mimeType} (TSV: count)", Value: func(n bb.ContentNode) any { return filesOf(n.Attachments) }},
	},
	Default: []string{"id", "kind", "path", "files"},
}

var fetchSchema = output.Schema[bb.FetchResult]{
	Fields: []output.Field[bb.FetchResult]{
		{Name: "status", Doc: joinKinds(bb.FetchStatuses), Value: func(r bb.FetchResult) any { return r.Status }},
		{Name: "path", Doc: "absolute local path", Value: func(r bb.FetchResult) any { return r.Path }},
		{Name: "name", Doc: "file name", Value: func(r bb.FetchResult) any { return r.Attachment.Name }},
		{Name: "size", Doc: "bytes on disk if downloaded or skipped, else the size Blackboard lists (0 if unknown)", Value: func(r bb.FetchResult) any {
			if r.Status == bb.FetchDownloaded || r.Status == bb.FetchSkipped {
				return r.Bytes
			}
			return r.Attachment.Size
		}},
		{Name: "mimeType", Doc: "MIME type", Value: func(r bb.FetchResult) any { return r.Attachment.MimeType }},
		{Name: "contentId", Doc: "content item the file belongs to", Value: func(r bb.FetchResult) any { return r.Attachment.Source }},
		{Name: "course", Doc: "object: id, code, name (TSV: code)", Value: func(r bb.FetchResult) any { return refOf(r.Course) }},
		{Name: "error", Doc: "failure message, empty unless failed", Value: func(r bb.FetchResult) any { return errString(r.Err) }},
	},
	Default: []string{"status", "path"},
}

type fileJSON struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

// fileList renders as a count in TSV and tables, an array in JSON.
type fileList []fileJSON

func (f fileList) String() string { return strconv.Itoa(len(f)) }

func filesOf(as []bb.Attachment) fileList {
	out := fileList{}
	for _, a := range as {
		out = append(out, fileJSON{Name: a.Name, Size: a.Size, MimeType: a.MimeType})
	}
	return out
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func newContentCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "content", Short: "Course materials"}
	cmd.AddCommand(newContentListCmd(e), newContentDownloadCmd(e))
	return cmd
}

// contentIDs validates ID arguments before any request.
func contentIDs(args []string) ([]bb.ContentID, error) {
	ids := make([]bb.ContentID, len(args))
	for i, a := range args {
		id, ok := bb.ParseContentID(a)
		if !ok {
			return nil, usageError{fmt.Sprintf("invalid content id %q: expected the form _123_1 from `bb content list`", a)}
		}
		ids[i] = id
	}
	return ids, nil
}

func resolveOne(ctx context.Context, c *bb.Client, arg string) (bb.Course, error) {
	_, picked, err := coursesAndResolve(ctx, c, []string{arg})
	if err != nil {
		return bb.Course{}, err
	}
	return picked[0], nil
}

func newContentListCmd(e *env) *cobra.Command {
	var filesOnly bool
	cmd := &cobra.Command{
		Use:   "list COURSE [ID]",
		Short: "List course materials",
		Long: `List a course's outline depth-first, one row per item, as Ultra shows it:
an Ultra document is one row with the files linked from its body. With ID,
list only that item and everything under it.

A whole course costs one request per folder (about 60 for a large course).
Listing under an ID costs one request per level above it plus its subtree.

Examples:
  bb content list math101
  bb content list math101 --files-only --json id,path,files
  bb content list math101 _20000_1`,
		Args: positional(cobra.RangeArgs(1, 2)),
	}
	o := output.Bind(cmd, contentSchema)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ids, err := contentIDs(args[1:])
		if err != nil {
			return err
		}
		var opts bb.WalkOptions
		if len(ids) == 1 {
			opts.Under = ids[0]
		}
		c := e.client()
		course, err := resolveOne(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		nodes, err := c.Walk(cmd.Context(), course.ID, opts)
		if err != nil {
			return err
		}
		if filesOnly {
			var keep []bb.ContentNode
			for _, n := range nodes {
				if len(n.Attachments) > 0 {
					keep = append(keep, n)
				}
			}
			nodes = keep
		}
		return output.Print(e.streams, contentSchema, o, nodes)
	}
	cmd.Flags().BoolVar(&filesOnly, "files-only", false, "only items with at least one file")
	return cmd
}

func newContentDownloadCmd(e *env) *cobra.Command {
	var dir string
	var all, dryRun bool
	cmd := &cobra.Command{
		Use:   "download COURSE (ID... | --all)",
		Short: "Download course files",
		Long: `Download the files linked from the given items and everything under them,
or from the whole course with --all. Files land at
DIR/<course code>/<outline path>/<file name>, the same path whichever way
they were selected.

Re-running is safe: a file already on disk with the expected size and
modification time is reported "skipped" without a request. Downloads go
to "<name>.part" and are renamed into place when complete.

--all walks every folder in the course (about 60 requests for a large
course); prefer picking ids from ` + "`bb content list`" + `. --dry-run prints
the plan (planned or skipped) and requests only the walk.

If any file fails, the others are still attempted and bb exits 6.

Examples:
  bb content download math101 _20001_1 --dir ~/bb
  bb content download math101 --all --dir ~/bb --dry-run`,
		Args: positional(cobra.MinimumNArgs(1)),
	}
	o := output.Bind(cmd, fetchSchema)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ids, err := contentIDs(args[1:])
		if err != nil {
			return err
		}
		walks := make([]bb.WalkOptions, len(ids))
		for i, id := range ids {
			walks[i].Under = id
		}
		switch {
		case len(ids) == 0 && !all:
			return usageError{"name content ids from `bb content list`, or pass --all to walk the whole course (about 60 requests)"}
		case len(ids) > 0 && all:
			return usageError{"--all takes no content ids"}
		case all:
			walks = []bb.WalkOptions{{}}
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		c := e.client()
		course, err := resolveOne(ctx, c, args[0])
		if err != nil {
			return err
		}
		var nodes []bb.ContentNode
		for _, w := range walks {
			ns, err := c.Walk(ctx, course.ID, w)
			if err != nil {
				return err
			}
			nodes = append(nodes, ns...)
		}
		var results []bb.FetchResult
		var failed int
		var abort error
		for _, p := range bb.Plan(course, nodes) {
			if dryRun {
				results = append(results, p.Check(absDir))
				continue
			}
			r := c.Fetch(ctx, p, absDir)
			results = append(results, r)
			if r.Status != bb.FetchFailed {
				continue
			}
			failed++
			// Every later file would fail the same way.
			if errors.Is(r.Err, auth.ErrNeedsHuman) || errors.Is(r.Err, bb.ErrRequestCap) || ctx.Err() != nil {
				abort = r.Err
				break
			}
		}
		if err := output.Print(e.streams, fetchSchema, o, results); err != nil {
			return err
		}
		if abort != nil {
			return abort
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d files failed; see --json path,error: %w", failed, len(results), errPartial)
		}
		return nil
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "download into `DIR`")
	cmd.Flags().BoolVar(&all, "all", false, "download every file in the course")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without downloading")
	return cmd
}
