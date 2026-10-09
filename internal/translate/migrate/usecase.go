package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// actorLink is an association between a use case and an actor, written as an
// actor usage of the use case def rather than as a connection def.
type actorLink struct {
	assoc, useCase, actor *sysmlv1.Element
	// end is the association's member end typed by the actor, whose name the
	// actor usage takes; name is the usage's name once placed.
	end  *sysmlv1.Element
	name string
	// conn is the connection written beside the use case for the association, nil when no package holds it.
	conn *defConn
	// block marks a link to a block rather than a use case: its ends are the
	// parts the association already writes, so only the connection is added.
	block bool
}

// defUsage is a usage of a definition written in a package so a connection can
// join it: a connection joins features, which a v1 actor, use case or requirement is not.
type defUsage struct {
	def, host     *sysmlv1.Element
	keyword, name string
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
}

// target is the report target of the connection.
func (c *defConn) target(m *migration) string {
	return m.qualified(append(m.segments(c.host), c.name))
}

// refineConnectionNote is the note of a «Refine» written as a connection.
const refineConnectionNote = "written as a connection between usages of its ends, which a diagram draws; the standard Refinement metadata annotates only a dependency, so the refine is kept as a comment"

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
	case catUseCaseDef:
		return "use case"
	case catRequirementDef:
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

// defUsageOf is the usage of def in host's body, declared there on first demand.
func (m *migration) defUsageOf(host, def *sysmlv1.Element, keyword string) *defUsage {
	if u := m.defUsages[host][def]; u != nil {
		return u
	}
	if m.defUsages[host] == nil {
		m.defUsages[host] = map[*sysmlv1.Element]*defUsage{}
	}
	base := lowerFirst(def.Name)
	if base == "" {
		base = lowerFirst(m.nameFor(def))
	}
	u := &defUsage{def: def, host: host, keyword: keyword, name: m.freshName(host, base)}
	m.defUsages[host][def] = u
	m.defUsageList = append(m.defUsageList, u)
	m.extras[host] = append(m.extras[host], func() {
		m.w.line(keyword + " " + writeName(u.name) + " : " + m.ref(def, host) + ";")
	})
	return u
}

// usageIn is the one usage written for def, which a view draws for it; nil for none.
func (m *migration) usageIn(def *sysmlv1.Element) *defUsage {
	return m.defUsages[m.usageHost(def)][def]
}

// connectUsages writes, in host's body, a connection named after e between the
// usages of the two definitions, each in its own package, and records it as
// the edge e is drawn as.
func (m *migration) connectUsages(e, host, from, to *sysmlv1.Element, fromKw, toKw, name, trailer string) *defConn {
	return m.connectTyped(e, nil, host, from, to, fromKw, toKw, name, trailer)
}

// connectTyped is connectUsages with the connection typed by def, the
// connection def written for e, when def is not nil.
func (m *migration) connectTyped(e, def, host, from, to *sysmlv1.Element, fromKw, toKw, name, trailer string) *defConn {
	fu, tu := m.defUsageOf(m.usageHost(from), from, fromKw), m.defUsageOf(m.usageHost(to), to, toKw)
	if name == "" {
		name = m.freshName(host, spoken(writeName(fu.name))+" to "+spoken(writeName(tu.name)))
	}
	c := &defConn{of: e, host: host, name: name, from: fu, to: tu, def: def}
	m.conns[e] = c
	m.extras[host] = append(m.extras[host], func() {
		typed := ""
		if def != nil {
			typed = " : " + m.ref(def, host)
		}
		header := "connection " + writeName(name) + typed + " connect " + m.usageRef(fu, host) + " to " + m.usageRef(tu, host)
		m.w.trailed(header, ";"+trailer, strings.TrimSpace(trailer), func() {
			if def != nil {
				return // the connection def carries the association's metadata
			}
			saved := m.scope
			m.scope = host
			m.metadataUsages(e)
			m.scope = saved
		})
	})
	return c
}

// usageRef writes a reference to usage u from inside scope's body.
func (m *migration) usageRef(u *defUsage, scope *sysmlv1.Element) string {
	return m.hostedRef(u.host, u.name, scope)
}

// connectActor writes the connection a link's association is drawn as, between
// a part usage of the actor and a use case usage, in the package owning the association.
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
		def, name = link.assoc, m.freshName(host, lowerFirst(name))
	}
	link.conn = m.connectTyped(link.assoc, def, host, from, to, fromKw, toKw, name, "")
}

// usageConnection writes a relationship d between two definitions as a
// connection between their usages in the package d is written in, trailer
// closing the line; nil when either end is no definition a usage can be written of.
func (m *migration) usageConnection(d *sysmlv1.Element, name string, client, supplier *sysmlv1.Element, trailer string) *defConn {
	ck, sk := m.usageKeyword(client), m.usageKeyword(supplier)
	if ck == "" || sk == "" {
		return nil
	}
	host, ok := m.connHost(d, client, supplier)
	if !ok {
		return nil
	}
	return m.connectUsages(d, host, client, supplier, ck, sk, name, trailer)
}

// connRefs are the references a view exposing e also exposes so the connection
// written for it is drawn: the connection and the two usages it joins.
func (m *migration) connRefs(e, scope *sysmlv1.Element) []string {
	c := m.conns[e]
	if c == nil {
		return nil
	}
	return []string{m.hostedRef(c.host, c.name, scope), m.usageRef(c.from, scope), m.usageRef(c.to, scope)}
}

