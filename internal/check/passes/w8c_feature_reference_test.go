package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w8cLibraryDiagnostics analyzes src as the named document against the standard
// library.
func w8cLibraryDiagnostics(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	return Analyze(name, root, nil, idx)
}

func w8cLibraryMessagesIn(t *testing.T, name, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		out = append(out, d.Message)
	}
	return out
}

// w8cLibraryErrorsIn returns the error messages of analysing src; a clean
// fixture must report none, or a lower tier's error masks the rule under test.
func w8cLibraryErrorsIn(t *testing.T, name, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		if d.Severity == diag.SeverityError {
			out = append(out, d.Message)
		}
	}
	return out
}

func TestW8CFeatureReferenceAccessibleAndValid(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V {
		feature n : Integer;
		struct S;
	}
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
	feature v1_S : Integer = v1.S;
	feature v1_V : Integer = V;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
		t.Errorf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
	}
	if w8cCount(msgs, msgReferentIsFeature) != 2 {
		t.Errorf("want two %q, got %v", msgReferentIsFeature, msgs)
	}
}

func TestW8CFeatureReferenceLocation(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
}`
	var spans []string
	for _, d := range w8cLibraryDiagnostics(t, "<t>.kerml", src) {
		if d.Message == msgSubsettingFeaturingTypes {
			spans = append(spans, strings.TrimSpace(src[d.Span.Offset:d.Span.End()]))
		}
	}
	if len(spans) != 1 || spans[0] != "v1::n" {
		t.Errorf("want the diagnostic at \"v1::n\", got %v", spans)
	}
}

func TestW8CFeatureReferenceEnumerationLiteralLegal(t *testing.T) {
	src := `package P {
	enum def Color { enum red; enum green; }
	attribute c : Color = Color::red;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
}

func TestW8CFeatureReferenceLegal(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1.n;
	feature m : Integer;
	feature m2 : Integer = m;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgSubsettingFeaturingTypes) != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
	if w8cCount(msgs, msgReferentIsFeature) != 0 {
		t.Errorf("unexpected %q: %v", msgReferentIsFeature, msgs)
	}
}

