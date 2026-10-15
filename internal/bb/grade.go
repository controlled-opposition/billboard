package bb

import (
	"context"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// GradeStatus is where one gradable item stands for the signed-in user.
// gradeStatus is the only place it is derived, for due items and grade rows
// alike, so the two commands cannot disagree about what "submitted" means.
type GradeStatus string

const (
	GradeGraded      GradeStatus = "graded"      // graded, or a score or display grade exists
	GradeSubmitted   GradeStatus = "submitted"   // received, not graded yet
	GradeUnsubmitted GradeStatus = "unsubmitted" // submittable online and nothing received
	GradeNone        GradeStatus = "none"        // nothing to submit online and no grade yet
	GradeExempt      GradeStatus = "exempt"
	GradeUnknown     GradeStatus = "unknown" // due items only: the course's gradebook could not be read
)

// GradeStatuses is every GradeStatus in display order; help text is built from it.
var GradeStatuses = []GradeStatus{GradeGraded, GradeSubmitted, GradeUnsubmitted, GradeNone, GradeExempt, GradeUnknown}

// Pending reports whether the item may still need the user's work. An
// unknown status counts, because a filter must not hide what it could not read.
func (s GradeStatus) Pending() bool { return s == GradeUnsubmitted || s == GradeUnknown }

// wireGrade is one row of gradebook/users/me. A column the user has no
// record for is absent from the response.
type wireGrade struct {
	ColumnID string   `json:"columnId"`
	Status   string   `json:"status"` // "Graded", "NeedsGrading", or absent
	Score    *float64 `json:"score"`
	Text     string   `json:"text"`
	Feedback string   `json:"feedback"`
	Exempt   bool     `json:"exempt"`
}

// gradeStatus is pure. rec is nil when the user has no record for the column.
func gradeStatus(rec *wireGrade, submittable bool) GradeStatus {
	switch {
	case rec != nil && rec.Exempt:
		return GradeExempt
	case rec != nil && rec.Status == "NeedsGrading":
		return GradeSubmitted
	case rec != nil && (rec.Status == "Graded" || rec.Score != nil || rec.Text != ""):
		return GradeGraded
	case submittable:
		return GradeUnsubmitted
	}
	return GradeNone
}

// myGrades reads GET /learn/api/public/v1/courses/{id}/gradebook/users/me,
// keyed by column. A course with no records answers 200 with no results.
func (c *Client) myGrades(ctx context.Context, id CourseID) (map[ColumnID]wireGrade, error) {
	recs, err := getPaged[wireGrade](ctx, c, "/learn/api/public/v1/courses/"+string(id)+"/gradebook/users/me")
	if err != nil {
		return nil, err
	}
	out := make(map[ColumnID]wireGrade, len(recs))
	for _, r := range recs {
		out[ColumnID(r.ColumnID)] = r
	}
	return out, nil
}

// ColumnKind is how a gradebook column is graded (grading.type).
type ColumnKind string

const (
	ColumnAttempts   ColumnKind = "attempts"   // submitted online
	ColumnManual     ColumnKind = "manual"     // entered by the instructor
	ColumnCalculated ColumnKind = "calculated" // a total or average, e.g. "Overall Grade"
	ColumnOther      ColumnKind = "other"
)

// ColumnKinds is every ColumnKind in display order; help text is built from it.
var ColumnKinds = []ColumnKind{ColumnAttempts, ColumnManual, ColumnCalculated, ColumnOther}

var columnKindByType = map[string]ColumnKind{
	"Attempts":   ColumnAttempts,
	"Manual":     ColumnManual,
	"Calculated": ColumnCalculated,
}

// Grade is one visible gradebook column joined with the user's record for it.
type Grade struct {
	Course   Course
	ColumnID ColumnID
	Title    string
	Kind     ColumnKind
	Status   GradeStatus // never GradeUnknown: a course that fails yields no rows
	Score    *float64    // nil when there is no score
	Possible float64     // 0 when the column has no points
	Display  string      // the grade as Blackboard displays it, e.g. "A-"
	Feedback string      // instructor feedback as plain text
	Due      time.Time   // zero when the column has no due date
	Counted  bool        // counts toward the course total
}

// Grades reads each course's GET /learn/api/public/v2/courses/{id}/gradebook/columns
// (paged) and gradebook/users/me, 2 requests per course, and returns the
// visible columns sorted by course code, then due (undated last), then title.
//
// A course whose gradebook answers 403 or 404 yields no rows and is reported
// in a CourseErrors returned with the rest. If every course failed, the
// first failure is returned as is, so one forbidden course exits 3.
func (c *Client) Grades(ctx context.Context, courses []Course) ([]Grade, error) {
	var out []Grade
	var failed CourseErrors
	for _, course := range courses {
		grades, err := c.courseGrades(ctx, course)
		if err != nil {
			ce, ok := courseError(course, err)
			if !ok {
				return nil, err
			}
			failed = append(failed, ce)
			continue
		}
		out = append(out, grades...)
	}
	slices.SortStableFunc(out, func(a, b Grade) int {
		if c := strings.Compare(a.Course.Code, b.Course.Code); c != 0 {
			return c
		}
		if a.Due.IsZero() != b.Due.IsZero() {
			if a.Due.IsZero() {
				return 1
			}
			return -1
		}
		if c := a.Due.Compare(b.Due); c != 0 {
			return c
		}
		return strings.Compare(a.Title, b.Title)
	})
	switch {
	case failed == nil:
		return out, nil
	case len(failed) == len(courses):
		return nil, failed[0].Err
	}
	return out, failed
}

func (c *Client) courseGrades(ctx context.Context, course Course) ([]Grade, error) {
	cols, err := getPaged[wireColumn](ctx, c, "/learn/api/public/v2/courses/"+string(course.ID)+"/gradebook/columns")
	if err != nil {
		return nil, err
	}
	recs, err := c.myGrades(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	return parseGrades(course, cols, recs), nil
}

type wireColumn struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Grading struct {
		Type string `json:"type"`
		Due  string `json:"due"`
	} `json:"grading"`
	Score struct {
		Possible float64 `json:"possible"`
	} `json:"score"`
	Availability struct {
		Available string `json:"available"`
	} `json:"availability"`
	IncludeInCalculations bool `json:"includeInCalculations"`
}

// parseGrades is pure: wire -> domain. It drops columns whose availability
// is not "Yes", which Ultra hides from students.
func parseGrades(course Course, cols []wireColumn, recs map[ColumnID]wireGrade) []Grade {
	var out []Grade
	for _, w := range cols {
		if w.Availability.Available != "Yes" {
			continue
		}
		kind, ok := columnKindByType[w.Grading.Type]
		if !ok {
			kind = ColumnOther
		}
		g := Grade{
			Course:   course,
			ColumnID: ColumnID(w.ID),
			Title:    w.Name,
			Kind:     kind,
			Possible: w.Score.Possible,
			Counted:  w.IncludeInCalculations,
		}
		if due, err := time.Parse(time.RFC3339, w.Grading.Due); err == nil {
			g.Due = due.UTC()
		}
		rec := lookup(recs, g.ColumnID)
		g.Status = gradeStatus(rec, kind == ColumnAttempts)
		if rec != nil {
			g.Score, g.Display, g.Feedback = rec.Score, rec.Text, htmlText(rec.Feedback)
		}
		out = append(out, g)
	}
	return out
}

// htmlText flattens an HTML fragment to one line of text with entities
// decoded. Block boundaries become spaces so paragraphs do not run together.
func htmlText(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	var b strings.Builder
	for n := range doc.Descendants() {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if blockElements[n.Data] {
				b.WriteByte(' ')
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var blockElements = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "td": true, "th": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "blockquote": true, "pre": true,
}

// lookup returns a pointer to recs[id], or nil when there is no record.
func lookup(recs map[ColumnID]wireGrade, id ColumnID) *wireGrade {
	if r, ok := recs[id]; ok {
		return &r
	}
	return nil
}
