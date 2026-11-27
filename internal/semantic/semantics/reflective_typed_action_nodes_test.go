package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An action node whose declaration states a prefix, typing, multiplicity or specialization is still the one
// element of its node's metaclass (SysML.xtext ActionNodePrefix, ActionNodeUsageDeclaration), owning no second node.
func TestReflectiveTypedActionNodeIsOneElement(t *testing.T) {
	const src = `package P {
		part def PD { attribute w; }
		part p : PD;
		attribute def Sig;
		action def T;
		action base : T;
		metadata def Tag;
		action def A {
			action typedAssign : T assign p.w := 1;
			action multAssign[1] assign p.w := 1;
			action subsetAssign :> base assign p.w := 1;
			action redefAssign :>> base assign p.w := 1;
			#Tag action taggedAssign : T assign p.w := 1;
			action typedIf : T if true { }
			action typedWhile : T while true { }
			action typedFor : T for i in (1, 2) { }
			action typedSend : T send Sig() to p;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "typed-action-nodes.sysml", source.KindSysML, src) {
		for path, want := range map[string]string{
			"P::A::typedAssign":  "AssignmentActionUsage",
			"P::A::multAssign":   "AssignmentActionUsage",
			"P::A::subsetAssign": "AssignmentActionUsage",
			"P::A::redefAssign":  "AssignmentActionUsage",
			"P::A::taggedAssign": "AssignmentActionUsage",
			"P::A::typedIf":      "IfActionUsage",
			"P::A::typedWhile":   "WhileLoopActionUsage",
			"P::A::typedFor":     "ForLoopActionUsage",
			"P::A::typedSend":    "SendActionUsage",
		} {
			sym := nestedSym(t, fixture.root, path)
			if mc := fixture.model.MetaclassOf(sym); mc == nil || mc.Name != want {
				t.Errorf("%s metaclass = %v, want %s", path, mc, want)
			}
			for _, node := range fixture.model.statementNodes(sym) {
				if mc := fixture.model.MetaclassOf(node); mc != nil && mc.Name == want {
					t.Errorf("%s owns a second %s", path, want)
				}
			}
		}
		owner := nestedSym(t, fixture.root, "P::A")
		if nodes := fixture.model.statementNodes(owner); len(nodes) != 0 {
			t.Errorf("P::A owns %d statement nodes beside its named ones, want none", len(nodes))
		}
	}
}
