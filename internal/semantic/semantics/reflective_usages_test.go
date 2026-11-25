package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// owningUsage/owningDefinition are the owningType when it is a Usage, respectively a
// Definition; usage/directedUsage are the Usages among a type's features (SysML v2 §8.3.6.4).
func TestReflectiveOwningUsageAndDefinition(t *testing.T) {
	const src = `package P {
		part def A;
		occurrence def Occ {
			occurrence occ1 {
				occurrence occ2;
				assert constraint c1 { true }
			}
		}
		occurrence top { occurrence child; }
		action def Act { in x : A; out y : A; part p : A; }
	}`
	for _, fixture := range reflectiveFixtures(t, "owning_usage.sysml", source.KindSysML, src) {
		occ1 := nestedSym(t, fixture.root, "P::Occ::occ1")
		assertReflectiveElements(t, fixture.model, occ1, "owningUsage")
		assertReflectiveElements(t, fixture.model, occ1, "owningDefinition", "P::Occ")
		assertReflectiveElements(t, fixture.model, occ1, "usage", "P::Occ::occ1::occ2", "P::Occ::occ1::c1")
		assertReflectiveElements(t, fixture.model, occ1, "directedUsage")
		occ2 := nestedSym(t, fixture.root, "P::Occ::occ1::occ2")
		assertReflectiveElements(t, fixture.model, occ2, "owningUsage", "P::Occ::occ1")
		assertReflectiveElements(t, fixture.model, occ2, "owningDefinition")
		top := nestedSym(t, fixture.root, "P::top")
		assertReflectiveElements(t, fixture.model, top, "owningUsage")
		assertReflectiveElements(t, fixture.model, top, "owningDefinition")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::top::child"), "owningUsage", "P::top")
		act := nestedSym(t, fixture.root, "P::Act")
		assertReflectiveElements(t, fixture.model, act, "directedUsage", "P::Act::x", "P::Act::y")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Act::p"), "owningDefinition", "P::Act")
		occ := nestedSym(t, fixture.root, "P::Occ")
		assertReflectiveElements(t, fixture.model, occ, "directedUsage")
		for _, feature := range []string{"owningUsage", "owningDefinition"} {
			if _, ok := fixture.model.ReflectiveElements(occ, feature); ok {
				t.Errorf("P::Occ.%s is derived; a definition is no Usage", feature)
			}
		}
	}
}
