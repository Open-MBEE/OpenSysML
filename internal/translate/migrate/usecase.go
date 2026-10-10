package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// actorLink is an association between a use case and an actor, written as a
// connection between the two usages rather than as member-end properties.
type actorLink struct {
	assoc, useCase, actor *sysmlv1.Element
	// end is the association's member end typed by the actor.
	end *sysmlv1.Element
	// conn is the connection written for the association, nil when no package holds it.
	conn *defConn
	// block marks a link to a block rather than a use case: its ends are the
	// parts the association already writes, so only the connection is added.
	block bool
}

// defUsage is the usage a connection joins for a v1 classifier: the classifier
// itself when it is written as a usage, as an actor or use case is, else a
// usage of the definition written in a package, since a connection joins
// features, which a requirement or block definition is not.
type defUsage struct {
	def, host     *sysmlv1.Element
	keyword, name string
	// self marks def as itself the usage, written where it is in the model.
	self bool
}

// defConn is a connection written in a package between two definitions' usages,
// standing for an actor association or a «Refine» a diagram draws as a line.
type defConn struct {
	// of is the v1 relationship the connection is written for.
	of, host *sysmlv1.Element
	name     string
	from, to *defUsage
	// def is the association written as the connection def typing the connection, nil for an untyped one.
	def *sysmlv1.Element
	// doc is what the connection's body documents: where and when an extend applies.
	doc string
}

// target is the report target of the connection.
func (c *defConn) target(m *migration) string {
	return m.qualified(append(m.segments(c.host), c.name))
}

// refineConnectionNote is the note of a «Refine» written as a connection.
const refineConnectionNote = "written as a connection between usages of its ends, which a diagram draws; the standard Refinement metadata annotates only a dependency, so the connection's name says it refines"

// usageKeyword is the keyword of a usage of a written definition a connection
// may join, "" for an element that is no such definition.
func (m *migration) usageKeyword(def *sysmlv1.Element) string {
	if !m.written(def) {
		return ""
	}
	cat, _ := m.classify(def)
	switch cat {
	case catPartDef:
		return "part"
	case catUseCase:
		return "use case"
	case catActor:
		return "part"
	case catRequirement:
		return "requirement"
	case catActionDef:
		return "action"
	case catOccurrenceDef:
		return "occurrence"
	case catItemDef:
		return "item"
	}
	return ""
}

// usageHost is the written package or model in or above e whose body holds the
// usages a connection written for e joins; nil when no ancestor is one.
func (m *migration) usageHost(e *sysmlv1.Element) *sysmlv1.Element {
	for p := e.Parent; p != nil; p = p.Parent {
		if (p.Type == "Package" || p.Type == "Model") && m.written(p) {
			return p
		}
	}
	return nil
}

// defUsageOf is the usage a connection in host's body joins for def: def
// itself when it is written as a usage, else a usage of it declared in host's
// body on first demand.
func (m *migration) defUsageOf(host, def *sysmlv1.Element, keyword string) *defUsage {
	if u := m.defUsages[host][def]; u != nil {
		return u
	}
	if m.defUsages[host] == nil {
		m.defUsages[host] = map[*sysmlv1.Element]*defUsage{}
	}
	if m.selfUsage(def) {
		u := &defUsage{def: def, host: def.Parent, keyword: keyword, name: m.nameOf(def), self: true}
		m.defUsages[host][def] = u
		return u
	}
	base := lowerFirst(def.Name)
	if base == "" {
		base = lowerFirst(m.nameFor(def))
	}
	u := &defUsage{def: def, host: host, keyword: keyword, name: m.freshName(host, base)}
	m.defUsages[host][def] = u
	m.extras[host] = append(m.extras[host], func() {
		m.w.line(keyword + " " + writeName(u.name) + " : " + m.ref(def, host) + ";")
	})
	return u
}

// selfUsage says whether def is written as a usage a connection joins directly.
func (m *migration) selfUsage(def *sysmlv1.Element) bool {
	cat, _ := m.classify(def)
	return cat == catUseCase || cat == catActor || cat == catRequirement
}

