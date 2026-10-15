package cli_test

import (
	"maps"
	"net/http"
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

func myGradesURI(courseID string) string {
	return "/learn/api/public/v1/courses/" + courseID + "/gradebook/users/me"
}

// dueRoutes answers the calendar and every course's gradebook: Problem Set 5
// waits for grading, Essay 2 has no record, the Midterm has no record and no
// online attempt, and the sketchbook is graded.
func dueRoutes() map[string]string {
	return map[string]string{
		calendarDefaultWindow: calendarJSON,
		myGradesURI("_503_1"): `{"results":[{"userId":"_901_1","columnId":"_7003_1","status":"NeedsGrading","exempt":false}]}`,
		myGradesURI("_501_1"): `{"results":[{"userId":"_901_1","columnId":"_6999_1","status":"Graded","score":9.0,"exempt":false}]}`,
		myGradesURI("_502_1"): `{"results":[]}`,
		myGradesURI("_777_1"): `{"results":[{"userId":"_901_1","columnId":"_7004_1","status":"Graded","score":8.5,"exempt":false}]}`,
	}
}

var coursesRequests = []string{
	"/learn/api/public/v1/users/me",
	"/learn/api/public/v1/users/_901_1/courses?expand=course",
	"/learn/api/public/v1/users/_901_1/courses?expand=course&offset=2",
	"/learn/api/public/v1/terms",
}

func expectRequests(t *testing.T, h *harness, want ...string) {
	t.Helper()
	if got := strings.Join(h.requests, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestDueListStatusKindsAndOrder(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.login()
	code, out, errOut := h.run(nil, "due", "list")
	expect(t, code, out, errOut, 0,
		"2026-10-15T03:59:00Z\tMATH211-02-F26\tProblem Set 5\tassignment\tsubmitted\n"+
			"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\tunsubmitted\n"+
			"2026-10-20T18:00:00Z\tMATH211-01-F26\tMidterm\tmanual\tnone\n"+
			"2026-10-22T03:59:00Z\tART100-01-F26\tSketchbook & Notes\tother\tgraded\n", "")
	expectRequests(t, h, append(coursesRequests,
		calendarDefaultWindow,
		myGradesURI("_503_1"),
		myGradesURI("_501_1"),
		myGradesURI("_502_1"),
		myGradesURI("_777_1"),
	)...)
}

func TestDueListJSONCarriesCourseObject(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--json", "title,course,kind,rawType,attemptable,columnId,status")
	expect(t, code, out, errOut, 0, `[`+
		`{"attemptable":true,"columnId":"_7003_1","course":{"code":"MATH211-02-F26","id":"_503_1","name":"Linear Algebra"},"kind":"assignment","rawType":"Assignment","status":"submitted","title":"Problem Set 5"},`+
		`{"attemptable":true,"columnId":"_7001_1","course":{"code":"SOC101-01-F26","id":"_501_1","name":"Data & Society"},"kind":"test","rawType":"Test","status":"unsubmitted","title":"Essay 2"},`+
		`{"attemptable":false,"columnId":"_7002_1","course":{"code":"MATH211-01-F26","id":"_502_1","name":"Linear Algebra"},"kind":"manual","rawType":"Manual Grade Column","status":"none","title":"Midterm"},`+
		`{"attemptable":true,"columnId":"_7004_1","course":{"code":"ART100-01-F26","id":"_777_1","name":"Drawing"},"kind":"other","rawType":"Discussion","status":"graded","title":"Sketchbook & Notes"}`+
		`]`+"\n", "")
}

func TestDueListUnreadableGradebookIsPartial(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.fail = map[string]int{myGradesURI("_501_1"): http.StatusForbidden}
	h.login()
	code, out, errOut := h.run(nil, "due", "list")
	expect(t, code, out, errOut, 6,
		"2026-10-15T03:59:00Z\tMATH211-02-F26\tProblem Set 5\tassignment\tsubmitted\n"+
			"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\tunknown\n"+
			"2026-10-20T18:00:00Z\tMATH211-01-F26\tMidterm\tmanual\tnone\n"+
			"2026-10-22T03:59:00Z\tART100-01-F26\tSketchbook & Notes\tother\tgraded\n",
		"bb: gradebook unreadable for SOC101-01-F26 (HTTP 403: You do not have permission to access grade column)\n")
}

func TestDueListPendingKeepsUnsubmittedAndUnknown(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--pending")
	expect(t, code, out, errOut, 0, "2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\tunsubmitted\n", "")

	h.fail = map[string]int{myGradesURI("_503_1"): http.StatusNotFound}
	code, out, errOut = h.run(nil, "due", "list", "--pending")
	expect(t, code, out, errOut, 6,
		"2026-10-15T03:59:00Z\tMATH211-02-F26\tProblem Set 5\tassignment\tunknown\n"+
			"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\tunsubmitted\n",
		"bb: gradebook unreadable for MATH211-02-F26 (HTTP 404: Not Found)\n")
}

func TestDueListGradebookLoginFailureAborts(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.fail = map[string]int{myGradesURI("_501_1"): http.StatusUnauthorized}
	h.login()
	code, out, errOut := h.run(nil, "due", "list")
	expect(t, code, out, errOut, 4, "",
		"bb: headless renewal failed: sign-in page asks for credentials or MFA: blackboard login required: a human must run `bb auth login`\n"+
			"bb: ask the human to run `bb auth login`, then retry\n")

	h.fail = map[string]int{myGradesURI("_501_1"): http.StatusServiceUnavailable}
	code, out, errOut = h.run(nil, "due", "list")
	expect(t, code, out, errOut, 7, "", "bb: HTTP 503: Service Unavailable\n")
}

func TestDueListCourseFilter(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = dueRoutes()
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--course", "soc101", "--course", "math211-01")
	expect(t, code, out, errOut, 0,
		"2026-10-15T03:59:00Z\tSOC101-01-F26\tEssay 2\ttest\tunsubmitted\n"+
			"2026-10-20T18:00:00Z\tMATH211-01-F26\tMidterm\tmanual\tnone\n", "")
	expectRequests(t, h, append(coursesRequests, calendarDefaultWindow, myGradesURI("_501_1"), myGradesURI("_502_1"))...)

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
	maps.Copy(h.routes, dueRoutes())
	h.login()
	code, out, errOut := h.run(nil, "due", "list", "--since", "2026-09-01T00:00:00Z", "--until", "2027-01-15T00:00:00Z")
	expect(t, code, out, errOut, 0, "2026-12-22T00:00:00Z\tSOC101-01-F26\tFinal Paper\ttest\tnone\n", "")
	expectRequests(t, h, append(coursesRequests, first, second, myGradesURI("_501_1"))...)
}

func TestDueListUsageErrorsSendNothing(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	code, out, errOut := h.run(nil, "due", "list", "--since", "tomorrow")
	expect(t, code, out, errOut, 2, "", "bb: invalid time \"tomorrow\": use RFC 3339, YYYY-MM-DD, or an offset like -1d, 14d, 36h\n")
	code, out, errOut = h.run(nil, "due", "list", "--since", "2d", "--until", "36h")
	expect(t, code, out, errOut, 2, "", "bb: --until must be after --since\n")
	code, out, errOut = h.run(nil, "due", "list", "--pending=maybe")
	expect(t, code, out, errOut, 2, "", "bb: invalid argument \"maybe\" for \"--pending\" flag: strconv.ParseBool: parsing \"maybe\": invalid syntax\n")
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want none", h.requests)
	}
}
