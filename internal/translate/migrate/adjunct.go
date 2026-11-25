package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// adjunct is what an «AdjunctProperty» stands for: the usage its principal is
// written as, or the reason the two are written apart.
type adjunct struct {
	principal *sysmlv1.Element
	apart     string
}

// adjunctPrincipals are the call actions and object nodes SysML v1 admits as a
// principal beside a connector, variable, parameter, submachine state and interaction use.
var adjunctPrincipals = map[string]bool{
	"CallBehaviorAction": true, "CallOperationAction": true, "StartObjectBehaviorAction": true,
	"ActivityParameterNode": true, "CentralBufferNode": true, "DataStoreNode": true, "ExpansionNode": true,
	"InputPin": true, "OutputPin": true, "ValuePin": true, "ActionInputPin": true,
	"Connector": true, "Variable": true, "Parameter": true, "InteractionUse": true,
}

// adjunctOf reads the «AdjunctProperty» applied to p against the constraints v1 puts on
// its principal; nil for a property the stereotype is not applied to.
func (m *migration) adjunctOf(p *sysmlv1.Element) *adjunct {
	if p.Type != "Property" || len(p.Stereotypes) == 0 {
		return nil
	}
	if a, ok := m.adjuncts[p]; ok {
		return a
	}
	var a *adjunct
	if s := stereo(p, "AdjunctProperty"); s != nil {
		a = m.readAdjunct(p, s)
	}
	m.adjuncts[p] = a
	return a
}

// principalPhrase opens a reason an adjunct stands apart with the principal it names.
func principalPhrase(principal *sysmlv1.Element) string {
	return "its principal " + describe(principal)
}

func (m *migration) readAdjunct(p *sysmlv1.Element, s *sysmlv1.Stereotype) *adjunct {
	var principal *sysmlv1.Element
	if ids := s.IDs("principal"); len(ids) > 0 {
		principal = m.model.Lookup(ids[0])
	}
	if principal == nil || principal.IsProxy() {
		return &adjunct{apart: "its principal is not in the document"}
	}
	if param := m.model.Ref(principal, "parameter"); param != nil && principal.Type == "ActivityParameterNode" {
		// A parameter node is written as the parameter it stands for.
		principal = param
	}
	a := &adjunct{principal: principal}
	if nodeKind(principal) == nodePin {
		m.principalPins[principal] = true
	}
	submachine := principal.Type == "State" && m.model.Ref(principal, "submachine") != nil
	switch {
	case !adjunctPrincipals[principal.Type] && !submachine:
		a.apart = principalPhrase(principal) + " is a " + principal.Type + ", which SysML v1 admits as no principal"
	case !m.adjunctOwns(p.Parent, principal):
		a.apart = principalPhrase(principal) + " is owned by " + qualifiedName(principal.Parent) + ", neither its owner nor a generalization of it"
	case !m.adjunctTyped(p, principal):
		a.apart = "its type is neither that of its principal " + describe(principal) + " nor a generalization of it"
	}
	return a
}

// adjunctOwns reports whether owner, or a classifier it specializes, owns principal at any depth.
func (m *migration) adjunctOwns(owner, principal *sysmlv1.Element) bool {
	if owner == nil {
		return false
	}
	for o := principal.Parent; o != nil; o = o.Parent {
		if o == owner || m.inherits(owner, o) {
			return true
		}
	}
	return false
}

// adjunctTyped reports whether the type of p is what its principal invokes or holds, or
// a generalization of it; a property with no type disagrees with nothing.
func (m *migration) adjunctTyped(p, principal *sysmlv1.Element) bool {
	t := m.model.Ref(p, "type")
	if t == nil {
		return true
	}
	var of []*sysmlv1.Element
	switch principal.Type {
	case "CallBehaviorAction":
		of = m.model.Refs(principal, "behavior")
	case "CallOperationAction":
		if op := m.model.Ref(principal, "operation"); op != nil {
			of = m.model.Refs(op, "method")
		}
	case "StartObjectBehaviorAction":
		// The behavior started is the object's own, which only its type at run time tells.
		return true
	case "State":
		of = m.model.Refs(principal, "submachine")
	case "InteractionUse":
		of = m.model.Refs(principal, "refersTo")
	default:
		of = m.model.Refs(principal, "type")
	}
	if len(of) == 0 {
		return false
	}
	for _, pt := range of {
		if pt != t && !m.inherits(pt, t) {
			return false
		}
	}
	return true
}

// principalOf returns the element whose usage stands for the adjunct property p as well;
// nil when p is no adjunct property or is written apart from its principal.
func (m *migration) principalOf(p *sysmlv1.Element) *sysmlv1.Element {
	a := m.adjunctOf(p)
	if a == nil || a.apart != "" || m.resolvingAdjunct[p] {
		return nil
	}
	m.resolvingAdjunct[p] = true
	defer delete(m.resolvingAdjunct, p)
	if !m.written(a.principal) {
		return nil
	}
	return a.principal
}

