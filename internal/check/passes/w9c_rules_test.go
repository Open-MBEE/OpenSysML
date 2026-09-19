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
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// w9cLibraryDiags analyzes src as SysML against the standard library, which the
// inherited-name rule needs to see the Action/Part diamond.
func w9cLibraryDiags(t *testing.T, src string, warm bool) []diag.Diagnostic {
	t.Helper()
	// The loader below is what populates the library here, so this starts empty:
	// loading it twice would re-add every library document.
	idx := symbols.NewIndex()
	libSrc := libs.DefaultSource()
	var cache *libs.Cache
	if warm {
		// Populate a cache of this test's own, so the library really is restored
		// from records rather than parsed.
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		c, err := libs.NewCache()
		if err != nil {
			t.Fatalf("cache: %v", err)
		}
		cache = c
		warmIdx := symbols.NewIndex()
		if err := libs.NewLoader(libSrc, cache).LoadAll(warmIdx); err != nil {
			t.Fatalf("warm library: %v", err)
		}
	}
	ld := libs.NewLoader(libSrc, cache)
	if err := ld.LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	if warm && ld.Hits() == 0 {
		// Facts derived again would make the warm run a second cold one, which
		// proves nothing about what a cache hit restores.
		t.Fatal("warm run derived the library facts instead of restoring them")
	}
	// Either path parses the library, so the diamond is read from declarations.
	for _, sym := range idx.LookupQualified("Actions::Action") {
		if sym.Decl == nil {
			t.Fatalf("warm=%v: Actions::Action carries no declaration", warm)
		}
	}
	root := parser.New(source.New("<t>.sysml", []byte(src))).ParseFile()
	idx.AddDocument("<t>.sysml", root)
	idx.ExpandWildcardImports()
	return Analyze("<t>.sysml", root, nil, idx)
}

func w9cMessages(diags []diag.Diagnostic, prefix string) []string {
	var out []string
	for _, d := range diags {
		if strings.HasPrefix(d.Message, prefix) {
			out = append(out, d.Message)
		}
	}
	return out
}

// An action typed by a part definition inherits self/start/done from both
// Action and Part (ActionUsage_invalid.sysml.xt:40), as a warning.
func TestW9CActionPartDiamondWarns(t *testing.T) {
	src := `package Test {
	part def ABlock;
	action def AnAction {
		action a : ABlock;
	}
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{
			msgW9CDuplicateInherited + " 'done' from Action, Part",
			msgW9CDuplicateInherited + " 'self' from Action, Part",
			msgW9CDuplicateInherited + " 'start' from Action, Part",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// A usage typed by a definition of its own kind inherits each name once.
func TestW9CConformingTypingStaysSilent(t *testing.T) {
	src := `package Test {
	action def AnAction;
	part def P {
		action a : AnAction;
	}
}`
	for _, warm := range []bool{false, true} {
		if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want none", warm, got)
		}
	}
}

func TestW9CBinaryInterfaceEndDiamondWarns(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "reference subsetting",
			src: `package Test {
	part def C {
		part p;
		interface i {
			end part ::> p;
			end port q1;
		}
	}
}`,
		},
		{
			name: "declared end",
			src: `package Test {
	part def C {
		interface i {
			end part p1;
			end port q1;
		}
	}
}`,
		},
		{
			name: "interface definition",
			src: `package Test {
	interface def I {
		end part p1;
		end port q1;
	}
}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, warm := range []bool{false, true} {
				got := w9cMessages(w9cLibraryDiags(t, tc.src, warm), msgW9CDuplicateInherited)
				want := []string{msgW9CDuplicateInherited + " 'self' from Part, Port"}
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("warm=%v: got %v, want %v", warm, got, want)
				}
			}
		})
	}
}