// F20 validateSubsettingFeaturingTypes: a body writes references the pilot
// checks for accessibility just as it checks a value expression, so a feature
// named through its owning type's namespace is not reachable there.
func TestW8CFeatureReferenceBodyInaccessible(t *testing.T) {
	cases := map[string]string{
		"constraint def body": `package P {
	part def Q { attribute n = 1; }
	constraint def C { P::Q::n > 0 }
}`,
		"asserted constraint": `package P {
	part def Q { attribute n = 1; }
	part def R { assert constraint { P::Q::n > 0 } }
}`,
		"calc return": `package P {
	part def Q { attribute n = 1; }
	calc def C { return x = P::Q::n; }
}`,
		"transition guard": `package P {
	part def Q { attribute n = 1; }
	state def S { state a; state b; transition first a if P::Q::n > 0 then b; }
}`,
		"require constraint": `package P {
	part def Q { attribute n = 1; }
	requirement def R { require constraint { P::Q::n > 0 } }
}`,
		"assume constraint": `package P {
	part def Q { attribute n = 1; }
	requirement def R { assume constraint { P::Q::n > 0 } }
}`,
		"require constraint with a parameter": `package P {
	part def Q { attribute n = 1; }
	requirement def R { require constraint c { in y = 1; P::Q::n > y } }
}`,
		"implicit calc result": `package P {
	part def Q { attribute n = 1; }
	calc def C { P::Q::n }
}`,
		"assignment value": `package P {
	part def Q { attribute n = 1; }
	action def A { attribute v = 0; assign v := P::Q::n; }
}`,
		"nested calc def reads the enclosing feature": `package P {
	part def Q { attribute n = 1; calc def E { n + 1 } }
}`,
		"nested constraint def reads the enclosing feature": `package P {
	part def Q { attribute n = 1; constraint def K { n > 0 } }
}`,
		"nested state def guard reads the enclosing feature": `package P {
	part def Q { attribute n = 1; state def S { state a; state b; transition first a if n > 0 then b; } }
}`,
		"nested action def assigns the enclosing feature": `package P {
	private import ScalarValues::*;
	part def Q { attribute n : Integer = 1; action def A { attribute v : Integer; assign v := n; } }
}`,
		"nested action def branches on the enclosing feature": `package P {
	private import ScalarValues::*;
	part def Q { attribute n : Integer = 1; action def A { if n > 0 { action x; } } }
}`,
		"nested calc def chains from the enclosing feature": `package P {
	part def Q { attribute n = 1; }
	part def R { part q : Q; calc def E { q.n + 1 } }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
			if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
				t.Errorf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
			}
		})
	}
}

// The traps the reverted attempt fell into: a body reaches its own type's
// features, inherited ones included, and a dot path reaches a nested one.
func TestW8CFeatureReferenceBodyAccessible(t *testing.T) {
	cases := map[string]string{
		"own feature": `package P {
	part def Q { attribute n = 1; constraint c { n > 0 } }
}`,
		"inherited feature": `package P {
	part def Base { attribute n = 1; }
	part def D :> Base { constraint c { n > 0 } }
}`,
		"redefined feature": `package P {
	part def Base { attribute n = 1; }
	part def D :> Base { attribute n redefines n; constraint c { n > 0 } }
}`,
		"dotted subject": `package P {
	part def Widget { attribute mass = 1; }
	requirement def R { subject s : Widget; require constraint { s.mass > 0 } }
}`,
		"dotted part path": `package P {
	part def Q { attribute n = 1; }
	part def R { part q : Q; constraint c { q.n > 0 } }
}`,
		"nested constraint parameter": `package P {
	part def W;
	requirement def R {
		subject s : W;
		in x = 1;
		require constraint q { in y = 2; y > 0 and x > 0 }
		assume constraint { in z = 3; z > x }
	}
}`,
		"typed nested constraint parameter": `package P {
	part def W;
	constraint def Q { in v default = 1; }
	requirement def R { subject s : W; require constraint w : Q { in v = 2; v > 0 } }
}`,
		"implicit calc result": `package P {
	calc def C { attribute m = 1; m }
}`,
		"trigger parameter in a guard": `package P {
	attribute def Temp;
	state def S {
		state a;
		state b;
		transition first a accept sig : Temp if sig > 0 then b;
	}
}`,
		"state entry local": `package P {
	state def S {
		state a {
			entry action e { attribute v = 1; assign v := v + 1; }
		}
	}
}`,
		"accept payload from a sibling node": `package P {
	attribute def Temp;
	action def A {
		action receiver accept msg : Temp;
		action processor { if msg > 0 { } }
	}
}`,
		"connector ends": `package P {
	part def Port;
	part def A { part x : Port; part y : Port; connect x to y; }
}`,
		"nested usages read the enclosing feature": `package P {
	part def Q {
		attribute n = 1;
		calc e { n + 1 }
		constraint k { n > 0 }
		state s { state a; state b; transition first a if n > 0 then b; }
	}
}`,
		"nested action usage reads the enclosing feature": `package P {
	private import ScalarValues::*;
	part def Q {
		attribute n : Integer = 1;
		action a { attribute v : Integer; assign v := n; if n > 0 { action x; } }
	}
}`,
		"nested definitions read their own features": `package P {
	part def Q {
		attribute n = 1;
		calc def E { attribute m = 2; m + 1 }
		constraint def K { attribute m = 2; m > 0 }
		state def S { attribute m = 2; state a; state b; transition first a if m > 0 then b; }
	}
}`,
		"nested definitions read inherited features": `package P {
	calc def Base { attribute m = 2; }
	constraint def CBase { attribute m = 2; }
	state def SBase { attribute m = 2; }
	part def Q {
		attribute n = 1;
		calc def E :> Base { m + 1 }
		constraint def K :> CBase { m > 0 }
		state def S :> SBase { state a; state b; transition first a if m > 0 then b; }
	}
}`,
		"nested definition reads a redefined feature": `package P {
	calc def Base { attribute m default = 2; }
	part def Q { attribute n = 1; calc def E :> Base { attribute :>> m = 3; m + 1 } }
}`,
		"nested definition specializing the enclosing one": `package P {
	calc def Q { attribute n = 1; calc def E :> Q { n + 1 } }
}`,
		"nested definition reads a package-level feature": `package P {
	attribute g = 5;
	part def Q { attribute n = 1; calc def E { g + 1 } }
}`,
		"package-level definition chains through its own part": `package P {
	part def Q { attribute n = 1; }
	part def R { part q : Q; }
	calc def F { part r : R; r.q.n + 1 }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errs := w8cLibraryErrorsIn(t, "<t>.sysml", src); len(errs) != 0 {
				t.Errorf("want a clean analysis, got %v", errs)
			}
		})
	}
}

