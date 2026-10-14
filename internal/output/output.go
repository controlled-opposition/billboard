// Package output renders domain rows for both audiences from one schema:
//
//	TTY      -> aligned table, human time ("in 2d", "Oct 15 23:59")
//	non-TTY  -> tab-separated, one row per line, fixed default columns,
//	            RFC 3339 times, no header (stable for awk/cut)
//	--json f -> JSON array of objects with exactly fields f
//	--jq / --template -> applied to that JSON (go-gh pkg/jq, pkg/template)
//
// A command declares a Schema[T] once. The --json field list, its
// validation, the TSV/table columns, and the "JSON FIELDS" section of
// --help are all derived from it, so adding a field is a one-line change
// that cannot leave help text stale.
package output

import (
	"io"

	"github.com/spf13/cobra"
)

// Field is one exported attribute of T. Name is the JSON key (camelCase)
// and the --json token. Value returns a JSON-encodable value (time.Time,
// string, int64, bool, []string, nested map for course).
type Field[T any] struct {
	Name  string
	Doc   string // one line, shown under JSON FIELDS in --help
	Value func(T) any
}

// Schema is the single source of truth for how T leaves the process.
type Schema[T any] struct {
	Fields  []Field[T]
	Default []string // columns for table + TSV, in order; each must name a Field
}

// Options are bound to flags by Bind. Exactly one renderer is active;
// --jq/--template without --json is a usage error (exit 2), as in gh.
type Options struct {
	JSON     []string
	JQ       string
	Template string
}

// Bind registers --json/--jq/--template on cmd and appends a JSON FIELDS
// section plus the schema's field docs to cmd.Long. Validation of --json
// names against s.Fields happens in a PreRunE it installs, so a typo fails
// before any network request.
func Bind[T any](cmd *cobra.Command, s Schema[T]) *Options { panic("not implemented") }

// Streams is the process I/O, with TTY detection done once (go-gh pkg/term).
type Streams struct {
	Out, Err io.Writer
	OutTTY   bool
}

// Print renders rows. An empty result prints nothing on non-TTY and "no
// results" on stderr for TTY; it is never an error (exit 0).
func Print[T any](st Streams, s Schema[T], o *Options, rows []T) error { panic("not implemented") }
