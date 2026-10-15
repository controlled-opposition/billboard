package bb

import "context"

// GradeStatus is where one gradable item stands for the signed-in user.
// gradeStatus is the only place it is derived, for due items and grade rows
// alike, so the two commands cannot disagree about what "submitted" means.
type GradeStatus string

const (
	GradeGraded      GradeStatus = "graded"      // graded, or a score or display grade exists
	GradeSubmitted   GradeStatus = "submitted"   // received, not graded yet
	GradeUnsubmitted GradeStatus = "unsubmitted" // submittable online and nothing received
	GradeNone        GradeStatus = "none"        // nothing to submit online and no grade yet
	GradeExempt      GradeStatus = "exempt"
	GradeUnknown     GradeStatus = "unknown" // due items only: the course's gradebook could not be read
)

// GradeStatuses is every GradeStatus in display order; help text is built from it.
var GradeStatuses = []GradeStatus{GradeGraded, GradeSubmitted, GradeUnsubmitted, GradeNone, GradeExempt, GradeUnknown}

// Pending reports whether the item may still need the user's work. An
// unknown status counts, because a filter must not hide what it could not read.
func (s GradeStatus) Pending() bool { return s == GradeUnsubmitted || s == GradeUnknown }

// wireGrade is one row of gradebook/users/me. A column the user has no
// record for is absent from the response.
type wireGrade struct {
	ColumnID string   `json:"columnId"`
	Status   string   `json:"status"` // "Graded", "NeedsGrading", or absent
	Score    *float64 `json:"score"`
	Text     string   `json:"text"`
	Feedback string   `json:"feedback"`
	Exempt   bool     `json:"exempt"`
}

// gradeStatus is pure. rec is nil when the user has no record for the column.
func gradeStatus(rec *wireGrade, submittable bool) GradeStatus {
	switch {
	case rec != nil && rec.Exempt:
		return GradeExempt
	case rec != nil && rec.Status == "NeedsGrading":
		return GradeSubmitted
	case rec != nil && (rec.Status == "Graded" || rec.Score != nil || rec.Text != ""):
		return GradeGraded
	case submittable:
		return GradeUnsubmitted
	}
	return GradeNone
}

// myGrades reads GET /learn/api/public/v1/courses/{id}/gradebook/users/me,
// keyed by column. A course with no records answers 200 with no results.
func (c *Client) myGrades(ctx context.Context, id CourseID) (map[ColumnID]wireGrade, error) {
	recs, err := getPaged[wireGrade](ctx, c, "/learn/api/public/v1/courses/"+string(id)+"/gradebook/users/me")
	if err != nil {
		return nil, err
	}
	out := make(map[ColumnID]wireGrade, len(recs))
	for _, r := range recs {
		out[ColumnID(r.ColumnID)] = r
	}
	return out, nil
}

// lookup returns a pointer to recs[id], or nil when there is no record.
func lookup(recs map[ColumnID]wireGrade, id ColumnID) *wireGrade {
	if r, ok := recs[id]; ok {
		return &r
	}
	return nil
}
