package bb

import (
	"context"
	"time"
)

// Window is a half-open time range [Since, Until). Built only by NewWindow,
// which rejects Until <= Since, so methods never re-check it.
type Window struct{ since, until time.Time }

func NewWindow(since, until time.Time) (Window, error) { panic("not implemented") }
func (w Window) Since() time.Time                      { return w.since }
func (w Window) Until() time.Time                      { return w.until }

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

// DueItem is one dated gradable thing. Submission state is deliberately
// absent: no probed endpoint reports it, and a field that is always
// "unknown" teaches agents to ignore it. Add Submitted when attempts are
// wired, as a new JSON field (additive, non-breaking).
type DueItem struct {
	Course      Course // joined from Courses by calendarId; Code/Name usable directly
	Title       string // unescaped
	Due         time.Time
	Kind        DueKind
	RawType     string // Blackboard eventType verbatim, for DueOther and debugging
	Attemptable bool   // calendar says it can be attempted online
	ColumnID    ColumnID
}

// DueFilter narrows Due. Zero value: every course, the window only.
type DueFilter struct {
	Window  Window
	Courses []CourseID // empty = all
}

// Due reads GET /learn/api/public/v1/calendars/items?since&until (the one
// source that answered 200 for every course; gradebook columns 403 in some),
// splitting the window into chunks if Blackboard caps the range, joins each
// item to a Course, and returns items sorted by Due then Course.Code.
// Items whose calendarId is not a known course (personal/institution
// calendars) are dropped.
//
// courses comes from Courses so a command pays for the list once.
func (c *Client) Due(ctx context.Context, courses []Course, f DueFilter) ([]DueItem, error) {
	panic("not implemented")
}

type wireCalendarItem struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // "GradebookColumn"
	CalendarID string `json:"calendarId"`
	Title      string `json:"title"`
	End        string `json:"end"`
	Dynamic    struct {
		EventType   string `json:"eventType"`
		Attemptable bool   `json:"attemptable"`
	} `json:"dynamicCalendarItemProps"`
}

func parseDue(items []wireCalendarItem, byID map[CourseID]Course) []DueItem { panic("not implemented") }
