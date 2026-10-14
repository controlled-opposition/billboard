package cli_test

import (
	"strings"
	"testing"
)

const calendarDefaultWindow = "/learn/api/public/v1/calendars/items?since=2026-10-13T12%3A00%3A00Z&until=2026-10-28T12%3A00%3A00Z"

const calendarJSON = `{"results":[
	{"id":"_7004_1","type":"GradebookColumn","calendarId":"_777_1","calendarName":"ART100-01-F26: Drawing (Fall 2026)",
	 "title":"Sketchbook & Notes","end":"2026-10-22T03:59:00.000Z","dynamicCalendarItemProps":{"eventType":"Discussion","attemptable":true}},
	{"id":"_7001_1","type":"GradebookColumn","calendarId":"_501_1","calendarName":"SOC101-01-F26: Data & Society (Fall 2026)",
	 "title":"Essay 2","end":"2026-10-15T03:59:00.000Z","dynamicCalendarItemProps":{"eventType":"Test","attemptable":true}},
	{"id":"_7002_1","type":"GradebookColumn","calendarId":"_502_1","calendarName":"MATH211-01-F26: Linear Algebra (Fall 2026)",
	 "title":"Midterm","end":"2026-10-20T18:00:00.000Z","dynamicCalendarItemProps":{"eventType":"Manual Grade Column","attemptable":false}},
	{"id":"_7003_1","type":"GradebookColumn","calendarId":"_503_1","calendarName":"MATH211-02-F26: Linear Algebra (Fall 2026)",
	 "title":"Problem Set 5","end":"2026-10-15T03:59:00.000Z","dynamicCalendarItemProps":{"eventType":"Assignment","attemptable":true}},
	{"id":"_7005_1","type":"GradebookColumn","calendarId":"_404_1","calendarName":"HIST150-01-S25: World History (Spring 2025)",
	 "title":"Dropped Course Quiz","end":"2026-10-16T03:59:00.000Z","dynamicCalendarItemProps":{"eventType":"Test","attemptable":true}},
	{"id":"_8001_1","type":"Course","calendarId":"_502_1","calendarName":"MATH211-01-F26: Linear Algebra (Fall 2026)",
	 "title":"Lecture","end":"2026-10-16T15:00:00.000Z","dynamicCalendarItemProps":{}}
]}`

func TestDueListDefaultWindowKindsAndOrder(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = map[string]string{calendarDefaultWindow: calendarJSON}
	h.login()
	code, out, errOut := h.run(nil, "due", "list")
	expect(t, code, out, errOut, 0,
		"2026-10-15T03:59:00Z\tMATH211-02-F26\tProblem Set 5\tassignment\n"+
			"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\n"+
			"2026-10-20T18:00:00Z\tMATH211-01-F26\tMidterm\tmanual\n"+
			"2026-10-22T03:59:00Z\tART100-01-F26\tSketchbook & Notes\tother\n", "")
	if got := h.requests[len(h.requests)-1]; got != calendarDefaultWindow {
		t.Errorf("calendar request = %s, want %s", got, calendarDefaultWindow)
	}
}

func TestDueListJSONCarriesCourseObject(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = map[string]string{calendarDefaultWindow: calendarJSON}
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--json", "title,course,kind,rawType,attemptable,columnId")
	expect(t, code, out, errOut, 0, `[`+
		`{"attemptable":true,"columnId":"_7003_1","course":{"code":"MATH211-02-F26","id":"_503_1","name":"Linear Algebra"},"kind":"assignment","rawType":"Assignment","title":"Problem Set 5"},`+
		`{"attemptable":true,"columnId":"_7001_1","course":{"code":"SOC101-01-F26","id":"_501_1","name":"Data & Society"},"kind":"test","rawType":"Test","title":"Essay 2"},`+
		`{"attemptable":false,"columnId":"_7002_1","course":{"code":"MATH211-01-F26","id":"_502_1","name":"Linear Algebra"},"kind":"manual","rawType":"Manual Grade Column","title":"Midterm"},`+
		`{"attemptable":true,"columnId":"_7004_1","course":{"code":"ART100-01-F26","id":"_777_1","name":"Drawing"},"kind":"other","rawType":"Discussion","title":"Sketchbook & Notes"}`+
		`]`+"\n", "")
}

func TestDueListCourseFilter(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = map[string]string{calendarDefaultWindow: calendarJSON}
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--course", "soc101", "--course", "math211-01")
	expect(t, code, out, errOut, 0,
		"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\n"+
			"2026-10-20T18:00:00Z\tMATH211-01-F26\tMidterm\tmanual\n", "")

	code, out, errOut = h.run(nil, "due", "list", "--course", "math211")
	expect(t, code, out, errOut, 5, "",
		"bb: course \"math211\" matches 2 courses; use a longer code or the id\n"+
			"  MATH211-01-F26\t_502_1\tLinear Algebra\tFall 2026\n"+
			"  MATH211-02-F26\t_503_1\tLinear Algebra\tFall 2026\n")
}

func TestDueListSplitsWideWindowsAndDedupes(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	item := `{"results":[{"id":"_7010_1","type":"GradebookColumn","calendarId":"_501_1","title":"Final Paper",
		"end":"2026-12-22T00:00:00.000Z","dynamicCalendarItemProps":{"eventType":"Test"}}]}`
	first := "/learn/api/public/v1/calendars/items?since=2026-09-01T00%3A00%3A00Z&until=2026-12-22T00%3A00%3A00Z"
	second := "/learn/api/public/v1/calendars/items?since=2026-12-22T00%3A00%3A00Z&until=2027-01-15T00%3A00%3A00Z"
	h.routes = map[string]string{first: item, second: item}
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--since", "2026-09-01T00:00:00Z", "--until", "2027-01-15T00:00:00Z")
	expect(t, code, out, errOut, 0, "2026-12-22T00:00:00Z\tSOC101-01-F26\tFinal Paper\ttest\n", "")
	if got := strings.Join(h.requests[len(h.requests)-2:], "\n"); got != first+"\n"+second {
		t.Errorf("calendar requests:\n%s\nwant:\n%s\n%s", got, first, second)
	}
}

func TestDueListBadWindowIsUsage(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	code, out, errOut := h.run(nil, "due", "list", "--since", "tomorrow")
	expect(t, code, out, errOut, 2, "", "bb: invalid time \"tomorrow\": use RFC 3339, YYYY-MM-DD, or an offset like -1d, 14d, 36h\n")
	code, out, errOut = h.run(nil, "due", "list", "--since", "2d", "--until", "36h")
	expect(t, code, out, errOut, 2, "", "bb: --until must be after --since\n")
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want none", h.requests)
	}
}
