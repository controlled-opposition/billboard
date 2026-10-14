package bb

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Course is one enrollment, flattened: membership + course + term name.
type Course struct {
	ID      CourseID // "_12345_1"
	Code    string   // "MATH101-01-F26" (course.courseId)
	Name    string   // HTML-unescaped: "Calculus & Analysis"
	Term    string   // "Fall 2026"; "" when the course has no term
	Role    string   // "Student", "Instructor", ...
	Current bool     // see isCurrent
	Open    bool     // membership AND course availability are both "Yes"
	URL     string   // externalAccessUrl (Ultra outline)
}

// Courses returns every membership (paged users/{id}/courses?expand=course,
// joined with /terms), sorted Current first, then Code. One call costs ~3
// requests; commands that need course names call it once and pass the slice
// around rather than re-fetching.
func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	me, err := c.Me(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := getPaged[wireMembership](ctx, c, "/learn/api/public/v1/users/"+string(me.ID)+"/courses?expand=course")
	if err != nil {
		return nil, err
	}
	terms, err := getPaged[wireTerm](ctx, c, "/learn/api/public/v1/terms")
	if err != nil {
		return nil, err
	}
	return parseCourses(ms, terms, c.now()), nil
}

// ResolveCourse turns a user/agent argument into exactly one course. Accepted
// forms, tried in order:
//  1. internal id "_12345_1"            (exact)
//  2. full code "MATH101-01-F26"         (case-insensitive exact)
//  3. code prefix "math101", "MATH101-01" (case-insensitive), among Current
//     courses first, then all courses
//
// No match -> *APIError{KindNotFound}. Several -> *APIError{KindAmbiguous,
// Candidates}. Never guesses. Pure: the caller supplies the list from
// Courses, so resolving N args costs zero extra requests.
func ResolveCourse(all []Course, arg string) (Course, error) {
	if id, ok := ParseCourseID(arg); ok {
		for _, c := range all {
			if c.ID == id {
				return c, nil
			}
		}
	}
	for _, c := range all {
		if strings.EqualFold(c.Code, arg) {
			return c, nil
		}
	}
	prefix := strings.ToLower(arg)
	for _, pool := range [][]Course{currentOnly(all), all} {
		var hits []Course
		for _, c := range pool {
			if strings.HasPrefix(strings.ToLower(c.Code), prefix) {
				hits = append(hits, c)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		default:
			return Course{}, &APIError{Kind: KindAmbiguous, Candidates: hits,
				Message: fmt.Sprintf("course %q matches %d courses; use a longer code or the id", arg, len(hits))}
		}
	}
	return Course{}, &APIError{Kind: KindNotFound, Message: fmt.Sprintf("no course matches %q; see `bb course list --all`", arg)}
}

func currentOnly(all []Course) []Course {
	var out []Course
	for _, c := range all {
		if c.Current {
			out = append(out, c)
		}
	}
	return out
}

// isCurrent: terms have no dates at W&M (all "Continuous"), so the season is
// parsed from the term name ("Fall 2026") into a date window and checked
// against now. A closed course is never current. A term name that does not
// parse is treated as current if the course is Open (fail visible, not
// silent: the course shows up rather than vanishing).
func isCurrent(termName string, open bool, now time.Time) bool {
	if !open {
		return false
	}
	season, yearStr, ok := strings.Cut(termName, " ")
	year, err := strconv.Atoi(yearStr)
	if !ok || err != nil {
		return true
	}
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	var from, to time.Time
	switch season {
	case "Spring":
		from, to = date(year, time.January, 1), date(year, time.June, 1)
	case "Summer":
		from, to = date(year, time.May, 15), date(year, time.September, 1)
	case "Fall":
		from, to = date(year, time.August, 1), date(year+1, time.January, 1)
	default:
		return true
	}
	return !now.Before(from) && now.Before(to)
}

type wireMembership struct {
	CourseID     string `json:"courseId"`
	CourseRoleID string `json:"courseRoleId"`
	Availability struct {
		Available string `json:"available"`
	} `json:"availability"`
	Course struct {
		ID                string `json:"id"`
		CourseID          string `json:"courseId"`
		Name              string `json:"name"`
		TermID            string `json:"termId"`
		ExternalAccessURL string `json:"externalAccessUrl"`
		Availability      struct {
			Available string `json:"available"`
		} `json:"availability"`
	} `json:"course"`
}

type wireTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// parseCourses is pure: wire -> domain, html.UnescapeString on every name.
func parseCourses(ms []wireMembership, terms []wireTerm, now time.Time) []Course {
	termName := make(map[string]string, len(terms))
	for _, t := range terms {
		termName[t.ID] = html.UnescapeString(t.Name)
	}
	out := make([]Course, 0, len(ms))
	for _, m := range ms {
		term := termName[m.Course.TermID]
		open := m.Availability.Available == "Yes" && m.Course.Availability.Available == "Yes"
		out = append(out, Course{
			ID:      CourseID(m.Course.ID),
			Code:    m.Course.CourseID,
			Name:    html.UnescapeString(m.Course.Name),
			Term:    term,
			Role:    m.CourseRoleID,
			Current: isCurrent(term, open, now),
			Open:    open,
			URL:     m.Course.ExternalAccessURL,
		})
	}
	slices.SortStableFunc(out, func(a, b Course) int {
		if a.Current != b.Current {
			if a.Current {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Code, b.Code)
	})
	return out
}
