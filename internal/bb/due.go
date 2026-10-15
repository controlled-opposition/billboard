package bb

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Window is a half-open time range [Since, Until). Built only by NewWindow,
// which rejects Until <= Since, so methods never re-check it.
type Window struct{ since, until time.Time }

func NewWindow(since, until time.Time) (Window, error) {
	if !until.After(since) {
		return Window{}, errors.New("--until must be after --since")
	}
	return Window{since, until}, nil
}
func (w Window) Since() time.Time { return w.since }
func (w Window) Until() time.Time { return w.until }

// DueKind is what the calendar says the item is. Blackboard's eventType is
// mapped here once; unknown values become DueOther with the raw string kept
// in DueItem.RawType so nothing is silently dropped.
type DueKind string

const (
	DueTest       DueKind = "test"       // eventType "Test" (tests, quizzes, many assignments)
	DueAssignment DueKind = "assignment" // eventType "Assignment"
	DueManual     DueKind = "manual"     // "Manual Grade Column": often an in-class exam, not a submission
	DueOther      DueKind = "other"
)

// DueKinds is every DueKind in display order; help text is built from it.
var DueKinds = []DueKind{DueTest, DueAssignment, DueManual, DueOther}

var dueKindByEventType = map[string]DueKind{
	"Test":                DueTest,
	"Assignment":          DueAssignment,
	"Manual Grade Column": DueManual,
}

// DueItem is one dated gradable thing.
type DueItem struct {
	Course      Course // joined from Courses by calendarId; Code/Name usable directly
	Title       string
	Due         time.Time
	Kind        DueKind
	RawType     string // Blackboard eventType verbatim, for DueOther and debugging
	Attemptable bool   // calendar says it can be attempted online
	ColumnID    ColumnID
	Status      GradeStatus // GradeUnknown when the course's gradebook could not be read
}

// DueFilter narrows Due. Zero value: every course, the window only.
type DueFilter struct {
	Window  Window
	Courses []CourseID // empty = all
}

// calendarSpan is the widest range one calendar request may ask for; wider
// windows are split. Blackboard answers 400 "since and until cannot span
// more than 16 weeks" beyond it.
const calendarSpan = 16 * 7 * 24 * time.Hour

// Due reads GET /learn/api/public/v1/calendars/items?since&until, joins
// each gradebook item to a Course, and returns items sorted by Due then
// Course.Code. Items from dropped courses are left out.
//
// Each item's Status comes from its course's gradebook/users/me, one
// request per course with an item; a calendar item id is a column id. A
// course whose gradebook answers 403 or 404 keeps its items as GradeUnknown
// and is reported in a CourseErrors returned with them.
//
// courses comes from Courses so a command pays for the list once.
func (c *Client) Due(ctx context.Context, courses []Course, f DueFilter) ([]DueItem, error) {
	var wire []wireCalendarItem
	for from := f.Window.since; from.Before(f.Window.until); from = from.Add(calendarSpan) {
		to := from.Add(calendarSpan)
		if to.After(f.Window.until) {
			to = f.Window.until
		}
		q := url.Values{"since": {from.UTC().Format(time.RFC3339)}, "until": {to.UTC().Format(time.RFC3339)}}
		items, err := getPaged[wireCalendarItem](ctx, c, "/learn/api/public/v1/calendars/items?"+q.Encode())
		if err != nil {
			return nil, err
		}
		wire = append(wire, items...)
	}
	items := parseDue(wire, courses)
	if len(f.Courses) > 0 {
		items = slices.DeleteFunc(items, func(d DueItem) bool { return !slices.Contains(f.Courses, d.Course.ID) })
	}
	var failed CourseErrors
	recsByCourse := map[CourseID]map[ColumnID]wireGrade{} // nil value: unreadable
	for i, it := range items {
		recs, seen := recsByCourse[it.Course.ID]
		if !seen {
			var err error
			if recs, err = c.myGrades(ctx, it.Course.ID); err != nil {
				ce, ok := courseError(it.Course, err)
				if !ok {
					return nil, err
				}
				failed = append(failed, ce)
			}
			recsByCourse[it.Course.ID] = recs
		}
		if recs == nil {
			items[i].Status = GradeUnknown
			continue
		}
		items[i].Status = gradeStatus(lookup(recs, it.ColumnID), it.Attemptable)
	}
	if failed != nil {
		return items, failed
	}
	return items, nil
}

type wireCalendarItem struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // "GradebookColumn"
	CalendarID   string `json:"calendarId"`
	CalendarName string `json:"calendarName"` // "MATH101-01-F26: Applied ... (Fall 2026)"
	Title        string `json:"title"`
	End          string `json:"end"`
	Dynamic      struct {
		EventType   string `json:"eventType"`
		Attemptable bool   `json:"attemptable"`
	} `json:"dynamicCalendarItemProps"`
}

// parseDue keeps gradebook items (other calendar types are class meetings
// and personal events, not things that are due), dedupes items that two
// window chunks both returned, and drops items of dropped courses
// (membership "Disabled"). A calendarId missing from courses falls back to
// the course named in calendarName rather than vanishing.
func parseDue(items []wireCalendarItem, courses []Course) []DueItem {
	byID := make(map[CourseID]Course, len(courses))
	for _, c := range courses {
		byID[c.ID] = c
	}
	seen := map[string]bool{}
	var out []DueItem
	for _, w := range items {
		if w.Type != "GradebookColumn" || seen[w.ID] {
			continue
		}
		seen[w.ID] = true
		course, known := byID[CourseID(w.CalendarID)]
		if known && !course.Open {
			continue
		}
		if !known {
			course = courseFromCalendarName(CourseID(w.CalendarID), w.CalendarName)
		}
		kind, ok := dueKindByEventType[w.Dynamic.EventType]
		if !ok {
			kind = DueOther
		}
		due, _ := time.Parse(time.RFC3339, w.End)
		out = append(out, DueItem{
			Course:      course,
			Title:       w.Title,
			Due:         due.UTC(),
			Kind:        kind,
			RawType:     w.Dynamic.EventType,
			Attemptable: w.Dynamic.Attemptable,
			ColumnID:    ColumnID(w.ID),
		})
	}
	slices.SortStableFunc(out, func(a, b DueItem) int {
		if c := a.Due.Compare(b.Due); c != 0 {
			return c
		}
		return strings.Compare(a.Course.Code, b.Course.Code)
	})
	return out
}

// courseFromCalendarName parses "CODE: Name (Term)".
func courseFromCalendarName(id CourseID, name string) Course {
	code, rest, _ := strings.Cut(name, ": ")
	c := Course{ID: id, Code: code, Name: rest}
	if i := strings.LastIndex(rest, " ("); i >= 0 && strings.HasSuffix(rest, ")") {
		c.Name, c.Term = rest[:i], rest[i+2:len(rest)-1]
	}
	return c
}
