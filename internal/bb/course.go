package bb

import (
	"context"
	"time"
)

// Course is one enrollment, flattened: membership + course + term name.
type Course struct {
	ID      CourseID // "_12345_1"
	Code    string   // "MATH101-01-F26" (course.courseId)
	Name    string   // HTML-unescaped: "Calculus & Analysis"
	Term    string   // "Fall 2026"; "" when the course has no term
	Role    string   // "Student", "Instructor", ...
	Current bool     // see isCurrent
	Open    bool     // membership AND course availability are both "Yes"
	URL     string   // externalAccessUrl (Ultra outline)
}

// Courses returns every membership (paged users/{me}/courses?expand=course,
// joined with /terms), sorted Current first, then Code. One call costs ~2-3
// requests; commands that need course names call it once and pass the slice
// around rather than re-fetching.
func (c *Client) Courses(ctx context.Context) ([]Course, error) { panic("not implemented") }

// ResolveCourse turns a user/agent argument into exactly one course. Accepted
// forms, tried in order:
//  1. internal id "_12345_1"            (exact)
//  2. full code "MATH101-01-F26"         (case-insensitive exact)
//  3. code prefix "math101", "MATH101-01" (case-insensitive), among Current
//     courses first, then all courses
//
// No match -> *APIError{KindNotFound}. Several -> *APIError{KindAmbiguous,
// Candidates}. Never guesses. Pure: the caller supplies the list from
// Courses, so resolving N args costs zero extra requests.
func ResolveCourse(all []Course, arg string) (Course, error) { panic("not implemented") }

// isCurrent: terms have no dates at W&M (all "Continuous"), so the season is
// parsed from the term name ("Fall 2026") into a date window and checked
// against now. Membership "Disabled" is never current. A term name that does
// not parse is treated as current if the course is Open (fail visible, not
// silent: the course shows up rather than vanishing).
func isCurrent(termName string, open bool, now time.Time) bool { panic("not implemented") }

type wireMembership struct {
	CourseID     string `json:"courseId"`
	CourseRoleID string `json:"courseRoleId"`
	Availability struct {
		Available string `json:"available"`
	} `json:"availability"`
	Course struct {
		ID                string `json:"id"`
		CourseID          string `json:"courseId"`
		Name              string `json:"name"`
		TermID            string `json:"termId"`
		ExternalAccessURL string `json:"externalAccessUrl"`
		Availability      struct {
			Available string `json:"available"`
		} `json:"availability"`
	} `json:"course"`
}

type wireTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// parseCourses is pure: wire -> domain, html.UnescapeString on every name.
func parseCourses(ms []wireMembership, terms []wireTerm, now time.Time) []Course {
	panic("not implemented")
}
