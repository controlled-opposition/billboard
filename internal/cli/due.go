package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

var dueSchema = output.Schema[bb.DueItem]{
	Fields: []output.Field[bb.DueItem]{
		{Name: "due", Doc: "due time, RFC 3339 UTC", Value: func(d bb.DueItem) any { return d.Due }},
		{Name: "title", Doc: "item title", Value: func(d bb.DueItem) any { return d.Title }},
		{Name: "course", Doc: "object: id, code, name", Value: func(d bb.DueItem) any { return courseJSON(d.Course) }},
		{Name: "kind", Doc: "test | assignment | manual | other (manual columns are often in-class exams)", Value: func(d bb.DueItem) any { return d.Kind }},
		{Name: "rawType", Doc: "Blackboard eventType verbatim", Value: func(d bb.DueItem) any { return d.RawType }},
		{Name: "attemptable", Doc: "can be attempted online", Value: func(d bb.DueItem) any { return d.Attemptable }},
		{Name: "columnId", Doc: "gradebook column id", Value: func(d bb.DueItem) any { return d.ColumnID }},
	},
	Default: []string{"due", "course", "title", "kind"}, // TSV renders course as its code
}

// dueFlags: --course may repeat; --since/--until accept RFC 3339, YYYY-MM-DD
// (local midnight), or a signed duration relative to now ("-2d", "14d").
type dueFlags struct {
	courses      []string
	since, until string
	now          func() time.Time
}

// window is pure: default [now, now+14d). Bad input -> usageError.
func (f dueFlags) window() (bb.Window, error) { panic("not implemented") }

// bb due list [--course X]... [--since T] [--until T] [--json ...]
// Examples in Long:
//
//	bb due list                         # next 14 days, all courses
//	bb due list --since -3d --until 7d  # include recently passed
//	bb due list --course math101 --json title,due,kind
func newDueCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "due", Short: "Upcoming due dates", Hidden: true}
	cmd.AddCommand(newDueListCmd(e))
	return cmd
}

func newDueListCmd(e *env) *cobra.Command { return notImplemented("list", "List due dates") }