// planConnections writes, before any view is, the connection each include,
// extend and «Refine» between definitions is drawn as, so a view exposes the
// usages the connections join wherever in the model they are hosted.
func (m *migration) planConnections(rels []*sysmlv1.Element) {
	for _, r := range rels {
		var from, to *sysmlv1.Element
		trailer := ""
		switch r.Type {
		case "Include":
			from, to, trailer = r.Parent, m.model.Ref(r, "addition"), " /* «include» */"
			if to != nil && m.written(to) {
				if cat, _ := m.classify(to); cat != catUseCaseDef {
					continue
				}
			}
		case "Extend":
			from, to, trailer = r.Parent, m.model.Ref(r, "extendedCase"), " /* "+m.extendComment(r)+" */"
		default:
			clients, suppliers := m.model.Refs(r, "client"), m.model.Refs(r, "supplier")
			if len(clients) != 1 || len(suppliers) != 1 {
				continue
			}
			from, to, trailer = clients[0], suppliers[0], " /* «Refine» */"
		}
		if from == nil || to == nil || !m.written(from) || !m.written(to) {
			continue
		}
		m.usageConnection(r, m.nameOf(r), from, to, trailer)
	}
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

// placeActors names the actor usage each link is written as, once every member
// of the use cases is named, and registers it in the use case's body.
func (m *migration) placeActors(links []*actorLink) {
	for _, link := range links {
		if link.block {
			m.connectActor(link)
			continue
		}
		base := m.nameOf(link.end)
		if base == "" {
			base = lowerFirst(link.actor.Name)
		}
		link.name = m.freshName(link.useCase, base)
		m.actors[link.assoc] = link
		m.extras[link.useCase] = append(m.extras[link.useCase], func() { m.actorMember(link) })
		m.connectActor(link)
	}
}

// actorMember writes the actor usage of a link in its use case's body and
// reports the association's ends it stands for.
func (m *migration) actorMember(link *actorLink) {
	decl := "actor " + writeName(link.name) + " : " + m.ref(link.actor, link.useCase)
	mult, mnote := m.multiplicity(link.end)
	m.w.line(decl + mult + ";")
	target := m.actorTarget(link)
	for _, end := range m.model.Refs(link.assoc, "memberEnd") {
		if end.Parent != link.assoc {
			continue
		}
		note := mnote
		if end != link.end {
			note = "the end at the use case is the use case def the actor is declared in"
		}
		m.add(end, verdictFor(note), target, note)
	}
}

// actorTarget is the report target of a link's actor usage.
func (m *migration) actorTarget(link *actorLink) string {
	return m.qualified(append(m.segments(link.useCase), link.name))
}

// useCaseBody writes the body of a use case def: its subject, the actors its
// associations link it to, its members and inclusions, and its classifier behavior.
func (m *migration) useCaseBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	m.subjects(e)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeComments(e)
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

// hasActors reports whether an actor usage is written in the use case's body:
// for an association reaching an actor, or for a property typed by one.
func (m *migration) hasActors(useCase *sysmlv1.Element) bool {
	for _, link := range m.actors {
		if link.useCase == useCase {
			return true
		}
	}
	for _, p := range useCase.Owned("ownedAttribute") {
		t := m.model.Ref(p, "type")
		if p.Type != "Port" && t != nil && t.Type == "Actor" && m.written(t) {
			return true
		}
	}
	return false
}

// include writes a UML Include as an include use case usage of the including
// use case, named after the included one.
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
	if cat, _ := m.classify(added); cat != catUseCaseDef {
		m.unmapped(inc, "the included element "+qualifiedName(added)+" becomes a "+cat.keyword()+", which an include use case cannot be typed by")
		return
	}
	name := m.nameOf(inc)
	if name == "" {
		name = m.freshName(inc.Parent, lowerFirst(m.nameFor(added)))
		m.names[inc], m.synthesized[inc] = name, true
	}
	m.madeUp(inc, writeName(name))
	m.wroteEdge(inc, m.scope, "include", name)
	m.w.line("include use case " + writeName(name) + " : " + m.ref(added, m.scope) + ";")
	if conn := m.conns[inc]; conn != nil {
		m.wroteEdgeAlso(inc, conn.host, "connection", nil, conn.name)
	}
	m.add(inc, Mapped, m.v2Name(inc), "")
	m.stereotypeComments(inc)
}

// extend writes a UML Extend as a dependency of the extending use case on the
// extended one, keeping the extension points and condition as a comment: v2
// has no extend relationship.
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
	comment := m.extendComment(ext)
	if detail := m.extensionDetail(ext); detail != "" {
		note += "; it applies " + detail + ", which only a comment keeps"
	}
	if conn := m.conns[ext]; conn != nil {
		m.wroteEdgeAlso(ext, conn.host, "connection", nil, conn.name)
		m.add(ext, Approximated, conn.target(m), strings.Replace(note, "a plain dependency on", "a connection between usages of the extending use case and", 1))
		m.stereotypeComments(ext)
		return
	}
	from, to := m.ref(ext.Parent, m.scope), m.ref(extended, m.scope)
	decl, target := "dependency ", ""
	name := m.nameOf(ext)
	if name == "" {
		if base := m.edgeName(ext, spoken(from)+" to "+spoken(to)); base != "" {
			name = m.freshName(m.scope, base)
			m.names[ext] = name
		}
	}
	if name != "" {
		decl += writeName(name) + " from "
		target = m.v2Name(ext)
		m.madeUp(ext, writeName(name))
	}
	m.wroteEdge(ext, m.scope, "dependency", name)
	m.w.line(decl + from + " to " + to + "; /* " + comment + " */")
	m.add(ext, Approximated, target, note)
	m.stereotypeComments(ext)
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
