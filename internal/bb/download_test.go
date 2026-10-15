package bb

import (
	"reflect"
	"strings"
	"testing"
)

func relPaths(ps []Planned) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.RelPath)
	}
	return out
}

func TestPlanSuffixesNamesThatDifferOnlyByCase(t *testing.T) {
	nodes := []ContentNode{{Path: []string{"Week 1"}, Attachments: []Attachment{
		{Name: "Notes.pdf", RID: "7001_1", path: "/bbcswebdav/pid-1-dt-content-rid-7001_1/xid-7001_1"},
		{Name: "notes.pdf", RID: "7002_1", path: "/bbcswebdav/pid-1-dt-content-rid-7002_1/xid-7002_1"},
	}}}
	got := relPaths(Plan(Course{Code: "HIST200-02-F26"}, nodes))
	want := []string{
		"HIST200-02-F26/Week 1/Notes (7001_1).pdf",
		"HIST200-02-F26/Week 1/notes (7002_1).pdf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RelPaths = %q, want %q", got, want)
	}
}

func TestSegmentTrimsLongNamesBeforeTheExtension(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"ascii", strings.Repeat("x", 250) + ".pdf", strings.Repeat("x", 196) + ".pdf"},
		{"multibyte stem", strings.Repeat("日", 90) + ".docx", strings.Repeat("日", 65) + ".docx"},
	}
	for _, c := range cases {
		if got := segment(c.in); got != c.want {
			t.Errorf("%s: segment = %q (%d bytes), want %q (%d bytes)", c.name, got, len(got), c.want, len(c.want))
		}
	}
}
