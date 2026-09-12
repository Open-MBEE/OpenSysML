package main

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// scopeNotes covers the note shapes an XPECT scope assertion is written in: a
// fenced block, a one-line arrow form, and the compact form whose anchor runs
// into the fence.
const scopeNotes = `//*
XPECT_SETUP org.omg.kerml.xpect.tests.testsuite.KerMLTest
	ResourceSet {
		ThisFile {}
	}
END_SETUP
*/
package test {
	//* XPECT scope at A ---
	   A, A.a, test.A
	--- */
	class B specializes A;
	// XPECT scope at B--> B, test.B
	class C specializes B;
	//* XPECT scope at aliass---
	   aliass, test.aliass
	--- */
	alias aliass for C;
	//* XPECT scope at Q::D ---
	   Q, Q.D
	--- */
	class E specializes Q::D;
}
`

func TestParseXTScopeAssertionShapes(t *testing.T) {
	f := parseXT("t.kerml.xt", "kerml", []byte(scopeNotes))
	if len(f.Problems) != 0 {
		t.Fatalf("problems: %v", f.Problems)
	}
	var got []string
	for _, a := range f.Assertions {
		if a.Kind != "scope" {
			continue
		}
		got = append(got, a.At+"="+strings.Join(a.Names, "|"))
	}
	want := []string{
		"A=A|A.a|test.A",
		"B=B|test.B",
		"aliass=aliass|test.aliass",
		"Q::D=Q|Q.D",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("scope assertions =\n%v\nwant\n%v", got, want)
	}
}

func TestParseXTScopeWithoutNamesIsAProblem(t *testing.T) {
	f := parseXT("t.kerml.xt", "kerml", []byte("// XPECT scope at A --->\nclass A;\n"))
	if len(f.Problems) == 0 {
		t.Errorf("an XPECT scope declaring no name should be a problem, got %+v", f.Assertions)
	}
}

func TestAnchorMetatypeReadsTheDeclarationHead(t *testing.T) {
	cases := []struct {
		src  string
		at   string
		ref  resolve.Reference
		want metatype
	}{
		{"class B specializes A;", "A", resolve.Reference{}, mtClassifier},
		{"feature b subsets zz;", "zz", resolve.Reference{}, mtFeature},
		{"feature b redefines zz;", "zz", resolve.Reference{Redefines: true}, mtFeature},
		{"feature b : A;", "A", resolve.Reference{}, mtType},
		{"public import P::A;", "P::A", resolve.Reference{}, mtAny},
		{"alias x for A;", "A", resolve.Reference{}, mtAny},
	}
	for _, c := range cases {
		offset := strings.Index(c.src, c.at)
		if got := anchorMetatype([]byte(c.src), offset, c.ref); got != c.want {
			t.Errorf("anchorMetatype(%q at %q) = %v, want %v", c.src, c.at, got, c.want)
		}
	}
}

// TestOnlyARedefinitionNarrowsTheScope reads the anchors the way scopeRow does:
// a subsetting is collected as a redefinition-style reference, but the pilot
// still enumerates the whole scope for it.
func TestOnlyARedefinitionNarrowsTheScope(t *testing.T) {
	src := "package test {\n" +
		"\tfeature A {\n\t\tfeature a;\n\t\talias aa for a;\n\t}\n" +
		"\tfeature B subsets A {\n" +
		"\t\tfeature b subsets aa;\n" +
		"\t\tfeature c redefines a;\n" +
		"\t\tfeature d subsets d;\n" +
		"\t}\n}\n"
	ws := model.NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	doc := ws.Document("t.kerml")
	refs := resolve.References(doc.AST, doc.Scope)
	cases := []struct {
		clause, target string
		narrows        bool
	}{
		{"subsets aa", "aa", false},
		{"redefines a", "a", true},
		{"subsets d", "d", false},
	}
	for _, c := range cases {
		offset := strings.Index(src, c.clause) + len(c.clause) - len(c.target)
		ref, _, ok := referenceAt(refs, offset, offset+len(c.target))
		if !ok {
			t.Fatalf("%q: no reference collected at %d", c.clause, offset)
		}
		if got := narrowsToInherited(ref); got != c.narrows {
			t.Errorf("%q: narrowsToInherited = %v, want %v (ref %+v)", c.clause, got, c.narrows, ref)
		}
	}
}

