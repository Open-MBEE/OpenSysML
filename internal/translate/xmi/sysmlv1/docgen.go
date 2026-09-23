package sysmlv1

import (
	"fmt"
	"strconv"
)

// DocGenDocument is one MDK DocGen document: a class carrying the Document
// stereotype of either DocGen profile, read into its view tree.
type DocGenDocument struct {
	// Class is the document class; Root its view tree, of which the
	// document class is the first view.
	Class *Element
	Root  *DocGenView
	// Application is the Document application that names the class.
	Application *Stereotype
}

// DocGenView is one view of a document: a class the SysML View stereotype
// applies to, placed by the composite properties of its parent view.
type DocGenView struct {
	// Class is the view class.
	Class *Element
	// Viewpoint is the viewpoint the view conforms to; nil when none.
	Viewpoint *Element
	// Method is the viewpoint's method activity, the behavior of its
	// operation named View or its method tag; nil when the viewpoint has none.
	Method *Element
	// Exposed are the elements the view exposes or imports, in document order.
	Exposed []ElementRef
	// Paragraphs are the collaborator paragraphs placed in this view, in the
	// order they are shown.
	Paragraphs []*DocGenParagraph
	// Children are the child views, in property order.
	Children []*DocGenView
	// Malformed lists what could not be read.
	Malformed []string
}

// DocGenParagraph is one collaborator paragraph: a comment the collaborator
// profile places in a view (its ownerId) of a document (its viewId), after
// the paragraph its siblingId names.
type DocGenParagraph struct {
	// Application is the CollaboratorParagraph or CollaboratorImageParagraph application.
	Application *Stereotype
	// Image reports a CollaboratorImageParagraph: the comment carries an attached image file.
	Image bool
	// Comment is the comment shown; nil when the application's base is dangling.
	Comment *Element
	// Malformed is why the paragraph cannot be shown, "" when it can.
	Malformed string
	// Placed reports whether siblingId named a paragraph of the same view,
	// which this one then follows; false also when siblingId is empty.
	Placed bool
}

// DocGenStep is one node of a DocGen activity chain: a collect, filter or
// sort step, a presentation node, a group, or a join of parallel branches.
type DocGenStep struct {
	// Node is the activity node.
	Node *Element
	// Kind is the DocGen stereotype the node or its called behavior carries,
	// "" when neither carries one.
	Kind string
	// Application is that stereotype's application.
	Application *Stereotype
	// Behavior is the behavior a CallBehaviorAction calls; nil otherwise.
	Behavior *Element
	// Targets are the node's explicit targets: its targets tag or its Expose
	// suppliers; nil when the step works on what the chain feeds it.
	Targets []ElementRef
	// Branches are the parallel chains a fork opened, each ending at the
	// node that rejoins them; Kind then names the join: "Union" for a merge,
	// "Intersection" for a join, "XOR" for a decision.
	Branches [][]*DocGenStep
	// Malformed is why the step could not be read, "" when it could.
	Malformed string
}

// Int reads an integer tag of the step's application; def when absent.
func (s *DocGenStep) Int(name string, def int) (int, error) {
	if s.Application == nil {
		return def, nil
	}
	v := s.Application.Tag(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not an integer", name, v)
	}
	return n, nil
}

// Flag reads a boolean tag of the step's application; def when absent.
func (s *DocGenStep) Flag(name string, def bool) bool {
	if s.Application == nil {
		return def
	}
	switch s.Application.Tag(name) {
	case "true":
		return true
	case "false":
		return false
	}
	return def
}

// docGenNamespaces are the two profiles DocGen content is read from.
var docGenNamespaces = []string{DocGenNS, DocGenCollaboratorNS}

// DocGen returns the DocGen application on e, from either DocGen profile.
func (e *Element) DocGen() *Stereotype {
	if e == nil {
		return nil
	}
	for _, s := range e.Stereotypes {
		for _, ns := range docGenNamespaces {
			if s.Namespace == ns {
				return s
			}
		}
	}
	return nil
}

