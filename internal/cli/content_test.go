package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic SOC101 outline (course _501_1, ROOT container _600_1):
//
//	Week 1                      folder
//	  Reading: Smith & Jones    Ultra document, two files
//	  Empty                     folder without children
//	Syllabus                    Ultra document, one file
//	Quiz 1                      test link
//	Hidden                      unavailable folder
const contents = "/learn/api/public/v1/courses/_501_1/contents"

type item map[string]any

func content(id, parent, title, handler string, pos int, extra item) item {
	it := item{
		"id": id, "parentId": parent, "title": title, "position": pos,
		"contentHandler": item{"id": handler}, "availability": item{"available": "Yes"},
		"modified": "2026-09-13T15:34:42.502Z",
		"links":    []item{{"href": "/ultra/redirect?redirectType=nautilus&courseId=_501_1&contentId=" + id, "rel": "alternate"}},
	}
	if handler == "resource/x-bb-folder" {
		it["hasChildren"] = true
	}
	for k, v := range extra {
		it[k] = v
	}
	if parent == "" {
		delete(it, "parentId")
	}
	return it
}

func body(id, parent, html string) item {
	return content(id, parent, "ultraDocumentBody", "resource/x-bb-document", 0,
		item{"body": html, "modified": "2026-09-01T10:00:00.123Z"})
}

func results(items ...item) string {
	b, _ := json.Marshal(item{"results": items})
	return string(b)
}

func one(it item) string {
	b, _ := json.Marshal(it)
	return string(b)
}

const (
	smithPath = "/bbcswebdav/pid-6111-dt-content-rid-9001_1/xid-9001_1"
	notesPath = "/bbcswebdav/pid-6111-dt-content-rid-9002_1/xid-9002_1"
	syllPath  = "/bbcswebdav/pid-6021-dt-content-rid-9003_1/xid-9003_1"
	smithPDF  = "%PDF-smith-and-jones"
	notesTxt  = "draft notes"
	syllPDF   = "%PDF-syllabus"
)

// readingBody links two files the way Ultra does: data-bbfile is
// entity-escaped JSON whose resourceUrl carries an expiring signature, and
// the second link has no resourceUrl, so its href is used.
var readingBody = `<p><a href="https://blackboard.wm.edu` + smithPath + `?sig=abc&amp;exp=1792020616" data-bbtype="attachment" ` +
	`data-bbfile="{&quot;fileName&quot;:&quot;Smith &amp; Jones.pdf&quot;,&quot;fileSize&quot;:20,&quot;mimeType&quot;:&quot;application/pdf&quot;,` +
	`&quot;resourceUrl&quot;:&quot;https://blackboard.wm.edu` + smithPath + `?sig=abc&amp;exp=1792020616&quot;}">Smith &amp; Jones.pdf</a></p>` +
	`<p><a href="https://example.org/elsewhere">not a file</a></p>` +
	`<p><a href="https://blackboard.wm.edu` + notesPath + `?sig=def" data-bbtype="attachment" ` +
	`data-bbfile="{&quot;fileName&quot;:&quot;Notes: draft/v2.txt&quot;,&quot;fileSize&quot;:11,&quot;mimeType&quot;:&quot;text/plain&quot;}">notes</a></p>`

// syllabusBody uses the older link shape: no data-bbtype, no fileName, and
// no fileSize, so only the mtime can prove a local copy current.
var syllabusBody = `<a data-bbfile="{&quot;displayName&quot;:&quot;syllabus.pdf&quot;,&quot;mimeType&quot;:&quot;application/pdf&quot;,` +
	`&quot;resourceUrl&quot;:&quot;` + syllPath + `?x=1&quot;}">syllabus</a>`

func outlineRoutes() map[string]string {
	week1 := content("_601_1", "_600_1", "Week 1", "resource/x-bb-folder", 0, nil)
	reading := content("_611_1", "_601_1", "Reading: Smith & Jones", "resource/x-bb-folder", 0, nil)
	return map[string]string{
		contents: results(
			content("_603_1", "_600_1", "Quiz 1", "resource/x-bb-asmt-test-link", 2, nil),
			week1,
			content("_602_1", "_600_1", "Syllabus", "resource/x-bb-folder", 1, nil),
			content("_604_1", "_600_1", "Hidden", "resource/x-bb-folder", 3, item{"availability": item{"available": "No"}}),
		),
		contents + "/_601_1/children": results(
			reading,
			content("_612_1", "_601_1", "Empty", "resource/x-bb-folder", 1, item{"hasChildren": false}),
		),
		contents + "/_611_1/children": results(body("_6111_1", "_611_1", readingBody)),
		contents + "/_602_1/children": results(body("_6021_1", "_602_1", syllabusBody)),
		contents + "/_611_1":          one(reading),
		contents + "/_601_1":          one(week1),
		contents + "/_600_1":          one(content("_600_1", "", "ROOT", "resource/x-bb-folder", 0, nil)),
		smithPath:                     smithPDF,
		notesPath:                     notesTxt,
		syllPath:                      syllPDF,
	}
}

