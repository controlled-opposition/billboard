package cli

import (
	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

// courseFields is shared by every schema that embeds a course (due items,
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

// bb course list [--all] [--json ...]
// Default: Current courses only. --all: every membership, including disabled.
func newCourseListCmd(e *env) *cobra.Command { panic("not implemented") }
