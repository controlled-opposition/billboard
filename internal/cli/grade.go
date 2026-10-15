package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

// gradeListStatuses leaves out unknown, which only due items carry.
var gradeListStatuses = slices.DeleteFunc(slices.Clone(bb.GradeStatuses), func(s bb.GradeStatus) bool { return s == bb.GradeUnknown })

var gradeSchema = output.Schema[bb.Grade]{
	Fields: []output.Field[bb.Grade]{
		{Name: "course", Doc: "object: id, code, name (TSV: code)", Value: func(g bb.Grade) any { return refOf(g.Course) }},
		{Name: "title", Doc: "column name", Value: func(g bb.Grade) any { return g.Title }},
		{Name: "status", Doc: joinKinds(gradeListStatuses), Value: func(g bb.Grade) any { return g.Status }},
		{Name: "score", Doc: "points earned; null when there is none", Value: func(g bb.Grade) any {
			if g.Score == nil {
				return nil
			}
			return *g.Score
		}},
		{Name: "possible", Doc: "points possible; 0 when the column has no points", Value: func(g bb.Grade) any { return g.Possible }},
		{Name: "display", Doc: "the grade as Blackboard displays it, e.g. A-", Value: func(g bb.Grade) any { return g.Display }},
		{Name: "feedback", Doc: "instructor feedback, as plain text", Value: func(g bb.Grade) any { return g.Feedback }},
		{Name: "due", Doc: "due time, RFC 3339 UTC; null when undated", Value: func(g bb.Grade) any {
			if g.Due.IsZero() {
				return nil
			}
			return g.Due
		}},
		{Name: "kind", Doc: joinKinds(bb.ColumnKinds) + " (calculated columns are course totals)", Value: func(g bb.Grade) any { return g.Kind }},
		{Name: "counted", Doc: "counts toward the course total", Value: func(g bb.Grade) any { return g.Counted }},
		{Name: "columnId", Doc: "gradebook column id (the same as due list columnId)", Value: func(g bb.Grade) any { return g.ColumnID }},
	},
	Default: []string{"course", "title", "status", "score", "possible", "display"},
}

func newGradeCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "grade", Short: "Grades and submission status"}
	cmd.AddCommand(newGradeListCmd(e))
	return cmd
}

func newGradeListCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [COURSE...]",
		Short: "List your gradebook",
		Long: `List every gradebook column you can see, with your status and score,
sorted by course, then due time (undated last), then title. Columns hidden
from students are left out. Calculated columns, such as "Overall Grade",
are course totals (kind calculated).

With no COURSE, list your current courses where you are a student. With
COURSE arguments, list exactly those courses, past ones included. Each
course costs 2 requests.

Statuses:
` + statusHelp(gradeListStatuses) + `
If one course's gradebook cannot be read (HTTP 403 or 404), the other
courses still print, stderr names the course, and bb exits 6. If no
course can be read, bb exits 3 or 5 and prints nothing.

Examples:
  bb grade list
  bb grade list math101 --json title,score,possible,feedback
  bb grade list --json course,title,display --jq '.[] | select(.title == "Overall Grade")'`,
	}
	o := output.Bind(cmd, gradeSchema)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		c := e.client()
		all, picked, err := coursesAndResolve(cmd.Context(), c, args)
		if err != nil {
			return err
		}
		if len(args) == 0 {
			for _, course := range all {
				if course.Current && course.Role == "Student" {
					picked = append(picked, course)
				}
			}
		}
		grades, err := c.Grades(cmd.Context(), picked)
		if err != nil && !bb.IsPartial(err) {
			return err
		}
		if perr := output.Print(e.streams, gradeSchema, o, grades); perr != nil {
			return perr
		}
		return err
	}
	return cmd
}
