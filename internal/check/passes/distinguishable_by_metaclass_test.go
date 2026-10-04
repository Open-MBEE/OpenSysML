package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nameConflictDiags runs src — whose language name's extension picks — against
// the standard library and returns its name-conflict diagnostics.
func nameConflictDiags(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var out []diag.Diagnostic
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Code == "name-conflict" {
			out = append(out, d)
		}
	}
	return out
}

// lineAt is the 1-based line offset sits on in src.
func lineAt(src string, offset int) int {
	return strings.Count(src[:offset], "\n") + 1
}

// metaclassCase is one model and the (line, message) name conflicts it should
// report under the metaclass clause of Membership::isDistinguishableFrom.
type metaclassCase struct {
	name string
	src  string
	want []string
}

func (tc metaclassCase) run(t *testing.T) {
	t.Helper()
	diags := nameConflictDiags(t, tc.name, tc.src)
	got := make([]string, 0, len(diags))
	for _, d := range diags {
		got = append(got, fmt.Sprintf("%d: %s", lineAt(tc.src, d.Span.Offset), d.Message))
	}
	if len(got) != len(tc.want) {
		t.Fatalf("%s: got %v, want %v", tc.src, got, tc.want)
	}
	for i := range tc.want {
		if got[i] != tc.want[i] {
			t.Fatalf("%s: got %v, want %v", tc.src, got, tc.want)
		}
	}
}

func TestDistinguishableByMetaclass(t *testing.T) {
	const dupOther = "Duplicate of other owned member name"
	for _, tc := range []metaclassCase{
		// Distinct, non-conforming metaclasses make equal names distinguishable.
		{"a.sysml", "package P { part def A; attribute def A; }", nil},
		{"a.kerml", "package P { class A; datatype A; }", nil},
		// Conforming metaclasses keep the warning.
		{"a.kerml", "package P { class A; class A; }", []string{"1: " + dupOther, "1: " + dupOther}},
		{"a.sysml", "package P { part def A; item def A; }", []string{"1: " + dupOther, "1: " + dupOther}},
		{"a.sysml", "package P { part def A; part def A; }", []string{"1: " + dupOther, "1: " + dupOther}},
		{"a.sysml", "package P { part p; part p; }", []string{"1: " + dupOther, "1: " + dupOther}},
		// The member element an alias names is what gets a metaclass.
		{"a.sysml", "package Q { part def A; }\npackage P { alias A for Q::A; attribute def A; }", nil},
		{"a.sysml", "package Q { part def A; }\npackage P { alias A for Q::A; part def A; }",
			[]string{"2: Duplicate of owned member name"}},
		// An alias whose target never resolved names an unknown element.
		{"a.sysml", "package P { alias A for Missing; attribute def A; }",
			[]string{"1: Duplicate of owned member name"}},
		// The clause applies to inherited memberships the same way.
		{"a.sysml", "package P { part def B { attribute def A; } part def C :> B { part def A; } }", nil},
		{"a.sysml", "package P { part def B { attribute def A; } part def C :> B { attribute def A; } }",
			[]string{"1: Duplicate of inherited member name 'A' from B"}},
		{"a.sysml", "package P { part def B1 { attribute def A; } part def B2 { part def A; } part def C :> B1, B2; }", nil},
		{"a.sysml", "package P { part def B1 { attribute def A; } part def B2 { attribute def A; } part def C :> B1, B2; }",
			[]string{"1: Duplicate of inherited member name 'A' from B1, B2"}},
		// A short name is a name for distinguishability, with the same clause.
		{"a.sysml", "package P { part def <A> X; attribute def A; }", nil},
		{"a.sysml", "package P { part def <A> X; part def A; }", []string{"1: " + dupOther, "1: " + dupOther}},
		// Members a library base contributes carry their own metaclasses:
		// AttributeDefinition vs ReferenceUsage, ItemDefinition vs PartUsage,
		// AttributeDefinition vs ActionUsage, AttributeUsage vs ReferenceUsage.
		{"a.sysml", "package P { part def X { attribute def self; } part def Y { item def start; } action def Z { attribute def start; } }", nil},
		{"a.sysml", "package P { part def X { attribute self; } }", nil},
	} {
		tc.run(t)
	}
}

