package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A subject of a requirement or case is the ReferenceUsage its SubjectMembership declares
// (SysML.xtext SubjectMember), an `in` parameter that is referential, as the pilot exports it.
func TestSubjectIsAReferenceUsageParameter(t *testing.T) {
	const src = `package P {
		part def Subsystem;
		requirement def MassLimit { subject sub : Subsystem; }
		use case def ObserveTarget { subject obs : Subsystem; actor who : Subsystem; }
		part def Ordinary { part p : Subsystem; }
	}`
	for _, fixture := range reflectiveFixtures(t, "subject.sysml", source.KindSysML, src) {
		for _, path := range []string{"P::MassLimit::sub", "P::ObserveTarget::obs"} {
			sym := nestedSym(t, fixture.root, path)
			if sym.Kind != symbols.SymbolReferenceUsage {
				t.Errorf("%s: kind %v, want referenceUsage", path, sym.Kind)
			}
			if meta := relMetaName(fixture.model, sym); meta != "ReferenceUsage" {
				t.Errorf("%s: metaclass %s, want ReferenceUsage", path, meta)
			}
			if direction, ok := ReflectiveDirection(sym); !ok || direction != ast.DirIn {
				t.Errorf("%s: direction %v (%t), want in", path, direction, ok)
			}
			for feature, want := range map[string]bool{"isReference": true, "isComposite": false} {
				got, ok := fixture.model.ReflectiveFeatureValue(sym, feature)
				if !ok || got.Kind != symbols.FilterValueBool || got.Bool != want {
					t.Errorf("%s.%s = %v (%t), want %t", path, feature, got, ok, want)
				}
			}
		}
		// An actor is a PartUsage parameter: `in` and referential too, without writing either.
		actor := nestedSym(t, fixture.root, "P::ObserveTarget::who")
		if meta := relMetaName(fixture.model, actor); meta != "PartUsage" {
			t.Errorf("actor: metaclass %s, want PartUsage", meta)
		}
		if direction, ok := ReflectiveDirection(actor); !ok || direction != ast.DirIn {
			t.Errorf("actor: direction %v (%t), want in", direction, ok)
		}
		if got, ok := fixture.model.ReflectiveFeatureValue(actor, "isComposite"); !ok || got.Bool {
			t.Errorf("actor.isComposite = %v (%t), want false", got, ok)
		}
		part := nestedSym(t, fixture.root, "P::Ordinary::p")
		if direction, ok := ReflectiveDirection(part); !ok || direction != ast.DirNone {
			t.Errorf("P::Ordinary::p: direction %v (%t), want none", direction, ok)
		}
		if got, ok := fixture.model.ReflectiveFeatureValue(part, "isComposite"); !ok || !got.Bool {
			t.Errorf("P::Ordinary::p.isComposite = %v (%t), want true", got, ok)
		}
	}
}
