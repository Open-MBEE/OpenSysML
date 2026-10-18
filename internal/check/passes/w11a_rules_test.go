package passes

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w11aMessages are the messages of src's diagnostics, sorted.
func w11aMessages(t *testing.T, src string, lib bool) []string {
	t.Helper()
	var diags []diag.Diagnostic
	if lib {
		diags = w9cLibraryDiags(t, src, false)
	} else {
		name := "<t>.kerml"
		root := parser.New(source.New(name, []byte(src))).ParseFile()
		idx := symbols.NewIndex()
		idx.AddDocument(name, root)
		idx.ExpandWildcardImports()
		diags = Analyze(name, root, nil, idx)
	}
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Message)
	}
	sort.Strings(out)
	return out
}

// w11aKerMLLibraryMessages are the messages of src analyzed as KerML against
// the standard library, which the inherited-name rule needs to see the library
// diamond.
func w11aKerMLLibraryMessages(t *testing.T, src string) []string {
	t.Helper()
	name := "<t>.kerml"
	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	out := []string{}
	for _, d := range Analyze(name, root, nil, idx) {
		out = append(out, d.Message)
	}
	sort.Strings(out)
	return out
}

func w11aWantMessages(t *testing.T, src string, lib bool, want ...string) {
	t.Helper()
	got := w11aMessages(t, src, lib)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// An attribute is typed by data types, so a structural definition is an error
// (AttributeUsage_invalid.sysml.xt:44).
func TestW11AAttributeTypedByStructure(t *testing.T) {
	src := `package Test {
	part def A;
	attribute def V;
	part def P {
		attribute a : A;
		attribute v : V;
		in attribute p : A;
	}
}`
	w11aWantMessages(t, src, true, "An attribute must be typed by attribute definitions.")
}

// An action, a port and a state are typed by definitions of their own kind,
// including where the typing is inherited through a reference subsetting
// (ActionUsage_invalid.sysml.xt:61, StateUsage_invalid.sysml.xt:87).
func TestW11AUsageTypedByWrongKind(t *testing.T) {
	cases := []struct{ src, want string }{
		{`package Test {
	part def ABlock;
	action def B { action a : ABlock; }
	ref b : B;
	action def C { perform b.a; }
}`, "An action must be typed by action definitions."},
		{`package Test {
	part def ABlock;
	part def B { port p : ABlock; }
}`, "A port must be typed by port definitions."},
		{`package Test {
	part def ABlock;
	part def B { state s : ABlock; }
}`, "A state must be typed by state definitions."},
	}
	for _, tc := range cases {
		got := w11aMessages(t, tc.src, true)
		found := false
		for _, msg := range got {
			if msg == tc.want {
				found = true
			}
		}
		if !found {
			t.Errorf("got %v, want %q", got, tc.want)
		}
	}
}

// The whole typing-kind family on a model whose types resolve: the pinned
// validate-sysml reports these eight wordings at the same columns.
func TestW11AUsageTypingKindFamily(t *testing.T) {
	src := `package Test {
	part def P;
	attribute def A;
	attribute bad : P;
	part badPart : A;
	item badItem : A;
	port badPort : A;
	action badAct : A;
	state badState : A;
	connection badConn : A;
	interface badIf : A;
}`
	var got []string
	for _, msg := range w11aMessages(t, src, true) {
		if strings.HasSuffix(msg, "definitions.") {
			got = append(got, msg)
		}
	}
	want := []string{
		"A connection must be typed by connection definitions.",
		"A port must be typed by port definitions.",
		"A state must be typed by state definitions.",
		"An action must be typed by action definitions.",
		"An attribute must be typed by attribute definitions.",
		"An interface must be typed by interface definitions.",
		"An occurrence, item or part must be typed by occurrence definitions.",
		"An occurrence, item or part must be typed by occurrence definitions.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A performed feature must be an action: a reference usage is not one, whatever
// types it (ActionUsage_invalid.sysml.xt:57). `exhibit` takes the same check;
// see docs/project/spec-compliance.md for the form it does not reach.
func TestW11AReferenceKinds(t *testing.T) {
	src := `package Test {
	part def B;
	action def AD;
	action def C {
		ref b : B;
		action a : AD;
		perform b;
		perform a;
	}
}`
	var got []string
	for _, msg := range w11aMessages(t, src, true) {
		if msg == msgReferenceAction || msg == msgReferenceState {
			got = append(got, msg)
		}
	}
	if strings.Join(got, "\n") != msgReferenceAction {
		t.Errorf("got %v, want [%q]", got, msgReferenceAction)
	}
}

// The declared forms `perform action f references p;` and `exhibit state e
// references p;` are judged like `perform p;` and `exhibit p;`: a part is
// neither, an action is no state, while a state is an action
// (validatePerformActionUsageReference, validateExhibitStateUsageReference).
func TestW11AReferenceKindsDeclaredForm(t *testing.T) {
	src := `package Test {
	part def B;
	state def SD;
	action def AD;
	part def C {
		part b : B;
		state s : SD;
		action a : AD;
		perform action f1 references b;
		exhibit state e1 references b;
		exhibit state e2 references a;
		perform action f2 references s;
		perform action f3 references a;
		exhibit state e3 references s;
	}
}`
	var got []string
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Code != "usage-reference-kind" {
			continue
		}
		line, col := lineCol(src, d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d %s", line, col, d.Message))
	}
	sort.Strings(got)
	want := []string{
		"10:31 " + msgReferenceState,
		"11:31 " + msgReferenceState,
		"9:32 " + msgReferenceAction,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A data type specializes neither a class nor an association, and a class
// neither a data type nor an association; a structure and a behavior do not
// specialize each other (Specialization_invalid.kerml.xt:35).
func TestW11AKerMLSpecializationFamilies(t *testing.T) {
	src := `package Test {
	datatype D1;
	class C1;
	abstract assoc A1;
	datatype D2 specializes D1, C1, A1;
	class C2 specializes C1, D1, A1;
	struct S1;
	behavior B1;
	struct S2 specializes B1;
	behavior B2 specializes S1;
	assoc struct AS specializes A1;
	interaction I specializes A1;
}`
	w11aWantMessages(t, src, false,
		msgW11ASpecializeClassOrAssoc, msgW11ASpecializeClassOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeBehavior, msgW11ASpecializeStructure)
}

// A specialization cycle does not remove the implicit base of either end, so
// both still inherit `self` from two library types
// (Specialization_invalid.kerml.xt:56).
func TestW11ASpecializationCycleKeepsImplicitBase(t *testing.T) {
	src := `package Test {
	struct S specializes B1;
	behavior B1 specializes S;
}`
	got := w11aKerMLLibraryMessages(t, src)
	want := msgW9CDuplicateInherited + " 'self' from Object, Performance"
	seen := 0
	for _, msg := range got {
		if msg == want {
			seen++
		}
	}
	if seen != 2 {
		t.Errorf("got %v, want %q twice", got, want)
	}
}

// A feature reached through two supertypes is one inherited member, not a
// duplicate, when a redefinition on one path replaces it: the diamond in
// examples/pilot-corpora sysml-examples/Vehicle Example/Annex_A_VehicleViews.sysml.
func TestW11ADiamondRedefinitionIsNotDuplicate(t *testing.T) {
	src := `package Test {
	part def Vehicle {
		attribute mass;
		port vehicleToRoadPort;
	}
	part vehicle_b : Vehicle {
		attribute :>> mass;
		port :>> vehicleToRoadPort;
	}
	part vehicle_c : Vehicle :> vehicle_b;
}`
	if got := w9cMessages(w9cLibraryDiags(t, src, false), msgW9CDuplicateInherited); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// An attribute typed by a feature has no classifier of its own, so its implicit
// value typing stands and draws a diamond; one typed by a definition, or
// redefining another feature, does not (AttributeUsage_invalid.sysml.xt:47).
func TestW11AImplicitValueTypingDiamond(t *testing.T) {
	src := `package Test {
	part def A { part a : A; }
	part def Vehicle {
		attribute viaFeature : A::a;
		attribute viaDef : A;
	}
}`
	want := []string{msgW9CDuplicateInherited + " 'self' from DataValue, Part"}
	got := w9cMessages(w9cLibraryDiags(t, src, false), msgW9CDuplicateInherited)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A usage's types are judged after dropping those its kind's base already
// specializes (pilot removeRedundantTypes): Base::Anything types an attribute
// whether declared, subsetted, redefined or inherited over several steps.
func TestW11AAttributeInheritsKerMLTyping(t *testing.T) {
	src := `package Test {
	attribute a : Base::Anything;
	attribute b :> a;
	attribute c :> b;
	attribute e : Base::DataValue;
	attribute f :> e;
	part def P { attribute g : Base::Anything; }
	part def Q :> P { attribute h :>> g; }
}`
	w11aWantMessages(t, src, true)
}

// A KerML type the base does not specialize is judged by its metaclass: a
// struct is no DataType, so it fails an attribute declared with it or
// inheriting it, as a part definition does.
func TestW11AAttributeTypedByKerMLStructure(t *testing.T) {
	src := `package Test {
	attribute a : Objects::Object;
	attribute b :> a;
	part def P;
	attribute c : P;
	attribute d :> c;
	attribute e : Occurrences::Occurrence;
}`
	msg := "An attribute must be typed by attribute definitions."
	w11aWantMessages(t, src, true, msg, msg, msg, msg, msg)
}

// The same rule holds for every usage kind: a KerML type above the kind's base
// is redundant, one of the kind's metaclass (a behavior for an action) types it,
// and any other is the kind's mismatch, declared or inherited.
func TestW11AUsageKindsInheritKerMLTyping(t *testing.T) {
	src := `package Test {
	part def D { out item o; in item i; }
	part x : D;
	part y : D;
	part p1 : Base::Anything;
	part p2 :> p1;
	item i1 : Objects::Object;
	item i2 :> i1;
	occurrence o1 : Occurrences::Occurrence;
	occurrence o2 :> o1;
	port q1 : Base::Anything;
	port q2 :> q1;
	action a1 : Performances::Performance;
	action a2 :> a1;
	action a3 : Occurrences::Occurrence;
	action a4 :> a3;
	state s1 : Performances::Performance;
	state s2 :> s1;
	connection k1 : Links::BinaryLink connect x to y;
	connection k2 :> k1 connect x to y;
	flow f1 : Transfers::Transfer from x.o to y.i;
	flow f2 :> f1 from x.o to y.i;
	calc c1 : Performances::Evaluation;
	calc c2 :> c1;
	constraint n1 : Performances::BooleanEvaluation;
	constraint n2 :> n1;
	requirement r1 : Base::Anything;
	requirement r2 :> r1;
	case u1 : Performances::Performance;
	case u2 :> u1;
}`
	w11aWantMessages(t, src, true)
}

func TestW11AUsageKindsRejectForeignKerMLTyping(t *testing.T) {
	src := `package Test {
	item i1 : Base::DataValue;
	action a1 : Links::Link;
	action a2 :> a1;
	port q1 : Base::DataValue;
	port q2 :> q1;
	calc c1 : Links::Link;
	calc c2 :> c1;
}`
	w11aWantMessages(t, src, true,
		"An occurrence, item or part must be typed by occurrence definitions.",
		"An action must be typed by action definitions.",
		"An action must be typed by action definitions.",
		"A port must be typed by port definitions.",
		"A port must be typed by port definitions.",
		"A calculation must be typed by one calculation definition.",
		"A calculation must be typed by one calculation definition.")
}
