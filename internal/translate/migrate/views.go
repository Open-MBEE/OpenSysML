package migrate

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// viewBody writes a view usage: its doc, the viewpoints it conforms to by
// generalization or tag, its members, and what its dependencies place in it.
func (m *migration) viewBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	m.additionalViewpointConformances(e)
	m.viewGeneralizations(e)
	m.taggedViewpoints(e)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeComments(e)
	m.scope = saved
}

// conforms reports whether a generalization is a v1 Conform: stereotyped so,
// or generalizing a viewpoint, as the profile spells it since SysML 1.4.
func (m *migration) conforms(g *sysmlv1.Element) bool {
	if has(g, "Conform") {
		return true
	}
	t := m.model.Ref(g, "general")
	if t == nil {
		return false
	}
	cat, _ := m.classify(t)
	return cat == catViewpoint
}

// viewGeneralizations reports a view's viewpoint conformances and written view bases.
func (m *migration) viewGeneralizations(e *sysmlv1.Element) {
	for _, g := range e.Owned("generalization") {
		if !m.conforms(g) {
			base := m.model.Ref(g, "general")
			if base != nil && m.written(base) {
				if cat, _ := m.classify(base); cat == catView {
					if general, _ := m.general(e, g, catView); general != "" {
						m.add(g, Mapped, m.v2Name(e), "")
					}
				}
			}
			continue
		}
		vp := m.model.Ref(g, "general")
		if note := m.viewpointNote(vp, e); note != "" {
			m.unmapped(g, note)
			continue
		}
		m.add(g, Mapped, m.v2Name(e), "")
	}
}

// viewpointNote says why vp cannot be satisfied: it is absent, external, not
// migrated or not a viewpoint; "" when a view can satisfy it.
func (m *migration) viewpointNote(vp, view *sysmlv1.Element) string {
	switch {
	case vp == nil:
		return "the viewpoint is not in the document"
	case vp.IsProxy():
		return "the viewpoint " + qualifiedName(vp) + livesOutsideDocument
	case !m.written(vp):
		_, why := m.classify(vp)
		return joinNotes("the viewpoint "+qualifiedName(vp)+" is not migrated", why)
	}
	if cat, _ := m.classify(vp); cat != catViewpoint {
		return qualifiedName(vp) + " becomes a " + cat.keyword() + ", which a view cannot satisfy"
	}
	return ""
}

// featuredNote says why a usage cannot be named from scope: a feature of a
// definition is accessible to that definition's members alone, not by
// qualified name from elsewhere. "" when the usage is accessible.
func (m *migration) featuredNote(u, scope *sysmlv1.Element) string {
	def := m.featuringDef(u)
	if def == nil {
		return ""
	}
	for cur := scope; cur != nil; cur = cur.Parent {
		if cur == def {
			return ""
		}
		if !m.isUsage(cur) {
			break
		}
	}
	cat, _ := m.classify(u)
	defCat, _ := m.classify(def)
	return "the " + cat.keyword() + " " + qualifiedName(u) + " is a feature of the " + defCat.keyword() + " " + qualifiedName(def) + ", which only its members can name"
}

// featuringDef is the innermost definition u is a feature of, through any
// usages between them; nil when u is featured by packages alone.
func (m *migration) featuringDef(u *sysmlv1.Element) *sysmlv1.Element {
	for cur := u.Parent; cur != nil; cur = cur.Parent {
		if cur.Parent == nil && cur.Type == "Model" {
			return nil
		}
		if m.isUsage(cur) {
			continue
		}
		if cat, _ := m.classify(cur); cat != catPackage {
			return cur
		}
	}
	return nil
}