// readDocuments reads every DocGen document once every document is read; a
// class several applications name is one document.
func (m *Model) readDocuments() {
	r := &docGenReader{m: m, comments: map[string][]*Stereotype{}, placed: map[*Stereotype]bool{}}
	for _, s := range m.Stereotypes {
		if isCollaboratorParagraph(s) {
			owner := s.Tag("ownerId")
			r.comments[owner] = append(r.comments[owner], s)
		}
	}
	seen := map[*Element]bool{}
	for _, s := range m.Stereotypes {
		if s.Name != "Document" || s.Base == nil || s.Base.Type != "Class" || seen[s.Base] {
			continue
		}
		if s.Namespace != DocGenNS && s.Namespace != DocGenCollaboratorNS {
			continue
		}
		seen[s.Base] = true
		r.doc = s.Base
		doc := &DocGenDocument{Class: s.Base, Application: s}
		doc.Root = r.view(s.Base, nil, map[*Element]bool{}, true)
		m.Documents = append(m.Documents, doc)
	}
	for _, s := range m.Stereotypes {
		if !isCollaboratorParagraph(s) || r.placed[s] {
			continue
		}
		p := &DocGenParagraph{Application: s, Comment: s.Base, Image: s.Name == "CollaboratorImageParagraph"}
		if id := s.Tag("viewId"); id != "" && !seen[m.Lookup(id)] {
			p.Malformed = fmt.Sprintf("viewId %q names no document", id)
		} else {
			p.Malformed = fmt.Sprintf("ownerId %q names no view of the document", s.Tag("ownerId"))
		}
		m.StrayParagraphs = append(m.StrayParagraphs, p)
	}
}

// isCollaboratorParagraph matches the collaborator profile's paragraph applications.
func isCollaboratorParagraph(s *Stereotype) bool {
	return s.Namespace == DocGenCollaboratorNS && (s.Name == "CollaboratorParagraph" || s.Name == "CollaboratorImageParagraph")
}

type docGenReader struct {
	m        *Model
	comments map[string][]*Stereotype
	// doc is the document class whose views are being read; placed are the
	// paragraph applications some view of some document has shown.
	doc    *Element
	placed map[*Stereotype]bool
}

// isDocGenView reports whether e is a view class: the SysML View stereotype
// or the DocGen view stereotype applies to it.
func isDocGenView(e *Element) bool {
	return e.Type == "Class" && (isSysMLStereotyped(e, "View") || e.DocGenView())
}

// DocGenView reports whether the DocGen profile's own view stereotype applies
// to e, which makes a class a view as the SysML «View» does.
func (e *Element) DocGenView() bool {
	s := e.DocGen()
	return s != nil && s.Namespace == DocGenNS && (s.Name == "view" || s.Name == "Dynamic_View")
}

// IsDocGenProfile reports whether ns is one of the DocGen profiles the
// document mapping reads.
func IsDocGenProfile(ns string) bool {
	return ns == DocGenNS || ns == DocGenCollaboratorNS
}

// view reads one view placed by property p of its parent (nil at the root)
// and, when recurse, its children; a view already on the path is not
// entered twice.
func (r *docGenReader) view(class, p *Element, path map[*Element]bool, recurse bool) *DocGenView {
	m := r.m
	v := &DocGenView{Class: class, Paragraphs: r.paragraphs(class)}
	path[class] = true
	defer delete(path, class)
	for _, g := range class.Owned("generalization") {
		if !isSysMLStereotyped(g, "Conform") {
			continue
		}
		if general := m.Ref(g, "general"); general != nil {
			v.Viewpoint = general
		} else {
			v.Malformed = append(v.Malformed, fmt.Sprintf("Conform general %q names no element", g.Attrs["general"]))
		}
	}
	if v.Viewpoint != nil {
		v.Method = m.viewpointMethod(v.Viewpoint)
	}
	v.Exposed = m.exposed(class)
	if p != nil && composite(p) {
		v.Exposed = append(v.Exposed, m.exposed(p)...)
	}
	if !recurse {
		return v
	}
	for _, p := range class.Owned("ownedAttribute") {
		t := m.Ref(p, "type")
		if p.Type != "Property" || t == nil || !isDocGenView(t) {
			continue
		}
		if path[t] {
			v.Malformed = append(v.Malformed, fmt.Sprintf("view %s contains itself", t.Name))
			continue
		}
		aggregation := p.Attrs["aggregation"]
		v.Children = append(v.Children, r.view(t, p, path, aggregation != "" && aggregation != "none"))
	}
	return v
}

