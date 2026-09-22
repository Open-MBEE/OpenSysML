package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// realizationForm is how a UML InterfaceRealization is written.
type realizationForm int

const (
	// realizeRefused: the realization has no v2 form; the note says why.
	realizeRefused realizationForm = iota
	// realizeSpecialize: the realizing definition specializes the interface's.
	realizeSpecialize
	// realizePort: the realizing part def gets a port typed by the interface's port def.
	realizePort
	// realizeCarried: a port of the realizing part def is already typed by the interface's port def.
	realizeCarried
)

// realization decides how ir, owned by the classifier that realizes its
// contract, is written: a definition specializes an interface of its own kind,
// a part def carries a port def as a port, and any other pairing is refused.
// target is the interface, or the port that carries the realization.
func (m *migration) realization(ir *sysmlv1.Element) (form realizationForm, target *sysmlv1.Element, note string) {
	client := ir.Parent
	contract := m.model.Ref(ir, "contract")
	if contract == nil {
		if suppliers := m.model.Refs(ir, "supplier"); len(suppliers) > 0 {
			contract = suppliers[0]
		}
	}
	switch {
	case contract == nil:
		d := m.dangling(ir, "contract")
		if d == "" {
			d = m.dangling(ir, "supplier")
		}
		return realizeRefused, nil, joinNotes("the realized interface is not in the document", d)
	case contract.IsProxy() || m.isLibrary(contract):
		return realizeRefused, contract, "the realized interface " + qualifiedName(contract) + " is library content, which is not written"
	case client == nil:
		return realizeRefused, contract, "no classifier owns the realization"
	case !m.written(client):
		return realizeRefused, contract, "the realizing classifier " + qualifiedName(client) + " is not migrated"
	case !m.written(contract):
		_, why := m.classify(contract)
		return realizeRefused, contract, joinNotes("the realized interface "+qualifiedName(contract)+" is not migrated", why)
	}
	ccat, _ := m.classify(client)
	tcat, _ := m.classify(contract)
	switch {
	case ccat == tcat && specializable(ccat):
		return realizeSpecialize, contract, ""
	case ccat == catPartDef && tcat == catPortDef:
		if p := m.portTypedBy(client, contract); p != nil {
			return realizeCarried, p, "the realization is carried by the port " + m.nameOf(p) + ", which is typed by the interface's port def: a part def cannot specialize a port def"
		}
		return realizePort, contract, "the realization is written as a port typed by the interface's port def: a part def cannot specialize a port def"
	}
	return realizeRefused, contract, "a " + ccat.keyword() + " cannot specialize the " + tcat.keyword() + " the interface " + qualifiedName(contract) + " becomes"
}

// portTypedBy returns the first written port of owner whose type is t, if any.
func (m *migration) portTypedBy(owner, t *sysmlv1.Element) *sysmlv1.Element {
	for _, p := range owner.Owned("ownedAttribute") {
		if p.Type == "Port" && m.model.Ref(p, "type") == t && m.written(p) {
			return p
		}
	}
	return nil
}

// specializable reports whether a definition of category cat may specialize
// another of the same category with `:>`.
func specializable(cat category) bool {
	switch cat {
	case catPartDef, catPortDef, catAttributeDef, catConstraintDef, catRequirementDef, catConnectionDef,
		catVerificationDef, catItemDef, catActionDef, catCalcDef, catStateDef:
		return true
	}
	return false
}

// realizedGenerals lists the interfaces e realizes that its declaration
// specializes, as references from scope, skipping any already among refs.
func (m *migration) realizedGenerals(e *sysmlv1.Element, refs []string) []string {
	var more []string
	for _, ir := range e.Owned("interfaceRealization") {
		form, contract, _ := m.realization(ir)
		if form != realizeSpecialize {
			continue
		}
		ref := m.ref(contract, m.scope)
		if !slices.Contains(refs, ref) && !slices.Contains(more, ref) {
			more = append(more, ref)
		}
	}
	return more
}

// interfaceRealization writes an InterfaceRealization as a member of its
// realizing classifier's body and reports it; a specialization was already
// written in the declaration.
func (m *migration) interfaceRealization(ir *sysmlv1.Element) {
	form, contract, note := m.realization(ir)
	switch form {
	case realizeRefused:
		m.unmapped(ir, note)
		return
	case realizeSpecialize:
		m.add(ir, Mapped, m.v2Name(ir.Parent), "written as a specialization of the interface's "+m.keywordOf(contract))
	case realizePort:
		name := m.freshName(ir.Parent, lowerFirst(m.nameOf(contract)))
		m.names[ir] = name
		m.w.line("port " + writeName(name) + " : " + m.ref(contract, m.scope) + ";")
		m.add(ir, Approximated, m.v2Name(ir), note)
	case realizeCarried:
		m.add(ir, Approximated, m.v2Name(contract), note)
	}
	m.stereotypeComments(ir)
}

// keywordOf is the v2 declaration keyword e is written with.
func (m *migration) keywordOf(e *sysmlv1.Element) string {
	cat, _ := m.classify(e)
	return cat.keyword()
}