func TestAdmitsFiltersByMetatype(t *testing.T) {
	cases := []struct {
		want metatype
		kind symbols.SymbolKind
		ok   bool
	}{
		{mtAny, symbols.SymbolPackage, true},
		{mtType, symbols.SymbolPackage, false},
		{mtType, symbols.SymbolAttributeUsage, true},
		{mtClassifier, symbols.SymbolAttributeUsage, false},
		{mtFeature, symbols.SymbolAttributeUsage, true},
		{mtFeature, symbols.SymbolPartDef, false},
		{mtAny, symbols.SymbolUnknown, false},
	}
	for _, c := range cases {
		if got := admits(c.want, c.kind); got != c.ok {
			t.Errorf("admits(%v, %v) = %v, want %v", c.want, c.kind, got, c.ok)
		}
	}
}

// scopeDiffAt compares the names visible at an anchor with a declared list.
func scopeDiffAt(t *testing.T, src, anchor string, declared []string) scopeDiff {
	t.Helper()
	ws := model.NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	doc := ws.Document("t.kerml")
	offset := strings.Index(src, anchor)
	scope := doc.Scope
	if s := scopeOwnerAt(doc.Scope, offset); s != nil {
		scope = s
	}
	return scopeDiffOf(ws, scope, ws.VisibleNames(scope, model.VisibleNamesOptions{}), mtAny, declared)
}

// scopeOwnerAt is the deepest scope of the document whose declaration holds
// offset, mirroring what the harness resolves an anchor against.
func scopeOwnerAt(root *symbols.Scope, offset int) *symbols.Scope {
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		if sp := sym.DeclSpan; offset >= sp.Offset && offset < sp.End() {
			return scopeOwnerAt(sym.Scope, offset)
		}
	}
	return root
}

func TestScopeDiffAgreementAndClasses(t *testing.T) {
	src := "package P {\n\tclassifier A;\n\tclassifier B;\n}\n"
	all := []string{"A", "B", "P", "P.A", "P.B"}

	if d := scopeDiffAt(t, src, "classifier B", all); d.distance() != 0 {
		t.Errorf("exact set should agree: missing %v extra %v", d.missing, d.extra)
	}
	if d := scopeDiffAt(t, src, "classifier B", append(all, "Absent")); len(d.missing) != 1 || len(d.extra) != 0 {
		t.Errorf("a declared name we do not offer is missing: %+v", d)
	}
	if d := scopeDiffAt(t, src, "classifier B", all[:4]); len(d.extra) != 1 || len(d.missing) != 0 {
		t.Errorf("a name we offer and the pilot does not is extra: %+v", d)
	}
}

func TestReachableAsComparesElementsNotSpellings(t *testing.T) {
	src := "package P {\n\tclassifier A;\n\tclassifier B;\n}\n"
	ws := model.NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	scope := ws.Document("t.kerml").Scope
	held := map[string]bool{"P::A": true}
	if !reachableAs(ws, scope, held, "P.A") {
		t.Error("P.A names P::A, which is held")
	}
	if reachableAs(ws, scope, held, "P.B") {
		t.Error("P.B names P::B, which is not held")
	}
	if reachableAs(ws, scope, held, "P.Absent") {
		t.Error("an unresolvable path reaches nothing")
	}
}

func TestNotImplicitDropsOnlyLibraryTailPaths(t *testing.T) {
	got := notImplicit([]string{"A.self", "A.that", "A.self.that", "A.a", "self", "A.other"})
	want := "A.a|A.other"
	if strings.Join(got, "|") != want {
		t.Errorf("notImplicit = %v, want %v", got, want)
	}
}
