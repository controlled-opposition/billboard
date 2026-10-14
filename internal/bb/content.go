package bb

import (
	"cmp"
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ContentKind is the user-visible kind, after folding Ultra's storage tricks.
type ContentKind string

const (
	ContentFolder   ContentKind = "folder"
	ContentDocument ContentKind = "document" // Ultra document (folded) or a classic document item
	ContentTest     ContentKind = "test"
	ContentFile     ContentKind = "file"
	ContentLink     ContentKind = "link"
	ContentUnknown  ContentKind = "unknown" // RawHandler says which
)

// ContentKinds is every ContentKind in display order; help text is built from it.
var ContentKinds = []ContentKind{ContentFolder, ContentDocument, ContentTest, ContentFile, ContentLink, ContentUnknown}

var contentKindByHandler = map[string]ContentKind{
	"resource/x-bb-folder":         ContentFolder,
	"resource/x-bb-document":       ContentDocument,
	"resource/x-bb-asmt-test-link": ContentTest,
	"resource/x-bb-file":           ContentFile,
	"resource/x-bb-externallink":   ContentLink,
}

// ContentNode is one item a student sees in the course outline. Ultra
// documents are already folded: the node's ID and Title are the folder's
// (what the UI shows), and its attachments come from the hidden
// ultraDocumentBody child, which never appears as a node.
type ContentNode struct {
	ID          ContentID
	Course      CourseID
	Parent      ContentID
	Title       string
	Path        []string // titles from the course root down to and including this node
	Depth       int      // len(Path)-1
	Kind        ContentKind
	RawHandler  string // contentHandler.id verbatim
	Available   bool
	Modified    time.Time
	URL         string       // absolute Ultra UI link
	Attachments []Attachment // files linked from this node's body
}

// Attachment is a downloadable file found in an HTML body as
// <a data-bbfile="{...}">.
type Attachment struct {
	Name     string
	MimeType string
	Size     int64     // bytes; 0 when unknown
	Modified time.Time // when the body linking the file last changed; downloaded files get this mtime
	Source   ContentID // the visible node it belongs to
	RID      string    // "11112222_1" from the bbcswebdav path; stable across walks, used for dedupe
	// path is the bbcswebdav path without the signed query string. The
	// cookie session alone fetches it, so it never expires.
	path string
}

// WalkOptions bound a content walk. Zero value: whole course.
type WalkOptions struct {
	Under ContentID // this node and its subtree instead of the whole course
}

// Walk lists a course's outline depth-first in display order (position),
// returning visible nodes with Path filled in and Ultra documents folded.
// Every folder costs one children request (a folder may be a document in
// disguise, which is only knowable from its children), so a whole course
// costs about 60 requests. With Under, Walk fetches the node, one item per
// ancestor up to the course root (so Path matches a whole-course walk), and
// the node's subtree.
//
// Unavailable items (availability.available != "Yes") are returned with
// Available=false and their subtree is not fetched.
func (c *Client) Walk(ctx context.Context, course CourseID, o WalkOptions) ([]ContentNode, error) {
	w := walker{c: c, course: course, base: "/learn/api/public/v1/courses/" + string(course) + "/contents"}
	if o.Under == "" {
		top, err := getPaged[wireContent](ctx, c, w.base)
		if err != nil {
			return nil, err
		}
		return w.items(ctx, top, nil)
	}
	var item wireContent
	if err := c.getJSON(ctx, w.base+"/"+string(o.Under), &item); err != nil {
		return nil, err
	}
	path, err := w.ancestors(ctx, item.ParentID)
	if err != nil {
		return nil, err
	}
	return w.items(ctx, []wireContent{item}, path)
}

type walker struct {
	c      *Client
	course CourseID
	base   string // ".../courses/{id}/contents"
}

// ancestors returns the titles from the course root down to id. The
// course's ROOT container is the one item without a parentId; it is not
// part of any path.
func (w walker) ancestors(ctx context.Context, id string) ([]string, error) {
	var titles []string
	for id != "" {
		var item wireContent
		if err := w.c.getJSON(ctx, w.base+"/"+id, &item); err != nil {
			return nil, err
		}
		if item.ParentID == "" {
			break
		}
		titles = append(titles, item.Title)
		id = item.ParentID
	}
	slices.Reverse(titles)
	return titles, nil
}

func (w walker) items(ctx context.Context, list []wireContent, parent []string) ([]ContentNode, error) {
	slices.SortStableFunc(list, func(a, b wireContent) int { return a.Position - b.Position })
	var out []ContentNode
	for _, item := range list {
		n := w.node(item, parent)
		if n.Kind == ContentDocument {
			n.Attachments = extractAttachments(n.ID, n.Modified, item.Body)
		}
		if n.Kind != ContentFolder || !n.Available || !item.HasChildren {
			out = append(out, n)
			continue
		}
		children, err := getPaged[wireContent](ctx, w.c, w.base+"/"+item.ID+"/children")
		if err != nil {
			return nil, err
		}
		if body, ok := documentBody(children); ok {
			n.Kind = ContentDocument
			n.Attachments = extractAttachments(n.ID, parseTime(body.Modified), body.Body)
			out = append(out, n)
			continue
		}
		out = append(out, n)
		sub, err := w.items(ctx, children, n.Path)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}

func (w walker) node(item wireContent, parent []string) ContentNode {
	kind, ok := contentKindByHandler[item.ContentHandler.ID]
	if !ok {
		kind = ContentUnknown
	}
	n := ContentNode{
		ID:         ContentID(item.ID),
		Course:     w.course,
		Parent:     ContentID(item.ParentID),
		Title:      item.Title,
		Path:       slices.Concat(parent, []string{item.Title}),
		Depth:      len(parent),
		Kind:       kind,
		RawHandler: item.ContentHandler.ID,
		Available:  item.Availability.Available == "Yes",
		Modified:   parseTime(item.Modified),
	}
	for _, l := range item.Links {
		if u, err := w.c.resolve(l.Href); l.Rel == "alternate" && err == nil {
			n.URL = u.String()
		}
	}
	return n
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t.UTC()
}

type wireContent struct {
	ID             string `json:"id"`
	ParentID       string `json:"parentId"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Position       int    `json:"position"`
	HasChildren    bool   `json:"hasChildren"`
	Modified       string `json:"modified"`
	ContentHandler struct {
		ID string `json:"id"`
	} `json:"contentHandler"`
	Availability struct {
		Available string `json:"available"`
	} `json:"availability"`
	Links []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
}

// ultraDocumentBodyTitle is the literal title Ultra gives the hidden child.
const ultraDocumentBodyTitle = "ultraDocumentBody"

// documentBody reports whether a folder's children make it an Ultra
// document: exactly one child, a document titled ultraDocumentBody, whose
// body is the document's content.
func documentBody(children []wireContent) (wireContent, bool) {
	if len(children) != 1 {
		return wireContent{}, false
	}
	b := children[0]
	return b, b.ContentHandler.ID == "resource/x-bb-document" && b.Title == ultraDocumentBodyTitle
}

var ridPattern = regexp.MustCompile(`-rid-(\d+_\d+)/`)

// extractAttachments finds <a data-bbfile="..."> in an HTML body. The HTML
// parser decodes the entity-escaped attribute; what remains is JSON with
// resourceUrl (falling back to href), mimeType, and usually fileName and
// fileSize. Older links carry only displayName or linkName and no size, and
// lack data-bbtype="attachment", so neither is required. Links that do not
// point at /bbcswebdav/ are skipped, so a download never leaves the
// Blackboard file store.
func extractAttachments(source ContentID, modified time.Time, body string) []Attachment {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	var out []Attachment
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" || attr(n, "data-bbfile") == "" {
			continue
		}
		var f struct {
			FileName    string `json:"fileName"`
			DisplayName string `json:"displayName"`
			LinkName    string `json:"linkName"`
			FileSize    int64  `json:"fileSize"`
			MimeType    string `json:"mimeType"`
			ResourceURL string `json:"resourceUrl"`
		}
		if json.Unmarshal([]byte(attr(n, "data-bbfile")), &f) != nil {
			continue
		}
		ref := f.ResourceURL
		if ref == "" {
			ref = attr(n, "href")
		}
		name := cmp.Or(f.FileName, f.DisplayName, f.LinkName)
		u, err := url.Parse(ref)
		if err != nil || !strings.HasPrefix(u.Path, "/bbcswebdav/") || name == "" {
			continue
		}
		var rid string
		if m := ridPattern.FindStringSubmatch(u.Path); m != nil {
			rid = m[1]
		}
		out = append(out, Attachment{
			Name:     name,
			MimeType: f.MimeType,
			Size:     f.FileSize,
			Modified: modified,
			Source:   source,
			RID:      rid,
			path:     u.EscapedPath(),
		})
	}
	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
