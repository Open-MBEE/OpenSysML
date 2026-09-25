package migrate

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/imagefile"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// docPlan is one DocGen document planned as a Document definition beside its
// class in the class's owner, with the report row it earns.
type docPlan struct {
	d    *sysmlv1.DocGenDocument
	host *sysmlv1.Element
	root *sectionPlan
	// anchors are the usages the Document declares to reach views held by
	// definitions, one per definition in first-use order.
	anchors []*anchor
	// notes are the approximations the document as a whole carries.
	notes []string
}

// anchor is a Document's reference usage of a definition, through which a
// Diagram block reaches a view the definition holds.
type anchor struct {
	def  *sysmlv1.Element
	name string
}

// anchor returns the Document's anchor of def, adding it on first use.
func (dp *docPlan) anchor(def *sysmlv1.Element) *anchor {
	for _, a := range dp.anchors {
		if a.def == def {
			return a
		}
	}
	a := &anchor{def: def}
	dp.anchors = append(dp.anchors, a)
	return a
}

// sectionPlan is one view of a document: the Document itself at the root, a
// Section below it, holding its content and its child views in order.
type sectionPlan struct {
	v     *sysmlv1.DocGenView
	name  string
	title string
	// content is what the view's method produces, in chain order; a nested
	// Dynamic View is content too, so it keeps its place among the blocks.
	content  []*contentPlan
	children []*sectionPlan
	// names are the member names claimed in this section's body.
	names columnNames
	// refused says why the method produced nothing, when it was refused whole.
	refused string
}

// contentPlan is one content block a presentation node produces, or the
// comment standing for a node that could not be lowered.
type contentPlan struct {
	kind  string
	name  string
	node  *sysmlv1.Element
	label string
	text  string
	caption,
	style string
	// captionNote says which block's caption a caption Paragraph is.
	captionNote string
	// query and rows are the row query's reserved name and expression, for
	// the query-backed kinds.
	query string
	rows  qx
	// location and alt are what an Image block shows and says for it.
	location string
	alt      string
	// source is the view a Diagram shows; anchor the Document's usage of the
	// definition holding it, when one does.
	source  *view
	anchor  *anchor
	section *sectionPlan
	notes   []string
	refused string
	// target is the block's qualified name once written, for its report row.
	target string
}

// docSuffix names a document's definition after its class.
const docSuffix = " Document"

const (
	unmigrated           = " is not migrated: "
	leavesOut            = "leaves out "
	onlyElementCollected = "the only element collected, the "
	titleRedefines       = "attribute redefines title = "
)

// planDocuments plans every DocGen document once views and tables are, so the
// names reserved account for each other and Diagram blocks find their views.
func (m *migration) planDocuments() {
	for _, d := range m.model.Documents {
		m.planDocument(d)
	}
	for _, p := range m.model.StrayParagraphs {
		m.strayParagraph(p)
	}
}

// docHost is the element whose body a document's definition is written in:
// the nearest written body above the class, nil for the top level.
func (m *migration) docHost(class *sysmlv1.Element) (host *sysmlv1.Element, ok bool) {
	for cur := class.Parent; cur != nil; cur = cur.Parent {
		host := m.bodyOf(cur)
		if !m.hostsViews(host) {
			continue
		}
		if m.flattened(host) {
			host = nil
		}
		return host, true
	}
	return nil, class.Parent == nil
}

func (m *migration) planDocument(d *sysmlv1.DocGenDocument) {
	class := d.Class
	host, ok := m.docHost(class)
	if !ok {
		note := "neither its owner " + kindOf(class.Parent) + " " + qualifiedName(class.Parent) + " nor any ancestor of it is written"
		m.report.Entries = append(m.report.Entries, *m.docEntry(d, Unmapped, "", note))
		return
	}
	title := strings.TrimSpace(class.Name)
	if title == "" {
		title = "Document"
	}
	dp := &docPlan{d: d, host: host}
	dp.root = &sectionPlan{v: d.Root, title: title, names: columnNames{}}
	dp.root.name = m.viewName(host, title+docSuffix)
	m.planSection(dp, dp.root)
	m.nameAnchors(dp)
	m.extras[host] = append(m.extras[host], func() { m.writeDocument(dp) })
}

// nameAnchors names the anchors after their definitions, clear of every
// member name the document declares, so a chain from one resolves anywhere in it.
func (m *migration) nameAnchors(dp *docPlan) {
	used := columnNames{}
	for _, names := range libraryMembers {
		for _, n := range names {
			used[n] = true
		}
	}
	claimed(dp.root, used)
	for _, a := range dp.anchors {
		base := lowerFirst(m.nameFor(a.def))
		a.name = base
		for i := 2; used[a.name]; i++ {
			a.name = fmt.Sprintf("%s %d", base, i)
		}
		used[a.name] = true
		dp.root.names[a.name] = true
	}
}

// claimed adds the member names of sec and every section under it to into.
func claimed(sec *sectionPlan, into columnNames) {
	for n := range sec.names {
		into[n] = true
	}
	for _, cp := range sec.content {
		if cp.section != nil {
			claimed(cp.section, into)
		}
	}
	for _, child := range sec.children {
		claimed(child, into)
	}
}

// planSection lowers a view's method into the section's content, then plans
// its child views as sections after the content, in declaration order.
func (m *migration) planSection(dp *docPlan, sec *sectionPlan) {
	v := sec.v
	dp.notes = append(dp.notes, v.Malformed...)
	m.planMethod(dp, sec)
	for _, p := range v.Paragraphs {
		sec.content = append(sec.content, m.collaboratorParagraph(sec, p))
	}
	for _, child := range v.Children {
		title := strings.TrimSpace(child.Class.Name)
		if title == "" {
			title = "Section"
		}
		cs := &sectionPlan{v: child, title: title, names: columnNames{}}
		cs.name = sec.names.claim(title)
		sec.children = append(sec.children, cs)
		m.planSection(dp, cs)
	}
}

// planMethod walks the activity chain of the view's viewpoint method into
// content blocks; a view without a method contributes only its structure,
// unless its viewpoint's method tag names something that is not one.
func (m *migration) planMethod(dp *docPlan, sec *sectionPlan) {
	v := sec.v
	if v.Method == nil {
		if v.MethodMalformed != "" {
			sec.refused = "the viewpoint " + qualifiedName(v.Viewpoint) + "'s method is not migrated: " + v.MethodMalformed
			m.report.Entries = append(m.report.Entries, *m.nodeEntry(v.Viewpoint, v.Viewpoint.Stereotype("Viewpoint"), Unmapped, sec.refused))
		}
		return
	}
	steps, end := m.model.DocGenChain(v.Method)
	if end != "" {
		sec.refused = "the method " + qualifiedName(v.Method) + unmigrated + end
		m.report.Entries = append(m.report.Entries, *m.nodeEntry(v.Method, v.Method.DocGen(), Unmapped, sec.refused))
		return
	}
	c := &chain{m: m, dp: dp, sec: sec, active: []*sysmlv1.Element{v.Method}}
	c.start(v)
	c.run(steps)
}

// start sets the chain's elements to what DocGen feeds a view's method: what
// the view exposes, or the view itself when it exposes nothing.
func (c *chain) start(v *sysmlv1.DocGenView) {
	if len(v.Exposed) > 0 {
		c.roots(v.Exposed, "the view "+qualifiedName(v.Class)+" exposes")
		return
	}
	c.roots([]sysmlv1.ElementRef{{ID: v.Class.ID, Element: v.Class}}, "the view")
	c.self = "the view " + qualifiedName(v.Class) + " exposes nothing, so its method works on the view itself"
}

// collaboratorParagraph plans a paragraph the View Editor attached to a view:
// its comment's body, verbatim, or a comment saying why not; an image
// paragraph becomes an Image block when the attached file can be located.
func (m *migration) collaboratorParagraph(sec *sectionPlan, p *sysmlv1.DocGenParagraph) *contentPlan {
	cp := &contentPlan{kind: "Paragraph", node: p.Comment, label: "«Paragraph» Comment"}
	if p.Image {
		cp.label = "«Image Paragraph» Comment"
	}
	if p.Malformed != "" {
		cp.refused = p.Malformed
	} else if p.Comment != nil {
		cp.text = commentBody(p.Comment)
	}
	switch {
	case cp.refused != "":
	case p.Image:
		m.planImage(sec, cp, p)
	case m.imageInBody(sec, cp, p.Comment, commentRawBody(p.Comment)):
	case cp.text == "":
		cp.refused = "the paragraph's comment has no body"
	}
	if cp.refused == "" && cp.name == "" {
		cp.name = sec.names.claim("paragraph")
	}
	if cp.refused != "" {
		m.report.Entries = append(m.report.Entries, *m.nodeEntry(p.Comment, p.Application, Unmapped, cp.refused))
	}
	return cp
}

// planImage turns an image paragraph's plan into an Image block: the comment's
// <img src>, or its AttachedFile tag, names the file the archive holds; the
// comment's body is the caption and the file name the alt text. An http(s)
// source needs no bytes: the document renders it at view time.
func (m *migration) planImage(sec *sectionPlan, cp *contentPlan, p *sysmlv1.DocGenParagraph) {
	cp.kind = "Image"
	cp.caption = cp.text
	cp.text = ""
	src, file := m.imageSource(p.Comment)
	var location, reason string
	switch {
	case isRemoteImage(src):
		location = src
	case file != "" || p.Comment.AttachedStream != "":
		location, reason = m.imageFile(file, p.Comment)
	}
	if location == "" && src != "" && !isRemoteImage(src) {
		location, reason = m.imageFile(src, p.Comment)
		if location == "" {
			if loc, ok := m.imageLocation(src); ok {
				location, reason = loc, ""
			}
		}
	}
	if location == "" {
		if serverImagePath(src) && m.imageBase == nil {
			reason = "the image " + strconv.Quote(src) + " is served by the View Editor; pass -image-base-url to show it"
		} else if reason == "" {
			reason = "the image paragraph's comment names no attached file"
		}
		m.fallbackImageParagraph(sec, cp, reason)
	} else {
		cp.location = location
	}
	if cp.kind != "Image" {
		return
	}
	cp.alt = file
	if cp.alt == "" {
		cp.alt = src
	}
	if cp.alt == "" {
		cp.alt = cp.caption
	}
	if _, _, n := firstImg(commentRawBody(p.Comment)); n > 1 {
		cp.notes = append(cp.notes, fmt.Sprintf("%d more images in the body are left out", n-1))
	}
	cp.name = sec.names.claim("image")
}