// usageIn is the one usage written for def, which a view draws for it; nil for none.
func (m *migration) usageIn(def *sysmlv1.Element) *defUsage {
	return m.defUsages[m.usageHost(def)][def]
}

// connectUsages writes, in host's body, a connection named after e between the
// usages of the two classifiers, and records it as the edge e is drawn as.
func (m *migration) connectUsages(e, host, from, to *sysmlv1.Element, fromKw, toKw, name string) *defConn {
	return m.connectTyped(e, nil, host, from, to, fromKw, toKw, name)
}

// connectTyped is connectUsages with the connection typed by def, the
// connection def written for e, when def is not nil.
func (m *migration) connectTyped(e, def, host, from, to *sysmlv1.Element, fromKw, toKw, name string) *defConn {
	fu, tu := m.defUsageOf(m.usageHost(from), from, fromKw), m.defUsageOf(m.usageHost(to), to, toKw)
	if name == "" {
		name = spoken(writeName(fu.name)) + " to " + spoken(writeName(tu.name))
	}
	if def != nil {
		// The def keeps e's name; the usage must not.
		name = m.freshName(host, name)
	} else {
		name = m.claimName(e, host, name)
	}
	c := &defConn{of: e, host: host, name: name, from: fu, to: tu, def: def}
	if m.conns[e] == nil {
		m.conns[e] = c
	} else {
		m.moreConns[e] = append(m.moreConns[e], c)
	}
	m.extras[host] = append(m.extras[host], func() {
		typed := ""
		if def != nil {
			typed = " : " + m.ref(def, host)
		}
		header := "connection " + writeName(name) + typed + " connect " + m.usageRef(fu, host) + " to " + m.usageRef(tu, host)
		m.w.trailed(header, ";", func() {
			if def != nil {
				return // the connection def carries the association's metadata
			}
			if c.doc != "" {
				m.w.line("doc /* " + c.doc + " */")
			}
			saved := m.scope
			m.scope = host
			m.stereotypeAnnotations(e)
			m.scope = saved
		})
	})
	return c
}

// usageRef writes a reference to usage u from inside scope's body.
func (m *migration) usageRef(u *defUsage, scope *sysmlv1.Element) string {
	if u.self {
		return m.ref(u.def, scope)
	}
	return m.hostedRef(u.host, u.name, scope)
}

// connectActor writes the connection a link's association is written as,
// between the actor's part usage and the use case usage, in the nearest
// package holding both, and reports the ends the association owns as its ends.
func (m *migration) connectActor(link *actorLink) {
	host, ok := m.connHost(link.assoc, link.actor, link.useCase)
	if !ok {
		return
	}
	from, to, fromKw, toKw := link.actor, link.useCase, "part", m.usageKeyword(link.useCase)
	if ends := m.model.Refs(link.assoc, "memberEnd"); len(ends) > 0 && ends[0] != link.end {
		// The connection's ends follow the def's, so the use case end comes first.
		from, to, fromKw, toKw = to, from, toKw, fromKw
	}
	name := m.nameOf(link.assoc)
	var def *sysmlv1.Element
	if m.associationAsConnectionDef(link.assoc) {
		// The association is a connection def; its one usage is named like a usage.
		def, name = link.assoc, lowerFirst(name)
	}
	link.conn = m.connectTyped(link.assoc, def, host, from, to, fromKw, toKw, name)
	if link.block {
		return
	}
	target := link.conn.target(m)
	for _, end := range m.model.Refs(link.assoc, "memberEnd") {
		if end.Parent != link.assoc {
			continue
		}
		note := ""
		if end == link.end {
			_, note = m.multiplicity(end)
			note = joinNotes(note, "the end at the actor is the connection's end at the actor's part usage")
		} else {
			note = "the end at the use case is the connection's end at the use case usage"
		}
		m.add(end, verdictFor(note), target, note)
	}
}

// usageConnection writes a relationship d between two classifiers as a
// connection between their usages in the nearest package holding both; nil
// when either end is no classifier a usage can be written of.
func (m *migration) usageConnection(d *sysmlv1.Element, name string, client, supplier *sysmlv1.Element) *defConn {
	ck, sk := m.usageKeyword(client), m.usageKeyword(supplier)
	if ck == "" || sk == "" {
		return nil
	}
	host, ok := m.connHost(d, client, supplier)
	if !ok {
		return nil
	}
	return m.connectUsages(d, host, client, supplier, ck, sk, name)
}

