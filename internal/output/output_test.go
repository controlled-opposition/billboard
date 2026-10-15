package output_test

import (
	"bytes"
	"testing"

	"github.com/controlled-opposition/billboard/internal/output"
)

type row struct {
	name  string
	score *float64
}

var schema = output.Schema[row]{
	Fields: []output.Field[row]{
		{Name: "name", Value: func(r row) any { return r.name }},
		{Name: "score", Value: func(r row) any {
			if r.score == nil {
				return nil
			}
			return *r.score
		}},
	},
	Default: []string{"name", "score"},
}

func TestNilIsEmptyCellAndJSONNull(t *testing.T) {
	four := 4.0
	rows := []row{{"a", &four}, {"b", nil}}
	cases := []struct {
		opts output.Options
		want string
	}{
		{output.Options{}, "a\t4\nb\t\n"},
		{output.Options{JSON: []string{"name", "score"}}, `[{"name":"a","score":4},{"name":"b","score":null}]` + "\n"},
	}
	for _, c := range cases {
		var out bytes.Buffer
		if err := output.Print(output.Streams{Out: &out, Err: &out}, schema, &c.opts, rows); err != nil {
			t.Fatal(err)
		}
		if out.String() != c.want {
			t.Errorf("got %q, want %q", out.String(), c.want)
		}
	}
}
