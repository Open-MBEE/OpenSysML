package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w8cVariableFeatureMessages counts the variability rules' messages for src analyzed under name.
func w8cVariableFeatureMessages(t *testing.T, name, src string) map[string]int {
	t.Helper()
	got := make(map[string]int)
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		switch d.Message {
		case msgInitialValueNotVariable, msgConstantNotVariable, msgVariableFeatureOwner, msgPortionFeatureVariable:
			got[d.Message]++
		}
	}
	return got
}

func TestW8CInitialValueRequiresVariableKerML(t *testing.T) {
	src := `package P {
	class C {
		feature x : C := null;
		var feature v : C := null;
		feature b : C = null;
		feature d : C default := null;
		feature e : C default = null;
		feature y : C;
		var feature w : C;
	}
	class D :> C {
		feature :>> y := null;
		var feature :>> w := null;
	}
	behavior B {
		in var feature p : C := null;
		in feature q : C := null;
	}
	feature top : C := null;
}`
	// KerML: only `var` (or `const`) declares variability; the owner is immaterial.
	got := w8cVariableFeatureMessages(t, "<t>.kerml", src)
	if got[msgInitialValueNotVariable] != 5 {
		t.Errorf("want five %q (x, d, D::y, q, top), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgConstantNotVariable] != 0 {
		t.Errorf("unexpected owner or constant messages in %v", got)
	}
}

func TestW8CInitialValueRequiresVariableSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	item def I {
		var attribute x : Integer := 1;
		attribute y : Integer := 2;
	}
	part def PD {
		attribute z : Integer := 3;
		part sub : PD := null;
		action a : A := null;
	}
	action def A {
		in attribute p : Integer := 0;
	}
	occurrence def O {
		timeslice slice : O := null;
	}
	attribute def AD {
		attribute w : Integer := 4;
		attribute ok : Integer = 5;
	}
	requirement def R {
		subject s : PD := null;
		require constraint c := true;
	}
	requirement r : R {
		subject := null;
		assume constraint := true;
	}
	part top : PD := null;
}`
	// A usage of an occurrence type may time-vary, except a portion or a composite action.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 4 {
		t.Errorf("want four %q (PD::a, O::slice, AD::w, top), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgConstantNotVariable] != 0 {
		t.Errorf("unexpected owner or constant messages in %v", got)
	}
}

func TestW8CConstantRequiresVariableSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def A;
	attribute def B {
		constant attribute x : A;
	}
	part def PD {
		constant attribute c : Integer = 1;
		constant part p : PD;
		constant action a : Act;
	}
	action def Act;
	constant attribute top : Integer = 2;
}`
	// A data type's attribute never varies; a part's does, a composite action does not.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgConstantNotVariable] != 3 {
		t.Errorf("want three %q (B::x, PD::a, top), got %v", msgConstantNotVariable, got)
	}
	if got[msgInitialValueNotVariable] != 0 || got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected initial or owner messages in %v", got)
	}
}

