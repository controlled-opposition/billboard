package cli

import (
	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

// courseJSON is shared by every schema that embeds a course (due items,
// download results) so "course" means the same object everywhere.
func courseJSON(c bb.Course) any {
	return map[string]any{"id": c.ID, "code": c.Code, "name": c.Name}
}

var courseSchema = output.Schema[bb.Course]{
	Fields: []output.Field[bb.Course]{
		{Name: "id", Doc: "internal id, e.g. _12345_1", Value: func(c bb.Course) any { return c.ID }},
		{Name: "code", Doc: "course code, e.g. MATH101-01-F26 (accepted as a course argument)", Value: func(c bb.Course) any { return c.Code }},
		{Name: "name", Doc: "course title", Value: func(c bb.Course) any { return c.Name }},
		{Name: "term", Doc: "term name", Value: func(c bb.Course) any { return c.Term }},
		{Name: "role", Doc: "your role", Value: func(c bb.Course) any { return c.Role }},
		{Name: "current", Doc: "in the current term", Value: func(c bb.Course) any { return c.Current }},
		{Name: "url", Doc: "Ultra outline URL", Value: func(c bb.Course) any { return c.URL }},
	},
	Default: []string{"code", "name", "term"},
}

func newCourseCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "course", Short: "Courses you are enrolled in"}
	cmd.AddCommand(newCourseListCmd(e))
	return cmd
}

func newCourseListCmd(e *env) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list [COURSE...]",
		Short: "List your courses",
		Long: `List courses in the current term, or every enrollment with --all.

With COURSE arguments, print only the course each one resolves to. A
COURSE is an internal id (_12345_1), a full code (MATH101-01-F26), or a
code prefix (math101). A prefix that matches several courses exits 5 and
lists the candidates on stderr.

Examples:
  bb course list
  bb course list --all --json code,name,term
  bb course list math101`,
	}
	o := output.Bind(cmd, courseSchema)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		courses, picked, err := coursesAndResolve(cmd.Context(), e.client(), args)
		if err != nil {
			return err
		}
		rows := picked
		if len(args) == 0 {
			rows = courses
			if !all {
				rows = nil
				for _, c := range courses {
					if c.Current {
						rows = append(rows, c)
					}
				}
			}
		}
		return output.Print(e.streams, courseSchema, o, rows)
	}
	cmd.Flags().BoolVar(&all, "all", false, "include past, future, and closed courses")
	return cmd
}