// connRefs are the references a view exposing e also exposes so the connection
// written for it is drawn: the connection and the two usages it joins.
func (m *migration) connRefs(e, scope *sysmlv1.Element) []string {
	var refs []string
	for _, c := range m.connsOf(e) {
		refs = append(refs, m.hostedRef(c.host, c.name, scope), m.usageRef(c.from, scope), m.usageRef(c.to, scope))
	}
	return refs
}

// connsOf are the connections written for e: one per pair of ends a
// dependency with several clients or suppliers joins.
func (m *migration) connsOf(e *sysmlv1.Element) []*defConn {
	c := m.conns[e]
	if c == nil {
		return nil
	}
	return append([]*defConn{c}, m.moreConns[e]...)
}

// pairConn is the connection written for d between client and supplier; nil for none.
func (m *migration) pairConn(d, client, supplier *sysmlv1.Element) *defConn {
	for _, c := range m.connsOf(d) {
		if c.from.def == client && c.to.def == supplier {
			return c
		}
	}
	return nil
}

// planConnections writes, before any view is, the connection each include,
// extend and «Refine» between classifiers is drawn as, so a view exposes the
// usages the connections join wherever in the model they are hosted. An
// unnamed relationship's connection is named by its kind, "A includes B": a
// comment would be a note to the tools that draw the connection.
func (m *migration) planConnections(rels []*sysmlv1.Element) {
	for _, r := range rels {
		var from, to *sysmlv1.Element
		verb := ""
		switch r.Type {
		case "Include":
			from, to, verb = r.Parent, m.model.Ref(r, "addition"), "includes"
			if to != nil && m.written(to) {
				if cat, _ := m.classify(to); cat != catUseCase {
					continue
				}
			}
		case "Extend":
			from, to, verb = r.Parent, m.model.Ref(r, "extendedCase"), "extends"
		default:
			clients, suppliers := m.model.Refs(r, "client"), m.model.Refs(r, "supplier")
			if len(clients) != 1 || len(suppliers) != 1 {
				continue
			}
			from, to, verb = clients[0], suppliers[0], "refines"
		}
		if from == nil || to == nil || !m.written(from) || !m.written(to) {
			continue
		}
		c := m.kindConnection(r, m.nameOf(r), from, to, verb)
		if c == nil {
			continue
		}
		if detail := m.extensionDetail(r); detail != "" {
			if c.doc == "" {
				c.doc = "extends"
			}
			c.doc += " " + detail
		}
	}
}

// kindConnection writes the connection a relationship of a kind is drawn as,
// named "A refines B" by its kind unless its author named it, when the kind
// is the connection's doc instead.
func (m *migration) kindConnection(r *sysmlv1.Element, name string, from, to *sysmlv1.Element, verb string) *defConn {
	kind := spoken(writeName(m.nameFor(from))) + " " + verb + " " + spoken(writeName(m.nameFor(to)))
	if m.nameOf(r) == "" {
		name = kind
	}
	c := m.usageConnection(r, name, from, to)
	if c != nil && name != kind {
		c.doc = kind
	}
	return c
}

// extendComment is the comment an extend's connection or dependency carries:
// the extension points and condition it applies.
func (m *migration) extendComment(ext *sysmlv1.Element) string {
	comment := "extends"
	if detail := m.extensionDetail(ext); detail != "" {
		comment += " " + detail
	}
	return comment
}

// connOf is the connection e is drawn as: the one written for e, or for the
// association e is a member end of; nil for none.
func (m *migration) connOf(e *sysmlv1.Element) *defConn {
	if c := m.conns[e]; c != nil {
		return c
	}
	if a := m.associationEnds[e]; a != nil {
		return m.conns[a]
	}
	return nil
}

