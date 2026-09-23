package migrate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	// query and rows are the row query's reserved name and expression, for
	// the query-backed kinds.
	query string
	rows  qx
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
// content blocks; a view without a method contributes only its structure.
func (m *migration) planMethod(dp *docPlan, sec *sectionPlan) {
	v := sec.v
	if v.Method == nil {
		return
	}
	steps, end := m.model.DocGenChain(v.Method)
	if end != "" {
		sec.refused = "the method " + qualifiedName(v.Method) + " is not migrated: " + end
		m.report.Entries = append(m.report.Entries, *m.nodeEntry(v.Method, v.Method.DocGen(), Unmapped, sec.refused))
		return
	}
	c := &chain{m: m, dp: dp, sec: sec, active: []*sysmlv1.Element{v.Method}}
	c.roots(v.Exposed, "the view "+qualifiedName(v.Class)+" exposes")
	c.run(steps)
}

// collaboratorParagraph plans a paragraph the View Editor attached to a view:
// its comment's body, verbatim, or a comment saying why not.
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
	case p.Image && cp.text == "":
		cp.refused = "the attached image " + attachedFile(p.Comment) + " has no caption, and a Diagram shows a view, not an image file"
	case p.Image:
		cp.notes = append(cp.notes, "the attached image "+attachedFile(p.Comment)+" is not written, since a Diagram shows a view, not an image file; its caption stands as the paragraph")
	case cp.text == "":
		cp.refused = "the paragraph's comment has no body"
	}
	if cp.refused == "" {
		cp.name = sec.names.claim("paragraph")
	}
	if cp.refused != "" {
		m.report.Entries = append(m.report.Entries, *m.nodeEntry(p.Comment, p.Application, Unmapped, cp.refused))
	}
	return cp
}

// attachedFile names the file the tool attached to comment c, quoted; "an
// unnamed file" when the attachment names none.
func attachedFile(c *sysmlv1.Element) string {
	for _, s := range c.Stereotypes {
		if s.Name == "AttachedFile" && s.Namespace == sysmlv1.MagicDrawProfileNS {
			if f := strings.TrimSpace(s.Tag("file")); f != "" {
				return strconv.Quote(f)
			}
		}
	}
	return "an unnamed file"
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
	// names but an Image shows.
	diagrams []*sysmlv1.Diagram
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

// roots sets the chain's elements to refs, named by qualified name; a ref
// that resolves to no written element breaks the chain.
func (c *chain) roots(refs []sysmlv1.ElementRef, role string) {
	c.ctx, c.diagrams, c.broken = qx{}, nil, ""
	if len(refs) == 0 {
		return
	}
	var names []string
	for _, ref := range refs {
		if d := c.m.model.Diagram(ref.ID); d != nil {
			c.diagrams = append(c.diagrams, d)
			continue
		}
		if ref.Element == nil {
			c.broken = role + " " + ref.ID + ", which resolves to no element"
			return
		}
		name, why := c.m.namedRoot(ref, "element")
		if why != "" {
			c.broken = role + " " + kindOf(ref.Element) + " " + qualifiedName(ref.Element) + ", which is not migrated"
			return
		}
		names = append(names, name)
	}
	if len(names) > 0 {
		c.ctx = qcall("Named", qstrs("qualifiedName", names...))
	}
}

// empty reports whether the chain has no elements to work on.
func (c *chain) empty() bool { return c.ctx.op == "" && c.ctx.lit == "" }

func (c *chain) run(steps []*sysmlv1.DocGenStep) {
	for _, s := range steps {
		c.step(s)
	}
}

// step lowers one node: a collect, filter or sort step transforms the
// elements; a presentation node adds a block; a group recurses.
func (c *chain) step(s *sysmlv1.DocGenStep) {
	if s.Malformed != "" {
		c.fail(s, s.Malformed)
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
	case "FilterByMetaclasses":
		c.filterTypes(s, "metaclasses")
	case "FilterByStereotypes":
		c.filterTypes(s, "stereotypes")
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
			c.fail(s, "the node "+qualifiedName(s.Node)+" carries no DocGen stereotype")
		}
	default:
		c.fail(s, "no query operation or content block stands for «"+s.Kind+"»")
	}
}