func TestW8CFeatureReferenceFilterConditions(t *testing.T) {
	t.Run("user feature path is inaccessible", func(t *testing.T) {
		src := `package R1 {
	private import ScalarValues::*;
	metaclass M { var feature a : Boolean[1]; }
}
package Q { filter R1::M::a; }`
		msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
		if w8cCount(msgs, msgSubsettingFeaturingTypes) != 1 {
			t.Fatalf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
		}
	})

	for _, tc := range []struct {
		name string
		src  string
	}{
		{
			"library metaclass path",
			`package Q {
	private import KerML::*;
	filter Element::name == "System" and not Type::isAbstract;
}`,
		},
		{
			"dot notation on a cast",
			`package Q {
	private import ScalarValues::*;
	metaclass X { var feature f : ScalarValues::Boolean[1]; }
	filter (as X).f;
}`,
		},
		{
			"metadata classification",
			`metadata def Safety;
package Q { filter @Safety; }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if msgs := w8cLibraryMessagesIn(t, "<t>.kerml", tc.src); len(msgs) != 0 {
				t.Fatalf("expected no diagnostics, got %v", msgs)
			}
		})
	}
}

// A behavior definition is the occurrence its `this` denotes: `this.<feature>`
// inside a def reads the def's own features, so an enclosing object's feature
// does not resolve through it — while a usage's bare feature read still reaches
// the object lexically.
func TestW8CFeatureReferenceThisInsideBehaviorDef(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	part def H {
		attribute level : Integer = 0;
		action def Nudge {
			action step { assign this.level := 1; }
		}
	}
}`
	msgs := w8cLibraryErrorsIn(t, "<t>.sysml", src)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "level") {
			found = true
		}
	}
	if !found || len(msgs) == 0 {
		t.Errorf("this.level inside action def Nudge should not resolve to H's level, got %v", msgs)
	}
}