// usageChain spells the feature path from definition d to its nested usage u.
func (m *migration) usageChain(u, d *sysmlv1.Element) string {
	var path []string
	for cur := u; cur != nil && cur != d; cur = cur.Parent {
		path = append(path, writeName(m.nameFor(cur)))
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return strings.Join(path, ".")
}

// taggedViewpoints accounts for the viewpoints named by «View» tags.
func (m *migration) taggedViewpoints(e *sysmlv1.Element) {
	v := stereo(e, "View")
	if v == nil {
		return
	}
	for _, tag := range viewpointTags {
		for _, id := range v.IDs(tag) {
			vp := m.model.Lookup(id)
			if vp == nil {
				m.downgrade(e, "the "+tag+" tag names "+id+notInDocument)
				continue
			}
			if note := m.viewpointNote(vp, e); note != "" {
				m.downgrade(e, "the "+tag+" tag is not written: "+note)
				continue
			}
		}
	}
}

// conformedViewpoints returns the first valid viewpoint reference for a view.
func (m *migration) conformedViewpoints(e *sysmlv1.Element) string {
	conformances := m.viewpointConformances(e)
	if len(conformances) == 0 {
		return ""
	}
	if conformances[0].kind == inheritedConformance {
		return ""
	}
	return m.ref(conformances[0].viewpoint, e)
}

// viewpointConformances returns valid viewpoint sources in migration precedence order.
func (m *migration) viewpointConformances(e *sysmlv1.Element) []viewpointConformance {
	return m.viewpointConformancesSeen(e, map[*sysmlv1.Element]bool{})
}

// viewpointConformancesSeen includes effective conformances without following a cycle.
func (m *migration) viewpointConformancesSeen(e *sysmlv1.Element, visiting map[*sysmlv1.Element]bool) []viewpointConformance {
	if e == nil || visiting[e] {
		return nil
	}
	visiting[e] = true
	defer delete(visiting, e)

	var out []viewpointConformance
	seen := map[*sysmlv1.Element]bool{}
	add := func(vp, source *sysmlv1.Element, kind conformanceSource) {
		if vp == nil || seen[vp] || m.viewpointNote(vp, e) != "" {
			return
		}
		seen[vp] = true
		out = append(out, viewpointConformance{viewpoint: vp, source: source, kind: kind})
	}
	for _, g := range e.Owned("generalization") {
		if m.conforms(g) {
			continue
		}
		base := m.model.Ref(g, "general")
		if base == nil || !m.written(base) {
			continue
		}
		if cat, _ := m.classify(base); cat != catView {
			continue
		}
		inherited := m.viewpointConformancesSeen(base, visiting)
		if len(inherited) > 0 {
			add(inherited[0].viewpoint, g, inheritedConformance)
		}
	}
	for _, g := range e.Owned("generalization") {
		if m.conforms(g) {
			add(m.model.Ref(g, "general"), g, generalizationConformance)
		}
	}
	if v := stereo(e, "View"); v != nil {
		for _, tag := range viewpointTags {
			for _, id := range v.IDs(tag) {
				add(m.model.Lookup(id), e, taggedConformance)
			}
		}
	}
	for _, conformance := range m.conformed[e] {
		add(conformance.viewpoint, conformance.source, dependencyConformance)
	}
	return out
}

// additionalViewpointConformances reports and comments conformance beyond a view's first viewpoint.
func (m *migration) additionalViewpointConformances(e *sysmlv1.Element) {
	conformances := m.viewpointConformances(e)
	if len(conformances) < 2 {
		return
	}
	for _, conformance := range conformances[1:] {
		if conformance.kind == inheritedConformance {
			continue
		}
		note := "v2 types a view by one view definition, so its conformance to " +
			qualifiedName(conformance.viewpoint) + " is kept as a comment"
		m.w.line("/* conforms to " + m.ref(conformance.viewpoint, e) + " */")
		switch conformance.kind {
		case generalizationConformance, dependencyConformance:
			m.downgrade(conformance.source, note)
		case taggedConformance:
			m.downgrade(e, note)
		}
	}
}

// conformanceSource identifies how a view names a viewpoint it conforms to.
type conformanceSource uint8

const (
	generalizationConformance conformanceSource = iota
	taggedConformance
	dependencyConformance
	inheritedConformance
)

// viewpointConformance records a view's viewpoint and the source that names it.
type viewpointConformance struct {
	viewpoint *sysmlv1.Element
	source    *sysmlv1.Element
	kind      conformanceSource
}

const (
	livesOutsideDocument = " lives outside the document and is not written"
	notInDocument        = ", which is not in the document"
	exposedSubject       = "the exposed "
	stakeholderSubject   = "the stakeholder "
)

// viewpointTags are the «View» tags naming the viewpoint a view conforms to.
var viewpointTags = []string{"viewpoint", "viewPoint"}

// placeConform registers a «Conform» dependency as a viewpoint type of each view
// it has as client; the report entry is written where it stands.
func (m *migration) placeConform(d *sysmlv1.Element) {
	pl := &placement{}
	m.unplaced[d] = pl
	pairs, failed, note := m.dependencyPairs(d)
	pl.failed = failed
	if note != "" {
		pl.notes = append(pl.notes, note)
	}
	for _, p := range pairs {
		if cat, _ := m.classify(p.client); cat != catView {
			pl.failed++
			pl.notes = append(pl.notes, "the client "+qualifiedName(p.client)+" is not a view: v2 lets a view alone satisfy a viewpoint")
			continue
		}
		if n := m.viewpointNote(p.supplier, p.client); n != "" {
			pl.failed++
			pl.notes = append(pl.notes, n)
			continue
		}
		pl.write(m.v2Name(p.client))
		view, vp := p.client, p.supplier
		m.conformed[view] = append(m.conformed[view], viewpointConformance{
			viewpoint: vp,
			source:    d,
			kind:      dependencyConformance,
		})
	}
}

// placeExpose registers an «Expose» dependency as expose members in the body
// of each view it has as client, and accounts for every pair none can carry.
func (m *migration) placeExpose(d *sysmlv1.Element) {
	pl := &placement{}
	m.unplaced[d] = pl
	clients, suppliers := m.model.Refs(d, "client"), m.model.Refs(d, "supplier")
	lostClients, lostSuppliers := m.model.Unresolved(d, "client"), m.model.Unresolved(d, "supplier")
	total := (len(clients) + len(lostClients)) * (len(suppliers) + len(lostSuppliers))
	if total == 0 {
		pl.failed = 1
		pl.notes = append(pl.notes, "the dependency's client or supplier is not in the document")
		return
	}
	for _, id := range lostClients {
		pl.notes = append(pl.notes, "the exposing element "+id+" is not in the document")
	}
	// A diagram is no model element: an id or href naming one resolves to nothing,
	// or to a proxy, and stands for the view written for it.
	var diagrams []*sysmlv1.Diagram
	var elements []*sysmlv1.Element
	shown := func(d *sysmlv1.Diagram) {
		if note := m.diagramViewNote(d); note != "" {
			pl.notes = append(pl.notes, note)
			return
		}
		diagrams = append(diagrams, d)
	}
	for _, id := range lostSuppliers {
		if d := m.model.Diagram(id); d != nil {
			shown(d)
			continue
		}
		pl.notes = append(pl.notes, m.absentNote(id))
	}
	for _, s := range suppliers {
		if d := m.model.Diagram(s.Href); s.IsProxy() && d != nil {
			shown(d)
			continue
		}
		elements = append(elements, s)
	}
	for _, c := range clients {
		if cat, _ := m.classify(c); cat != catView {
			pl.notes = append(pl.notes, "the client "+qualifiedName(c)+" is not a view: v2 exposes elements from a view alone")
			continue
		}
		for _, s := range elements {
			if note := m.exposeNote(s); note != "" {
				pl.notes = append(pl.notes, note)
				continue
			}
			pl.write(m.v2Name(c))
			view, exposed := c, s
			m.extras[view] = append(m.extras[view], func() {
				ref := m.exposeRef(view, exposed)
				m.recordViewExposure(view, exposed, ref)
				m.w.line("expose " + ref + ";")
			})
		}
		for _, d := range diagrams {
			pl.write(m.v2Name(c))
			view, shown := c, d
			m.extras[view] = append(m.extras[view], func() {
				ref := m.viewRef(m.viewOf[shown], view)
				m.recordViewExposure(view, nil, ref)
				m.w.line("expose " + ref + ";")
			})
		}
	}
	pl.failed = total - len(pl.targets)
}

// exposeNote says why a view cannot expose s: it is external or not migrated;
// "" when an expose member can name it.
func (m *migration) exposeNote(s *sysmlv1.Element) string {
	switch {
	case s.IsProxy():
		return exposedSubject + qualifiedName(s) + livesOutsideDocument
	case !m.written(s):
		_, why := m.classify(s)
		return joinNotes(exposedSubject+kindOf(s)+" "+describe(s)+" is not migrated and is not written", why)
	}
	return ""
}

// exposeRef writes what a view exposes: a namespace with its contents, as v1
// exposes a package's members with it, any other element by itself.
func (m *migration) exposeRef(view, s *sysmlv1.Element) string {
	ref := m.memberRef(s, view)
	switch s.Type {
	case "Package", "Model":
		return ref + "::**"
	}
	return ref
}

// recordViewExposure records the names introduced into a view by an exposure.
func (m *migration) recordViewExposure(view, exposed *sysmlv1.Element, ref string) {
	switch {
	case strings.HasSuffix(ref, "::*"):
		m.recordWildcardViewExposure(view, exposed, false)
	case strings.HasSuffix(ref, "::**"):
		m.recordWildcardViewExposure(view, exposed, true)
	default:
		m.recordExposedViewName(view, exposureName(ref))
	}
}

// recordWildcardViewExposure records names imported by a wildcard exposure.
func (m *migration) recordWildcardViewExposure(view, exposed *sysmlv1.Element, recursive bool) {
	if exposed == nil {
		return
	}
	var record func(*sysmlv1.Element)
	record = func(member *sysmlv1.Element) {
		if !m.written(member) {
			return
		}
		m.recordExposedViewName(view, m.nameOf(member))
		if recursive {
			for _, child := range member.Children {
				record(child)
			}
		}
	}
	for _, member := range exposed.Children {
		record(member)
	}
}

// recordExposedViewName records an exposed name in the view's namespace.
func (m *migration) recordExposedViewName(view *sysmlv1.Element, name string) {
	if name == "" {
		return
	}
	if m.viewNames[view] == nil {
		m.viewNames[view] = map[string]bool{}
	}
	m.viewNames[view][name] = true
}

// absentNote says what an unresolved reference to id was: notation the tool
// keeps outside the model, or nothing in the document.
func (m *migration) absentNote(id string) string {
	for _, ext := range m.model.Extensions {
		for _, el := range ext.Elements {
			if el.ID != id {
				continue
			}
			name := el.Name
			if name == "" {
				name = id
			}
			return exposedSubject + strings.TrimPrefix(el.Type, "uml:") + " '" + name + "' is notation the tool keeps outside the model, which v2 does not carry"
		}
	}
	return "the exposed element " + id + " is not in the document"
}

// viewpointBody writes a viewpoint definition and its nested viewpoint usage.
func (m *migration) viewpointBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	name := m.freshName(e, lowerFirst(m.nameFor(e)))
	m.w.madeUp(writeName(name))
	m.w.block("viewpoint "+writeName(name), func() {
		m.viewpointTags(e)
	})
	m.w.line("satisfy " + writeName(name) + ";")
	m.rendering(e)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeComments(e)
	m.scope = saved
}

