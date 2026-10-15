package bb

import (
	"errors"
	"fmt"
	"strings"
)

// Kind classifies failures by what the caller should do next. The CLI maps
// Kind (plus auth.ErrNeedsHuman) to exit codes in exactly one switch.
type Kind int

const (
	KindOther       Kind = iota
	KindNotFound         // bad id, or course arg matched nothing
	KindAmbiguous        // course arg matched several; Candidates set
	KindForbidden        // 403: e.g. the gradebook of a dropped course
	KindUnavailable      // 5xx, 429, network: retry later
)

type APIError struct {
	Kind       Kind
	Status     int      // HTTP status, 0 if not an HTTP failure
	Message    string   // Blackboard's message, or ours
	Candidates []Course // only for KindAmbiguous
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
	}
	return e.Message
}

var ErrWriteRefused = errors.New("write request refused; bb is read-only")

// ErrRequestCap is returned once a process has used its --max-requests.
var ErrRequestCap = errors.New("request cap reached; raise it with --max-requests if this walk really needs more")

// CourseErrors lists courses whose gradebook could not be read. A method
// returns it next to a result that is complete for every other course.
type CourseErrors []CourseError

type CourseError struct {
	Course Course
	Err    *APIError // KindForbidden or KindNotFound
}

func (e CourseErrors) Error() string {
	parts := make([]string, len(e))
	for i, ce := range e {
		parts[i] = fmt.Sprintf("%s (%v)", ce.Course.Code, ce.Err)
	}
	return "gradebook unreadable for " + strings.Join(parts, ", ")
}

// IsPartial reports whether err is a CourseErrors, so the result returned
// with it is worth printing.
func IsPartial(err error) bool {
	var ce CourseErrors
	return errors.As(err, &ce)
}

// courseError returns err as a per-course failure when it means "this
// course's gradebook is closed to you": 403 or 404. Anything else (login,
// request cap, 5xx) would fail every later course too, so callers abort.
func courseError(c Course, err error) (CourseError, bool) {
	var ae *APIError
	if errors.As(err, &ae) && (ae.Kind == KindForbidden || ae.Kind == KindNotFound) {
		return CourseError{Course: c, Err: ae}, true
	}
	return CourseError{}, false
}
