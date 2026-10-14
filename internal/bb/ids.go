package bb

// Blackboard primary keys look like "_12345_1". Distinct named types keep a
// course id from being passed where a content id is expected; the compiler
// catches the swap that the API would answer with a confusing 404.
type (
	UserID    string
	CourseID  string // internal pk, e.g. "_12345_1"; NOT the human code "MATH101-01-F26"
	ContentID string
	ColumnID  string // gradebook column; calendar item ids are column ids
)

// ParseCourseID accepts only the "_<digits>_<digits>" form. Human course
// codes go through ResolveCourse instead.
func ParseCourseID(s string) (CourseID, bool)   { panic("not implemented") }
func ParseContentID(s string) (ContentID, bool) { panic("not implemented") }
