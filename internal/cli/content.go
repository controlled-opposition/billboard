package cli

import (
	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

var contentSchema = output.Schema[bb.ContentNode]{
	Fields: []output.Field[bb.ContentNode]{
		{Name: "id", Doc: "content id (accepted by `content download COURSE ID`)", Value: func(n bb.ContentNode) any { return n.ID }},
		{Name: "kind", Doc: "folder | document | test | file | link | unknown", Value: func(n bb.ContentNode) any { return n.Kind }},
		{Name: "title", Doc: "title as shown in Ultra", Value: func(n bb.ContentNode) any { return n.Title }},
		{Name: "path", Doc: "titles from course root, joined with ' / ' in TSV", Value: func(n bb.ContentNode) any { return n.Path }},
		{Name: "depth", Doc: "0 = top level", Value: func(n bb.ContentNode) any { return n.Depth }},
		{Name: "parentId", Doc: "parent content id", Value: func(n bb.ContentNode) any { return n.Parent }},
		{Name: "available", Doc: "visible to you", Value: func(n bb.ContentNode) any { return n.Available }},
		{Name: "modified", Doc: "last modified, RFC 3339", Value: func(n bb.ContentNode) any { return n.Modified }},
		{Name: "url", Doc: "Ultra link", Value: func(n bb.ContentNode) any { return n.URL }},
		{Name: "files", Doc: "array of {name, size, mimeType}", Value: func(n bb.ContentNode) any { return filesJSON(n.Attachments) }},
	},
	Default: []string{"id", "kind", "path", "files"}, // TSV renders files as a count
}

var fetchSchema = output.Schema[bb.FetchResult]{
	Fields: []output.Field[bb.FetchResult]{
		{Name: "status", Doc: "planned | downloaded | skipped | failed", Value: func(r bb.FetchResult) any { return r.Status }},
		{Name: "path", Doc: "absolute local path", Value: func(r bb.FetchResult) any { return r.Path }},
		{Name: "name", Doc: "file name", Value: func(r bb.FetchResult) any { return r.Attachment.Name }},
		{Name: "size", Doc: "bytes", Value: func(r bb.FetchResult) any { return r.Attachment.Size }},
		{Name: "mimeType", Doc: "MIME type", Value: func(r bb.FetchResult) any { return r.Attachment.MimeType }},
		{Name: "contentId", Doc: "content item the file belongs to", Value: func(r bb.FetchResult) any { return r.Attachment.Source }},
		{Name: "course", Doc: "object: id, code, name", Value: func(r bb.FetchResult) any { return courseJSON(r.Course) }},
		{Name: "error", Doc: "failure message, empty unless failed", Value: func(r bb.FetchResult) any { return errString(r.Err) }},
	},
	Default: []string{"status", "path"},
}

func filesJSON(as []bb.Attachment) any { panic("not implemented") }
func errString(err error) string       { panic("not implemented") }

func newContentCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "content", Short: "Course materials"}
	cmd.AddCommand(newContentListCmd(e), newContentDownloadCmd(e))
	return cmd
}

// bb content list COURSE [--under ID] [--depth N] [--kind K]... [--files-only]
// One row per visible node, depth-first. --files-only keeps nodes with at
// least one attachment (the agent's "what can I download" view).
func newContentListCmd(e *env) *cobra.Command { panic("not implemented") }

// bb content download COURSE [ID...] [--dir D] [--match GLOB] [--dry-run] [--force]
//
// No IDs: walk the whole course. With IDs: walk only those subtrees
// (Under), which is the cheap path after `content list`. Files land at
// D/<course code>/<outline path>/<file name>. Re-running is safe: existing
// files of the expected size are reported "skipped". --dry-run prints the
// plan with status "planned" and makes only the walk requests.
// Any failed file -> exit 6 after all others are attempted.
func newContentDownloadCmd(e *env) *cobra.Command { panic("not implemented") }