// surviving returns the element e is written as: the principal of a collapsed adjunct
// property, else e itself.
func (m *migration) surviving(e *sysmlv1.Element) *sysmlv1.Element {
	if e == nil {
		return nil
	}
	if p := m.principalOf(e); p != nil {
		return p
	}
	return e
}

// adjunctApart says why the adjunct property p is written apart from its principal; "" for
// a property that is no adjunct property or is collapsed into its principal.
func (m *migration) adjunctApart(p *sysmlv1.Element) string {
	a := m.adjunctOf(p)
	if a == nil || m.principalOf(p) != nil {
		return ""
	}
	why := a.apart
	switch {
	case why != "":
	case nodeKind(a.principal) == nodePin && a.principal.Parent != nil && a.principal.Parent.Type != "CallBehaviorAction":
		why = principalPhrase(a.principal) + " is a pin of the " + a.principal.Parent.Type + " " + describe(a.principal.Parent) +
			", and only a pin of a call behavior action is written as one usage with its property"
	default:
		why = principalPhrase(a.principal) + " is not written as a v2 usage"
	}
	return "«AdjunctProperty» is not collapsed into its principal: " + why
}

// adjunctFeature reports the adjunct property p as the usage of its principal and says
// so; false leaves p to be written as a usage of its own, noting why on its entry.
func (m *migration) adjunctFeature(p *sysmlv1.Element) bool {
	principal := m.principalOf(p)
	if principal == nil {
		if why := m.adjunctApart(p); why != "" {
			m.downgrade(p, why+"; the property is written as a usage of its own")
		}
		return false
	}
	for _, c := range p.Children {
		if c.Role == "lowerValue" || c.Role == "upperValue" {
			// The usage keeps the multiplicity of the principal, which v1 has the property's admit.
			continue
		}
		m.unmapped(c, "owned by an «AdjunctProperty» that is written as the usage of its principal, which has no place for a "+kindOf(c))
	}
	var others []string
	for _, s := range p.Stereotypes {
		if !appliesStandard(s, "AdjunctProperty") && toolProfile(s.Namespace) == "" {
			others = append(others, "«"+s.Name+"»")
		}
	}
	verdict, lost := Mapped, ""
	if len(others) > 0 {
		verdict = Approximated
		lost = "; " + strings.Join(others, ", ") + " applied to the property is not written: the usage of its principal carries no application of the property's"
	}
	written := m.v2Name(principal)
	if nodeKind(principal) == nodePin {
		// A pin is recorded as the parameter of its action, as its own entry is.
		written = m.v2Name(principal.Parent) + "." + m.nameFor(principal)
	}
	m.add(p, verdict, written, "«AdjunctProperty» and its principal "+kindOf(principal)+" "+describe(principal)+
		" are one v2 usage, written for the principal; what referred to the property refers to that usage"+lost)
	return true
}

// adjunctChain returns the written names leading from the owner of the collapsed adjunct property p to the
// usage of its principal, more than one for a principal nested in the owner's body; nil for another property.
func (m *migration) adjunctChain(p *sysmlv1.Element) []string {
	principal := m.principalOf(p)
	if principal == nil {
		return nil
	}
	full := m.path(principal)
	steps := full[len(full)-1:]
	for o := principal.Parent; o != nil; o = o.Parent {
		if o == p.Parent || m.inherits(p.Parent, o) {
			if base := m.path(o); len(base) < len(full) {
				steps = full[len(base):]
			}
			break
		}
	}
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = writeName(s.name)
	}
	return names
}

// adjunctNestedIn returns the member of p's owner that the principal of the collapsed adjunct property p is
// written inside; nil when the principal is a member of the owner itself. It names nothing, so planning may ask.
func (m *migration) adjunctNestedIn(p *sysmlv1.Element) *sysmlv1.Element {
	principal := m.principalOf(p)
	if principal == nil {
		return nil
	}
	for o := memberOwner(principal); o != nil && o != p.Parent && !m.inherits(p.Parent, o); o = memberOwner(o) {
		if o.Type != "Region" || m.regionWrittenAsState(o) {
			return o
		}
	}
	return nil
}

// adjunctExposure says what reaches the usage of principal through an adjunct property
// collapsed into it, so the usage must not be hidden; "" when nothing does.
func (m *migration) adjunctExposure(principal *sysmlv1.Element) string {
	for _, p := range m.adjunctExposed[principal] {
		if m.principalOf(p) == principal {
			return m.exposed[p]
		}
	}
	return ""
}