// viewpointTags writes the nested viewpoint usage's parameters and documentation.
func (m *migration) viewpointTags(e *sysmlv1.Element) {
	vp := stereo(e, "Viewpoint")
	if vp == nil {
		return
	}
	m.w.line("subject;")
	wroteStakeholder := false
	for _, id := range vp.IDs("stakeholder") {
		wroteStakeholder = wroteStakeholder || m.stakeholderWritable(e, id)
		m.stakeholder(e, id)
	}
	if wroteStakeholder {
		m.add(e, Mapped, "", subjectNote("stakeholders"))
	}
	for _, c := range m.concernLists[e] {
		text := m.commentBody(c)
		if info := m.concerns[c]; info != nil && info.homed {
			m.w.line("frame " + m.ref(c, e) + ";")
			continue
		}
		if text == "" {
			m.add(c, Skipped, "", "empty comment")
			continue
		}
		m.frameConcern(text)
		m.add(c, Approximated, "", concernFallbackNote(c))
	}
	for _, concern := range vp.Tags["concern"] {
		m.frameConcern(concern)
	}
	if doc := m.viewpointDoc(e); doc != "" {
		m.w.lines(prefixFirst("doc ", commentLines(doc)))
	}
	if purpose := vp.Tag("purpose"); purpose != "" {
		m.w.block("require constraint", func() {
			m.w.lines(prefixFirst("doc ", commentLines(purpose)))
		})
	}
	if vp.Tag("language") != "" || vp.Tag("presentation") != "" {
		m.downgrade(e, "SysMLv1Library::ViewpointData is unavailable, so language and presentation are retained as documentation")
	}
}

