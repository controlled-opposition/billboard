package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"

	"github.com/itchyny/gojq"
)

// JQ is a compiled --jq expression.
type JQ struct{ code *gojq.Code }

// ParseJQ compiles expr, so a malformed expression fails before any request.
func ParseJQ(expr string) (*JQ, error) {
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", expr, err)
	}
	code, err := gojq.Compile(q, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", expr, err)
	}
	return &JQ{code}, nil
}

// Run applies q to the JSON document in r and writes one line per result.
// Strings, numbers, booleans, and null print raw, as `jq -r` does (null as
// an empty line). Arrays and objects print as one-line JSON with & < >
// left as they are and numbers at full precision.
func (q *JQ) Run(r io.Reader, w io.Writer) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var in any
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	iter := q.code.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, isErr := v.(error); isErr {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				return nil
			}
			return err
		}
		if s, ok := raw(v); ok {
			if _, err := fmt.Fprintln(w, s); err != nil {
				return err
			}
			continue
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
}

func raw(v any) (string, bool) {
	switch v := v.(type) {
	case nil:
		return "", true
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.Itoa(v), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case *big.Int:
		return v.String(), true
	}
	return "", false
}