// paragraphs reads the collaborator paragraphs placed in a view of the
// current document (viewId, when set, names the document class): in
// application order, each moved behind the paragraph its siblingId names.
func (r *docGenReader) paragraphs(class *Element) []*DocGenParagraph {
	byComment := map[string]*DocGenParagraph{}
	var out []*DocGenParagraph
	for _, s := range r.comments[class.ID] {
		if id := s.Tag("viewId"); id != "" && id != r.doc.ID {
			continue
		}
		r.placed[s] = true
		p := &DocGenParagraph{Application: s, Comment: s.Base, Image: s.Name == "CollaboratorImageParagraph"}
		switch {
		case s.Base == nil:
			p.Malformed = fmt.Sprintf("base_Element %q names no element", s.BaseID)
		case s.Base.Type != "Comment":
			p.Malformed = fmt.Sprintf("base_Element names a %s, not a Comment", s.Base.Type)
		case s.Tag("property") != "" && s.Tag("property") != collaboratorBody:
			p.Malformed = fmt.Sprintf("property %q is not the comment body", s.Tag("property"))
		}
		if s.BaseID != "" {
			byComment[s.BaseID] = p
		}
		out = append(out, p)
	}
	followers := map[*DocGenParagraph][]*DocGenParagraph{}
	var heads []*DocGenParagraph
	for _, p := range out {
		after := byComment[p.Application.Tag("siblingId")]
		if after == nil || after == p {
			heads = append(heads, p)
			continue
		}
		p.Placed = true
		followers[after] = append(followers[after], p)
	}
	ordered := make([]*DocGenParagraph, 0, len(out))
	placed := map[*DocGenParagraph]bool{}
	var place func(p *DocGenParagraph)
	place = func(p *DocGenParagraph) {
		if placed[p] {
			return
		}
		placed[p] = true
		ordered = append(ordered, p)
		for _, f := range followers[p] {
			place(f)
		}
	}
	for _, p := range heads {
		place(p)
	}
	// Paragraphs only reachable through a siblingId cycle keep document order.
	for _, p := range out {
		if !placed[p] {
			p.Placed = false
			place(p)
		}
	}
	return ordered
}

// collaboratorBody is the property tag naming the comment body.
const collaboratorBody = "META:QPROP:Element:body"

// composite reports whether a property is a composite end.
func composite(p *Element) bool { return p.Attrs["aggregation"] == "composite" }

// exposed lists the suppliers of e's Expose dependencies and the targets of
// its element and package imports, as DocGen feeds them to the method.
func (m *Model) exposed(e *Element) []ElementRef {
	var out []ElementRef
	for _, imp := range e.Owned("elementImport") {
		out = append(out, m.elementRefs(imp, "importedElement")...)
	}
	for _, imp := range e.Owned("packageImport") {
		out = append(out, m.elementRefs(imp, "importedPackage")...)
	}
	for _, dep := range m.dependenciesOf(e) {
		if isSysMLStereotyped(dep, "Expose") {
			out = append(out, m.elementRefs(dep, "supplier")...)
		}
	}
	return out
}

// dependenciesOf lists the dependencies whose client is e, wherever the
// document packages them, in document order.
func (m *Model) dependenciesOf(e *Element) []*Element {
	if m.clients == nil {
		m.clients = map[*Element][]*Element{}
		for _, root := range m.Roots {
			m.indexClients(root)
		}
	}
	return m.clients[e]
}

