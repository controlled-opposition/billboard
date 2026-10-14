package bb

import (
	"context"
	"time"
)

// ContentKind is the user-visible kind, after folding Ultra's storage tricks.
type ContentKind string

const (
	ContentFolder   ContentKind = "folder"
	ContentDocument ContentKind = "document" // Ultra document: folder + one ultraDocumentBody child, folded
	ContentTest     ContentKind = "test"     // resource/x-bb-asmt-test-link
	ContentFile     ContentKind = "file"     // resource/x-bb-file (none seen yet; handled via /attachments)
	ContentLink     ContentKind = "link"
	ContentUnknown  ContentKind = "unknown" // RawHandler says which
)

// ContentNode is one item a student sees in the course outline. Ultra
// documents are already folded: the node's ID and Title are the folder's
// (what the UI shows), and its body/attachments come from the hidden
// ultraDocumentBody child, which never appears as a node.
type ContentNode struct {
	ID          ContentID
	Course      CourseID
	Parent      ContentID // "" at the top level
	Title       string    // unescaped
	Path        []string  // titles from the course root down to and including this node
	Depth       int       // len(Path)-1
	Kind        ContentKind
	RawHandler  string // contentHandler.id verbatim
	Available   bool
	Modified    time.Time
	URL         string       // absolute Ultra UI link
	Attachments []Attachment // files linked from this node; empty for folders
}

// Attachment is a downloadable file found in content. For Ultra documents it
// comes from an <a data-bbtype="attachment" data-bbfile="{...}"> in the body;
// for x-bb-file items, from /attachments. The signed bbcswebdav URL is
// unexported: it expires, so it is only valid within the walk that produced
// it, and no caller should persist or print it as an identifier.
type Attachment struct {
	Name     string // data-bbfile fileName, unescaped
	MimeType string
	Size     int64     // bytes; 0 when unknown
	Source   ContentID // the visible node it belongs to
	RID      string    // "11112222_1" from the bbcswebdav path; stable across walks, used for dedupe
	href     string    // signed, expiring; only Download reads it
}

// WalkOptions bound a content walk. Zero value: whole course, unlimited depth.
type WalkOptions struct {
	Under ContentID // start below this node instead of ROOT (its subtree only)
	Depth int       // 0 = unlimited; 1 = direct children only
}

// Walk lists a course's outline depth-first in display order (position),
// returning visible nodes with Path filled in and Ultra documents folded.
// Every folder costs one children request (a folder may be a document in
// disguise, which is only knowable from its children), so Walk is the
// expensive call (~60 requests for a full course); `bb content download
// COURSE ID...` uses Under to fetch only what it needs.
//
// Unavailable items (availability.available != "Yes") are returned with
// Available=false and their subtree is not fetched.
func (c *Client) Walk(ctx context.Context, course CourseID, o WalkOptions) ([]ContentNode, error) {
	panic("not implemented")
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

// fold decides, for a folder and its fetched children, whether it is an
// Ultra document. Rule: exactly one child, handler resource/x-bb-document,
// title ultraDocumentBodyTitle. Pure; the main fixture-driven test target.
func fold(folder wireContent, children []wireContent) (node ContentNode, isDocument bool) {
	panic("not implemented")
}

// extractAttachments parses an Ultra HTML body (golang.org/x/net/html), finds
// <a data-bbtype="attachment">, decodes data-bbfile (HTML-entity-escaped
// JSON: fileName, fileSize, mimeType, resourceUrl), and falls back to href
// when resourceUrl is absent. RID is parsed from "/bbcswebdav/pid-N-dt-content-rid-<RID>/".
// Pure.
func extractAttachments(source ContentID, body string) []Attachment { panic("not implemented") }