// A library diamond whose members' metaclasses conform still warns — Part and
// Port `self` are both ReferenceUsage, and DataValue and Occurrence `self` are
// both KerML Feature — beside the Action/Part diamond the clause silences.
func TestConformingLibraryDiamondStillWarns(t *testing.T) {
	for _, tc := range []metaclassCase{
		{"a.sysml", `package Test {
	part def B {
		interface i {
			end part a;
			end port q;
		}
	}
}`, []string{"4: Duplicate of inherited member name 'self' from Part, Port"}},
		{"a.sysml", `package Test {
	attribute def A;
	occurrence def O {
		timeslice t : A;
	}
}`, []string{"4: Duplicate of inherited member name 'self' from DataValue, Occurrence"}},
		{"a.sysml", `package Test {
	part def ABlock;
	action def AnAction {
		action a : ABlock;
	}
}`, nil},
	} {
		tc.run(t)
	}
}

// The metaclass clause does not change name resolution: a member name still
// resolves to the same declaration it did when every duplicate warned.
func TestDistinguishableByMetaclassResolution(t *testing.T) {
	const name = "<t>.sysml"
	const src = "package P {\n\tpart def A;\n\tattribute def A;\n\tpart x : A;\n\tattribute y : A;\n}\n"
	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var typingErr, conflicts int
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Code == "name-conflict" {
			conflicts++
		}
		if d.Code != "name-conflict" && strings.Contains(d.Message, "typed by attribute definitions") {
			typingErr++
			if lineAt(src, d.Span.Offset) != 5 {
				t.Fatalf("attribute typing error on line %d, want 5", lineAt(src, d.Span.Offset))
			}
		}
	}
	if conflicts != 0 {
		t.Fatalf("got %d name-conflict diagnostics, want none", conflicts)
	}
	if typingErr != 1 {
		t.Fatalf("got %d attribute-typing errors, want 1", typingErr)
	}

	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)
	scope := idx.DocumentRoot(name)
	pkg, _ := scope.LookupLocal("P")
	members := pkg.Scope.LookupLocalAll("x")
	if len(members) != 1 {
		t.Fatalf("found %d members named x, want 1", len(members))
	}
	x := members[0]
	var target ast.Node
	for _, rel := range semantics.RelationshipsOf(x) {
		if rel != nil && rel.Kind == ast.RelTyping {
			target = rel.Target
		}
	}
	if target == nil {
		t.Fatal("part x declares no typing relationship")
	}
	var partDef *symbols.Symbol
	for _, sym := range pkg.Scope.LookupLocalAll("A") {
		if d, ok := sym.Decl.(*ast.Definition); ok && d.Kind == ast.DefPart {
			partDef = sym
		}
	}
	if partDef == nil {
		t.Fatal("found no part def A")
	}
	for i := 0; i < 2; i++ {
		resolved, ok := r.ResolveTarget(x.OwnerScope, target)
		if !ok || resolved != partDef {
			t.Fatalf("run %d: A resolved to %v (ok=%v), want the part def", i, resolved, ok)
		}
	}
}

// The fixture pair the pinned validate-sysml is compared against: every
// still-warning case sits in its own package, as does every now-clean one.
func TestDistinguishableByMetaclassFixtures(t *testing.T) {
	diags := nameConflictFixture(t, "distinguishable_by_metaclass.sysml")
	want := map[string]int{
		"Duplicate of other owned member name":               8,
		"Duplicate of owned member name":                     1,
		"Duplicate of inherited member name 'X' from B":      1,
		"Duplicate of inherited member name 'X' from B1, B2": 1,
	}
	got := map[string]int{}
	for _, d := range diags {
		got[d.Message]++
	}
	if len(diags) != 11 {
		t.Fatalf("got %v, want 11 name conflicts", diags)
	}
	for msg, n := range want {
		if got[msg] != n {
			t.Fatalf("got %d %q, want %d (all: %v)", got[msg], msg, n, diags)
		}
	}
	if diags := nameConflictFixture(t, "distinguishable_by_metaclass_clean.sysml"); len(diags) != 0 {
		t.Fatalf("clean fixture: got %v, want none", diags)
	}
}

func nameConflictFixture(t *testing.T, file string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range libraryFixtureDiags(t, file) {
		if d.Code == "name-conflict" {
			out = append(out, d)
		}
	}
	return out
}