func TestW9CNonBinaryConnectorEndsStaySilent(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "three-ended interface",
			body: `interface i {
				end part p1;
				end port q1;
				end port q2;
			}`,
		},
		{
			name: "binary connection",
			body: `connection c {
				end part p1;
				end part p2;
			}`,
		},
		{
			name: "binary connection definition",
			body: `connection def C {
				end part p1;
				end part p2;
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := `package Test {
	part def C {
		` + tc.body + `
	}
}`
			for _, warm := range []bool{false, true} {
				if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
					t.Errorf("warm=%v: got %v, want none", warm, got)
				}
			}
		})
	}
}

// Short names are names for distinguishability: a repeated short name, and a
// short name repeating another member's name, are both reported
// (ShortNameTests_Distinguishibility1/2.kerml.xt:20,22).
func TestW9CShortNameDistinguishability(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		lines []int
	}{
		{
			name: "repeated short name",
			src: `package Test {
	classifier <one> two;
	classifier <one> three;
}`,
			lines: []int{2, 3},
		},
		{
			name: "short name repeats a name",
			src: `package Test {
	classifier <one> two;
	classifier <two> three;
}`,
			lines: []int{2, 3},
		},
		{
			name: "distinct names and short names",
			src: `package Test {
	classifier <one> two;
	classifier <three> four;
}`,
			lines: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w9cWantLines(t, tc.src, "name-conflict", tc.lines...)
		})
	}
}

// A requirement's subject, assume and require members take part in
// distinguishability under their short names like any other member.
func TestW9CRequirementMemberShortNameDistinguishability(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		lines []int
	}{
		{
			name: "subject short name repeats a constraint short name",
			src: `package Test {
	requirement def R {
		subject <s> x;
		assume constraint <s> ac;
	}
}`,
			lines: []int{3, 4},
		},
		{
			name: "require short name repeats a member name",
			src: `package Test {
	requirement def R {
		attribute rc;
		require constraint <rc> c;
	}
}`,
			lines: []int{3, 4},
		},
		{
			name: "distinct short names",
			src: `package Test {
	requirement def R {
		subject <s> x;
		assume constraint <a> ac;
		require constraint <r> rc;
	}
}`,
			lines: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w9cWantLines(t, tc.src, "name-conflict", tc.lines...)
		})
	}
}

// A member reusing a name its one library base supplies is indistinguishable
// from it: `state start;` against StatePerformances::StateAction::start
// (examples/orthogonal-regions-demo.sysml:12).
func TestW9COwnedNameAgainstOneLibraryBaseWarns(t *testing.T) {
	src := `package Test {
	state S {
		state start;
		state idle;
	}
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{msgW9CDuplicateInherited + " 'start' from StateAction"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// A library base reached through user supertypes supplies its names just the
// same; the pinned validate-sysml reports both warnings at the same columns.
func TestW9CInheritedNameThroughUserSupertypesWarns(t *testing.T) {
	src := `package Test {
	private import Occurrences::Occurrence;
	private import Actions::Action;
	part def Base :> Occurrence;
	part def Mid :> Base;
	part def Leaf :> Mid { part portions; }
	part def Ok :> Mid { part :>> portions; }
	action def MyAct :> Action;
	action def Sub :> MyAct { action done; }
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{
			msgW9CDuplicateInherited + " 'portions' from Occurrence",
			msgW9CDuplicateInherited + " 'done' from Action",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// Redefining or subsetting the inherited feature makes the reused name its own
// name, so nothing is indistinguishable; an unreused name is silent too.
func TestW9COwnedNameSpecializingItsLibraryBaseStaysSilent(t *testing.T) {
	srcs := []string{
		`package Test {
	state S {
		state :>> start;
	}
}`,
		`package Test {
	state S {
		state redefines start;
	}
}`,
		`package Test {
	state S {
		state begin;
	}
}`,
	}
	for _, src := range srcs {
		for _, warm := range []bool{false, true} {
			if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
				t.Errorf("warm=%v: %s got %v, want none", warm, src, got)
			}
		}
	}
}

// The fixtures the pinned validate-sysml is compared against: it reports the
// same two warnings on the first and is silent on the second.
func TestW9CInheritedNameLibraryBaseFixtures(t *testing.T) {
	got := w9cMessages(libraryFixtureDiags(t, "inherited_name_library_base.sysml"), msgW9CDuplicateInherited)
	want := []string{
		msgW9CDuplicateInherited + " 'start' from StateAction",
		msgW9CDuplicateInherited + " 'done' from Action",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
	if diags := libraryFixtureDiags(t, "inherited_name_library_base_clean.sysml"); len(diags) != 0 {
		t.Errorf("clean fixture: got %v, want none", diags)
	}
}

// A usage subsetting an inherited library feature inherits that feature's own
// members too: a redefinition naming them is silent, one naming only the
// typing definition's namesake still conflicts, and an unredefined name reached
// from both is a diamond. The pinned validate-sysml reports the same three.
func TestW9CMembersInheritedThroughLibraryFeature(t *testing.T) {
	src := `package Test {
	private import ShapeItems::*;
	item def Ok :> Polyhedron {
		item tf : Quadrilateral [1] :> faces {
			ref :>> Quadrilateral::edges, Ok::faces::edges;
			ref :>> Quadrilateral::vertices, Ok::faces::vertices;
		}
	}
	item def Partial :> Polyhedron {
		item tf : Quadrilateral [1] :> faces {
			ref :>> Quadrilateral::edges;
		}
	}
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{
			msgW9CDuplicateInherited + " 'vertices' from Quadrilateral, faces",
			msgW9CDuplicateInherited + " 'edges' from faces",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// Inherited short names conflict like primary ones (KerML 7.2.2); the pinned
// validate-sysml reports each line below at the same column.
func TestW9CInheritedShortNamesConflict(t *testing.T) {
	src := `package Test {
	private import ISQSpaceTime::*;
	private import ISQ::LengthValue;
	attribute def OwnedShort :> CylindricalPosition3dVector {
		attribute h : LengthValue;
	}
	attribute def OwnedShortAsShort :> CylindricalPosition3dVector {
		attribute <h> myHeight : LengthValue;
	}
	attribute def OwnedBoth :> CylindricalPosition3dVector {
		attribute <h> height : LengthValue;
	}
	attribute def RedefShort :> CylindricalPosition3dVector {
		attribute h :>> height;
	}
	attribute def RedefLongAsShort :> CylindricalPosition3dVector {
		attribute <height> tall :>> h;
	}
	attribute def Through {
		attribute pos :> cylindricalPosition3dVector {
			attribute h : LengthValue;
		}
	}
	attribute def Diamond :> CylindricalPosition3dVector, PlanetaryPosition3dVector;
	attribute def DiamondMasked :> CylindricalPosition3dVector, PlanetaryPosition3dVector {
		attribute h :>> CylindricalPosition3dVector::height, PlanetaryPosition3dVector::altitude;
	}
	attribute def AliasShort :> CylindricalPosition3dVector {
		alias <h> H for RedefShort::h;
	}
	attribute def AliasLong :> CylindricalPosition3dVector {
		alias <hh> height for RedefShort::h;
	}
	attribute def AliasSelf :> CylindricalPosition3dVector {
		alias <h> H for height;
		alias height for CylindricalPosition3dVector::height;
	}
}`
	for _, warm := range []bool{false, true} {
		diags := w9cLibraryDiags(t, src, warm)
		var got []string
		for _, d := range diags {
			if strings.HasPrefix(d.Message, msgW9CDuplicateInherited) {
				line, col := lineCol(src, d.Span.Offset)
				got = append(got, fmt.Sprintf("%d:%d %s", line, col, d.Message))
			}
		}
		want := []string{
			"5:13 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector",
			"8:14 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector",
			"11:14 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector",
			"11:17 " + msgW9CDuplicateInherited + " 'height' from CylindricalPosition3dVector",
			"21:14 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector",
			"24:2 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector, PlanetaryPosition3dVector",
			"24:2 " + msgW9CDuplicateInherited + " 'mRef' from CylindricalPosition3dVector, PlanetaryPosition3dVector",
			"25:2 " + msgW9CDuplicateInherited + " 'mRef' from CylindricalPosition3dVector, PlanetaryPosition3dVector",
			"29:10 " + msgW9CDuplicateInherited + " 'h' from CylindricalPosition3dVector",
			"32:14 " + msgW9CDuplicateInherited + " 'height' from CylindricalPosition3dVector",
		}
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got\n%s\nwant\n%s", warm, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

// lineCol is the 1-based line and column of a byte offset in src.
func lineCol(src string, offset int) (int, int) {
	line := 1 + strings.Count(src[:offset], "\n")
	col := offset - strings.LastIndex(src[:offset], "\n")
	return line, col
}

// ShapeItems redefines through its own features the way Ok above does, so
// analyzed as a document against the library it carries no inherited-name
// conflict; the pinned validate-sysml reports none either.
func TestW9CShapeItemsHasNoInheritedNameConflict(t *testing.T) {
	const name = "Domain Libraries/Geometry/ShapeItems.sysml"
	src, err := libs.DefaultSource().Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	for _, warm := range []bool{false, true} {
		if got := w9cMessages(w9cLibraryDiags(t, string(src), warm), msgW9CDuplicateInherited); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want none", warm, got)
		}
	}
}

// A metadata usage body names owned redefinitions of the metadata definition's
// features (SysML.xtext MetadataBodyUsage), so reusing those names is silent
// (examples/pilot-corpora RationaleMetadataExample.sysml:11).
func TestW9CMetadataBodyNamesStaySilent(t *testing.T) {
	src := `package Test {
	private import ModelingMetadata::Rationale;
	part engine;
	metadata why : Rationale about engine {
		text = "because";
	}
}`
	for _, warm := range []bool{false, true} {
		if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want none", warm, got)
		}
	}
}

// A standard library package outside the standard library is a warning
// (LibraryPackage_invalid_notStandard.kerml.xt:15); a plain library package is not.
func TestW9CUserStandardLibraryPackage(t *testing.T) {
	w9cWantLines(t, "standard library package Outer {\n\tlibrary package Inner;\n}", "library-package", 1)
	w9cWantLines(t, "library package Outer {\n\tlibrary package Inner;\n}", "library-package")
}

// Binding features of unrelated types is a warning; a specializing type conforms
// (BindingConnector_Invalid3.sysml.xt:56).
func TestW9CBoundFeatureTypes(t *testing.T) {
	src := `package Test {
	part def A;
	part def B;
	part def C specializes B;
	part P {
		part a : A;
		part b : B;
		part c : C;
		bind a = b;
		bind c = b;
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 9)
}

// A variant is implicitly typed by its variation (SysML v2 §7.20), so binding
// a feature typed by the variation to one of its variants conforms; an
// unrelated declared type is still reported. The pinned pilot agrees.
func TestW9CBoundVariantConformsToItsVariation(t *testing.T) {
	src := `package Test {
	part def Base;
	part def Other;
	variation part def V {
		variant part v1 : Base;
		variant part v2 : Base;
	}
	part P {
		part vp : V;
		part w : V;
		part o : Other;
		bind w = vp.v1;
		bind vp.v2 = w;
		bind o = vp.v1;
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 14)
}

// A result expression is implicitly bound to the result parameter, so a
// non-conforming one is reported at the owning function; conforming, Boolean,
// untyped and default-valued shapes stay silent. The pinned pilot agrees.
func TestW9CResultExpressionBindingKerML(t *testing.T) {
	src := `package Test {
	datatype D; datatype C;
	feature c : C;
	feature d : D;
	function F1 { return r : D; c }
	function F2 { return r : D; d }
	function F3 { in x : C; return r : D; x }
	expr E1 { return r : D; c }
	function F4 { return r : ScalarValues::Boolean; c == c }
	function F5 { return r; c }
	function F6 { return r : D; F2() }
	function F7 { return r : D; F1() }
	function F8 { return r : C; F1() }
}`
	w9cKerMLWantLines(t, src, "bound-feature-types", 5, 7, 8, 13)
}

// A Boolean-valued expression body conforms to BooleanEvaluation, so binding one
// to a feature or result so typed draws nothing; a bare Boolean result bound to an
// Evaluation, and a body bound to Boolean or a data type, are reported. The
// pinned pilot agrees line for line.
func TestW9CBooleanBodyBindingKerML(t *testing.T) {
	src := `package Test {
	datatype D;
	feature c : D;
	feature b1 : Performances::BooleanEvaluation = { c == c };
	function F { return r : Performances::BooleanEvaluation; { c == c } }
	function G { return r : Performances::BooleanEvaluation; c == c }
	predicate Pr { c == c }
	function H { return r : Performances::Evaluation; c == c }
	function I { return r : Base::Anything; c == c }
	function J { return r : Performances::BooleanEvaluation; c }
	function K { return r : Performances::Evaluation; c }
	function L { return r : Performances::Evaluation; { c } }
	function M { return r : ScalarValues::Boolean; { c == c } }
	function N { return r : D; { c == c } }
	feature b2 : Performances::BooleanEvaluation = { not (c == c) };
	feature b3 : Performances::BooleanEvaluation = { c };
	feature b4 : Performances::Evaluation = { c };
}`
	diags := kermlLibraryDiags(t, src)
	var got []int
	for _, d := range diags {
		if d.Code != "bound-feature-types" {
			t.Errorf("unexpected %s %q", d.Code, d.Message)
			continue
		}
		got = append(got, w8dLine(src, d.Span))
	}
	w9cCheckLines(t, got, "bound-feature-types", []int{6, 8, 10, 11, 13, 14})
}

// SysML calculation definitions and usages bind their result expression the
// same way; nested subjects and satisfy-by bind to the outer subject parameter.
func TestW9CImplicitBindingsSysML(t *testing.T) {
	src := `package Test {
	part def PD; part def PD2 :> PD; item def ID; attribute def AD;
	calc def CD1 { in x : ScalarValues::String; return : ScalarValues::Integer; x }
	calc def CD2 { in x : ScalarValues::Integer; return : ScalarValues::Integer; x }
	calc def CD3 { in x : ScalarValues::Integer; return : ScalarValues::Boolean; x == 1 }
	calc def CD4 { in x : ScalarValues::Integer; return : ScalarValues::Integer; CD2(x) }
	calc def CD5 { in x : ScalarValues::Integer; return : ScalarValues::String; CD2(x) }
	calc def CD6 { in x : ScalarValues::String; return : ScalarValues::Integer; }
	calc def CD7 { in x : ScalarValues::Integer; return : ScalarValues::Integer; }
	part p : PD; part p2 : PD2; item i : ID; attribute a : AD;
	requirement def R { subject s : PD; }
	requirement def R2 :> R;
	requirement r1 : R;
	requirement r2 : R { subject s : PD2; }
	requirement r3 : R2;
	requirement def R4 { subject s : PD; requirement r5 : R { subject s5 : ID; } }
	requirement def R6 { subject s : PD; requirement r7 { subject s7 : ID; } }
	requirement def R8 { subject s : PD; abstract requirement r9 { subject s9 : ID; } }
	requirement def R10 { subject s : PD; requirement r11 { subject s11 : ID = p; } }
	requirement def R12 { subject s : PD; requirement r13 { subject s13 : ID default p; } }
	case def C { subject cs : PD; case cc { subject cs2 : ID; } }
	case def C2 { subject cs : PD; requirement cr { subject cr2 : ID; } }
	part u : PD {
		satisfy r1 by i;
		satisfy r1 by p2;
		satisfy r2 by p;
		satisfy r3 by a;
		satisfy requirement rr : R by a;
		satisfy requirement : R by p;
		satisfy requirement rq : R2 by i;
		satisfy requirement rs :> r1 by i;
		satisfy r1;
		satisfy R by a;
		calc c1 : CD6 { x }
		calc c2 : CD7 { x }
	}
	item k : ID { satisfy r1; }
}`
	w9cWantLines(t, src, "bound-feature-types", 3, 7, 17, 19, 21, 24, 27, 28, 30, 31, 34)
}

// A bound value typed by several types conforms when any one of them conforms
// to any type of the feature it fills, or the reverse; a feature typed only by
// its value takes every type of that value; arithmetic and conditionals, whose
// result types are not statically known, stay silent. The pinned pilot agrees
// line for line.
func TestW9CMultiTypedValuesSysML(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def A; part def B; part def C;
	attribute def Meters :> Real; attribute def Seconds :> Real;
	part ab : A, B; part a : A; part c : C;
	attribute m : Meters; attribute sec : Seconds;
	calc def GivesAB { return : A, B; }
	calc def GivesA { return : A; }
	calc c1 { return : B; ab }
	calc c2 { return : C; ab }
	calc c3 { return : B; GivesAB() }
	calc c4 { return : C; GivesAB() }
	calc c5 { return : B; GivesA() }
	calc c6 { return : B, C; GivesA() }
	calc c7 { return : B, C; GivesAB() }
	calc c8 { return : Seconds; m + 1 }
	calc c9 { return : String; m + 1 }
	calc c10 { return : Integer; if true ? m else sec }
	calc c11 { return : B; if true ? ab else ab }
	requirement def RB { subject s : B; }
	requirement def RC { subject s : C; }
	requirement r1 : RB { subject s : B = ab; }
	requirement r2 : RC { subject s : C = ab; }
	requirement r3 : RB { subject s : B = a; }
	part def Sys {
		part ab2 : A, B;
		part a2 : A;
		satisfy requirement rb1 : RB by ab2;
		satisfy requirement rc1 : RC by ab2;
		satisfy requirement rb2 : RB by a2;
	}
	part abs : A, B [*];
	calc c12 { return : B; abs#(1) }
	calc c13 { return : B; abs.?{in x; true} }
	calc c14 { return : C; abs#(1) }
	calc c15 { return : C; abs.?{in x; true} }
	part x = ab;
	requirement r4 : RB { subject s : B = x; }
	requirement r5 : RC { subject s : C = x; }
	calc c16 { return : B; x }
	calc c17 { return : C; x }
}`
	w9cWantLines(t, src, "bound-feature-types", 10, 12, 13, 14, 23, 24, 29, 30, 35, 36, 39, 41)
}

// A satisfy-by is judged against the subject that survives redefinition through a
// diamond, whichever branch is written first and whether the restating branch is a
// general or a referenced usage. The pinned pilot agrees line for line.
func TestW9CSatisfySubjectRedefinedThroughDiamond(t *testing.T) {
	src := `package Test {
	part def A;
	part def B :> A;
	part def X :> A;
	part b : B;
	part x : X;
	requirement def Base { subject s : A; }
	requirement def L :> Base;
	requirement r : Base { subject s2 : B :>> s; }
	requirement d : L ::> r;
	requirement def D :> L, Base { subject s3 : B :>> s; }
	requirement def E :> L, D;
	requirement def F :> D, L;
	part def Sys {
		satisfy d by x;
		satisfy requirement e : E by x;
		satisfy requirement f : F by x;
		satisfy requirement g : L by x;
		satisfy requirement h : E by b;
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 15, 16, 17)
}

// Operator results are judged by the library function's declared result: a
// data value never fills a part, a Boolean never a Meters, while a conditional
// or `??` (typed Anything) and arithmetic bound to a data type stay silent. The
// pinned pilot agrees line for line.
func TestW9COperatorResultsSysML(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	attribute def Meters :> Real; attribute def Seconds :> Real;
	part def PD; part def QD;
	attribute m : Meters; attribute sec : Seconds;
	part pd : PD; part qd : QD;
	calc c1 { return : PD; m + m }
	calc c2 { return : PD; m > sec }
	calc c3 { return : PD; -m }
	calc c4 { return : PD; pd as QD }
	calc c5 { return : PD; if true ? qd else qd }
	calc c6 { return : PD; qd ?? qd }
	calc c7 { return : PD; (qd, qd) }
	calc c8 { return : Seconds; m + m }
	calc c9 { return : String; m + m }
	calc c10 { return : Boolean; m + m }
	calc c11 { return : PD; 1 }
	calc c12 { return : PD; "s" }
	calc c13 { return : PD; null }
	calc c14 { return : PD; true }
	calc c15 { return : Boolean; m }
	calc c16 { return : Meters; sec }
	calc c17 { return : Meters; m > sec }
	calc c18 { return : Meters; not true }
	calc c19 { return : Boolean; not true }
	calc c20 { return : Meters; m ?? sec }
	calc c21 { return : PD; qd }
	calc c22 { return : PD; (qd, qd)#(1) }
	calc c23 { return : PD; (pd, pd)#(1) }
	requirement def R { subject s : PD; }
	requirement r1 : R { subject s : PD = m + m; }
	requirement r2 : R { subject s : PD = qd as PD; }
	requirement r3 : R { subject s : PD = pd as QD; }
}`
	w9cWantLines(t, src, "bound-feature-types", 7, 8, 9, 10, 17, 18, 20, 21, 22, 23, 27, 31, 33)
}

// Redefining an untyped library feature replaces its implicit value typing, so
// no diamond is drawn through it (examples/pilot-corpora TradeStudyTest.sysml:18).
func TestW9CRedefinedUntypedFeatureStaysSilent(t *testing.T) {
	srcs := []string{
		`package Test {
	part def Engine;
	analysis def A {
		subject : Engine;
		return part : Engine;
	}
}`,
		`package Test {
	private import Flows::*;
	part def Fuel;
	flow def FuelFlow {
		ref :>> payload : Fuel;
	}
}`,
	}
	for _, src := range srcs {
		for _, warm := range []bool{false, true} {
			if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
				t.Errorf("warm=%v: got %v, want none", warm, got)
			}
		}
	}
}

// A variant only references a member of its variation, so it draws no diamond
// (examples/pilot-corpora VehicleVariabilityModel.sysml:128).
func TestW9CVariantStaysSilent(t *testing.T) {
	src := `package Test {
	part def ABlock;
	action def AnActivity;
	part P {
		action a4 : ABlock;
		action a6 : ABlock;
		variation action a : AnActivity {
			variant a4;
			variant a6;
		}
	}
}`
	got := w9cMessages(w9cLibraryDiags(t, src, false), msgW9CDuplicateInherited)
	for _, msg := range got {
		if !strings.Contains(msg, "from Action, Part") {
			t.Errorf("unexpected %q", msg)
		}
	}
	if len(got) != 6 { // a4 and a6 themselves, not the variants referencing them
		t.Errorf("got %v, want the two typed actions only", got)
	}
}

// The wave-9c rules are registered once each and level-scoped (AGENTS.md §4).
func TestW9CPassesAreRegistered(t *testing.T) {
	want := map[string]PassLevel{
		"W9CShortNameDistinguishabilityPass": LevelNameResolution,
		"W9CUserStandardLibraryPass":         LevelNameResolution,
		"W9CInheritedNameConflictPass":       LevelType,
		"W9CBoundFeatureTypesPass":           LevelType,
	}
	seen := map[string]int{}
	for _, p := range DefaultRegistry().passes {
		name := fmt.Sprintf("%T", p)
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if level, ok := want[name]; ok {
			seen[name]++
			if p.Level() != level {
				t.Errorf("%s at level %v, want %v", name, p.Level(), level)
			}
		}
	}
	for name := range want {
		if seen[name] != 1 {
			t.Errorf("%s registered %d time(s), want 1", name, seen[name])
		}
	}
}

// w9cWantLines asserts the 1-based lines carrying a diagnostic with code.
func w9cWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	w9cCheckLines(t, w8dLines(t, src, code), code, want)
}

// w9cKerMLWantLines is w9cWantLines for a KerML source.
func w9cKerMLWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	var got []int
	for _, d := range only(kermlLibraryDiags(t, src), code) {
		got = append(got, w8dLine(src, d.Span))
	}
	w9cCheckLines(t, got, code, want)
}

func w9cCheckLines(t *testing.T, got []int, code string, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", code, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", code, got, want)
		}
	}
}