// fallbackImageParagraph leaves an image plan whose file cannot be located as
// the paragraph its caption makes, noted with the reason; an empty caption
// refuses the plan instead.
func (m *migration) fallbackImageParagraph(sec *sectionPlan, cp *contentPlan, reason string) {
	if cp.caption == "" {
		cp.refused = reason
		return
	}
	cp.notes = append(cp.notes, reason+"; its caption stands as the paragraph")
	cp.kind = "Paragraph"
	cp.text = cp.caption
	cp.caption = ""
	cp.name = sec.names.claim("paragraph")
}

// attachmentName is the raw file name the AttachedFile tag states.
func attachmentName(c *sysmlv1.Element) string {
	for _, s := range c.Stereotypes {
		if s.Name == "AttachedFile" && s.Namespace == sysmlv1.MagicDrawProfileNS {
			if f := strings.TrimSpace(s.Tag("file")); f != "" {
				return f
			}
		}
	}
	return ""
}

var (
	imgTagRe    = regexp.MustCompile(`(?i)<img[^>]*>`)
	imgSourceRe = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	imgAltRe    = regexp.MustCompile(`(?i)\balt\s*=\s*["']([^"']*)["']`)
)

// commentRawBody is the comment's body as written, tags and all.
func commentRawBody(c *sysmlv1.Element) string {
	if c == nil {
		return ""
	}
	body := c.Attrs["body"]
	if o := firstOwned(c, "body"); o != nil && strings.TrimSpace(body) == "" {
		body = o.Text
	}
	return body
}

// imageSource reads the image an image-paragraph comment names: the first
// <img src> in its body, and the AttachedFile tag's file name.
func (m *migration) imageSource(c *sysmlv1.Element) (src, file string) {
	if match := imgSourceRe.FindStringSubmatch(commentRawBody(c)); match != nil {
		src = html.UnescapeString(match[1])
	}
	return src, attachmentName(c)
}

// isRemoteImage reports a location rendered where it stands: an http(s) URL
// the archive need not hold.
func isRemoteImage(src string) bool {
	lower := strings.ToLower(src)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// serverImagePath reports a src the View Editor could serve: a root-relative
// or relative path, not a URL or a Windows file path.
func serverImagePath(src string) bool {
	if src == "" || isRemoteImage(src) || strings.HasPrefix(src, `\\`) {
		return false
	}
	u, err := url.Parse(src)
	return err == nil && !u.IsAbs()
}

// imageLocation resolves an <img src> to a location the document renders where
// it stands: the URL itself, or a server path resolved against the base URL.
func (m *migration) imageLocation(src string) (string, bool) {
	if isRemoteImage(src) {
		return src, true
	}
	if m.imageBase == nil || !serverImagePath(src) {
		return "", false
	}
	ref, err := url.Parse(src)
	if err != nil {
		return "", false
	}
	return m.imageBase.ResolveReference(ref).String(), true
}

// firstImg reads the first <img> of a raw body: its src and alt attributes,
// and how many <img>s the body holds.
func firstImg(body string) (src, alt string, n int) {
	imgs := imgTagRe.FindAllString(body, -1)
	for _, tag := range imgs {
		if match := imgSourceRe.FindStringSubmatch(tag); match != nil {
			src = html.UnescapeString(match[1])
			if match := imgAltRe.FindStringSubmatch(tag); match != nil {
				alt = html.UnescapeString(match[1])
			}
			break
		}
	}
	return src, alt, len(imgs)
}

// imageInBody plans the first <img> of a paragraph body's as an Image block —
// the body's text is its caption and the img's alt its alt — when the source
// resolves in the archive or against the base URL; a body with several images
// is noted for those left out.
func (m *migration) imageInBody(sec *sectionPlan, cp *contentPlan, node *sysmlv1.Element, body string) bool {
	src, alt, n := firstImg(body)
	if src == "" {
		return false
	}
	location, _ := m.imageFile(src, node)
	ok := location != ""
	if !ok {
		location, ok = m.imageLocation(src)
	}
	if !ok {
		if serverImagePath(src) && m.imageBase == nil {
			cp.notes = append(cp.notes, "the image "+strconv.Quote(src)+" is served by the View Editor; pass -image-base-url to show it")
		}
		return false
	}
	cp.kind = "Image"
	cp.location = location
	cp.caption = cp.text
	cp.text = ""
	cp.alt = alt
	if cp.alt == "" {
		cp.alt = cp.caption
	}
	if n > 1 {
		cp.notes = append(cp.notes, fmt.Sprintf("%d more images in the body are left out", n-1))
	}
	cp.name = sec.names.claim("image")
	return true
}

// imageFile registers the image bytes the attachment names (stream, exact
// entry, or unique base name) for writing under images/; reason says why not.
func (m *migration) imageFile(name string, c *sysmlv1.Element) (location, reason string) {
	named := "the attached image " + strconv.Quote(name)
	if name == "" {
		named = "the attached image"
	}
	data, entry, ct, reason := m.archivedImage(named, name, c)
	if reason != "" {
		return "", reason
	}
	return m.addFile(imagefile.Name(name, entry, ct), data), ""
}

// archivedImage finds the image bytes the attachment name names in the archive
// (c's attached stream, the exact entry, or the unique base name) and their
// content type; reason says, of named, why there are none.
func (m *migration) archivedImage(named, name string, c *sysmlv1.Element) (data []byte, entry, ct, reason string) {
	entry, ambiguous := m.findEntry(name, c)
	if ambiguous > 1 {
		return nil, "", "", named + " matches " + strconv.Itoa(ambiguous) + " archive entries; the attachment names no stream"
	}
	if entry == "" {
		return nil, "", "", named + " is not in the archive"
	}
	data, ok := m.model.Attachment(entry)
	if !ok {
		return nil, "", "", named + " is not in the archive"
	}
	ct = imagefile.ContentType(data)
	if ct == "" {
		return nil, "", "", named + " is not an image (content type " + imagefile.Described(data) + ")"
	}
	return data, entry, ct, ""
}

// findEntry names the archive entry holding the attachment name names: the
// comment's attached stream, the exact entry, or the unique entry with that
// base name; ambiguous reports a base name several entries share.
func (m *migration) findEntry(name string, c *sysmlv1.Element) (entry string, ambiguous int) {
	names := m.model.AttachmentNames()
	if len(names) == 0 {
		return "", 0
	}
	if c != nil && slices.Contains(names, c.AttachedStream) {
		return c.AttachedStream, 0
	}
	if name == "" {
		return "", 0
	}
	if slices.Contains(names, name) {
		return name, 0
	}
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	for _, n := range names {
		if path.Base(n) == base {
			entry = n
			ambiguous++
		}
	}
	if ambiguous > 1 {
		return "", ambiguous
	}
	return entry, 0
}

// unsafeFileChars are the bytes replaced in a written image file's name.
var unsafeFileChars = strings.NewReplacer("\\", "_", "/", "_", ":", "_", " ", "_", "?", "_", "#", "_", "%", "_")

// addFile registers data for writing beside the notation as images/<name>,
// sanitized and deduplicated: identical bytes already registered reuse the
// path, a colliding name gains a numeric suffix.
func (m *migration) addFile(name string, data []byte) string {
	base := unsafeFileChars.Replace(name)
	if base == "" || base == "." || base == ".." {
		base = "image"
	}
	if path, ok := m.fileContents[string(data)]; ok {
		return path
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	path := "images/" + base
	for i := 2; ; i++ {
		if have, taken := m.files[path]; !taken {
			break
		} else if !bytes.Equal(have, data) {
			path = "images/" + stem + "-" + strconv.Itoa(i) + ext
		} else {
			break
		}
	}
	m.files[path] = data
	m.fileContents[string(data)] = path
	m.imagesWritten++
	return path
}

// strayParagraph reports a collaborator paragraph attached to no document view.
func (m *migration) strayParagraph(p *sysmlv1.DocGenParagraph) {
	note := "the paragraph is attached to no view of a document"
	if p.Malformed != "" {
		note = p.Malformed
	}
	m.report.Entries = append(m.report.Entries, *m.nodeEntry(p.Comment, p.Application, Unmapped, note))
}

// chain lowers one activity chain: the elements the steps have collected so
// far, and the section its presentation nodes fill.
type chain struct {
	m   *migration
	dp  *docPlan
	sec *sectionPlan
	// ctx is the current elements; empty when the chain works on none.
	ctx qx
	// diagrams are the current elements that are diagrams, which no query
	// names but an Image shows; each step transforms them as it does ctx.
	diagrams []*sysmlv1.Diagram
	// holders are the current source elements other than diagrams, followed so
	// a collect step gathers the diagrams they own, as DocGen's does.
	holders []*sysmlv1.Element
	// vague says which step left the holders, and so the diagrams, unknown.
	vague string
	// hazy says which step left only the holders unknown, the diagrams still
	// known: a collect over the holders may add diagrams, and turns vague.
	hazy string
	// self says the chain started on the view itself, which exposes nothing.
	self string
	// dropped names the filter that kept none of the diagrams, while none is current.
	dropped string
	// none says why the chain is known to hold no element at all, once a step left none.
	none string
	// broken says why the current elements are unknown, once a step failed.
	broken string
	// notes are approximations the collected elements carry into what shows them.
	notes []string
	// active are the activities being lowered, outermost first, so a
	// recursive call is refused rather than followed.
	active []*sysmlv1.Element
}

func (c *chain) sub() *chain {
	s := *c
	s.notes = append([]string(nil), c.notes...)
	s.diagrams = append([]*sysmlv1.Diagram(nil), c.diagrams...)
	s.holders = append([]*sysmlv1.Element(nil), c.holders...)
	s.active = append([]*sysmlv1.Element(nil), c.active...)
	return &s
}

// body is the activity or structured node a step's chain is read from, or
// why it cannot be entered.
func (c *chain) body(s *sysmlv1.DocGenStep) (*sysmlv1.Element, string) {
	body := s.Node
	if s.Behavior != nil {
		body = s.Behavior
	}
	for _, a := range c.active {
		if a == body {
			return nil, "the " + kindOf(body) + " " + qualifiedName(body) + " calls itself, and a recursive section has no static spelling"
		}
	}
	return body, ""
}

func (c *chain) note(s string) {
	if s != "" && !contains(c.notes, s) {
		c.notes = append(c.notes, s)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// roots sets the chain's elements to refs, named by qualified name once each;
// a ref that resolves to no written element breaks the chain, though the
// source elements stay followed for the diagrams they own.
func (c *chain) roots(refs []sysmlv1.ElementRef, role string) {
	c.ctx, c.diagrams, c.holders, c.vague, c.hazy, c.self, c.dropped, c.none, c.broken = qx{}, nil, nil, "", "", "", "", "", ""
	var names []string
	for _, ref := range refs {
		if d := c.m.model.Diagram(ref.ID); d != nil {
			c.diagrams = appendDiagram(c.diagrams, d)
			continue
		}
		if ref.Element == nil {
			why := role + " " + ref.ID + ", which " + c.m.unresolvedRef(ref)
			c.broken, c.vague = why, why
			return
		}
		c.holders = append(c.holders, ref.Element)
		if c.broken != "" {
			continue
		}
		name, why := c.m.namedRoot(ref, "element")
		if why != "" {
			c.broken = role + " " + kindOf(ref.Element) + " " + qualifiedName(ref.Element) + ", which is not migrated"
			continue
		}
		if !contains(names, name) {
			names = append(names, name)
		}
	}
	if len(names) > 0 && c.broken == "" {
		c.ctx = qcall("Named", qstrs("qualifiedName", names...))
	}
}

// appendDiagram adds d to ds unless it is there already.
func appendDiagram(ds []*sysmlv1.Diagram, d *sysmlv1.Diagram) []*sysmlv1.Diagram {
	for _, x := range ds {
		if x == d {
			return ds
		}
	}
	return append(ds, d)
}

// keepDiagrams keeps the current diagrams keep admits, or the others when
// the step excludes instead of including.
func (c *chain) keepDiagrams(s *sysmlv1.DocGenStep, keep func(*sysmlv1.Diagram) bool) {
	include := s.Application.Tag("include") != "false"
	var kept []*sysmlv1.Diagram
	for _, d := range c.diagrams {
		if keep(d) == include {
			kept = append(kept, d)
		}
	}
	if len(kept) == 0 && len(c.diagrams) > 0 {
		c.dropped = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " drops all the diagrams collected"
	}
	c.diagrams = kept
}

// empty reports whether the chain has no elements to work on.
func (c *chain) empty() bool { return c.ctx.op == "" && c.ctx.lit == "" }

// idle reports whether a query step has nothing at all to transform.
func (c *chain) idle() bool {
	return c.empty() && len(c.diagrams) == 0 && len(c.holders) == 0 && c.vague == "" && c.hazy == ""
}

// blur records that a step left the source elements, diagrams among them,
// unknown, so the diagrams a later collect would gather are unknown too.
func (c *chain) blur(s *sysmlv1.DocGenStep, why string) {
	if c.vague == "" {
		c.vague = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " " + why
	}
	c.holders, c.hazy = nil, ""
}

// haze records that a step left the holders unknown while the diagrams, which
// it decided, stay known: no diagram hides among the unknown until a collect.
func (c *chain) haze(s *sysmlv1.DocGenStep, why string) {
	if c.vague == "" && c.hazy == "" {
		c.hazy = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " " + why
	}
	c.holders = nil
}

// rows is the elements the chain is known to hold, or why they are unknown.
func (c *chain) rows() (rowSet, string) {
	for _, why := range []string{c.broken, c.vague, c.hazy} {
		if why != "" {
			return rowSet{}, why
		}
	}
	return rowSet{listed: c.holders}, ""
}

func (c *chain) run(steps []*sysmlv1.DocGenStep) {
	for _, s := range steps {
		c.step(s)
	}
}

// step lowers one node: a collect, filter or sort step transforms the
// elements; a presentation node adds a block; a group recurses.
func (c *chain) step(s *sysmlv1.DocGenStep) {
	if s.Malformed != "" {
		c.abort(s, s.Malformed)
		return
	}
	if len(s.Targets) > 0 {
		c.roots(s.Targets, "the node "+qualifiedName(s.Node)+" targets")
		if c.broken != "" {
			c.fail(s, c.broken)
			return
		}
	}
	switch s.Kind {
	case "CollectOwnedElements":
		c.collect(s, "Descendants")
	case "CollectOwners":
		c.collect(s, "Ancestors")
	case "CollectByDirectedRelationshipStereotypes":
		c.collectRelated(s)
	case "CollectThingsOnDiagram":
		c.collectShown(s)
	case "CollectByAssociation":
		c.collectAssociated(s)
	case "FilterByMetaclasses":
		c.filterTypes(s, "metaclasses")
	case "FilterByStereotypes":
		c.filterTypes(s, "stereotypes")
	case "FilterByDiagramType":
		c.filterDiagramTypes(s)
	case "FilterByNames":
		c.filterNames(s)
	case "SortByName":
		c.sort(s, "name")
	case "SortByAttribute":
		if attr, why := c.attribute(s, "desiredAttribute"); why != "" {
			c.fail(s, why)
		} else {
			c.sort(s, attr)
		}
	case "RemoveDuplicates":
		// Every query operation removes duplicates.
	case "Union", "Intersection", "XOR":
		c.join(s)
	case "CollectionAndFilterGroup":
		c.group(s, true)
	case "StructuredQuery":
		c.group(s, false)
	case "TableStructure":
		c.table(s)
	case "BulletedList":
		c.list(s)
	case "Paragraph":
		c.paragraph(s)
	case "Image":
		c.image(s)
	case "Dynamic_View", "DynamicView":
		c.dynamicView(s)
	case "":
		switch {
		case s.Behavior != nil:
			c.group(s, false)
		case len(s.Targets) > 0:
			// A node that only resets the targets.
		default:
			c.abort(s, "the node "+qualifiedName(s.Node)+" carries no DocGen stereotype")
		}
	default:
		c.abort(s, "no query operation or content block stands for «"+s.Kind+"»")
	}
}

// fail records why a step is not migrated: a query step breaks the chain for
// what follows, a presentation node stands as a comment in the section. The
// source elements stay followed, since the step's effect on them is known.
func (c *chain) fail(s *sysmlv1.DocGenStep, why string) {
	switch s.Kind {
	case "TableStructure", "BulletedList", "Paragraph", "Image", "Dynamic_View", "DynamicView":
		c.refuse(s, why)
	default:
		if c.broken == "" {
			c.broken = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + unmigrated + why
		}
		c.ctx = qx{}
		c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, Unmapped, why))
	}
}

// abort fails a step whose effect on the source elements is unknown too.
func (c *chain) abort(s *sysmlv1.DocGenStep, why string) {
	c.blur(s, "is not migrated: "+why)
	c.fail(s, why)
}

// kind names a step for a reader: its stereotype, else its metaclass.
func (c *chain) kind(s *sysmlv1.DocGenStep) string {
	if s.Kind != "" {
		return s.Kind
	}
	return s.Node.Type
}

// refuse stands a comment in the section for a presentation node.
func (c *chain) refuse(s *sysmlv1.DocGenStep, why string) {
	cp := &contentPlan{kind: c.kind(s), node: s.Node, label: "«" + c.kind(s) + "» " + s.Node.Type, refused: why}
	c.sec.content = append(c.sec.content, cp)
	c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, Unmapped, why))
}