// viewpointDoc is the documentation a viewpoint's unsupported tags write.
func (m *migration) viewpointDoc(e *sysmlv1.Element) string {
	vp := stereo(e, "Viewpoint")
	if vp == nil {
		return ""
	}
	var doc []string
	for _, tag := range []string{"language", "presentation"} {
		if vs := vp.Tags[tag]; len(vs) > 0 {
			doc = append(doc, tag+": "+strings.Join(m.tagValues(vs), ", "))
		}
	}
	for _, id := range vp.IDs("method") {
		t := m.model.Lookup(id)
		if t == nil || !m.renderableMethod(t) {
			doc = append(doc, "method: "+strings.Join(m.tagValues([]string{id}), ", "))
			m.downgrade(e, "the method tag entry "+id+" cannot be rendered as an action")
		}
	}
	return strings.Join(doc, "\n")
}

// concernInfo records the stakeholders and owner eligibility for a concern comment.
type concernInfo struct {
	stakeholders    []*sysmlv1.Element
	homed           bool
	viewpointListed bool
}

// prepareConcerns indexes concern comments before declarations are written.
func (m *migration) prepareConcerns() {
	var concernOrder []*sysmlv1.Element
	var walk func(*sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		recordConcerns := func(s *sysmlv1.Stereotype, stakeholder bool) {
			if s == nil {
				return
			}
			for _, id := range s.IDs("concernList") {
				c := m.model.Lookup(id)
				if c == nil {
					m.downgrade(e, "the concernList tag names "+id+notInDocument)
					continue
				}
				if c.Type != "Comment" {
					m.downgrade(e, "the concernList tag names the "+kindOf(c)+" "+qualifiedName(c)+", which is not a comment and frames no concern")
					continue
				}
				m.concernLists[e] = append(m.concernLists[e], c)
				info := m.concerns[c]
				if info == nil {
					info = &concernInfo{}
					m.concerns[c] = info
					concernOrder = append(concernOrder, c)
				}
				if stakeholder {
					if !slices.Contains(info.stakeholders, e) {
						info.stakeholders = append(info.stakeholders, e)
					}
				} else {
					info.viewpointListed = true
				}
			}
		}
		recordConcerns(stereo(e, "Viewpoint"), false)
		recordConcerns(stereo(e, "Stakeholder"), true)
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, root := range m.model.Roots {
		walk(root)
	}
	for _, c := range concernOrder {
		info := m.concerns[c]
		owner := c.Parent
		if owner != nil {
			if m.flattened(owner) {
				info.homed = true
			} else if owner.Type == "Package" {
				info.homed = !m.isLibrary(owner) && m.written(owner)
			} else {
				cat, _ := m.classify(owner)
				info.homed = (cat == catPartDef || cat == catActionDef || cat == catRequirementDef ||
					cat == catUseCaseDef || cat == catView || cat == catViewpoint) && m.written(owner)
			}
		}
		if info.homed {
			m.names[c] = m.freshName(owner, "concern")
			m.synthesized[c] = true
		}
		if info.homed || info.viewpointListed {
			m.framed[c] = true
		}
	}
}

// concernCommentFallbackNote explains why a stakeholder-only concern stays a comment.
func concernCommentFallbackNote() string {
	return "the concern is kept as a comment because its owner cannot own a v2 concern usage and no viewpoint frames it"
}

// concernCommentFallback returns the downgrade for an unhomed concern no viewpoint lists.
func (m *migration) concernCommentFallback(c *sysmlv1.Element) string {
	info := m.concerns[c]
	if info == nil || info.homed || info.viewpointListed || len(info.stakeholders) == 0 {
		return ""
	}
	return concernCommentFallbackNote()
}

// concernFallbackNote explains why a concern comment remains an anonymous frame.
func concernFallbackNote(c *sysmlv1.Element) string {
	if c.Parent == nil {
		return "the concern comment has no written package or classifier owner, so it remains an anonymous framed concern"
	}
	return "the concern comment is owned by the " + kindOf(c.Parent) + " " + qualifiedName(c.Parent) + ", which cannot own a v2 concern usage"
}

// renderableMethod reports whether e can be emitted as a rendering action.
func (m *migration) renderableMethod(e *sysmlv1.Element) bool {
	if e == nil || e.IsProxy() || !m.written(e) {
		return false
	}
	switch e.Type {
	case "Operation", "Activity", "OpaqueBehavior", "FunctionBehavior", "Interaction":
		return true
	}
	return false
}

// rendering writes a Viewpoint's Create operations and written method tags.
func (m *migration) rendering(e *sysmlv1.Element) {
	vp := stereo(e, "Viewpoint")
	if vp == nil {
		return
	}
	var methods []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	add := func(method *sysmlv1.Element) {
		if !m.renderableMethod(method) || seen[method] {
			return
		}
		seen[method] = true
		methods = append(methods, method)
	}
	for _, op := range e.Owned("ownedOperation") {
		if stereo(op, "Create") == nil {
			continue
		}
		method := m.bodyMethod(op)
		if method == nil || !m.renderableMethod(method) {
			method = op
		}
		add(method)
	}
	for _, id := range vp.IDs("method") {
		add(m.model.Lookup(id))
	}
	if len(methods) == 0 {
		return
	}
	m.w.block("rendering", func() {
		for _, method := range methods {
			m.w.line("action : " + m.ref(method, e) + ";")
		}
	})
}

// subjectNote explains the anonymous subject written before a case's or
// viewpoint's other parameters, which v2 places after the subject.
func subjectNote(params string) string {
	return "v2 places the subject before the " + params + ", so an anonymous subject is declared"
}

// stakeholderWritable reports whether the stakeholder tag id becomes a usage.
func (m *migration) stakeholderWritable(vp *sysmlv1.Element, id string) bool {
	s := m.model.Lookup(id)
	if s == nil || s.IsProxy() || !m.written(s) {
		return false
	}
	cat, _ := m.classify(s)
	return cat == catPartDef
}

// stakeholder writes one stakeholder usage of a viewpoint, typed by the part
// def the stakeholder class becomes.
func (m *migration) stakeholder(vp *sysmlv1.Element, id string) {
	s := m.model.Lookup(id)
	switch {
	case s == nil:
		m.downgrade(vp, "the stakeholder tag names "+id+notInDocument)
		return
	case s.IsProxy():
		m.downgrade(vp, stakeholderSubject+qualifiedName(s)+livesOutsideDocument)
		return
	case !m.written(s):
		m.downgrade(vp, stakeholderSubject+qualifiedName(s)+" is not migrated and is not written")
		return
	}
	if cat, _ := m.classify(s); cat != catPartDef {
		m.downgrade(vp, stakeholderSubject+qualifiedName(s)+" becomes a "+cat.keyword()+", which cannot type a stakeholder")
		return
	}
	name := m.freshName(vp, lowerFirst(m.nameFor(s)))
	m.w.madeUp(writeName(name))
	m.w.line("stakeholder " + writeName(name) + " : " + m.ref(s, vp) + ";")
}

// frameConcern writes a concern a viewpoint frames, documented by its text.
func (m *migration) frameConcern(text string) {
	m.w.block("frame concern", func() {
		m.w.lines(prefixFirst("doc ", commentLines(text)))
	})
}
