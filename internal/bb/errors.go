package bb

import "errors"

// Kind classifies failures by what the caller should do next. The CLI maps
// Kind (plus auth.ErrNeedsHuman) to exit codes in exactly one switch.
type Kind int

const (
	KindOther       Kind = iota
	KindNotFound         // bad id, or course arg matched nothing
	KindAmbiguous        // course arg matched several; Candidates set
	KindForbidden        // 403: e.g. gradebook columns in some courses
	KindUnavailable      // 5xx, 429, network: retry later
)

type APIError struct {
	Kind       Kind
	Status     int      // HTTP status, 0 if not an HTTP failure
	Message    string   // Blackboard's message, or ours
	Candidates []Course // only for KindAmbiguous
}

func (e *APIError) Error() string { panic("not implemented") }

var ErrWriteRefused = errors.New("bb: write request refused; client is read-only")