// ready reports whether a presentation node has elements to show, refusing it
// when the chain is broken or empty.
func (c *chain) ready(s *sysmlv1.DocGenStep) bool {
	switch {
	case c.broken != "":
		c.refuse(s, "the elements it shows pass through "+c.broken)
		return false
	case c.empty() && len(c.diagrams) > 0:
		c.refuse(s, "it shows no element: only diagrams are current, and a diagram is shown by an Image, not listed")
		return false
	case c.empty() && c.none != "":
		c.nothing(s)
		return false
	case c.empty():
		c.refuse(s, "it shows no element: the view exposes nothing and the node targets nothing")
		return false
	}
	return true
}

// nothing records a presentation node over no element, which DocGen shows
// nothing for either, except a table, whose headings it prints over no row.
func (c *chain) nothing(s *sysmlv1.DocGenStep) {
	verdict, note := Mapped, "it shows nothing: "+c.none
	if s.Kind == "TableStructure" {
		verdict, note = Approximated, "it lists nothing: "+c.none+"; the table DocGen prints, headings over no row, is left out"
	}
	c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, verdict, note))
}

// clear empties the chain for a known reason, which what follows reports.
func (c *chain) clear(why string) {
	c.ctx, c.diagrams, c.holders, c.vague, c.hazy = qx{}, nil, nil, "", ""
	c.dropped, c.none = why, why
}

// keepNone lowers a filter that names nothing as DocGen runs it: including
// keeps no element, excluding keeps them all.
func (c *chain) keepNone(s *sysmlv1.DocGenStep, why string) {
	if s.Application.Tag("include") == "false" {
		return
	}
	c.clear("«" + c.kind(s) + "» " + qualifiedName(s.Node) + " keeps nothing: " + why)
}

// depth reads a collect step's depth: 0 or absent is unbounded.
func (c *chain) depth(s *sysmlv1.DocGenStep) (int, string) {
	raw := strings.TrimSpace(s.Application.Tag("depth"))
	if raw == "" {
		return 0, ""
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, "the depth " + strconv.Quote(raw) + " is not a whole number"
	}
	return n, ""
}

// collect lowers CollectOwnedElements and CollectOwners to a walk of the tree.
// A diagram owns no element; its owners join the elements, named. The source
// elements are walked the same way, gathering the diagrams they own.
func (c *chain) collect(s *sysmlv1.DocGenStep, op string) {
	if c.idle() {
		return
	}
	depth, why := c.depth(s)
	if why != "" {
		c.abort(s, why)
		return
	}
	var owners []string
	if op == "Ancestors" {
		owners, why = c.diagramOwners(depth)
	}
	c.collectHolders(op, depth)
	if why != "" {
		c.fail(s, why)
	}
	if c.broken != "" {
		return
	}
	var results []qx
	if !c.empty() {
		args := []qarg{qarg1("source", c.ctx)}
		if depth > 0 {
			args = append(args, qint1("maxDepth", depth))
		}
		results = append(results, qcall(op, args...))
	}
	if len(owners) > 0 {
		results = append(results, qcall("Named", qstrs("qualifiedName", owners...)))
	}
	if len(results) == 0 {
		c.ctx = qx{}
		return
	}
	c.ctx = union(results)
}

