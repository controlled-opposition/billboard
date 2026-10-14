package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

var dueSchema = output.Schema[bb.DueItem]{
	Fields: []output.Field[bb.DueItem]{
		{Name: "due", Doc: "due time, RFC 3339 UTC", Value: func(d bb.DueItem) any { return d.Due }},
		{Name: "title", Doc: "item title", Value: func(d bb.DueItem) any { return d.Title }},
		{Name: "course", Doc: "object: id, code, name (TSV: code)", Value: func(d bb.DueItem) any { return refOf(d.Course) }},
		{Name: "kind", Doc: joinKinds(bb.DueKinds) + " (manual columns are often in-class exams)", Value: func(d bb.DueItem) any { return d.Kind }},
		{Name: "rawType", Doc: "Blackboard eventType verbatim", Value: func(d bb.DueItem) any { return d.RawType }},
		{Name: "attemptable", Doc: "can be attempted online", Value: func(d bb.DueItem) any { return d.Attemptable }},
		{Name: "columnId", Doc: "gradebook column id", Value: func(d bb.DueItem) any { return d.ColumnID }},
	},
	Default: []string{"due", "course", "title", "kind"},
}

func newDueCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "due", Short: "Upcoming due dates"}
	cmd.AddCommand(newDueListCmd(e))
	return cmd
}

func newDueListCmd(e *env) *cobra.Command {
	var courses []string
	var since, until string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List due dates from the Blackboard calendar",
		Long: `List graded items with a due date between --since and --until, from the
Blackboard calendar, sorted by due time. Items from dropped courses are
left out. Blackboard does not report whether you already submitted.

--since and --until take RFC 3339 (2026-10-15T03:59:00Z), a date
(2026-10-15, local midnight), or a signed offset from now in days or Go
duration units (-1d, 14d, 36h).

Examples:
  bb due list
  bb due list --since -3d --until 7d
  bb due list --course math101 --json title,due,kind`,
		Args: positional(cobra.NoArgs),
	}
	o := output.Bind(cmd, dueSchema)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		now := e.now()
		w, err := window(since, until, now)
		if err != nil {
			return err
		}
		c := e.client()
		all, picked, err := coursesAndResolve(cmd.Context(), c, courses)
		if err != nil {
			return err
		}
		f := bb.DueFilter{Window: w}
		for _, p := range picked {
			f.Courses = append(f.Courses, p.ID)
		}
		items, err := c.Due(cmd.Context(), all, f)
		if err != nil {
			return err
		}
		return output.Print(e.streams, dueSchema, o, items)
	}
	cmd.Flags().StringArrayVar(&courses, "course", nil, "only this `COURSE` (code, code prefix, or id); repeatable")
	cmd.Flags().StringVar(&since, "since", "-1d", "start of the window")
	cmd.Flags().StringVar(&until, "until", "14d", "end of the window")
	return cmd
}

// window is pure, so a bad flag fails before any request.
func window(since, until string, now time.Time) (bb.Window, error) {
	s, err := parseWhen(since, now)
	if err != nil {
		return bb.Window{}, err
	}
	u, err := parseWhen(until, now)
	if err != nil {
		return bb.Window{}, err
	}
	w, err := bb.NewWindow(s, u)
	if err != nil {
		return bb.Window{}, usageError{err.Error()}
	}
	return w, nil
}

func parseWhen(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation(time.DateOnly, s, time.Local); err == nil {
		return t, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil {
			return now.AddDate(0, 0, n), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(d), nil
	}
	return time.Time{}, usageError{fmt.Sprintf("invalid time %q: use RFC 3339, YYYY-MM-DD, or an offset like -1d, 14d, 36h", s)}
}