func (m *Model) indexClients(e *Element) {
	if e.Type == "Dependency" || e.Type == "Abstraction" || e.Type == "Realization" {
		for _, c := range m.Refs(e, "client") {
			m.clients[c] = append(m.clients[c], e)
		}
	}
	for _, c := range e.Children {
		m.indexClients(c)
	}
}

// elementRefs lists a role's targets with their raw ids, dangling ones included.
func (m *Model) elementRefs(e *Element, role string) []ElementRef {
	var out []ElementRef
	for _, id := range e.RefIDs(role) {
		out = append(out, ElementRef{ID: id, Element: m.Lookup(id)})
	}
	return out
}

// viewpointMethod finds the method activity of a viewpoint: the behaviors
// of its Viewpoint application's method tag, else its classifier behavior,
// else the owned behaviors that specify its operation named View.
func (m *Model) viewpointMethod(vp *Element) *Element {
	if s := vp.Stereotype("Viewpoint"); s != nil {
		for _, id := range s.IDs("method") {
			if b := m.Lookup(id); b != nil && b.Type == "Activity" {
				return b
			}
		}
	}
	if b := m.Ref(vp, "classifierBehavior"); b != nil && b.Type == "Activity" {
		return b
	}
	for _, b := range vp.Owned("ownedBehavior") {
		spec := m.Ref(b, "specification")
		if b.Type == "Activity" && spec != nil && spec.Parent == vp && spec.Name == "View" {
			return b
		}
	}
	return nil
}

// isSysMLStereotyped reports whether a stereotype of the SysML profile
// with the given name applies to e.
func isSysMLStereotyped(e *Element, name string) bool {
	for _, s := range e.Stereotypes {
		if s.Name == name && IsSysMLNamespace(s.Namespace) {
			return true
		}
	}
	return false
}

// DocGenChain reads the activity chain of a DocGen behavior or structured
// node: the steps reached from its initial node along single control flows,
// with parallel branches folded into the step that rejoins them.
func (m *Model) DocGenChain(a *Element) ([]*DocGenStep, string) {
	var initial *Element
	for _, n := range a.Owned("node") {
		if n.Type == "InitialNode" {
			initial = n
			break
		}
	}
	if initial == nil {
		return nil, "no initial node"
	}
	w := &chainWalker{m: m, out: m.flows(a, "source", "target"), in: m.flows(a, "target", "source")}
	steps, end := w.walk(initial, nil)
	return steps, end
}

type chainWalker struct {
	m       *Model
	out, in map[*Element][]*Element
	seen    map[*Element]bool
}

// flows indexes the edges of an activity or structured node by one end,
// listing the other end in edge order.
func (m *Model) flows(a *Element, from, to string) map[*Element][]*Element {
	idx := map[*Element][]*Element{}
	for _, e := range a.Owned("edge") {
		s, t := m.Ref(e, from), m.Ref(e, to)
		if s != nil && t != nil {
			idx[s] = append(idx[s], t)
		}
	}
	return idx
}

// walk follows single control flows from cur until the chain ends or stop
// is reached; it returns the steps and why the chain ended, "" when cleanly.
func (w *chainWalker) walk(cur *Element, stop *Element) ([]*DocGenStep, string) {
	if w.seen == nil {
		w.seen = map[*Element]bool{}
	}
	var steps []*DocGenStep
	for {
		next := w.out[cur]
		if len(next) == 0 {
			return steps, ""
		}
		if len(next) > 1 && cur.Type != "ForkNode" {
			return steps, fmt.Sprintf("%s has %d outgoing flows", nodeName(cur), len(next))
		}
		if cur.Type == "ForkNode" {
			step, join, why := w.fork(cur, next)
			if why != "" {
				return steps, why
			}
			steps = append(steps, step)
			cur = join
			continue
		}
		n := next[0]
		if n == stop {
			return steps, ""
		}
		if w.seen[n] {
			return steps, fmt.Sprintf("%s is reached twice", nodeName(n))
		}
		w.seen[n] = true
		if n.Type == "ActivityFinalNode" || n.Type == "FlowFinalNode" {
			return steps, ""
		}
		if n.Type != "ForkNode" {
			steps = append(steps, w.m.docGenStep(n))
		}
		cur = n
	}
}