// collectHolders walks the source elements as the collect step does: down to
// the elements and diagrams they own, or up to their owners and the diagrams'.
func (c *chain) collectHolders(op string, depth int) {
	holders, diagrams := c.holders, c.diagrams
	c.diagrams, c.dropped, c.none = nil, "", ""
	// Unknown holders own unknown diagrams; their owners are elements only.
	if c.hazy != "" && op != "Ancestors" {
		c.vague, c.hazy = c.hazy, ""
	}
	if c.vague != "" {
		c.holders = nil
		return
	}
	seen := map[*sysmlv1.Element]bool{}
	var out []*sysmlv1.Element
	add := func(e *sysmlv1.Element) {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	if op == "Ancestors" {
		for _, d := range diagrams {
			if owner := diagramOwner(d); owner != nil {
				owners(owner, depth, add)
			}
		}
		for _, h := range holders {
			if h.Parent != nil {
				owners(h.Parent, depth, add)
			}
		}
	} else {
		for _, h := range holders {
			c.m.owned(h, depth, add, func(d *sysmlv1.Diagram) { c.diagrams = appendDiagram(c.diagrams, d) })
		}
	}
	c.holders = out
}

// diagramOwners names the owners of the current diagrams up to depth, all of
// them for 0, or says which owner is not migrated. The root Model, which is
// not written, ends the walk as it does for Ancestors.
func (c *chain) diagramOwners(depth int) (names []string, why string) {
	for _, d := range c.diagrams {
		owner := diagramOwner(d)
		for i := 0; owner != nil && (depth == 0 || i < depth); i, owner = i+1, owner.Parent {
			if isTopLevel(owner) {
				break
			}
			if !c.m.written(owner) {
				return nil, "the diagram '" + d.Name + "' is owned by the " + kindOf(owner) + " " + qualifiedName(owner) + ", which is not migrated"
			}
			if name := c.m.plainName(owner); !contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names, ""
}

// collectShown lowers CollectThingsOnDiagram as DocGen runs it: the model
// elements the current diagrams show, named; anything else is dropped.
func (c *chain) collectShown(s *sysmlv1.DocGenStep) {
	if c.idle() {
		return
	}
	diagrams, holders := c.diagrams, c.holders
	c.ctx, c.diagrams, c.holders, c.dropped, c.none, c.hazy = qx{}, nil, nil, "", "", ""
	if c.vague != "" {
		c.fail(s, "the diagrams it reads are known only when the query runs, and no query operation reads what a diagram shows")
		return
	}
	// One diagram whose symbols are unread leaves the whole collection unknown,
	// listed or not: a Named query over the rest would pass for complete.
	var unread, listed []string
	for _, d := range diagrams {
		what := "what the " + diagramKind(d) + " '" + d.Name + "' shows"
		switch {
		case d.Drawn:
		case d.Stream != "" && len(d.Shown) > 0:
			unread = append(unread, what+" beyond the "+plural(len(d.Shown), "element")+" the tool lists, whose symbols cannot be read")
		case d.Stream != "" || len(d.Shown) == 0:
			unread = append(unread, what+", which the archive does not record")
		default:
			listed = append(listed, "reads the "+plural(len(d.Shown), "element")+" the tool lists as used on the "+diagramKind(d)+" '"+d.Name+"', whose symbols are not serialized; the list need not be all it shows")
		}
	}
	if len(unread) > 0 {
		c.abort(s, "it collects "+strings.Join(unread, " and "))
		return
	}
	for _, n := range listed {
		c.note(n)
	}
	var names []string
	for _, d := range diagrams {
		unknown, unwritten, folded := 0, 0, 0
		for _, ref := range d.Shown {
			if sd := c.m.model.Diagram(ref.ID); sd != nil {
				c.diagrams = appendDiagram(c.diagrams, sd)
				continue
			}
			switch name, why := c.m.namedRoot(ref, "element"); {
			case ref.Element == nil:
				unknown++
			case why != "" && c.m.foldedInto(ref.Element):
				folded++
			case why != "":
				c.holders = appendElement(c.holders, ref.Element)
				unwritten++
			default:
				c.holders = appendElement(c.holders, ref.Element)
				if !contains(names, name) {
					names = append(names, name)
				}
			}
		}
		shown := " shown on the " + diagramKind(d) + " '" + d.Name + "' "
		if unknown > 0 {
			c.note(leavesOut + plural(unknown, "element") + shown + "that the archive does not describe")
		}
		if unwritten > 0 {
			c.note(leavesOut + plural(unwritten, "element") + shown + "that the migration does not write")
		}
		if folded > 0 {
			c.note(leavesOut + plural(folded, "element") + shown + "written within the elements owning them, with no v2 element of their own")
		}
	}
	if len(names) > 0 {
		c.ctx = qcall("Named", qstrs("qualifiedName", names...))
	}
	if c.empty() && len(c.diagrams) == 0 && c.vague == "" {
		c.none = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " collects nothing: " + c.shownNothing(diagrams, holders)
		c.dropped = c.none
	}
}

// shownNothing says why no element is shown on the current diagrams: there is
// none, or each shows no model element the query names.
func (c *chain) shownNothing(diagrams []*sysmlv1.Diagram, holders []*sysmlv1.Element) string {
	if len(diagrams) == 0 {
		if len(holders) == 0 {
			return "no diagram is collected for it to read"
		}
		return noneOf(holders) + ", and only a diagram shows elements"
	}
	var parts []string
	for _, d := range diagrams {
		if len(d.Shown) == 0 {
			parts = append(parts, "the "+diagramKind(d)+" '"+d.Name+"' "+showsNothing(d, "shows no model element"))
		} else {
			parts = append(parts, "the "+diagramKind(d)+" '"+d.Name+"' shows no element the query names")
		}
	}
	return strings.Join(parts, "; ")
}

// plural counts nouns: "1 element", "3 elements".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// collectAssociated lowers CollectByAssociation as DocGen runs it: from each
// current classifier, the types of its attributes of the aggregation kind,
// followed on to depth. The types are known statically, so they are named.
func (c *chain) collectAssociated(s *sysmlv1.DocGenStep) {
	if c.idle() {
		return
	}
	depth, why := c.depth(s)
	if why != "" {
		c.abort(s, why)
		return
	}
	kind := c.aggregation(s)
	// A diagram is no classifier, so it has no attributes to follow.
	c.diagrams, c.dropped, c.none = nil, "", ""
	if c.vague != "" || c.hazy != "" {
		c.fail(s, "the elements it starts from are known only when the query runs, and no query operation tells a "+kind+" feature from the others")
		return
	}
	holders := c.holders
	c.holders, c.ctx = nil, qx{}
	var names []string
	// reached is the shallowest level each type was met at; a shallower path
	// walks it again, since more depth remains below it.
	reached := map[*sysmlv1.Element]int{}
	var walk func(e *sysmlv1.Element, level int)
	walk = func(e *sysmlv1.Element, level int) {
		if depth > 0 && level > depth {
			return
		}
		for _, p := range e.Owned("ownedAttribute") {
			t := c.m.model.Ref(p, "type")
			if aggregationOf(p) != kind || t == nil {
				continue
			}
			at, met := reached[t]
			if met && at <= level {
				continue
			}
			if !met {
				c.holders = append(c.holders, t)
				if name, why := c.m.namedRoot(sysmlv1.ElementRef{ID: t.ID, Element: t}, "type"); why != "" {
					c.note(why + ", so it is left out")
				} else if !contains(names, name) {
					names = append(names, name)
				}
			}
			reached[t] = level
			walk(t, level+1)
		}
	}
	for _, h := range holders {
		walk(h, 1)
	}
	if len(names) > 0 {
		c.ctx = qcall("Named", qstrs("qualifiedName", names...))
		return
	}
	if len(c.holders) == 0 {
		c.none = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " collects nothing: " + c.noAssociated(holders, kind)
		c.dropped = c.none
	}
}

// noAssociated says why no type is reached from the elements by attributes of
// the aggregation kind: there is no element, or none has such an attribute.
func (c *chain) noAssociated(holders []*sysmlv1.Element, kind string) string {
	switch len(holders) {
	case 0:
		return "no element is collected for it to follow"
	case 1:
		return onlyElementCollected + kindOf(holders[0]) + " " + qualifiedName(holders[0]) + ", has no typed attribute of " + kind + " aggregation"
	}
	return "none of the " + strconv.Itoa(len(holders)) + " elements collected has a typed attribute of " + kind + " aggregation"
}

// aggregation reads the association kind a CollectByAssociation follows, a
// literal named or referred to; composite when none is.
func (c *chain) aggregation(s *sysmlv1.DocGenStep) string {
	kind := strings.TrimSpace(s.Application.Tag("associationType"))
	if lit := c.m.model.Lookup(kind); lit != nil && lit.Name != "" {
		kind = lit.Name
	}
	switch kind {
	case "none", "shared":
		return kind
	}
	return "composite"
}

// aggregationOf is a property's aggregation kind, none unless it says otherwise.
func aggregationOf(p *sysmlv1.Element) string {
	if a := p.Attrs["aggregation"]; a != "" {
		return a
	}
	return "none"
}

// collectRelated lowers CollectByDirectedRelationshipStereotypes to a walk of
// each supported relationship kind, united.
func (c *chain) collectRelated(s *sysmlv1.DocGenStep) {
	if c.idle() {
		return
	}
	// A migrated diagram is a view, which is the end of no relationship.
	c.diagrams, c.dropped, c.none = nil, "", ""
	if len(c.holders) > 0 || c.hazy != "" || !c.empty() {
		c.haze(s, "follows relationships to elements only the query finds")
	}
	if c.empty() {
		return
	}
	refs := c.m.model.TagRefs(s.Application, "stereotypes")
	if len(refs) == 0 {
		c.fail(s, "it names no relationship stereotype")
		return
	}
	depth, why := c.depth(s)
	if why != "" {
		c.fail(s, why)
		return
	}
	dir := "outgoing"
	if s.Application.Tag("directionOut") == "false" {
		dir = "incoming"
	}
	var walks []qx
	for _, ref := range refs {
		kind, why := c.m.relationshipKind(ref)
		if why != "" {
			c.fail(s, why)
			return
		}
		args := []qarg{qarg1("source", c.ctx), qarg1("relationshipKind", qstr(kind)), qarg1("direction", qstr(dir))}
		if depth > 0 {
			args = append(args, qint1("maxDepth", depth))
		}
		walks = append(walks, qcall("RelatedElements", args...))
	}
	c.ctx = union(walks)
}

// relationshipKind names the RelatedElements kind a relationship stereotype
// walks, or why it has none.
func (m *migration) relationshipKind(ref sysmlv1.ElementRef) (kind, why string) {
	s := m.model.StereotypeRef(ref.ID)
	if s.Name == "" {
		href := ref.ID
		if ref.Element != nil && ref.Element.Href != "" {
			href = ref.Element.Href
		}
		if doc, name, ok := standardHref(href); ok && doc == "SysML" {
			s.Name = name
			s.Namespace, _, _ = strings.Cut(href, "#")
		}
	}
	switch {
	case s.Name == "" && ref.Element != nil && ref.Element.Type == "Stereotype":
		return "", "the relationship stereotype «" + ref.Element.Name + "» is the user's own: RelatedElements walks no user relationship"
	case s.Name == "":
		return "", "the relationship stereotype " + ref.ID + " is not described by the archive"
	case !isStandardNamespace(s.Namespace):
		return "", "the relationship stereotype «" + s.Name + "» is the user's own: RelatedElements walks no user relationship"
	}
	if kind, ok := relationKinds[s.Name]; ok {
		return kind, ""
	}
	return "", "RelatedElements walks no «" + s.Name + "» relationship"
}

// filterTypes lowers FilterByMetaclasses and FilterByStereotypes to the type
// filters tables use, kept or excepted.
func (c *chain) filterTypes(s *sysmlv1.DocGenStep, tag string) {
	if c.idle() {
		return
	}
	refs := c.m.model.TagRefs(s.Application, tag)
	if len(refs) == 0 {
		c.keepNone(s, "it names no "+map[string]string{"metaclasses": "metaclass", "stereotypes": "stereotype"}[tag])
		return
	}
	c.keepDiagrams(s, func(d *sysmlv1.Diagram) bool {
		for _, ref := range refs {
			if c.m.diagramOfType(ref, d) {
				return true
			}
		}
		return false
	})
	derived := s.Application.Tag("considerDerived") != "false"
	c.keepHolders(s, func(e *sysmlv1.Element) (bool, bool) {
		known := true
		for _, ref := range refs {
			keep, ok := c.m.holderOfType(ref, e, derived)
			if keep && ok {
				return true, true
			}
			known = known && ok
		}
		return false, known
	})
	if c.empty() {
		return
	}
	l := &lowered{}
	kept := c.m.typedRows(c.ctx, refs, true, false, l)
	if l.refused != "" {
		c.fail(s, l.refused)
		return
	}
	for _, n := range l.notes {
		c.note(n)
	}
	if tag == "stereotypes" && s.Application.Tag("considerDerived") == "false" {
		c.note("elements of the stereotypes specializing " + strings.Join(c.labels(refs), ", ") + " are kept too")
	}
	if s.Application.Tag("include") == "false" {
		c.ctx = qcall("Except", qarg1("source", c.ctx), qarg1("exclude", kept))
		return
	}
	c.ctx = kept
}

// filterDiagramTypes lowers FilterByDiagramType as DocGen runs it: only
// diagrams pass, those of a diagram type named, or the others when excluding.
func (c *chain) filterDiagramTypes(s *sysmlv1.DocGenStep) {
	if c.idle() {
		return
	}
	types := c.diagramTypes(s)
	holders, diagrams, hazy := c.holders, c.diagrams, c.hazy
	c.ctx, c.holders, c.diagrams, c.hazy = qx{}, nil, nil, ""
	// Naming no type, the filter keeps none or all, whatever their types.
	for _, d := range diagrams {
		if d.Kind == "" && len(types) > 0 {
			c.blur(s, "keeps or drops the diagram '"+d.Name+"', whose diagram type the archive does not record")
			continue
		}
		c.diagrams = append(c.diagrams, d)
	}
	c.keepDiagrams(s, func(d *sysmlv1.Diagram) bool { return contains(types, d.Kind) })
	if len(c.diagrams) > 0 || c.vague != "" {
		return
	}
	step := "«" + c.kind(s) + "» " + qualifiedName(s.Node)
	switch {
	case len(diagrams) == 0 && hazy != "":
		c.none = step + " drops all the elements collected, whichever they are, and it keeps only diagrams: " + hazy
	case len(diagrams) == 0 && len(holders) == 0:
		return
	case len(diagrams) == 0:
		c.none = step + " drops " + noneOf(holders) + ", and it keeps only diagrams"
	case len(types) == 0:
		c.none = step + " drops all the diagrams collected: it names no diagram type"
	case s.Application.Tag("include") == "false":
		c.none = step + " drops all the diagrams collected: each is " + orList(types)
	default:
		c.none = step + " drops all the diagrams collected: none is " + orList(types)
	}
	c.dropped = c.none
}

// diagramTypes reads the diagram types a FilterByDiagramType names, as the
// tool spells them: literally, or by a reference to a named element.
func (c *chain) diagramTypes(s *sysmlv1.DocGenStep) []string {
	var types []string
	for _, raw := range s.Application.Tags["diagramTypes"] {
		name := strings.TrimSpace(raw)
		if e := c.m.model.Lookup(name); e != nil && e.Name != "" {
			name = e.Name
		}
		if name != "" && !contains(types, name) {
			types = append(types, name)
		}
	}
	return types
}

// orList joins alternatives: "a", "a or b", "a, b or c".
func orList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "a " + s
	}
	if len(quoted) < 2 {
		return strings.Join(quoted, "")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}

// keepHolders keeps the source elements keep admits, or the others when the
// step excludes; a verdict keep cannot tell leaves them unknown.
func (c *chain) keepHolders(s *sysmlv1.DocGenStep, keep func(*sysmlv1.Element) (keep, known bool)) {
	include := s.Application.Tag("include") != "false"
	var kept []*sysmlv1.Element
	for _, e := range c.holders {
		ok, known := keep(e)
		if !known {
			c.haze(s, "keeps or drops the "+kindOf(e)+" "+qualifiedName(e)+", which cannot be told before the query runs")
			return
		}
		if ok == include {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 && len(c.holders) > 0 && len(c.diagrams) == 0 && c.dropped == "" {
		c.dropped = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " drops " + noneOf(c.holders)
	}
	c.holders = kept
}

// noneOf describes collected elements none of which is a diagram.
func noneOf(es []*sysmlv1.Element) string {
	if len(es) == 1 {
		return onlyElementCollected + kindOf(es[0]) + " " + qualifiedName(es[0]) + ", which is not a diagram"
	}
	return "all " + strconv.Itoa(len(es)) + " elements collected, none of them a diagram"
}

// diagramOfType reports whether the element type ref admits diagram d: the
// UML Diagram metaclass or one above it, or the stereotype of its diagram kind.
func (m *migration) diagramOfType(ref sysmlv1.ElementRef, d *sysmlv1.Diagram) bool {
	e := ref.Element
	if e == nil {
		return false
	}
	if doc, name, ok := standardHref(e.Href); ok {
		return doc == "UML" && (name == "Element" || name == "NamedElement" || name == "Diagram")
	}
	if e.IsProxy() {
		if s := m.model.StereotypeRef(e.ID); s.Name != "" {
			return s.Name == d.Kind
		}
		return e.Name != "" && e.Name == d.Kind
	}
	return e.Type == "Stereotype" && e.Name == d.Kind
}

// labels names the type refs as a reader knows them.
func (c *chain) labels(refs []sysmlv1.ElementRef) []string {
	var out []string
	for _, ref := range refs {
		out = append(out, c.m.typeFilter(ref).label)
	}
	return out
}

// filterNames lowers FilterByNames: each name is a whole-string regular
// expression, as DocGen matches them, and an element matching any is kept
// in its place, so the patterns become one alternation.
func (c *chain) filterNames(s *sysmlv1.DocGenStep) {
	if c.idle() {
		return
	}
	names := s.Application.Tags["names"]
	if len(names) == 0 {
		c.abort(s, "it names no name pattern")
		return
	}
	var patterns []string
	var compiled []*regexp.Regexp
	for _, n := range names {
		pattern := "^(?:" + n + ")$"
		re, err := regexp.Compile(pattern)
		if err != nil {
			c.abort(s, "the name pattern "+strconv.Quote(n)+" is not a regular expression the query can match")
			return
		}
		patterns = append(patterns, pattern)
		compiled = append(compiled, re)
	}
	matches := func(name string) bool {
		for _, re := range compiled {
			if re.MatchString(name) {
				return true
			}
		}
		return false
	}
	c.keepDiagrams(s, func(d *sysmlv1.Diagram) bool { return matches(d.Name) })
	// DocGen matches the v1 name; a query matches the v2 one, which differs
	// where the write names an element anew, an anonymous one.
	var renamed []*sysmlv1.Element
	for _, e := range c.holders {
		if c.m.written(e) && matches(e.Name) != matches(c.m.writtenName(e)) {
			renamed = append(renamed, e)
		}
	}
	c.keepHolders(s, func(e *sysmlv1.Element) (bool, bool) { return matches(e.Name), true })
	if c.empty() {
		return
	}
	if len(renamed) > 0 {
		c.keepNamed(s, renamed)
		return
	}
	kept := qcall("WhereName", qarg1("source", c.ctx), qarg1("operator", qstr("matches")), qarg1("value", qstr(strings.Join(patterns, "|"))))
	if s.Application.Tag("include") == "false" {
		c.ctx = qcall("Except", qarg1("source", c.ctx), qarg1("exclude", kept))
		return
	}
	c.ctx = kept
}

// keepNamed spells a name filter as the elements it keeps, since a WhereName
// query over the v2 names would keep or drop the renamed elements otherwise.
func (c *chain) keepNamed(s *sysmlv1.DocGenStep, renamed []*sysmlv1.Element) {
	step := "«" + c.kind(s) + "» " + qualifiedName(s.Node)
	var names []string
	nameless := 0
	for _, e := range c.holders {
		switch {
		case !c.m.written(e):
		case c.m.writtenName(e) == "":
			nameless++
		default:
			names = append(names, c.m.plainName(e))
		}
	}
	var why []string
	for _, e := range renamed {
		why = append(why, "the "+kindOf(e)+" "+qualifiedName(e)+" is named "+c.m.writtenName(e)+" in v2")
	}
	c.note(step + " names the elements it keeps, since a WhereName query matches v2 names: " + strings.Join(why, "; "))
	if nameless > 0 {
		c.note(step + " leaves out " + plural(nameless, "element") + " it keeps that no query names, anonymous in v2")
	}
	if len(names) == 0 {
		c.ctx = qx{}
		if len(c.diagrams) == 0 {
			c.none = c.dropped
			if c.none == "" {
				c.none = step + " keeps no element a query names"
			}
		}
		return
	}
	c.ctx = qcall("Named", qstrs("qualifiedName", names...))
}

// sort lowers a sort step to OrderBy over a query property and orders the current
// diagrams and source elements as the query orders its rows: missing values last, ties kept.
func (c *chain) sort(s *sysmlv1.DocGenStep, property string) {
	if c.idle() {
		return
	}
	dir := "ascending"
	if s.Application.Tag("reverse") == "true" {
		dir = "descending"
	}
	byKey := func(a, b string) int {
		if a == "" || b == "" {
			return strings.Compare(b, a)
		}
		if dir == "descending" {
			a, b = b, a
		}
		return strings.Compare(a, b)
	}
	slices.SortStableFunc(c.diagrams, func(a, b *sysmlv1.Diagram) int {
		return byKey(diagramKey(a, property), diagramKey(b, property))
	})
	slices.SortStableFunc(c.holders, func(a, b *sysmlv1.Element) int {
		return byKey(c.m.sortKey(a, property), c.m.sortKey(b, property))
	})
	if c.empty() {
		return
	}
	c.ctx = qcall("OrderBy", qarg1("source", c.ctx), qarg1("property", qstr(property)),
		qarg1("direction", qstr(dir)), qarg1("missing", qstr("last")), qarg1("multiple", qstr("first")))
}

// diagramKey is the value a diagram's view has for a query property: its name,
// or the doc written from the diagram's own comment; "" when it has none.
func diagramKey(d *sysmlv1.Diagram, property string) string {
	switch property {
	case "name":
		return d.Name
	case "documentation":
		return docKey(commentText(d.Documentation))
	}
	return ""
}

// sortKey is the value e's v2 declaration has for a query property, as OrderBy
// reads it: its v2 name, or the first doc written in its body; "" for none.
func (m *migration) sortKey(e *sysmlv1.Element, property string) string {
	switch property {
	case "name":
		return m.writtenName(e)
	case "documentation":
		return docKey(m.documentation(e))
	}
	return ""
}

// docKey is the body a query reads back from the doc comment written for text.
func docKey(text string) string {
	if text == "" {
		return ""
	}
	return source.CommentBody(strings.Join(commentLines(text), "\n"))
}

// attribute reads a desiredAttribute tag as the query property it names.
func (c *chain) attribute(s *sysmlv1.DocGenStep, tag string) (property, why string) {
	refs := c.m.model.TagRefs(s.Application, tag)
	if len(refs) == 0 {
		return "", "it names no attribute"
	}
	name := literalName(refs[0])
	if name == "" {
		name = strings.TrimSpace(refs[0].ID)
	}
	switch name {
	case "Name":
		return "name", ""
	case "Documentation":
		return "documentation", ""
	}
	return "", "no query property stands for the attribute " + name
}

// literalName is the name of an enumeration literal a tag refers to, read
// from the element or from the fragment of its href.
func literalName(ref sysmlv1.ElementRef) string {
	if ref.Element != nil && ref.Element.Name != "" {
		return ref.Element.Name
	}
	href := ref.ID
	if ref.Element != nil && ref.Element.Href != "" {
		href = ref.Element.Href
	}
	if _, name, ok := standardHref(href); ok {
		return name
	}
	frag := href
	if i := strings.LastIndexByte(frag, '#'); i >= 0 {
		frag = frag[i+1:]
	}
	if i := strings.LastIndexByte(frag, '.'); i >= 0 && !strings.HasPrefix(frag, "_") {
		return frag[i+1:]
	}
	return ""
}

// join lowers a fork whose branches rejoin: each branch works on the current
// elements; a Union joins their results, other joins have no spelling.
func (c *chain) join(s *sysmlv1.DocGenStep) {
	if c.broken != "" {
		return
	}
	var results []qx
	var diagrams []*sysmlv1.Diagram
	var holders []*sysmlv1.Element
	dropped, vague, hazy, none := c.dropped, c.vague, c.hazy, c.none
	// Each branch starts from the chain's doubt and ends with its own, none
	// once it names its targets anew; the union carries what the branches end with.
	if len(s.Branches) > 0 {
		dropped, vague, hazy, none = "", "", "", ""
	}
	for _, branch := range s.Branches {
		sub := c.sub()
		sub.run(branch)
		if sub.broken != "" {
			c.broken, c.vague = sub.broken, sub.vague
			return
		}
		for _, n := range sub.notes {
			c.note(n)
		}
		if !sub.empty() {
			results = append(results, sub.ctx)
		}
		for _, d := range sub.diagrams {
			diagrams = appendDiagram(diagrams, d)
		}
		for _, h := range sub.holders {
			holders = appendElement(holders, h)
		}
		if sub.dropped != "" {
			dropped = sub.dropped
		}
		if sub.none != "" {
			none = sub.none
		}
		if vague == "" {
			vague = sub.vague
		}
		if hazy == "" {
			hazy = sub.hazy
		}
	}
	if s.Kind != "Union" {
		c.abort(s, "its branches rejoin by "+strings.ToLower(s.Kind)+", which only a Union spelling exists for")
		return
	}
	c.diagrams, c.dropped, c.holders, c.vague, c.hazy, c.none = diagrams, dropped, holders, vague, hazy, none
	if vague != "" {
		c.hazy = ""
	}
	if len(diagrams) > 0 {
		c.dropped = ""
	}
	if len(diagrams) > 0 || len(holders) > 0 || len(results) > 0 || vague != "" || hazy != "" {
		c.none = ""
	}
	if len(results) == 0 {
		c.ctx = qx{}
		return
	}
	c.ctx = union(results)
}

// group lowers a nested chain: a CollectionAndFilterGroup's result flows on,
// a StructuredQuery's or a plain call's does not.
func (c *chain) group(s *sysmlv1.DocGenStep, flows bool) {
	body, why := c.body(s)
	if why != "" {
		c.abort(s, why)
		return
	}
	steps, end := c.m.model.DocGenChain(body)
	if end != "" {
		c.abort(s, "its body "+qualifiedName(body)+unmigrated+end)
		return
	}
	if s.Application != nil && s.Application.Tag("loop") == "true" {
		c.note("«" + c.kind(s) + "» " + qualifiedName(s.Node) + " loops over its elements one by one; the query works on them together")
	}
	sub := c.sub()
	sub.active = append(sub.active, body)
	sub.run(steps)
	if !flows {
		return
	}
	c.ctx, c.diagrams, c.holders, c.vague, c.hazy, c.broken = sub.ctx, sub.diagrams, sub.holders, sub.vague, sub.hazy, sub.broken
	c.dropped, c.none = sub.dropped, sub.none
	for _, n := range sub.notes {
		c.note(n)
	}
}

// caption is a presentation node's title: its titles tag, else its name.
func (c *chain) caption(s *sysmlv1.DocGenStep, fallback string) string {
	for _, t := range s.Application.Tags["titles"] {
		if t = strings.TrimSpace(t); t != "" {
			return t
		}
	}
	if t := strings.TrimSpace(s.Node.Name); t != "" {
		return t
	}
	if s.Behavior != nil {
		if t := strings.TrimSpace(s.Behavior.Name); t != "" {
			return t
		}
	}
	return fallback
}

// title is a table's or figure's title as DocGen prints it: the given title
// between the node's titlePrefix and titleSuffix.
func (c *chain) title(s *sysmlv1.DocGenStep, title string) string {
	return strings.TrimSpace(s.Application.Tag("titlePrefix") + title + s.Application.Tag("titleSuffix"))
}

// captionText is the caption DocGen prints under a table's or figure's title:
// the i-th captions entry while showCaptions holds, else nothing.
func (c *chain) captionText(s *sysmlv1.DocGenStep, i int) string {
	if s.Application.Tag("showCaptions") == "false" {
		return ""
	}
	captions := s.Application.Tags["captions"]
	if i >= len(captions) {
		return ""
	}
	return commentText(captions[i])
}

// captionParagraph plans the Paragraph holding a block's caption, which a
// document prints under the block; note says whose caption it is.
func (c *chain) captionParagraph(s *sysmlv1.DocGenStep, note, text string) {
	cp := &contentPlan{kind: "Paragraph", node: s.Node, label: "«" + c.kind(s) + "» " + s.Node.Type, text: text, captionNote: note}
	cp.name = c.sec.names.claim("paragraph")
	c.sec.content = append(c.sec.content, cp)
}

// block plans a query-backed block: its query name is reserved in the
// document's host, its member name in the section.
func (c *chain) block(s *sysmlv1.DocGenStep, kind, caption string, rows qx) *contentPlan {
	cp := &contentPlan{kind: kind, node: s.Node, label: "«" + c.kind(s) + "» " + s.Node.Type, caption: caption, rows: rows}
	cp.name = c.sec.names.claim(strings.ToLower(kind))
	cp.query = c.m.viewName(c.dp.host, c.dp.root.title+" "+caption+rowsSuffix)
	cp.notes = append(cp.notes, c.notes...)
	c.sec.content = append(c.sec.content, cp)
	return cp
}

// table lowers a TableStructure: the current elements projected by columns.
func (c *chain) table(s *sysmlv1.DocGenStep) {
	if !c.ready(s) {
		return
	}
	body := s.Node
	if s.Behavior != nil {
		body = s.Behavior
	}
	colSteps, end := c.m.model.DocGenChain(body)
	if end != "" {
		c.refuse(s, "its columns could not be read: "+end)
		return
	}
	p := &projection{}
	var notes []string
	for _, col := range colSteps {
		prop, expr, why := c.column(col)
		switch {
		case why != "":
			notes = append(notes, "the column «"+c.kind(col)+"» "+qualifiedName(col.Node)+" is not written: "+why)
		case prop != "":
			p.property(prop)
		default:
			p.column(expr.name, qlit(expr.expression))
		}
	}
	if s.Application.Tag("includeDoc") == "true" {
		p.property("documentation")
	}
	if p.empty() {
		why := "none of its columns reads what a query can"
		if len(notes) > 0 {
			why += ": " + strings.Join(notes, "; ")
		}
		c.refuse(s, why)
		return
	}
	project, projectNotes := p.build(c.ctx)
	notes = append(notes, projectNotes...)
	cp := c.block(s, "Table", c.title(s, c.caption(s, "Table")), project)
	cp.notes = append(cp.notes, notes...)
	if s.Application.Tag("loop") == "true" {
		cp.notes = append(cp.notes, "the table loops over its elements one table each; one table lists them together")
	}
	if text := c.captionText(s, 0); text != "" {
		c.captionParagraph(s, "the paragraph is the Table's caption", text)
	}
}

// columnExpr is a Column over a feature of the row's type.
type columnExpr struct {
	name, expression string
}

// column lowers one column node: a query property, or a Column reading a
// feature of the document's classifiers, or why neither.
func (c *chain) column(col *sysmlv1.DocGenStep) (prop string, expr columnExpr, why string) {
	if col.Malformed != "" {
		return "", expr, col.Malformed
	}
	if col.Application == nil {
		return "", expr, "it carries no DocGen column stereotype"
	}
	if steps, _ := c.m.model.DocGenChain(col.Node); len(steps) > 0 {
		return "", expr, "it collects elements before reading them, which a Column does not"
	}
	switch col.Kind {
	case "TableAttributeColumn":
		attr, why := c.attribute(col, "desiredAttribute")
		return attr, expr, why
	case "TablePropertyColumn":
		refs := c.m.model.TagRefs(col.Application, "desiredProperty")
		if len(refs) == 0 {
			return "", expr, "it names no property"
		}
		if f := refs[0].Element; f != nil {
			if prop := requirementProperty(f); prop != "" {
				return prop, expr, ""
			}
		}
		rs, unknown := c.rows()
		if unknown != "" && monteCarloFeature(refs[0].Element) != "" {
			return "", expr, "whether an instance the table lists records the statistic cannot be told: " + unknown
		}
		s := c.m.columnKey(sysmlv1.Column{Kind: sysmlv1.ColumnFeature, Feature: refs[0], ID: refs[0].ID}, c.dp.host, rs)
		if s.why != "" {
			return "", expr, s.why
		}
		if s.path {
			return "", columnExpr{name: c.caption(col, s.caption), expression: s.key}, ""
		}
		c.m.expose(s.feature, "a column of a document table reads it")
		name := c.caption(col, s.key)
		return "", columnExpr{name: name, expression: c.m.ref(s.feature, c.dp.host) + " ?? \"\""}, ""
	case "TableExpressionColumn":
		e := strings.TrimSpace(col.Application.Tag("expression"))
		if p, ok := queryProperties[e]; ok {
			return p, expr, ""
		}
		return "", expr, "the expression " + strconv.Quote(e) + " is not a bare query property (name, documentation, qualifiedName, owner, id)"
	}
	return "", expr, "no Column stands for a «" + col.Kind + "»"
}

// list lowers a BulletedList: the current elements' names, and documentation
// when asked, as a bullet or numbered list.
func (c *chain) list(s *sysmlv1.DocGenStep) {
	if !c.ready(s) {
		return
	}
	a := s.Application
	if len(c.m.model.TagRefs(a, "stereotypeProperties")) > 0 {
		c.refuse(s, "it lists stereotype properties, which the query cannot read")
		return
	}
	var props []string
	if a.Tag("showTargets") != "false" {
		props = append(props, "name")
	}
	if a.Tag("includeDoc") == "true" {
		props = append(props, "documentation")
	}
	if len(props) == 0 {
		c.refuse(s, "it shows neither its elements nor their documentation")
		return
	}
	rows := c.ctx
	if a.Tag("sortElementsByName") == "true" {
		rows = qcall("OrderBy", qarg1("source", rows), qarg1("property", qstr("name")),
			qarg1("direction", qstr("ascending")), qarg1("missing", qstr("last")), qarg1("multiple", qstr("first")))
	}
	style := "bullet"
	if a.Tag("orderedList") == "true" {
		style = "number"
	}
	cp := c.block(s, "List", c.caption(s, "List"), qcall("Project", qarg1("source", rows), qstrs("properties", props...)))
	cp.style = style
	if len(props) > 1 {
		cp.notes = append(cp.notes, "each item's documentation follows its name")
	}
}

// paragraph lowers a Paragraph: its body verbatim, or the documentation of the
// current elements.
func (c *chain) paragraph(s *sysmlv1.DocGenStep) {
	a := s.Application
	if a.Tag("evaluateOcl") == "true" || a.Tag("tryOcl") == "true" {
		c.refuse(s, "its body is evaluated as OCL, which no query evaluates")
		return
	}
	if len(c.m.model.TagRefs(a, "stereotypeProperties")) > 0 {
		c.refuse(s, "it reads stereotype properties, which the query cannot")
		return
	}
	if raw := a.Tag("body"); raw != "" {
		cp := &contentPlan{kind: "Paragraph", node: s.Node, label: "«Paragraph» " + s.Node.Type, text: commentText(raw)}
		if c.m.imageInBody(c.sec, cp, s.Node, raw) || cp.text != "" {
			if cp.name == "" {
				cp.name = c.sec.names.claim("paragraph")
			}
			c.sec.content = append(c.sec.content, cp)
			return
		}
	}
	if c.broken == "" && c.empty() && len(s.Targets) == 0 && a.Tag("body") == "" {
		c.refuse(s, "it has no body and shows no element")
		return
	}
	if !c.ready(s) {
		return
	}
	prop := "documentation"
	if len(c.m.model.TagRefs(a, "desiredAttribute")) > 0 {
		attr, why := c.attribute(s, "desiredAttribute")
		if why != "" {
			c.refuse(s, why)
			return
		}
		prop = attr
	}
	c.block(s, "Paragraph", c.caption(s, "Paragraph"), qcall("Project", qarg1("source", c.ctx), qstrs("properties", prop)))
}

// image lowers an Image: one Diagram block per diagram among the current
// elements, showing its migrated view, captioned by the diagram's title and
// followed by its caption paragraph when DocGen shows captions. A chain whose
// diagrams are not all known draws none: a partial set would pass for the whole.
func (c *chain) image(s *sysmlv1.DocGenStep) {
	if c.broken != "" {
		c.refuse(s, "the diagrams it shows pass through "+c.broken)
		return
	}
	if c.vague != "" {
		c.refuse(s, "the diagrams it shows are not known: "+c.vague)
		return
	}
	if len(c.diagrams) == 0 {
		c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, Mapped, "it draws nothing: "+c.noDiagrams()))
		return
	}
	titles := s.Application.Tags["titles"]
	for i, d := range c.diagrams {
		v := c.m.viewOf[d]
		if v == nil || !v.placed {
			c.refuse(s, "the Diagram '"+d.Name+"' is not written as a view")
			continue
		}
		form, why := c.m.form(d)
		if form.rendering == textualRendering {
			c.refuse(s, joinNotes("the "+diagramKind(d)+" '"+d.Name+"' is a view rendered as textual notation, which a document does not draw", why))
			continue
		}
		if empty := c.m.emptyView(v, form); empty != "" {
			if c.noteImage(s, i, d, titles) {
				continue
			}
			note := "no Diagram shows the " + diagramKind(d) + " '" + d.Name + "': " + empty + ", so the figure would be empty and is left out"
			verdict := Approximated
			if d.Drawn && len(d.Shown) == 0 && len(d.Free) == 0 {
				verdict = Mapped
			}
			if src, _, _ := firstImg(d.Documentation); src != "" && serverImagePath(src) && c.m.imageBase == nil {
				note += "; the note's image " + strconv.Quote(src) + " is served by the View Editor; pass -image-base-url to show it"
			}
			if text := c.captionText(s, i); text != "" {
				note += "; its caption stands alone"
				c.captionParagraph(s, "the paragraph is the caption of the figure left out for the diagram '"+d.Name+"'", text)
			}
			c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, verdict, note))
			continue
		}
		def, _, why := c.m.viewSteps(v)
		if why != "" {
			c.refuse(s, why)
			continue
		}
		cp := &contentPlan{kind: "Diagram", node: s.Node, label: "«Image» " + s.Node.Type, source: v}
		if def != nil {
			cp.anchor = c.dp.anchor(def)
		}
		title := strings.TrimSpace(d.Name)
		if i < len(titles) && strings.TrimSpace(titles[i]) != "" {
			title = strings.TrimSpace(titles[i])
		}
		cp.caption = c.title(s, title)
		cp.name = c.sec.names.claim("diagram")
		c.sec.content = append(c.sec.content, cp)
		if text := c.captionText(s, i); text != "" {
			c.captionParagraph(s, "the paragraph is the Diagram's caption", text)
		}
	}
}