func TestW8CConstantIsVariableKerML(t *testing.T) {
	src := `package P {
	class C {
		const feature k : C;
		const feature j : C := null;
		portion const feature pc : C;
		portion feature pp : C;
	}
	datatype D {
		const feature a : D;
		const feature b : D = null;
		const feature c : D := null;
	}
	struct S { const feature s : S; }
	assoc struct AS { const end feature a; const end feature b; var feature v : C; }
	assoc A { end feature a; end feature b; var feature v : C; }
	behavior B { in const feature p : C; }
	const feature top : C;
}`
	// KerML `const` implies `var`: a constant feature is never non-variable, and it
	// is owned by an occurrence type and is no portion like any variable feature.
	got := w8cVariableFeatureMessages(t, "<t>.kerml", src)
	if got[msgVariableFeatureOwner] != 5 {
		t.Errorf("want five %q (D::a, D::b, D::c, A::v, top), got %v", msgVariableFeatureOwner, got)
	}
	if got[msgPortionFeatureVariable] != 1 {
		t.Errorf("want one %q (C::pc), got %v", msgPortionFeatureVariable, got)
	}
	if got[msgConstantNotVariable] != 0 || got[msgInitialValueNotVariable] != 0 {
		t.Errorf("unexpected constant or initial messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesDoNotDoubleReport(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def AD {
		var attribute x : Integer := 1;
	}
}`
	// `var` in a data type is an owner error; a SysML usage's variability is derived, not declared.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgVariableFeatureOwner] != 1 || got[msgInitialValueNotVariable] != 1 {
		t.Errorf("want one owner and one initial message, got %v", got)
	}
}

func TestW8CVariableFeatureRulesNeedTheOccurrenceLibrary(t *testing.T) {
	src := `package P {
	occurrence def O {
		attribute x := 1;
		constant attribute c = 2;
	}
	item def I { attribute y := 3; }
}`
	// Without Occurrences::Occurrence a SysML usage's variability cannot be derived: stay silent.
	name := "<t>.sysml"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Message == msgInitialValueNotVariable || d.Message == msgConstantNotVariable {
			t.Errorf("library-free analysis reported %q", d.Message)
		}
	}
}

func TestW8CVariableFeatureRulesSurviveADuplicateOccurrenceName(t *testing.T) {
	idx := newTestIndex()
	shadow := "<shadow>.sysml"
	shadowRoot := parser.New(source.New(shadow,
		[]byte(`package Occurrences { occurrence def Occurrence; }`))).ParseFile()
	idx.AddDocument(shadow, shadowRoot)
	name := "<t>.sysml"
	root := parser.New(source.New(name, []byte(`package P {
	private import ScalarValues::*;
	attribute def AD { attribute x : Integer := 1; }
	part def PD { attribute y : Integer := 2; }
}`))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	got := 0
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Message == msgInitialValueNotVariable {
			got++
		}
	}
	// A workspace symbol sharing the library's name must not switch the rules off.
	if got != 1 {
		t.Errorf("want one initial-value message, got %d", got)
	}
}

func TestW8CVariableFeatureRulesIgnoreAWorkspaceOnlyOccurrenceName(t *testing.T) {
	idx := symbols.NewIndex()
	shadow := "<shadow>.sysml"
	shadowRoot := parser.New(source.New(shadow,
		[]byte(`package Occurrences { occurrence def Occurrence; }`))).ParseFile()
	idx.AddDocument(shadow, shadowRoot)
	name := "<t>.sysml"
	root := parser.New(source.New(name,
		[]byte(`package P { part def PD { attribute y := 2; constant attribute c = 1; } }`))).ParseFile()
	idx.AddDocument(name, root)
	// A workspace declaration is not the bundled library: variability stays underivable.
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Message == msgInitialValueNotVariable || d.Message == msgConstantNotVariable {
			t.Errorf("library-free analysis reported %q", d.Message)
		}
	}
}

func TestW8CVariableFeatureRulesAreElementScoped(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def AD {
		attribute w : Integer := 4;
		constant attribute k : Integer = 1;
		attribute v : Integer := missing;
		attribute broken : Missing := 5;
	}
	attribute def Unresolved :> Missing {
		attribute u : Integer := 6;
	}
	part def PD {
		var attribute ok : Integer := 7;
		part p : Missing := null;
	}
	part def Elsewhere :> Missing;
}`
	// Only the feature's own head and its owner's gate it: the unresolved `Missing`
	// elsewhere hides nothing, an unresolved value hides nothing, and an unresolved
	// typing on the feature or its owner hides only that feature.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 2 {
		t.Errorf("want two %q (AD::w, AD::v), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 1 {
		t.Errorf("want one %q (AD::k), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected owner messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesSurviveUnrelatedPackageMemberFailures(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute before : Integer := 1;
	constant attribute kBefore : Integer = 2;
	part unrelated : Missing;
	attribute after : Integer := 3;
	constant attribute kAfter : Integer = 4;
	attribute broken : Missing := 5;
	constant attribute kBroken : Missing = 6;
}`
	// A package has no typing to fail, so a member's failure gates only that member.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 2 {
		t.Errorf("want two %q (before, after), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 2 {
		t.Errorf("want two %q (kBefore, kAfter), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected owner messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesSurviveUnrelatedNestedOwnerFailures(t *testing.T) {
	src := `package Outer {
	private import ScalarValues::*;
	part unrelatedOuter : Missing;
	package Inner {
		attribute before : Integer := 1;
		part unrelatedInner : Missing;
		constant attribute kAfter : Integer = 2;
		namespace N {
			attribute n : Integer := 3;
			part unrelatedN : Missing;
		}
	}
}
library package Lib {
	private import ScalarValues::*;
	attribute l : Integer := 4;
	part unrelatedLib : Missing;
	constant attribute kl : Integer = 5;
}`
	// Nested packages, namespaces and library packages gate nothing either.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 3 {
		t.Errorf("want three %q (Inner::before, N::n, Lib::l), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 2 {
		t.Errorf("want two %q (Inner::kAfter, Lib::kl), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected owner messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesStillGateOnATypedOwnerHead(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	part unrelated : Missing;
	attribute def AD :> Missing {
		attribute w : Integer := 1;
	}
	attribute def Plain;
	attribute a : Missing {
		attribute x : Integer := 2;
		constant attribute k : Integer = 3;
	}
	attribute ok : Plain {
		attribute y : Integer := 4;
		attribute unrelatedNested : Missing;
	}
}`
	// A definition or usage owner still gates through its head, not through its body.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 1 {
		t.Errorf("want one %q (ok::y), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 0 || got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected constant or owner messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesSurviveRootStateAndControlNodeOwnerFailures(t *testing.T) {
	src := `private import ScalarValues::*;
attribute before : Integer := 1;
part unrelated : Missing;
constant attribute k : Integer = 2;
action def AD {
	attribute a : Integer := 3;
	fork f { attribute fa : Integer := 4; part unrelatedFork : Missing; }
	join j { attribute ja : Integer := 5; part unrelatedJoin : Missing; }
	merge m { attribute ma : Integer := 6; part unrelatedMerge : Missing; }
	decide d { attribute da : Integer := 7; part unrelatedDecide : Missing; }
	part unrelatedAD : Missing;
}
state def SD {
	state s { attribute sa : Integer := 8; part unrelatedState : Missing; }
	entry action e { attribute ea : Integer := 9; part unrelatedEntry : Missing; }
	part unrelatedSD : Missing;
}
attribute later : Integer := 10;
attribute def X { attribute x : Integer := 11; }`
	// The root namespace gates nothing; a control node or state is an occurrence,
	// so what it owns is variable and its members' failures gate only themselves.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 3 {
		t.Errorf("want three %q (before, later, X::x), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 1 {
		t.Errorf("want one %q (k), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgPortionFeatureVariable] != 0 {
		t.Errorf("unexpected owner or portion messages in %v", got)
	}
}

func TestW8CVariableOwnerAndPortionRulesAreElementScoped(t *testing.T) {
	src := `package P {
	feature unrelatedTop : Missing;
	datatype D {
		var feature before : D;
		feature unrelatedD : Missing;
		var feature later : D;
		var feature broken : Missing;
	}
	class C {
		portion var feature p : C;
		feature unrelatedC : Missing;
		portion var feature brokenPortion : Missing;
		var feature ok : C;
	}
	datatype Unresolved :> Missing {
		var feature hidden : Unresolved;
	}
	class Broken :> Missing {
		portion var feature hiddenPortion : Broken;
	}
	var feature top : D;
}
var feature root : P::D;`
	// The owner and portion rules gate like the value rules: on the feature's own
	// head and a typed owner's head, never on a sibling or a package.
	got := w8cVariableFeatureMessages(t, "<t>.kerml", src)
	if got[msgVariableFeatureOwner] != 4 {
		t.Errorf("want four %q (D::before, D::later, top, root), got %v", msgVariableFeatureOwner, got)
	}
	if got[msgPortionFeatureVariable] != 1 {
		t.Errorf("want one %q (C::p), got %v", msgPortionFeatureVariable, got)
	}
	if got[msgInitialValueNotVariable] != 0 || got[msgConstantNotVariable] != 0 {
		t.Errorf("unexpected initial or constant messages in %v", got)
	}
}

func TestW8CVariableFeatureRulesSurviveAnOwnerValueFailure(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def AD;
	attribute p : AD = missing {
		attribute a : Integer := 1;
		constant attribute k : Integer = 2;
	}
	attribute q : AD := missing {
		attribute b : Integer := 3;
	}
	attribute r : AD default missing {
		attribute c : Integer := 4;
	}
	attribute broken : Missing = 5 {
		attribute hidden : Integer := 6;
	}
}`
	// An owner's value is no part of its typing head, so it gates nothing nested.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 4 {
		t.Errorf("want four %q (p::a, q, q::b, r::c), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgConstantNotVariable] != 1 {
		t.Errorf("want one %q (p::k), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected owner messages in %v", got)
	}
}

// A `var`/`const` cross feature has no owning type (pilot: `Must be owned by an
// occurrence type`), and a SysML `constant` one never may time-vary.
func TestW8CCrossFeaturePrefixIsTheCrossFeatures(t *testing.T) {
	kerml := `package P {
	class C1; class C2;
	assoc A {
		end var x1 : C1 [1] feature x : C1;
		end const x2 : C2 [1] feature y : C2;
	}
	assoc B {
		end portion var x1 : C1 [1] feature x : C1;
		end var [1] feature y : C2;
	}
	assoc D {
		end in x1 : C1 [1] feature x : C1;
		end derived abstract composite x2 : C2 [1] feature y : C2;
	}
}`
	got := w8cVariableFeatureMessages(t, "<t>.kerml", kerml)
	if got[msgVariableFeatureOwner] != 4 {
		t.Errorf("want four %q (A::x::x1, A::y::x2, B::x::x1, B::y's), got %v", msgVariableFeatureOwner, got)
	}
	if got[msgPortionFeatureVariable] != 1 {
		t.Errorf("want one %q (B::x::x1), got %v", msgPortionFeatureVariable, got)
	}
	if got[msgConstantNotVariable] != 0 || got[msgInitialValueNotVariable] != 0 {
		t.Errorf("unexpected constant or initial messages in %v", got)
	}

	sysml := `package P {
	part def C;
	attribute def D;
	connection def K {
		end constant x1 : C [1] item x : C;
		end in derived ref x2 : C [1] item y : C;
	}
	connection def L {
		end constant x1 : D [1] attribute x : D;
		end attribute y : D;
	}
}`
	got = w8cVariableFeatureMessages(t, "<t>.sysml", sysml)
	if got[msgConstantNotVariable] != 2 {
		t.Errorf("want two %q (K::x::x1, L::x::x1), got %v", msgConstantNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgPortionFeatureVariable] != 0 || got[msgInitialValueNotVariable] != 0 {
		t.Errorf("unexpected owner, portion or initial messages in %v", got)
	}
}
