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
		if actor == nil || useCase == nil || actor.Type != "Actor" || useCase.Type != "UseCase" {
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
		base := m.nameOf(link.end)
		if base == "" {
			base = lowerFirst(link.actor.Name)
		}
		link.name = m.freshName(link.useCase, base)
		m.actors[link.assoc] = link
		m.extras[link.useCase] = append(m.extras[link.useCase], func() { m.actorMember(link) })
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
		m.names[inc] = name
	}
	m.w.line("include use case " + writeName(name) + " : " + m.ref(added, m.scope) + ";")
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
	comment := "extends"
	if detail := m.extensionDetail(ext); detail != "" {
		comment += " " + detail
		note += "; it applies " + detail + ", which only a comment keeps"
	}
	decl, target := "dependency ", ""
	if name := m.nameOf(ext); name != "" {
		decl += writeName(name) + " from "
		target = m.v2Name(ext)
	}
	m.w.line(decl + m.ref(ext.Parent, m.scope) + " to " + m.ref(extended, m.scope) + "; /* " + comment + " */")
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