// noteImage plans a figure's Image block when the empty diagram's note holds
// an <img> whose source resolves in the archive or against the base URL: its
// title is the caption and the img's alt the alt text, and a note saying more
// than the title follows as the caption paragraph.
func (c *chain) noteImage(s *sysmlv1.DocGenStep, i int, d *sysmlv1.Diagram, titles []string) bool {
	src, alt, _ := firstImg(d.Documentation)
	if src == "" {
		return false
	}
	location, _ := c.m.imageFile(src, nil)
	if location == "" {
		location, _ = c.m.imageLocation(src)
	}
	if location == "" {
		return false
	}
	title := strings.TrimSpace(d.Name)
	if i < len(titles) && strings.TrimSpace(titles[i]) != "" {
		title = strings.TrimSpace(titles[i])
	}
	cp := &contentPlan{kind: "Image", node: s.Node, label: "«Image» " + s.Node.Type,
		location: location, caption: c.title(s, title), alt: alt}
	if cp.alt == "" {
		cp.alt = cp.caption
	}
	cp.name = c.sec.names.claim("image")
	c.sec.content = append(c.sec.content, cp)
	if text := commentText(d.Documentation); text != "" && !captionCovers(cp.caption, text) {
		c.captionParagraph(s, "the paragraph is the note the figure's image carries", text)
	}
	if text := c.captionText(s, i); text != "" {
		c.captionParagraph(s, "the paragraph is the Diagram's caption", text)
	}
	c.m.report.Entries = append(c.m.report.Entries,
		*c.m.nodeEntry(s.Node, s.Application, Approximated, "the figure shows the image the diagram's note carries, "+location))
	return true
}