// `context.<feature>` inside a def and a bare feature read inside a usage both
// analyse clean: the def reads its context parameter, the usage sees the object.
func TestW8CFeatureReferenceContextParameterAndUsageReads(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	part def H {
		attribute level : Integer = 0;
		action def Nudge {
			in ref context : H;
			action step { assign context.level := 1; }
		}
		action nudge : Nudge { in ref :>> context = this; }
		state s {
			entry; then on;
			state on { entry action e { assign level := level + 1; } }
		}
	}
}`
	if msgs := w8cLibraryErrorsIn(t, "<t>.sysml", src); len(msgs) != 0 {
		t.Errorf("context reads and usage-level bare reads must be clean, got %v", msgs)
	}
}

func TestW8CInvocationArgumentValuesAreAccessible(t *testing.T) {
	const inaccessible = `package M {
	private import ScalarValues::*;
	part def T { attribute X : String; }
	calc def F { in e : ScalarValue[0..1]; return r : Boolean; }
	calc def Q { F(e = T::X ?? "") }
	calc def Q2 { F(e = T::X) }
	}`
	var got []string
	var gotOffsets []int
	for _, d := range w8cLibraryDiagnostics(t, "<t>.sysml", inaccessible) {
		if d.Code == "feature-reference-featuring-types" {
			got = append(got, strings.TrimSpace(inaccessible[d.Span.Offset:d.Span.End()]))
			gotOffsets = append(gotOffsets, d.Span.Offset)
		}
	}
	if len(got) != 2 || got[0] != "T::X" || got[1] != "T::X" {
		t.Errorf("want both invocation argument references to be checked, got %v", got)
	}
	wantOffsets := []int{strings.Index(inaccessible, "T::X ??"), strings.LastIndex(inaccessible, "T::X")}
	if len(gotOffsets) != len(wantOffsets) {
		t.Fatalf("want invocation argument spans at %v, got %v", wantOffsets, gotOffsets)
	}
	for i := range wantOffsets {
		if gotOffsets[i] != wantOffsets[i] {
			t.Errorf("invocation argument span %d = %d, want %d", i, gotOffsets[i], wantOffsets[i])
		}
	}

	const accessible = `package M {
	private import ScalarValues::*;
	part def T { attribute X : String; }
	part t : T;
	calc def F { in e : ScalarValue[0..1]; return r : Boolean; }
	calc def Q { F(e = t.X ?? "") }
}`
	if errs := w8cLibraryErrorsIn(t, "<t>.sysml", accessible); len(errs) != 0 {
		t.Errorf("accessible invocation argument should be clean, got %v", errs)
	}
}

func TestW8CInvocationFunctionValuesAndColumnExpressions(t *testing.T) {
	const src = `package M {
	private import ScalarValues::*;
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	package Meta {
		private import ScalarValues::*;
		metadata def Tagged { attribute tags : String[0..*] ordered = ("alpha", "beta"); }
	}
	calc def Sq { in a : Real; return : Real = a * a; }
	calc def Apply {
		in calc f { in v : Real; return : Real; }
		in a : Real;
		return : Real = f(a);
	}
	calc def UseFunction { in a : Real; return : Real = Apply(Sq, a); }
	calc def BodyClosure {
		in k : Real;
		calc scale { in v : Real; return : Real = v * k; }
	}
	calc def OuterClosure { in a : Real; return : Real = Apply(BodyClosure::scale, a); }
	part def Scaler {
		attribute k : Real;
		calc scale { in v : Real; return : Real = v * k; }
	}
	calc def ObjectCalc { in a : Real; return : Real = Apply(Scaler::scale, a); }
	calc def Tags :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
			properties = ("name"),
			columns = (Column(name = "Tags", expression = Meta::Tagged::tags ?? "")))
	}
}`
	if errs := w8cLibraryErrorsIn(t, "<t>.sysml", src); len(errs) != 0 {
		t.Errorf("function-valued arguments and query-scope column expressions should be clean, got %v", errs)
	}
}

func TestW8CAssignmentTargetChecksOnlyAnInaccessibleChainHead(t *testing.T) {
	const src = `package M {
	private import ScalarValues::*;
	part def P {
		attribute k : Integer;
		exhibit state s { attribute j : Integer; }
		action u {
			action def D {
				action a { assign k := k + 1; }
				action b { assign s.j := 2; }
				action c { assign k := s.j; }
			}
		}
	}
	}`
	var got []string
	var gotOffsets []int
	for _, d := range w8cLibraryDiagnostics(t, "<t>.sysml", src) {
		if d.Code == "feature-reference-featuring-types" {
			got = append(got, strings.TrimSpace(src[d.Span.Offset:d.Span.End()]))
			gotOffsets = append(gotOffsets, d.Span.Offset)
		}
	}
	if len(got) != 3 || got[0] != "k" || got[1] != "s" || got[2] != "s" {
		t.Errorf("want one RHS k and the two inaccessible s chain heads, got %v", got)
	}
	wantOffsets := []int{
		strings.Index(src, "assign k := k + 1;") + strings.LastIndex("assign k := k + 1;", "k +"),
		strings.Index(src, "assign s.j := 2;") + len("assign "),
		strings.Index(src, "assign k := s.j;") + len("assign k := "),
	}
	if len(gotOffsets) != len(wantOffsets) {
		t.Fatalf("want assignment references at %v, got %v", wantOffsets, gotOffsets)
	}
	for i := range wantOffsets {
		if gotOffsets[i] != wantOffsets[i] {
			t.Errorf("assignment reference span %d = %d, want %d", i, gotOffsets[i], wantOffsets[i])
		}
	}

	const accessible = `package M {
	private import ScalarValues::*;
	part def P {
		var attribute k : Integer;
		action u { assign k := 1; }
	}
}`
	if errs := w8cLibraryErrorsIn(t, "<t>.sysml", accessible); len(errs) != 0 {
		t.Errorf("bare assignment target should remain clean, got %v", errs)
	}
}

func TestW8CNestedTransitionEndpointsUseTheirStateBody(t *testing.T) {
	const src = `package P {
	attribute def Sig;
	part def Q {
		state def S {
			entry; then A;
			state A;
			state B {
				entry; then C;
				state C;
				transition t1 first C then A;
				transition t2 first C accept Sig then A;
				transition t3 first C accept Sig do action e { } then A;
			}
			transition t4 first B.C then A;
			transition t5 first B.C accept Sig then A;
			transition t6 first B accept Sig then A;
			state D {
				entry; then B::C;
			}
		}
		state def R {
			state W;
			state X { state Y; }
			transition t7 first X.Y then W;
			transition t8 first W then X::Y;
			transition t9 first W then X.Y;
		}
	}
	}`
	var got []string
	var gotOffsets []int
	for _, d := range w8cLibraryDiagnostics(t, "<t>.sysml", src) {
		if d.Code == "feature-reference-featuring-types" {
			got = append(got, strings.TrimSpace(src[d.Span.Offset:d.Span.End()]))
			gotOffsets = append(gotOffsets, d.Span.Offset)
		}
	}
	want := []string{"A", "A", "A", "B::C", "X::Y"}
	if len(got) != len(want) {
		t.Fatalf("want transition endpoint references %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("transition endpoint %d = %q, want %q", i, got[i], want[i])
		}
	}
	wantOffsets := []int{
		strings.Index(src, "transition t1 first C then A;") + strings.LastIndex("transition t1 first C then A;", "then A") + len("then "),
		strings.Index(src, "transition t2 first C accept Sig then A;") + strings.LastIndex("transition t2 first C accept Sig then A;", "then A") + len("then "),
		strings.Index(src, "transition t3 first C accept Sig do action e { } then A;") + strings.LastIndex("transition t3 first C accept Sig do action e { } then A;", "then A") + len("then "),
		strings.Index(src, "entry; then B::C;") + strings.Index("entry; then B::C;", "then ") + len("then "),
		strings.Index(src, "transition t8 first W then X::Y;") + strings.Index("transition t8 first W then X::Y;", "X::Y"),
	}
	if len(gotOffsets) != len(wantOffsets) {
		t.Fatalf("want transition endpoints at %v, got %v", wantOffsets, gotOffsets)
	}
	for i := range wantOffsets {
		if gotOffsets[i] != wantOffsets[i] {
			t.Errorf("transition endpoint span %d = %d, want %d", i, gotOffsets[i], wantOffsets[i])
		}
	}

	const accepted = `package N {
	state def Modes { state off; }
	state def Behavior {
		state modes : Modes;
		transition reset first modes.off then Modes::off;
		transition rootReset first modes.off then $::N::Modes::off;
	}
	state def S {
		state A1;
		state B1 { state C1; }
		transition first B1.C1 then A1;
	}
}`
	if errs := w8cLibraryErrorsIn(t, "<t>.sysml", accepted); len(errs) != 0 {
		t.Errorf("a dotted endpoint path written at the common state should be clean, got %v", errs)
	}

	const nestedEntry = `package N {
	state def S {
		state A1;
		state B1 {
			state C1;
			accept after 1 [SI::s] then A1;
		}
	}
	}`
	var nested []string
	var nestedOffsets []int
	for _, d := range w8cLibraryDiagnostics(t, "<t>.sysml", nestedEntry) {
		if d.Code == "feature-reference-featuring-types" {
			nested = append(nested, strings.TrimSpace(nestedEntry[d.Span.Offset:d.Span.End()]))
			nestedOffsets = append(nestedOffsets, d.Span.Offset)
		}
	}
	if len(nested) != 1 || nested[0] != "A1" {
		t.Errorf("want the nested entry transition's enclosing-state target to be rejected, got %v", nested)
	}
	wantNestedOffset := strings.LastIndex(nestedEntry, "then A1;") + len("then ")
	if len(nestedOffsets) != 1 || nestedOffsets[0] != wantNestedOffset {
		t.Errorf("nested entry target span = %v, want %d", nestedOffsets, wantNestedOffset)
	}
}

// its body values judged like one written in a definition's body.
func TestW8CFeatureReferenceRootAnnotationBody(t *testing.T) {
	const invalid = `part def Acme;
	metadata def Org { ref owner; }
	@Org { owner = Acme; }`
	msgs := w8cLibraryMessagesIn(t, "<t>.sysml", invalid)
	if w8cCount(msgs, msgReferentIsFeature) != 1 {
		t.Errorf("want one %q, got %v", msgReferentIsFeature, msgs)
	}
	const clean = `part def Acme;
	part acme : Acme;
	metadata def Org { ref owner; ref kind; }
	@Org { owner = acme; kind = Acme meta SysML::PartDefinition; }`
	if errs := w8cLibraryErrorsIn(t, "<t>.sysml", clean); len(errs) != 0 {
		t.Errorf("want a clean analysis, got %v", errs)
	}
}
func TestW8CFeatureReferenceViaBoundaries(t *testing.T) {
	const want = msgSubsettingFeaturingTypes
	reject := map[string]string{
		"nested transition": `package P {
			item def E;
			part def Owner {
				port pin;
				state def S {
					state a;
					state b;
					transition first a accept E via pin then b;
				}
			}
		}`,
		"nested action definition": `package P {
			item def E;
			part def Owner {
				port pin;
				action def A {
					action r accept e : E via pin;
				}
			}
		}`,
		"nested deferred buffer": `package P {
			item def E;
			part def Owner {
				port pin;
				state def S {
					state s {
						do action buffer {
							action r accept e : E via pin;
						}
					}
				}
			}
		}`,
		"send via in nested definition": `package P {
			item def E;
			part def Owner {
				port pin;
				action def A {
					send new E() via pin;
				}
			}
		}`,
		"send via with body in nested definition": `package P {
			item def E;
			part def Owner {
				port pin;
				action def A {
					send new E() via pin { }
				}
			}
		}`,
	}
	for name, src := range reject {
		t.Run("reject "+name, func(t *testing.T) {
			errs := w8cLibraryErrorsIn(t, "<t>.sysml", src)
			if len(errs) != 1 || errs[0] != want {
				t.Fatalf("want exactly one %q, got %v", want, errs)
			}
		})
	}

	accept := map[string]string{
		"transition through context": `package P {
			item def E;
			part def Owner {
				port pin;
				state def S {
					in ref context : Owner;
					state a;
					state b;
					transition first a accept E via context.pin then b;
				}
			}
		}`,
		"action through context": `package P {
			item def E;
			part def Owner {
				port pin;
				action def A {
					in ref context : Owner;
					action r accept e : E via context.pin;
				}
			}
		}`,
		"buffer through context": `package P {
			item def E;
			part def Owner {
				port pin;
				state def S {
					in ref context : Owner;
					state s {
						do action buffer {
							action r accept e : E via context.pin;
						}
					}
				}
			}
		}`,
		"own parameter": `package P {
			item def E;
			port def Port;
			action def A {
				in ref q : Port;
				action r accept e : E via q;
			}
		}`,
		"usage-owned parameter": `package P {
			item def E;
			port def Port;
			part def Owner {
				action def A {
					action r accept e : E via q {
						in ref q : Port;
					}
				}
			}
		}`,
		"this context chain": `package P {
			item def E;
			part def Owner {
				port x;
				action def A {
					in ref context : Owner;
					action r accept e : E via this.context.x;
				}
			}
		}`,
		"bare via in a usage body": `package P {
			item def E;
			part def Owner {
				port pin;
				state def S;
				action def A;
			}
			part owner : Owner {
				exhibit state s : S {
					action r accept e : E via pin;
				}
				perform action a : A {
					action r2 accept e2 : E via pin;
				}
			}
		}`,
	}
	for name, src := range accept {
		t.Run("accept "+name, func(t *testing.T) {
			if errs := w8cLibraryErrorsIn(t, "<t>.sysml", src); len(errs) != 0 {
				t.Fatalf("want a clean fixture, got %v", errs)
			}
		})
	}
}

// Stochastic::Probability::p is read when its succession's decision is reached,
// so its value names what the succession's own guards may: the features in reach
// of the action holding the succession, in both spellings, named or not. Any
// other metadata value stays judged from the metadata type's feature.
func TestW8CFeatureReferenceProbabilityReadsFromTheSuccession(t *testing.T) {
	const want = msgSubsettingFeaturingTypes
	clean := map[string]string{
		"own attribute": `package P {
	private import ScalarValues::*;
	action def Route {
		attribute w : Real default = 0.5;
		first start;
		then decide d;
		succession fast first d then done { @Stochastic::Probability { p = w; } }
		succession slow first d then other { metadata Stochastic::Probability { p = 1.0 - w; } }
		action done; action other;
	}
}`,
		"context parameter's attribute": `package P {
	private import ScalarValues::*;
	part def Mission { attribute pr : Real default = 0.25; }
	action def Route {
		in ref context : Mission;
		first start;
		then decide d;
		succession fast first d then done { @Stochastic::Probability { p = context.pr; } }
		first d then other { @Stochastic::Probability { p = 1.0 - context.pr; } }
		action done; action other;
	}
}`,
		"nested definition's context": `package P {
	private import ScalarValues::*;
	part def Mission {
		attribute pr : Real default = 0.25;
		part def Sub :> Mission {
			perform action run {
				action def Inner {
					in ref context : Sub;
					first start;
					then decide d;
					succession fast first d then done { @Stochastic::Probability { p = context.pr; } }
					action done;
				}
				action call : Inner { in ref :>> context = this; }
			}
		}
	}
}`,
		"usage body": `package P {
	private import ScalarValues::*;
	part def Analysis {
		attribute pFast : Real default = 0.5;
		action route {
			first start;
			then decide d;
			succession fast first d then done { @Stochastic::Probability { p = pFast; } }
			action done;
		}
	}
}`,
	}
	for name, src := range clean {
		if errs := w8cLibraryErrorsIn(t, name+".sysml", src); len(errs) != 0 {
			t.Errorf("%s: want a clean analysis, got %v", name, errs)
		}
	}
	reject := map[string]string{
		"named succession": `package P {
	private import ScalarValues::*;
	part def Other { attribute q : Real default = 0.5; }
	action def Route {
		first start;
		then decide d;
		succession fast first d then done { @Stochastic::Probability { p = P::Other::q; } }
		action done;
	}
}`,
		"anonymous succession": `package P {
	private import ScalarValues::*;
	part def Other { attribute q : Real default = 0.5; }
	action def Route {
		first start;
		then decide d;
		first d then done { @Stochastic::Probability { p = P::Other::q; } }
		action done;
	}
}`,
		"metadata usage spelling": `package P {
	private import ScalarValues::*;
	part def Other { attribute q : Real default = 0.5; }
	action def Route {
		first start;
		then decide d;
		succession fast first d then done { metadata Stochastic::Probability { p = P::Other::q; } }
		action done;
	}
}`,
		"unqualified nested definition's read": `package P {
	private import ScalarValues::*;
	part def Mission {
		attribute pr : Real default = 0.25;
		part def Sub :> Mission {
			action def Inner {
				in ref context : Sub;
				first start;
				then decide d;
				succession fast first d then done { @Stochastic::Probability { p = pr; } }
				action done;
			}
		}
	}
}`,
	}
	for name, src := range reject {
		msgs := w8cLibraryMessagesIn(t, name+".sysml", src)
		if w8cCount(msgs, want) != 1 {
			t.Errorf("%s: want one %q, got %v", name, want, msgs)
		}
	}
}