func (h *harness) fileRequests() []string {
	var out []string
	for _, r := range h.requests {
		if strings.HasPrefix(r, "/bbcswebdav/") {
			out = append(out, r)
		}
	}
	return out
}

func TestContentListFoldsDocumentsAndCountsFiles(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	h.login()
	code, out, errOut := h.run(nil, "content", "list", "soc101")
	expect(t, code, out, errOut, 0,
		"_601_1\tfolder\tWeek 1\t0\n"+
			"_611_1\tdocument\tWeek 1 / Reading: Smith & Jones\t2\n"+
			"_612_1\tfolder\tWeek 1 / Empty\t0\n"+
			"_602_1\tdocument\tSyllabus\t1\n"+
			"_603_1\ttest\tQuiz 1\t0\n"+
			"_604_1\tfolder\tHidden\t0\n", "")
	walk := h.requests[4:]
	want := []string{contents, contents + "/_601_1/children", contents + "/_611_1/children", contents + "/_602_1/children"}
	if strings.Join(walk, "\n") != strings.Join(want, "\n") {
		t.Errorf("walk requests:\n%s\nwant:\n%s", strings.Join(walk, "\n"), strings.Join(want, "\n"))
	}
}

func TestContentListSubtreeExtractsEscapedFiles(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	h.login()
	code, out, errOut := h.run(nil, "content", "list", "soc101", "_611_1", "--json", "id,kind,path,depth,modified,url,files")
	expect(t, code, out, errOut, 0,
		`[{"depth":1,"files":[`+
			`{"name":"Smith & Jones.pdf","size":20,"mimeType":"application/pdf"},`+
			`{"name":"Notes: draft/v2.txt","size":11,"mimeType":"text/plain"}],`+
			`"id":"_611_1","kind":"document","modified":"2026-09-13T15:34:42.502Z","path":["Week 1","Reading: Smith & Jones"],`+
			`"url":"`+h.srv.URL+`/ultra/redirect?redirectType=nautilus&courseId=_501_1&contentId=_611_1"}]`+"\n", "")
}