// connHost is the package a connection between usages of a and b is written
// in (nil for the top level): the nearest package holding both, so each
// definition has one usage there however many relationships join it; else the
// package d is in. ok is false when there is none.
func (m *migration) connHost(d, a, b *sysmlv1.Element) (host *sysmlv1.Element, ok bool) {
	above := map[*sysmlv1.Element]bool{}
	for p := a.Parent; p != nil; p = p.Parent {
		above[p] = true
	}
	for p := b.Parent; p != nil; p = p.Parent {
		switch {
		case !above[p]:
		case isTopLevel(p):
			return nil, true
		case (p.Type == "Package" || p.Type == "Model") && m.written(p):
			return p, true
		}
	}
	host = m.usageHost(d)
	return host, host != nil
}

// hostedRef refers from scope to name, declared in host's body (the top level for nil).
func (m *migration) hostedRef(host *sysmlv1.Element, name string, scope *sysmlv1.Element) string {
	return m.refMember(host, name, namespaces(append(m.segments(host), name)), scope, false)
}

// actorLink recognises an association linking a written use case to a written
// actor by an end the use case does not own itself; nil for any other association.
func (m *migration) actorLink(a *sysmlv1.Element) *actorLink {
	if a.Type != "Association" {
		return nil
	}
	ends := m.model.Refs(a, "memberEnd")
	if len(ends) != 2 {
		return nil
	}
	for i, end := range ends {
		actor := m.model.Ref(end, "type")
		useCase := m.model.Ref(ends[1-i], "type")
		if actor == nil || useCase == nil || actor.Type != "Actor" {
			continue
		}
		if useCase.Type != "UseCase" {
			if m.usageKeyword(useCase) == "part" && m.usageKeyword(actor) == "part" {
				return &actorLink{assoc: a, useCase: useCase, actor: actor, end: end, block: true}
			}
			continue
		}
		if end.Parent == useCase || !m.written(actor) || !m.written(useCase) {
			return nil
		}
		return &actorLink{assoc: a, useCase: useCase, actor: actor, end: end}
	}
	return nil
}

// placeActors writes the connection each link's association is written as,
// once every member of the use cases is named, and records the link.
func (m *migration) placeActors(links []*actorLink) {
	for _, link := range links {
		if !link.block {
			m.actors[link.assoc] = link
		}
		m.connectActor(link)
	}
}

// useCaseBody writes the body of a use case usage: its subject, its members
// and inclusions, and its classifier behavior.
func (m *migration) useCaseBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	m.subjects(e)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeMarkers(e)
	m.scope = saved
}

// subjects writes a use case's subject; v2 admits one per case, so any further
// subject is written as a reference usage of its kind. A subject that becomes
// a usage, as a view does, is subset rather than typed.
func (m *migration) subjects(e *sysmlv1.Element) {
	if d := m.dangling(e, "subject"); d != "" {
		m.downgrade(e, d)
	}
	first := true
	for _, s := range m.model.Refs(e, "subject") {
		if !m.written(s) {
			m.downgrade(e, "the subject "+qualifiedName(s)+" is not migrated and is not written")
			continue
		}
		if m.isUsage(s) {
			if note := m.featuredNote(s, e); note != "" {
				m.downgrade(e, "the subject is not written: "+note)
				continue
			}
		}
		name := m.freshName(e, lowerFirst(m.nameFor(s)))
		m.w.madeUp(writeName(name))
		if first {
			m.w.line("subject " + writeName(name) + m.typing(s) + m.ref(s, e) + ";")
			first = false
			continue
		}
		kw, note := m.typeKeyword(s)
		m.w.line("ref " + kw + " " + writeName(name) + m.typing(s) + m.ref(s, e) + ";")
		m.downgrade(e, joinNotes("v2 admits one subject per case, so the subject "+qualifiedName(s)+" is written as the reference usage "+name, note))
	}
	if first && m.hasActors(e) {
		m.w.line("subject;")
		m.add(e, Mapped, "", subjectNote("actors"))
	}
}

// hasActors reports whether an actor usage is written in the use case's body,
// for a property typed by an actor.
func (m *migration) hasActors(useCase *sysmlv1.Element) bool {
	for _, p := range useCase.Owned("ownedAttribute") {
		t := m.model.Ref(p, "type")
		if p.Type != "Port" && t != nil && t.Type == "Actor" && m.written(t) {
			return true
		}
	}
	return false
}

