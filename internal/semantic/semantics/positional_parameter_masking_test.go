package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const positionalMaskingSource = `
action def TakePicture { in scene; out picture; }
action takePicture : TakePicture { in s; out p; }
action mismatched : TakePicture { out q; }
action byClause : TakePicture { in x :>> picture; }
action def B1 { in p1; }
action def B2 { in p2; }
action two : B1, B2 { in item; }
calc def F { return r; }
calc g : F { return s; }
`

func parameterOf(t *testing.T, root *symbols.Scope, owner, name string) *symbols.Symbol {
	t.Helper()
	found := sym(t, root, owner).Scope.LookupLocalAll(name)
	if len(found) != 1 {
		t.Fatalf("%s::%s: %d declarations", owner, name, len(found))
	}
	return found[0]
}

// A parameter without a clause redefines the general's parameter at its position
// (KerML 8.4.4.6), which its owner then does not inherit, as the pilot resolves it.
func TestPositionalParameterRedefinitionMasksTheGeneralParameter(t *testing.T) {
	m, root := buildModel(t, positionalMaskingSource)
	scene := parameterOf(t, root, "TakePicture", "scene")
	picture := parameterOf(t, root, "TakePicture", "picture")

	takePicture := sym(t, root, "takePicture")
	if !m.InheritanceMasked(takePicture, scene) || !m.InheritanceMasked(takePicture, picture) {
		t.Fatalf("s and p redefine scene and picture by position; neither is inherited")
	}
	if names := visibleNames(m, takePicture); names["scene"] != 0 || names["picture"] != 0 {
		t.Fatalf("takePicture's body must not see the redefined names: %v", names)
	}

	// A position whose directions disagree is a violation, not an inherited feature.
	mismatched := sym(t, root, "mismatched")
	q := parameterOf(t, root, "mismatched", "q")
	if got := m.MaskedParameterRedefinitions(q); len(got) != 1 || got[0] != scene {
		t.Fatalf("q masks scene at its position whatever the direction, got %v", got)
	}
	if got := m.ImplicitParameterRedefinitions(q); len(got) != 0 {
		t.Fatalf("q redefines nothing by position with a conforming direction, got %v", got)
	}
	if !m.InheritanceMasked(mismatched, scene) || m.InheritanceMasked(mismatched, picture) {
		t.Fatalf("mismatched does not inherit scene and still inherits picture")
	}

	// A clause masks what it names; the position then redefines nothing.
	byClause := sym(t, root, "byClause")
	if m.InheritanceMasked(byClause, scene) || !m.InheritanceMasked(byClause, picture) {
		t.Fatalf("x :>> picture masks picture only")
	}
}

// Two generals fill the position: both parameters are redefined and neither is inherited.
func TestPositionalParameterRedefinitionMasksUnderEveryGeneral(t *testing.T) {
	m, root := buildModel(t, positionalMaskingSource)
	two := sym(t, root, "two")
	p1 := parameterOf(t, root, "B1", "p1")
	p2 := parameterOf(t, root, "B2", "p2")
	if !m.InheritanceMasked(two, p1) || !m.InheritanceMasked(two, p2) {
		t.Fatalf("item redefines p1 and p2")
	}
	if names := visibleNames(m, two); names["p1"] != 0 || names["p2"] != 0 {
		t.Fatalf("two's body must not see p1 or p2: %v", names)
	}
}

// A result parameter redefines the general's result parameter, position aside.
func TestResultParameterRedefinitionMasksTheGeneralResult(t *testing.T) {
	m, root := buildModel(t, positionalMaskingSource)
	g := sym(t, root, "g")
	r := parameterOf(t, root, "F", "r")
	if !m.InheritanceMasked(g, r) {
		t.Fatalf("g's result redefines F's result r")
	}
}

// An unnamed result redefining the general's by position answers to its name
// (KerML 8.2.4) wherever the owner's specializations look it up: by name, among
// their members, and in their shape.
func TestUnnamedResultRedefinitionAnswersToTheRedefinedName(t *testing.T) {
	m, root := buildModel(t, `
calc def Eval { return result : Boolean; }
calc def BoolEval :> Eval { return : Boolean; }
calc def Check :> BoolEval { }
`)
	check := sym(t, root, "Check")
	var unnamed *symbols.Symbol
	sym(t, root, "BoolEval").Scope.ForEachMember(func(s *symbols.Symbol) bool { unnamed = s; return false })
	if unnamed == nil || unnamed.Name != "" {
		t.Fatalf("BoolEval's return is unnamed, got %v", unnamed)
	}
	if got := m.EffectiveNameOf(unnamed); got != "result" {
		t.Fatalf("EffectiveNameOf(return) = %q, want result", got)
	}
	if got, ok := m.LookupMember(check, "result"); !ok || got != unnamed {
		t.Fatalf("LookupMember(Check, result) = %v, %v; want BoolEval's return", got, ok)
	}
	if !m.HasMember(check, unnamed) {
		t.Fatalf("Check's members must hold BoolEval's return under its redefined name")
	}
	var names []string
	for _, f := range m.ShapeFeatures(check) {
		names = append(names, f.Name)
		if f.Name == "result" && f.Symbol != unnamed {
			t.Fatalf("shape result is %v, want BoolEval's return", f.Symbol)
		}
	}
	if len(names) != 1 || names[0] != "result" {
		t.Fatalf("shape names = %v, want [result]", names)
	}
}
