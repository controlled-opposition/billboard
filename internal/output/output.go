// Package output renders domain rows for both audiences from one schema:
//
//	TTY      -> aligned table with a header, local times
//	non-TTY  -> tab-separated, one row per line, fixed default columns,
//	            RFC 3339 UTC times, no header (stable for awk/cut)
//	--json f -> JSON array of objects with exactly fields f
//	--jq / --template -> applied to that JSON (gojq, go-gh pkg/template)
//
// A command declares a Schema[T] once. The --json field list, its
// validation, the TSV/table columns, and the "JSON FIELDS" section of
// --help are all derived from it, so adding a field is a one-line change
// that cannot leave help text stale.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/cli/go-gh/v2/pkg/template"
	"github.com/spf13/cobra"
)

// Field is one exported attribute of T. Name is the JSON key (camelCase)
// and the --json token. Value returns a JSON-encodable value, or an untyped
// nil for "no value": null in JSON, an empty cell in TSV and tables.
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

func (s Schema[T]) field(name string) (Field[T], bool) {
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field[T]{}, false
}

// Options are bound to flags by Bind. Exactly one renderer is active;
// --jq/--template without --json is a usage error, as in gh.
type Options struct {
	JSON     []string
	JQ       string
	Template string
	jq       *JQ
}

// Bind registers --json/--jq/--template on cmd and appends a JSON FIELDS
// section with the schema's field docs to cmd.Long. Flag validation runs in
// a PreRunE it installs, so a typo or a malformed --jq or --template fails
// before any network request; its errors go through cmd.FlagErrorFunc so
// the CLI classifies them as usage.
func Bind[T any](cmd *cobra.Command, s Schema[T]) *Options {
	o := &Options{}
	names := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		names[i] = f.Name
	}
	cmd.Flags().StringSliceVar(&o.JSON, "json", nil, "output JSON with the given comma-separated `fields`")
	cmd.Flags().StringVarP(&o.JQ, "jq", "q", "", "filter JSON output with a jq `expression`")
	cmd.Flags().StringVarP(&o.Template, "template", "t", "", "format JSON output with a Go `template`")

	var help strings.Builder
	help.WriteString("\n\nJSON FIELDS\n")
	tw := tabwriter.NewWriter(&help, 0, 0, 2, ' ', 0)
	for _, f := range s.Fields {
		fmt.Fprintf(tw, "  %s\t%s\n", f.Name, f.Doc)
	}
	tw.Flush()
	cmd.Long = strings.TrimRight(cmd.Long, "\n") + strings.TrimRight(help.String(), "\n")

	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		fail := func(format string, a ...any) error { return c.FlagErrorFunc()(c, fmt.Errorf(format, a...)) }
		for _, n := range o.JSON {
			if !slices.Contains(names, n) {
				return fail("unknown JSON field %q; available: %s", n, strings.Join(names, ", "))
			}
		}
		if c.Flags().Changed("json") && len(o.JSON) == 0 {
			return fail("--json needs at least one field; available: %s", strings.Join(names, ", "))
		}
		if (o.JQ != "" || o.Template != "") && len(o.JSON) == 0 {
			return fail("--jq and --template require --json")
		}
		if o.JQ != "" && o.Template != "" {
			return fail("use only one of --jq and --template")
		}
		if o.JQ != "" {
			q, err := ParseJQ(o.JQ)
			if err != nil {
				return fail("%v", err)
			}
			o.jq = q
		}
		if o.Template != "" {
			if err := template.New(io.Discard, 0, false).Parse(o.Template); err != nil {
				return fail("invalid template %q: %v", o.Template, err)
			}
		}
		return nil
	}
	return o
}

// Streams is the process I/O, with TTY detection done once by the caller.
type Streams struct {
	Out, Err io.Writer
	OutTTY   bool
	Width    int // terminal columns for the table; 0 = unknown
}

// Print renders rows. An empty result prints nothing on non-TTY and "no
// results" on stderr for TTY; it is never an error.
func Print[T any](st Streams, s Schema[T], o *Options, rows []T) error {
	if len(o.JSON) > 0 {
		return printJSON(st, s, o, rows)
	}
	if len(rows) == 0 {
		if st.OutTTY {
			fmt.Fprintln(st.Err, "no results")
		}
		return nil
	}
	width := st.Width
	if width <= 0 {
		width = 80
	}
	tp := tableprinter.New(st.Out, st.OutTTY, width)
	if st.OutTTY {
		header := make([]string, len(s.Default))
		for i, n := range s.Default {
			header[i] = strings.ToUpper(n)
		}
		tp.AddHeader(header)
	}
	for _, r := range rows {
		for _, n := range s.Default {
			f, _ := s.field(n)
			tp.AddField(text(f.Value(r), st.OutTTY))
		}
		tp.EndRow()
	}
	return tp.Render()
}

func printJSON[T any](st Streams, s Schema[T], o *Options, rows []T) error {
	objs := make([]map[string]any, len(rows))
	for i, r := range rows {
		obj := make(map[string]any, len(o.JSON))
		for _, n := range o.JSON {
			f, _ := s.field(n)
			obj[n] = f.Value(r)
		}
		objs[i] = obj
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if st.OutTTY && o.JQ == "" && o.Template == "" {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(objs); err != nil {
		return err
	}
	switch {
	case o.JQ != "":
		if o.jq == nil {
			q, err := ParseJQ(o.JQ)
			if err != nil {
				return err
			}
			o.jq = q
		}
		return o.jq.Run(&buf, st.Out)
	case o.Template != "":
		t := template.New(st.Out, st.Width, false)
		if err := t.Parse(o.Template); err != nil {
			return err
		}
		if err := t.Execute(&buf); err != nil {
			return err
		}
		return t.Flush()
	}
	_, err := st.Out.Write(buf.Bytes())
	return err
}

// text renders one value as a single table cell: no tabs or newlines, so
// one row stays one line.
func text(v any, tty bool) string {
	var s string
	switch v := v.(type) {
	case nil:
		s = ""
	case time.Time:
		switch {
		case v.IsZero():
			s = ""
		case tty:
			s = v.Local().Format("Jan 2 2006 15:04")
		default:
			s = v.UTC().Format(time.RFC3339)
		}
	case []string:
		s = strings.Join(v, " / ")
	default:
		s = fmt.Sprint(v)
	}
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}