// captionCovers reports whether a figure's caption says everything its note
// does: the note, whitespace-normalized, is the caption or a prefix of it.
func captionCovers(caption, note string) bool {
	caption = strings.Join(strings.Fields(caption), " ")
	note = strings.Join(strings.Fields(note), " ")
	return strings.HasPrefix(caption, note)
}

// noDiagrams says why no diagram is current for an Image, as DocGen would
// find none either: what the chain collected, and that none of it is a diagram.
func (c *chain) noDiagrams() string {
	var why string
	switch {
	case c.dropped != "":
		why = c.dropped
	case c.hazy != "":
		why = "no diagram is among the elements collected, whichever they are: " + c.hazy
	case len(c.holders) == 0:
		why = "nothing is collected for it to draw"
	case len(c.holders) == 1:
		why = onlyElementCollected + kindOf(c.holders[0]) + " " + qualifiedName(c.holders[0]) + ", is not a diagram"
	default:
		why = "none of the " + strconv.Itoa(len(c.holders)) + " elements collected is a diagram"
	}
	if c.self != "" {
		why = c.self + ", and " + why
	}
	return why + "; an Image draws only diagrams"
}

// dynamicView lowers a Dynamic View node: a Section titled after it, holding
// what its own chain produces over the current elements.
func (c *chain) dynamicView(s *sysmlv1.DocGenStep) {
	a := s.Application
	title := strings.TrimSpace(a.Tag("title"))
	if title == "" {
		title = c.caption(s, "Section")
	}
	title = a.Tag("titlePrefix") + title + a.Tag("titleSuffix")
	sec := &sectionPlan{title: title, names: columnNames{}}
	sec.name = c.sec.names.claim(title)
	cp := &contentPlan{kind: "Section", node: s.Node, label: "«Dynamic View» " + s.Node.Type, section: sec, name: sec.name}
	c.sec.content = append(c.sec.content, cp)
	body, why := c.body(s)
	if why == "" {
		var steps []*sysmlv1.DocGenStep
		steps, why = c.m.model.DocGenChain(body)
		if why != "" {
			why = "its body " + qualifiedName(body) + unmigrated + why
		} else {
			sub := c.sub()
			sub.sec = sec
			sub.active = append(sub.active, body)
			if a.Tag("loop") == "true" {
				sub.note("the section loops over its elements one section each; one section shows them together")
			}
			sub.run(steps)
			return
		}
	}
	sec.refused = why
	c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, Unmapped, sec.refused))
}

