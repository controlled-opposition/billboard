package cli_test

import (
	"net/http"
	"testing"
)

func columnsURI(courseID string) string {
	return "/learn/api/public/v2/courses/" + courseID + "/gradebook/columns"
}

const (
	linearColumnsPage1 = `{"results":[
		{"id":"_9001_1","name":"Problem Set 3","grading":{"type":"Attempts","due":"2026-10-01T03:59:00.000Z","attemptsAllowed":2},
		 "score":{"possible":10.00000},"availability":{"available":"Yes"},"includeInCalculations":true},
		{"id":"_9002_1","name":"Problem Set 4","grading":{"type":"Attempts","due":"2026-10-08T03:59:00.000Z"},
		 "score":{"possible":10.00000},"availability":{"available":"Yes"},"includeInCalculations":true},
		{"id":"_9003_1","name":"Problem Set 5","grading":{"type":"Attempts","due":"2026-10-15T03:59:00.000Z"},
		 "score":{"possible":10.00000},"availability":{"available":"Yes"},"includeInCalculations":true},
		{"id":"_9004_1","name":"Midterm","grading":{"type":"Manual","due":"2026-10-20T18:00:00.000Z"},
		 "score":{"possible":100.00000},"availability":{"available":"Yes"},"includeInCalculations":true},
		{"id":"_9005_1","name":"Participation","grading":{"type":"Manual"},
		 "score":{"possible":5.00000},"availability":{"available":"Yes"},"includeInCalculations":true},
		{"id":"_9006_1","name":"Quiz 1","grading":{"type":"Attempts","due":"2026-09-20T03:59:00.000Z"},
		 "score":{"possible":5.00000},"availability":{"available":"Yes"},"includeInCalculations":true}
	],"paging":{"nextPage":"/learn/api/public/v2/courses/_502_1/gradebook/columns?offset=6"}}`

	linearColumnsPage2 = `{"results":[
		{"id":"_9007_1","name":"Draft Notes","grading":{"type":"Attempts","due":"2026-10-02T03:59:00.000Z"},
		 "score":{"possible":5.00000},"availability":{"available":"No"},"includeInCalculations":true},
		{"id":"_9008_1","name":"Overall Grade","grading":{"type":"Calculated"},
		 "score":{"possible":100.00000},"availability":{"available":"Yes"},"includeInCalculations":false},
		{"id":"_9009_1","name":"Peer Review","grading":{"type":"Delegated","due":"2026-10-25T03:59:00.000Z"},
		 "score":{"possible":10.00000},"availability":{"available":"Yes"},"includeInCalculations":true}
	]}`

	// Problem Set 5 and the Midterm have no record: nothing submitted.
	linearGrades = `{"results":[
		{"userId":"_901_1","columnId":"_9001_1","status":"Graded","score":4.0,"feedback":"<p>Show the row reduction &amp; check signs.</p><p>See <b>Theorem</b> 2.</p>","exempt":false},
		{"userId":"_901_1","columnId":"_9002_1","status":"NeedsGrading","exempt":false},
		{"userId":"_901_1","columnId":"_9005_1","exempt":true},
		{"userId":"_901_1","columnId":"_9006_1","status":"Graded","exempt":false},
		{"userId":"_901_1","columnId":"_9007_1","status":"Graded","score":5.0,"exempt":false},
		{"userId":"_901_1","columnId":"_9008_1","text":"A-","score":93.0,"exempt":false},
		{"userId":"_901_1","columnId":"_9009_1","status":"InProgress","exempt":false}
	]}`
)

func gradeRoutes() map[string]string {
	return map[string]string{
		columnsURI("_502_1"):               linearColumnsPage1,
		columnsURI("_502_1") + "?offset=6": linearColumnsPage2,
		myGradesURI("_502_1"):              linearGrades,
		columnsURI("_503_1"):               `{"results":[]}`,
		myGradesURI("_503_1"):              `{"results":[]}`,
		columnsURI("_501_1"): `{"results":[{"id":"_9101_1","name":"Essay 1","grading":{"type":"Attempts","due":"2026-10-01T03:59:00.000Z"},
			"score":{"possible":20.00000},"availability":{"available":"Yes"},"includeInCalculations":true}]}`,
		myGradesURI("_501_1"): `{"results":[{"userId":"_901_1","columnId":"_9101_1","status":"Graded","score":18.0,"exempt":false}]}`,
	}
}