// fork reads the branches of a fork up to the node that rejoins them.
func (w *chainWalker) fork(fork *Element, heads []*Element) (*DocGenStep, *Element, string) {
	step := &DocGenStep{Node: fork, Application: fork.DocGen()}
	if step.Application != nil {
		step.Kind = step.Application.Name
	}
	var join *Element
	for _, head := range heads {
		branch, end := w.branch(head, &join)
		if end != "" {
			return nil, nil, end
		}
		step.Branches = append(step.Branches, branch)
	}
	if join == nil {
		return nil, nil, fmt.Sprintf("%s's branches never rejoin", nodeName(fork))
	}
	if kind := joinKind(join); kind != "" {
		step.Kind = kind
	} else {
		step.Malformed = fmt.Sprintf("%s joins branches without a Union", nodeName(join))
	}
	return step, join, ""
}

// branch walks one fork branch until a node with several incoming flows,
// which every branch must share.
func (w *chainWalker) branch(head *Element, join **Element) ([]*DocGenStep, string) {
	var steps []*DocGenStep
	cur := head
	for {
		if len(w.in[cur]) > 1 {
			if *join == nil {
				*join = cur
			} else if *join != cur {
				return steps, fmt.Sprintf("branches rejoin at both %s and %s", nodeName(*join), nodeName(cur))
			}
			return steps, ""
		}
		if w.seen[cur] {
			return steps, fmt.Sprintf("%s is reached twice", nodeName(cur))
		}
		w.seen[cur] = true
		steps = append(steps, w.m.docGenStep(cur))
		next := w.out[cur]
		if len(next) != 1 {
			return steps, fmt.Sprintf("%s has %d outgoing flows inside a fork", nodeName(cur), len(next))
		}
		cur = next[0]
	}
}

// joinKind names the set operation a rejoining node performs.
func joinKind(n *Element) string {
	switch n.Type {
	case "MergeNode":
		return "Union"
	case "JoinNode":
		return "Intersection"
	case "DecisionNode":
		return "XOR"
	}
	return ""
}

// docGenStep types one activity node.
func (m *Model) docGenStep(n *Element) *DocGenStep {
	s := &DocGenStep{Node: n}
	if n.Type == "CallBehaviorAction" {
		s.Behavior = m.Ref(n, "behavior")
		if s.Behavior == nil && len(n.RefIDs("behavior")) > 0 {
			s.Malformed = fmt.Sprintf("behavior %q names no element", n.RefIDs("behavior")[0])
		}
	}
	s.Application = n.DocGen()
	if s.Application == nil && s.Behavior != nil {
		s.Application = s.Behavior.DocGen()
	}
	if s.Application != nil {
		s.Kind = s.Application.Name
	}
	s.Targets = m.stepTargets(n, s.Behavior)
	return s
}

// stepTargets reads a node's explicit targets as DocGen does: its targets
// tag, else its Expose suppliers, else the same of its called behavior.
func (m *Model) stepTargets(n, behavior *Element) []ElementRef {
	for _, e := range []*Element{n, behavior} {
		if e == nil {
			continue
		}
		if s := e.DocGen(); s != nil {
			if refs := m.TagRefs(s, "targets"); len(refs) > 0 {
				return refs
			}
		}
		if refs := m.exposed(e); len(refs) > 0 {
			return refs
		}
	}
	return nil
}

func nodeName(n *Element) string {
	if n.Name != "" {
		return fmt.Sprintf("%s %q", n.Type, n.Name)
	}
	return n.Type
}