// include writes a UML Include as an include use case usage of the including
// use case referring to the included one's usage.
func (m *migration) include(inc *sysmlv1.Element) {
	added := m.model.Ref(inc, "addition")
	switch {
	case added == nil:
		m.unmapped(inc, joinNotes("the included use case is not in the document", m.dangling(inc, "addition")))
		return
	case !m.written(added):
		_, why := m.classify(added)
		m.unmapped(inc, joinNotes("the included use case "+qualifiedName(added)+" is not migrated", why))
		return
	}
	if cat, _ := m.classify(added); cat != catUseCase {
		m.unmapped(inc, "the included element "+qualifiedName(added)+" becomes a "+cat.keyword()+", which an include use case cannot refer to")
		return
	}
	m.wroteEdge(inc, m.scope, "include", "")
	verdict, note := Mapped, ""
	if name := m.nameOf(inc); name != "" {
		verdict, note = Approximated, "an include referring to a use case usage has no name, so "+name+" is dropped"
	}
	if conn := m.conns[inc]; conn != nil {
		// The connection drawn for the include carries its annotations.
		m.w.line("include " + m.ref(added, m.scope) + ";")
		m.wroteEdgeAlso(inc, conn.host, "connection", nil, conn.name)
	} else {
		m.w.trailed("include "+m.ref(added, m.scope), ";", func() { m.stereotypeAnnotations(inc) })
	}
	m.add(inc, verdict, m.qualified(m.segments(inc.Parent)), note)
}

// extend writes a UML Extend as a dependency of the extending use case on the
// extended one, named by its kind, keeping the extension points and condition
// as its doc: v2 has no extend relationship.
func (m *migration) extend(ext *sysmlv1.Element) {
	extended := m.model.Ref(ext, "extendedCase")
	switch {
	case extended == nil:
		m.unmapped(ext, joinNotes("the extended use case is not in the document", m.dangling(ext, "extendedCase")))
		return
	case !m.written(extended) || !m.written(ext.Parent):
		m.unmapped(ext, "the extended use case "+qualifiedName(extended)+" or the extending one is not migrated")
		return
	}
	note := "v2 has no extend: the extension is written as a plain dependency on the extended use case"
	detail := m.extensionDetail(ext)
	if detail != "" {
		note += "; it applies " + detail + ", which only its doc keeps"
	}
	if conn := m.conns[ext]; conn != nil {
		m.wroteEdgeAlso(ext, conn.host, "connection", nil, conn.name)
		m.add(ext, Approximated, conn.target(m), strings.Replace(note, "a plain dependency on", "a connection between usages of the extending use case and", 1))
		return
	}
	from, to := m.ref(ext.Parent, m.scope), m.ref(extended, m.scope)
	kind := spoken(from) + " extends " + spoken(to)
	name, doc := m.nameOf(ext), ""
	if name == "" {
		name = m.freshName(m.scope, kind)
		m.names[ext] = name
	} else {
		doc = kind
	}
	if detail != "" {
		doc = m.extendComment(ext)
	}
	m.wroteEdge(ext, m.scope, "dependency", name)
	m.w.trailed("dependency "+writeName(name)+" from "+from+" to "+to, ";", func() {
		if doc != "" {
			m.w.line("doc /* " + doc + " */")
		}
		m.stereotypeAnnotations(ext)
	})
	m.add(ext, Approximated, m.v2Name(ext), note)
}

// extensionDetail says where and when an extension applies: at its extension
// points, when its condition holds; "" when it names neither.
func (m *migration) extensionDetail(ext *sysmlv1.Element) string {
	var parts []string
	var points []string
	for _, p := range m.model.Refs(ext, "extensionLocation") {
		points = append(points, describe(p))
	}
	for _, id := range m.model.Unresolved(ext, "extensionLocation") {
		points = append(points, id+" (not in the document)")
	}
	if len(points) > 0 {
		parts = append(parts, "at extension point(s) "+strings.Join(points, ", "))
	}
	if cond := firstOwned(ext, "condition"); cond != nil {
		if spec := firstOwned(cond, "specification"); spec != nil {
			parts = append(parts, "when "+describeValue(spec))
		}
	}
	return strings.Join(parts, " ")
}