// writeDocument writes a planned document: its queries first, then the
// Document definition holding its sections and blocks.
func (m *migration) writeDocument(dp *docPlan) {
	m.writeQueries(dp.root, m.queryPrefix(dp.host))
	var notes []string
	target := m.qualified(append(m.segments(dp.host), dp.root.name))
	m.inside(blockNames("Document", dp.root.names), func() {
		m.w.block("part def "+writeName(dp.root.name)+" :> "+m.queryPrefix(dp.host)+"Document", func() {
			m.w.line(titleRedefines + stringLiteral(dp.root.title) + ";")
			for _, a := range dp.anchors {
				m.w.line("ref " + writeName(a.name) + " : " + m.memberRef(a.def, dp.host) + ";")
			}
			notes = m.writeSectionBody(dp, dp.root, target)
		})
	})
	notes = append(notes, dp.notes...)
	note := "the «Document» is written as a Document definition of " + strconv.Itoa(len(dp.root.children)) + " section(s)"
	note = joinNotes(note, strings.Join(notes, "; "))
	verdict := Mapped
	if len(notes) > 0 {
		verdict = Approximated
	}
	m.report.Entries = append(m.report.Entries, *m.docEntry(dp.d, verdict, "part def "+target, note))
	for _, cp := range m.blocks(dp.root) {
		if cp.refused != "" {
			continue
		}
		m.report.Entries = append(m.report.Entries, *m.blockEntry(cp))
	}
}