// fail records why a step is not migrated: a query step breaks the chain for
// what follows, a presentation node stands as a comment in the section.
func (c *chain) fail(s *sysmlv1.DocGenStep, why string) {
	switch s.Kind {
	case "TableStructure", "BulletedList", "Paragraph", "Image", "Dynamic_View", "DynamicView":
		c.refuse(s, why)
	default:
		if c.broken == "" {
			c.broken = "«" + c.kind(s) + "» " + qualifiedName(s.Node) + " is not migrated: " + why
		}
		c.m.report.Entries = append(c.m.report.Entries, *c.m.nodeEntry(s.Node, s.Application, Unmapped, why))
	}
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
	case c.empty():
		c.refuse(s, "it shows no element: the view exposes nothing and the node targets nothing")
		return false
	}
	return true
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
func (c *chain) collect(s *sysmlv1.DocGenStep, op string) {
	if c.broken != "" || c.empty() {
		return
	}
	depth, why := c.depth(s)
	if why != "" {
		c.fail(s, why)
		return
	}
	args := []qarg{qarg1("source", c.ctx)}
	if depth > 0 {
		args = append(args, qint1("maxDepth", depth))
	}
	c.ctx = qcall(op, args...)
}

// collectRelated lowers CollectByDirectedRelationshipStereotypes to a walk of
// each supported relationship kind, united.
func (c *chain) collectRelated(s *sysmlv1.DocGenStep) {
	if c.broken != "" || c.empty() {
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
	if c.broken != "" || c.empty() {
		return
	}
	refs := c.m.model.TagRefs(s.Application, tag)
	if len(refs) == 0 {
		c.fail(s, "it names no "+strings.TrimSuffix(tag, "es")+"")
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

// labels names the type refs as a reader knows them.
func (c *chain) labels(refs []sysmlv1.ElementRef) []string {
	var out []string
	for _, ref := range refs {
		out = append(out, c.m.typeFilter(ref).label)
	}
	return out
}

// filterNames lowers FilterByNames: each name is a whole-string regular
// expression, as DocGen matches them.
func (c *chain) filterNames(s *sysmlv1.DocGenStep) {
	if c.broken != "" || c.empty() {
		return
	}
	names := s.Application.Tags["names"]
	if len(names) == 0 {
		c.fail(s, "it names no name pattern")
		return
	}
	var matches []qx
	for _, n := range names {
		pattern := "^(?:" + n + ")$"
		if _, err := regexp.Compile(pattern); err != nil {
			c.fail(s, "the name pattern "+strconv.Quote(n)+" is not a regular expression the query can match")
			return
		}
		matches = append(matches, qcall("WhereName", qarg1("source", c.ctx), qarg1("operator", qstr("matches")), qarg1("value", qstr(pattern))))
	}
	kept := union(matches)
	if s.Application.Tag("include") == "false" {
		c.ctx = qcall("Except", qarg1("source", c.ctx), qarg1("exclude", kept))
		return
	}
	c.ctx = kept
}

// sort lowers a sort step to OrderBy over a query property.
func (c *chain) sort(s *sysmlv1.DocGenStep, property string) {
	if c.broken != "" || c.empty() {
		return
	}
	dir := "ascending"
	if s.Application.Tag("reverse") == "true" {
		dir = "descending"
	}
	c.ctx = qcall("OrderBy", qarg1("source", c.ctx), qarg1("property", qstr(property)),
		qarg1("direction", qstr(dir)), qarg1("missing", qstr("last")), qarg1("multiple", qstr("first")))
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
	for _, branch := range s.Branches {
		sub := c.sub()
		sub.run(branch)
		if sub.broken != "" {
			c.broken = sub.broken
			return
		}
		for _, n := range sub.notes {
			c.note(n)
		}
		if !sub.empty() {
			results = append(results, sub.ctx)
		}
	}
	if s.Kind != "Union" {
		c.fail(s, "its branches rejoin by "+strings.ToLower(s.Kind)+", which only a Union spelling exists for")
		return
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
		c.fail(s, why)
		return
	}
	steps, end := c.m.model.DocGenChain(body)
	if end != "" {
		c.fail(s, "its body "+qualifiedName(body)+" is not migrated: "+end)
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
	c.ctx, c.diagrams, c.broken = sub.ctx, sub.diagrams, sub.broken
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
	var props []string
	var cols []qx
	var notes []string
	claimed := columnNames{}
	for _, col := range colSteps {
		prop, expr, why := c.column(col)
		switch {
		case why != "":
			notes = append(notes, "the column «"+c.kind(col)+"» "+qualifiedName(col.Node)+" is not written: "+why)
		case prop != "":
			if !contains(props, prop) {
				props = append(props, prop)
			}
		default:
			cols = append(cols, qcall("Column", qarg1("name", qstr(claimed.claim(expr.name))), qarg1("expression", qlit(expr.expression))))
		}
	}
	if s.Application.Tag("includeDoc") == "true" && !contains(props, "documentation") {
		props = append(props, "documentation")
	}
	if len(props) == 0 && len(cols) == 0 {
		why := "none of its columns reads what a query can"
		if len(notes) > 0 {
			why += ": " + strings.Join(notes, "; ")
		}
		c.refuse(s, why)
		return
	}
	args := []qarg{qarg1("source", c.ctx)}
	if len(props) > 0 {
		args = append(args, qstrs("properties", props...))
	}
	if len(cols) > 0 {
		args = append(args, qlist("columns", cols...))
	}
	cp := c.block(s, "Table", c.caption(s, "Table"), qcall("Project", args...))
	cp.notes = append(cp.notes, notes...)
	if s.Application.Tag("loop") == "true" {
		cp.notes = append(cp.notes, "the table loops over its elements one table each; one table lists them together")
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
		key, f, why := c.m.columnKey(sysmlv1.Column{Kind: sysmlv1.ColumnFeature, Feature: refs[0], ID: refs[0].ID}, c.dp.host)
		if why != "" {
			return "", expr, why
		}
		c.m.expose(f, "a column of a document table reads it")
		name := c.caption(col, key)
		return "", columnExpr{name: name, expression: c.m.ref(f, c.dp.host) + " ?? \"\""}, ""
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
	if body := commentText(a.Tag("body")); body != "" {
		cp := &contentPlan{kind: "Paragraph", node: s.Node, label: "«Paragraph» " + s.Node.Type, text: body}
		cp.name = c.sec.names.claim("paragraph")
		c.sec.content = append(c.sec.content, cp)
		return
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
// elements, showing its migrated view.
func (c *chain) image(s *sysmlv1.DocGenStep) {
	if c.broken != "" {
		c.refuse(s, "the diagrams it shows pass through "+c.broken)
		return
	}
	if len(c.diagrams) == 0 {
		c.refuse(s, "it shows no diagram: only a diagram the view exposes or the node targets directly has a view to show")
		return
	}
	captions := s.Application.Tags["captions"]
	show := s.Application.Tag("showCaptions") != "false"
	for i, d := range c.diagrams {
		v := c.m.viewOf[d]
		if v == nil || !v.placed {
			c.refuse(s, "the Diagram '"+d.Name+"' is not written as a view")
			continue
		}
		if rendering(d) == textualRendering {
			c.refuse(s, "the "+diagramKind(d)+" '"+d.Name+"' is a view rendered as textual notation, which a document does not draw")
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
		cp.caption = strings.TrimSpace(d.Name)
		if show && i < len(captions) && strings.TrimSpace(captions[i]) != "" {
			cp.caption = strings.TrimSpace(captions[i])
		}
		cp.name = c.sec.names.claim("diagram")
		c.sec.content = append(c.sec.content, cp)
	}
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
			why = "its body " + qualifiedName(body) + " is not migrated: " + why
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
			m.w.line("attribute redefines title = " + stringLiteral(dp.root.title) + ";")
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
			m.w.line("attribute redefines title = " + stringLiteral(child.title) + ";")
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
			m.w.line("attribute redefines title = " + stringLiteral(cp.section.title) + ";")
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
	return e
}