const linearTSV = "MATH211-01-F26\tQuiz 1\tgraded\t\t5\t\n" +
	"MATH211-01-F26\tProblem Set 3\tgraded\t4\t10\t\n" +
	"MATH211-01-F26\tProblem Set 4\tsubmitted\t\t10\t\n" +
	"MATH211-01-F26\tProblem Set 5\tunsubmitted\t\t10\t\n" +
	"MATH211-01-F26\tMidterm\tnone\t\t100\t\n" +
	"MATH211-01-F26\tPeer Review\tnone\t\t10\t\n" +
	"MATH211-01-F26\tOverall Grade\tgraded\t93\t100\tA-\n" +
	"MATH211-01-F26\tParticipation\texempt\t\t5\t\n"

func TestGradeListCurrentCourses(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = gradeRoutes()
	h.login()
	code, out, errOut := h.run(nil, "grade", "list")
	expect(t, code, out, errOut, 0, linearTSV+"SOC101-01-F26\tEssay 1\tgraded\t18\t20\t\n", "")
	expectRequests(t, h, append(coursesRequests,
		columnsURI("_502_1"),
		columnsURI("_502_1")+"?offset=6",
		myGradesURI("_502_1"),
		columnsURI("_503_1"),
		myGradesURI("_503_1"),
		columnsURI("_501_1"),
		myGradesURI("_501_1"),
	)...)
}

func TestGradeListJSON(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = gradeRoutes()
	h.login()
	code, out, errOut := h.run(nil, "grade", "list", "math211-01",
		"--json", "course,title,status,score,possible,display,feedback,due,kind,counted,columnId",
		"--jq", `.[] | select(.title == "Problem Set 3" or .title == "Overall Grade" or .title == "Peer Review")`)
	expect(t, code, out, errOut, 0,
		`{"columnId":"_9001_1","counted":true,"course":{"code":"MATH211-01-F26","id":"_502_1","name":"Linear Algebra"},"display":"","due":"2026-10-01T03:59:00Z","feedback":"Show the row reduction `+"\x5cu0026"+` check signs. See Theorem 2.","kind":"attempts","possible":10,"score":4,"status":"graded","title":"Problem Set 3"}`+"\n"+
			`{"columnId":"_9009_1","counted":true,"course":{"code":"MATH211-01-F26","id":"_502_1","name":"Linear Algebra"},"display":"","due":"2026-10-25T03:59:00Z","feedback":"","kind":"other","possible":10,"score":null,"status":"none","title":"Peer Review"}`+"\n"+
			`{"columnId":"_9008_1","counted":false,"course":{"code":"MATH211-01-F26","id":"_502_1","name":"Linear Algebra"},"display":"A-","due":null,"feedback":"","kind":"calculated","possible":100,"score":93,"status":"graded","title":"Overall Grade"}`+"\n", "")
}

func TestGradeListPartialAndAllFailed(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = gradeRoutes()
	h.fail = map[string]int{columnsURI("_503_1"): http.StatusForbidden}
	h.login()
	code, out, errOut := h.run(nil, "grade", "list")
	expect(t, code, out, errOut, 6, linearTSV+"SOC101-01-F26\tEssay 1\tgraded\t18\t20\t\n",
		"bb: gradebook unreadable for MATH211-02-F26 (HTTP 403: You do not have permission to access grade column)\n")

	code, out, errOut = h.run(nil, "grade", "list", "math211-02")
	expect(t, code, out, errOut, 3, "", "bb: HTTP 403: You do not have permission to access grade column\n")

	h.fail[myGradesURI("_501_1")] = http.StatusNotFound
	code, out, errOut = h.run(nil, "grade", "list", "soc101", "math211-02")
	expect(t, code, out, errOut, 5, "", "bb: HTTP 404: Not Found\n")
}

func TestGradeListAbortsOnUnavailable(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = gradeRoutes()
	h.fail = map[string]int{myGradesURI("_503_1"): http.StatusBadGateway}
	h.login()
	code, out, errOut := h.run(nil, "grade", "list")
	expect(t, code, out, errOut, 7, "", "bb: HTTP 502: Bad Gateway\n")
	expectRequests(t, h, append(coursesRequests,
		columnsURI("_502_1"),
		columnsURI("_502_1")+"?offset=6",
		myGradesURI("_502_1"),
		columnsURI("_503_1"),
		myGradesURI("_503_1"),
	)...)
}
