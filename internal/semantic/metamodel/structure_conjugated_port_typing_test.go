package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// conjugatedPortTypingModel types a port by a conjugated port definition, so the
// port owns a ConjugatedPortTyping and Cmd's implied '~Cmd' owns a PortConjugation.
const conjugatedPortTypingModel = `package P {
    port def Cmd;
    part x { port cmdIn : ~Cmd; }
}`

// ownedRelationshipOf is the one owned relationship of element whose metaclass is
// the expected one, read through the symbol structure.
func ownedRelationshipOf(t *testing.T, e *Evaluator, element Element, expected string) Element {
	t.Helper()
	owned, ok := e.Property(element, "Element", "ownedRelationship")
	if !ok || owned.Kind != SequenceValue {
		t.Fatalf("Element::ownedRelationship = %#v, %t", owned, ok)
	}
	var found []Element
	for _, value := range owned.Values {
		if value.Kind != ElementValue {
			t.Fatalf("ownedRelationship value %#v is no element", value)
		}
		if is, known := e.isMetaclass(value.Element, expected); known && is {
			found = append(found, value.Element)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d owned %s relationships, want 1", len(found), expected)
	}
	return found[0]
}

// A ConjugatedPortTyping's conjugatedPortDefinition is its target, the implied Cmd::'~Cmd' (SysML v2
// 8.3.12.3), its portDefinition that conjugate's original; a PortConjugation's is its owning type (8.3.12.4).
func TestSymbolStructureConjugatedPortTypingDefinitions(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "conjugated.sysml", conjugatedPortTypingModel)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	lookup := func(name string) *symbols.Symbol {
		syms := e.resolver.Index().LookupQualified(name)
		if len(syms) != 1 {
			t.Fatalf("%s resolves to %d symbols", name, len(syms))
		}
		return syms[0]
	}
	portSym, originalSym := lookup("P::x::cmdIn"), lookup("P::Cmd")
	conjugateSym := e.model.ConjugatedPortDefinitionOf(originalSym)
	if conjugateSym == nil {
		t.Fatal("P::Cmd has no implied conjugated port definition")
	}
	port, original, conjugate := structure.ElementFor(portSym), structure.ElementFor(originalSym), structure.ElementFor(conjugateSym)
	typing := ownedRelationshipOf(t, e, port, "ConjugatedPortTyping")
	conjugation := ownedRelationshipOf(t, e, conjugate, "PortConjugation")
	for _, read := range []struct {
		element           Element
		metaclass, name   string
		want              Element
		wantQualifiedName string
	}{
		{typing, "ConjugatedPortTyping", "conjugatedPortDefinition", conjugate, "P::Cmd::'~Cmd'"},
		{typing, "ConjugatedPortTyping", "portDefinition", original, "P::Cmd"},
		{conjugation, "PortConjugation", "conjugatedPortDefinition", conjugate, "P::Cmd::'~Cmd'"},
	} {
		value, ok := e.Property(read.element, read.metaclass, read.name)
		if !ok || !value.Success {
			t.Errorf("%s::%s = %#v, %t: want it derived", read.metaclass, read.name, value, ok)
			continue
		}
		if value.Kind != ElementValue || value.Element.Key() != read.want.Key() {
			t.Errorf("%s::%s = %#v; want %s", read.metaclass, read.name, value, read.wantQualifiedName)
		}
	}
}