// writeQueries writes the row queries of every query-backed block under sec.
func (m *migration) writeQueries(sec *sectionPlan, prefix string) {
	for _, cp := range m.blocks(sec) {
		if cp.query != "" && cp.refused == "" {
			m.writeQueryDef(cp.query, prefix, cp.rows)
		}
	}
}

// blocks lists every block under sec, nested Dynamic View sections included.
func (m *migration) blocks(sec *sectionPlan) []*contentPlan {
	var out []*contentPlan
	for _, cp := range sec.content {
		out = append(out, cp)
		if cp.section != nil {
			out = append(out, m.blocks(cp.section)...)
		}
	}
	for _, child := range sec.children {
		out = append(out, m.blocks(child)...)
	}
	return out
}

// libraryMembers lists the members each DocumentQueries block inherits, and
// the calc a query-backed one holds; a reference inside it steers clear of them.
var libraryMembers = map[string][]string{
	"Document":  {"title"},
	"Section":   {"title"},
	"Paragraph": {"text", "values"},
	"Table":     {"caption", "groupBy", "rows"},
	"List":      {"style", "items"},
	"Diagram":   {"caption", "kind", "direction", "palette", "source"},
	"Image":     {"location", "caption", "alt"},
}

// blockNames is the member set of a block of the library kind whose own
// members are named own.
func blockNames(kind string, own columnNames) columnNames {
	names := columnNames{}
	for n := range own {
		names[n] = true
	}
	for _, n := range libraryMembers[kind] {
		names[n] = true
	}
	return names
}

// blockPart writes a part usage of the library kind named name holding body,
// with the names it declares in scope for the references body writes.
func (m *migration) blockPart(host *sysmlv1.Element, name, kind string, own columnNames, body func()) {
	m.inside(blockNames(kind, own), func() {
		m.w.block("part "+writeName(name)+" : "+m.queryPrefix(host)+kind, body)
	})
}

// writeSectionBody writes a section's blocks then its child sections under
// path, and returns the notes its blocks carry.
func (m *migration) writeSectionBody(dp *docPlan, sec *sectionPlan, path string) []string {
	var notes []string
	if sec.refused != "" {
		m.w.lines(commentLines("not migrated: " + sec.refused))
	}
	for _, cp := range sec.content {
		notes = append(notes, m.writeBlock(dp, cp, path)...)
	}
	for _, child := range sec.children {
		m.blockPart(dp.host, child.name, "Section", child.names, func() {
			m.w.line(titleRedefines + stringLiteral(child.title) + ";")
			notes = append(notes, m.writeSectionBody(dp, child, path+"::"+writeName(child.name))...)
		})
	}
	return notes
}

// writeBlock writes one block, or the comment standing for a refused node.
func (m *migration) writeBlock(dp *docPlan, cp *contentPlan, path string) []string {
	if cp.refused != "" {
		m.w.lines(commentLines("not migrated: " + cp.label + " '" + nodeLabel(cp.node) + "' — " + cp.refused))
		return nil
	}
	cp.target = path + "::" + writeName(cp.name)
	switch cp.kind {
	case "Section":
		var notes []string
		m.blockPart(dp.host, cp.name, "Section", cp.section.names, func() {
			m.w.line(titleRedefines + stringLiteral(cp.section.title) + ";")
			notes = m.writeSectionBody(dp, cp.section, cp.target)
		})
		return notes
	case "Paragraph":
		m.blockPart(dp.host, cp.name, "Paragraph", nil, func() {
			if cp.query != "" {
				m.w.line("calc values : " + m.siblingRef(dp.host, cp.query) + ";")
			} else {
				m.w.line("attribute redefines text = " + stringLiteral(cp.text) + ";")
			}
		})
	case "Table":
		m.blockPart(dp.host, cp.name, "Table", nil, func() {
			m.w.line("attribute redefines caption = " + stringLiteral(cp.caption) + ";")
			m.w.line("calc rows : " + m.siblingRef(dp.host, cp.query) + ";")
		})
	case "List":
		m.blockPart(dp.host, cp.name, "List", nil, func() {
			m.w.line("attribute redefines style = " + stringLiteral(cp.style) + ";")
			m.w.line("calc items : " + m.siblingRef(dp.host, cp.query) + ";")
		})
	case "Diagram":
		m.blockPart(dp.host, cp.name, "Diagram", nil, func() {
			m.w.line("attribute redefines caption = " + stringLiteral(cp.caption) + ";")
			m.w.line("ref redefines source = " + m.diagramSource(dp, cp) + ";")
		})
	case "Image":
		m.blockPart(dp.host, cp.name, "Image", nil, func() {
			m.w.line("attribute redefines location = " + stringLiteral(cp.location) + ";")
			if cp.caption != "" {
				m.w.line("attribute redefines caption = " + stringLiteral(cp.caption) + ";")
			}
			if cp.alt != "" {
				m.w.line("attribute redefines alt = " + stringLiteral(cp.alt) + ";")
			}
		})
	}
	return cp.notes
}

// diagramSource names a Diagram block's view: by name where that reaches it,
// else by the feature chain from its anchor or the first usage under its package.
func (m *migration) diagramSource(dp *docPlan, cp *contentPlan) string {
	_, steps, _ := m.viewSteps(cp.source)
	var b strings.Builder
	switch {
	case cp.anchor != nil:
		b.WriteString(writeName(cp.anchor.name))
	case len(steps) == 1:
		return m.viewRef(cp.source, dp.host)
	default:
		b.WriteString(m.ref(steps[0].elem, dp.host))
		steps = steps[1:]
	}
	for _, s := range steps {
		b.WriteString(".")
		b.WriteString(writeName(s.name))
	}
	return b.String()
}

// nodeLabel names a node for a comment: its name, else its metaclass.
func nodeLabel(n *sysmlv1.Element) string {
	if n == nil {
		return ""
	}
	if n.Name != "" {
		return n.Name
	}
	return "<" + n.Type + ">"
}

// docEntry is the report row of a DocGen document, keyed by its application.
func (m *migration) docEntry(d *sysmlv1.DocGenDocument, v Verdict, target, note string) *Entry {
	id := d.Class.ID
	if d.Application != nil {
		id = d.Application.ID
	}
	return &Entry{ID: id, Kind: "«Document» Class", Name: qualifiedName(d.Class), Target: target, Verdict: v, Note: note}
}

// nodeEntry is the report row of a DocGen node or paragraph comment, keyed by
// its stereotype application so the node's own row is left alone.
func (m *migration) nodeEntry(n *sysmlv1.Element, app *sysmlv1.Stereotype, v Verdict, note string) *Entry {
	e := &Entry{Verdict: v, Note: note}
	if n != nil {
		e.ID, e.Kind, e.Name = n.ID, kindOf(n), qualifiedName(n)
	}
	if app != nil {
		e.ID = app.ID
		if n != nil {
			e.Kind = "«" + app.Name + "» " + n.Type
		} else {
			e.Kind = "«" + app.Name + "»"
		}
	}
	return e
}

// blockEntry is the report row of a written block: its node, mapped to the
// member of the Document standing for it.
func (m *migration) blockEntry(cp *contentPlan) *Entry {
	verdict := Mapped
	if len(cp.notes) > 0 {
		verdict = Approximated
	}
	var app *sysmlv1.Stereotype
	if cp.node != nil {
		app = cp.node.DocGen()
	}
	e := m.nodeEntry(cp.node, app, verdict, strings.Join(cp.notes, "; "))
	e.Target = "part " + cp.target
	if cp.query != "" {
		e.Note = joinNotes("its rows are the query "+writeName(cp.query), e.Note)
	}
	if cp.captionNote != "" {
		e.Note = joinNotes(cp.captionNote, e.Note)
	}
	return e
}