func TestContentDownloadSubtreeThenRerunSkips(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	h.login()
	dir := t.TempDir()
	docDir := filepath.Join(dir, "SOC101-01-F26", "Week 1", "Reading_ Smith & Jones")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A crashed earlier run left a partial file behind.
	if err := os.WriteFile(filepath.Join(docDir, "Smith & Jones.pdf.part"), []byte("%PDF-trunc"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := h.run(nil, "content", "download", "soc101", "_611_1", "--dir", dir)
	expect(t, code, out, errOut, 0,
		"downloaded\t"+filepath.Join(docDir, "Smith & Jones.pdf")+"\n"+
			"downloaded\t"+filepath.Join(docDir, "Notes_ draft_v2.txt")+"\n", "")
	entries, _ := os.ReadDir(docDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, ","); got != "Notes_ draft_v2.txt,Smith & Jones.pdf" {
		t.Errorf("files on disk = %s, want only the two downloads", got)
	}
	if b, _ := os.ReadFile(filepath.Join(docDir, "Smith & Jones.pdf")); string(b) != smithPDF {
		t.Errorf("Smith & Jones.pdf = %q, want %q", b, smithPDF)
	}
	st, _ := os.Stat(filepath.Join(docDir, "Notes_ draft_v2.txt"))
	if want := time.Date(2026, 9, 1, 10, 0, 0, 123e6, time.UTC); !st.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want the body's modified %v", st.ModTime().UTC(), want)
	}
	if got := strings.Join(h.fileRequests(), ","); got != smithPath+","+notesPath {
		t.Errorf("file requests = %s, want unsigned paths %s,%s", got, smithPath, notesPath)
	}

	before := len(h.requests)
	code, out, errOut = h.run(nil, "content", "download", "soc101", "_611_1", "--dir", dir)
	expect(t, code, out, errOut, 0,
		"skipped\t"+filepath.Join(docDir, "Smith & Jones.pdf")+"\n"+
			"skipped\t"+filepath.Join(docDir, "Notes_ draft_v2.txt")+"\n", "")
	for _, r := range h.requests[before:] {
		if strings.HasPrefix(r, "/bbcswebdav/") {
			t.Errorf("rerun requested %s, want no download requests", r)
		}
	}
}

func TestContentDownloadSizeIsBytesOnDisk(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	h.routes[contents+"/_602_1"] = one(content("_602_1", "_600_1", "Syllabus", "resource/x-bb-folder", 1, nil))
	h.login()
	dir := t.TempDir()
	want := `[{"name":"syllabus.pdf","size":13,"status":"%s"}]` + "\n"

	code, out, errOut := h.run(nil, "content", "download", "soc101", "_602_1", "--dir", dir, "--json", "status,name,size")
	expect(t, code, out, errOut, 0, fmt.Sprintf(want, "downloaded"), "")

	code, out, errOut = h.run(nil, "content", "download", "soc101", "_602_1", "--dir", dir, "--json", "status,name,size")
	expect(t, code, out, errOut, 0, fmt.Sprintf(want, "skipped"), "")
}

func TestContentDownloadSubtreePathMatchesFullWalk(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	h.login()
	dir := t.TempDir()
	reading := filepath.Join(dir, "SOC101-01-F26", "Week 1", "Reading_ Smith & Jones")

	code, out, errOut := h.run(nil, "content", "download", "soc101", "--all", "--dir", dir, "--dry-run")
	expect(t, code, out, errOut, 0,
		"planned\t"+filepath.Join(reading, "Smith & Jones.pdf")+"\n"+
			"planned\t"+filepath.Join(reading, "Notes_ draft_v2.txt")+"\n"+
			"planned\t"+filepath.Join(dir, "SOC101-01-F26", "Syllabus", "syllabus.pdf")+"\n", "")

	code, out, errOut = h.run(nil, "content", "download", "soc101", "_611_1", "--dir", dir, "--dry-run")
	expect(t, code, out, errOut, 0,
		"planned\t"+filepath.Join(reading, "Smith & Jones.pdf")+"\n"+
			"planned\t"+filepath.Join(reading, "Notes_ draft_v2.txt")+"\n", "")
	if got := h.fileRequests(); len(got) != 0 {
		t.Errorf("dry run requested %v, want no downloads", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "SOC101-01-F26")); !os.IsNotExist(err) {
		t.Errorf("dry run created files: %v", err)
	}
}

func TestContentDownloadPartialFailureExitsSix(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.routes = outlineRoutes()
	delete(h.routes, notesPath)
	h.login()
	dir := t.TempDir()
	reading := filepath.Join(dir, "SOC101-01-F26", "Week 1", "Reading_ Smith & Jones")
	code, out, errOut := h.run(nil, "content", "download", "soc101", "--all", "--dir", dir, "--json", "status,name,error")
	expect(t, code, out, errOut, 6,
		`[{"error":"","name":"Smith & Jones.pdf","status":"downloaded"},`+
			`{"error":"HTTP 404: Not found","name":"Notes: draft/v2.txt","status":"failed"},`+
			`{"error":"","name":"syllabus.pdf","status":"downloaded"}]`+"\n",
		"bb: 1 of 3 files failed; see --json path,error: some items failed\n")
	if _, err := os.Stat(filepath.Join(reading, "Notes_ draft_v2.txt.part")); !os.IsNotExist(err) {
		t.Errorf("failed download left a .part file: %v", err)
	}

	code, out, errOut = h.run(nil, "content", "download", "soc101", "--all", "--dir", dir, "--dry-run")
	expect(t, code, out, errOut, 0,
		"skipped\t"+filepath.Join(reading, "Smith & Jones.pdf")+"\n"+
			"planned\t"+filepath.Join(reading, "Notes_ draft_v2.txt")+"\n"+
			"skipped\t"+filepath.Join(dir, "SOC101-01-F26", "Syllabus", "syllabus.pdf")+"\n", "")
}

func TestContentDownloadUsageErrorsSendNothing(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	cases := []struct {
		args []string
		err  string
	}{
		{[]string{"content", "download", "soc101"}, "bb: name content ids from `bb content list`, or pass --all to walk the whole course (about 60 requests)\n"},
		{[]string{"content", "download", "soc101", "_611_1", "--all"}, "bb: --all takes no content ids\n"},
		{[]string{"content", "download", "soc101", "week1"}, "bb: invalid content id \"week1\": expected the form _123_1 from `bb content list`\n"},
		{[]string{"content", "list", "soc101", "_1_1", "_2_1"}, "bb: accepts between 1 and 2 arg(s), received 3\n"},
	}
	for _, c := range cases {
		code, out, errOut := h.run(nil, c.args...)
		expect(t, code, out, errOut, 2, "", c.err)
	}
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want none", h.requests)
	}
}
