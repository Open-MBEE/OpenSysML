package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// filterIn parses cond as a filter condition written at the root of the named
// document of an index the model is built over.
func filterIn(t *testing.T, m *Model, doc, cond string) symbols.ElementFilter {
	t.Helper()
	p := parser.New(source.New("<filter>", []byte(cond)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse filter condition %q: %v", cond, p.Diagnostics)
	}
	root := m.resolver.Index().DocumentRoot(doc)
	if root == nil {
		t.Fatalf("%s has no root scope", doc)
	}
	return symbols.ElementFilter{Expr: expr, Scope: root, Span: expr.Span()}
}

// verdict evaluates a condition for one element, failing on an unevaluable one.
func verdict(t *testing.T, m *Model, f symbols.ElementFilter, cand *symbols.Symbol) bool {
	t.Helper()
	got, err := m.EvalElementFilter(f, cand)
	if err != nil {
		t.Fatalf("unexpected error evaluating the condition for %s: %v", cand.Name, err)
	}
	return got
}

// A condition naming a metadata type through an alias is compiled to the Type
// the alias names, so it selects what that Type annotates (KerML 8.3.2.3).
func TestFilterThroughAnAliasNamesTheAliasedType(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		metadata def CrashSafety :> Safety;
		alias S for Safety;
		#Safety part belt;
		#CrashSafety part airBag;
		part radio;
	`)
	f := symbols.ElementFilter{Scope: root}
	f.Expr = filterIn(t, m, "t.sysml", "@S").Expr
	f.Span = f.Expr.Span()
	pred := m.CompileElementFilter(f)
	if pred == nil || pred.Op != symbols.FilterClassify || !symbols.SameElement(pred.Type, sym(t, root, "Safety")) {
		t.Fatalf("@S must compile to a classification by the Safety element, got %+v", pred)
	}
	for name, want := range map[string]bool{"belt": true, "airBag": true, "radio": false} {
		if got := verdict(t, m, f, sym(t, root, name)); got != want {
			t.Errorf("@S for %s = %v, want %v", name, got, want)
		}
	}
}

// Two documents each opening Meta::Safety declare two Types under one qualified
// name; a condition names the one its own document resolves to, so an element
// the other annotates does not conform, and no name is built to decide it.
func TestFilterNamingANamesakeTypeSelectsByElement(t *testing.T) {
	idx := symbols.NewIndex()
	addTestDoc(t, idx, "a.sysml", `package Meta { metadata def Safety; }
		package LibA { #Meta::Safety part belt; }`)
	addTestDoc(t, idx, "b.sysml", `package Meta { metadata def Safety; }`)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	belt := idx.LookupQualified("LibA::belt")[0]

	own := filterIn(t, m, "a.sysml", "@Meta::Safety")
	other := filterIn(t, m, "b.sysml", "@Meta::Safety")
	if !verdict(t, m, own, belt) {
		t.Fatalf("belt must conform to the Safety that annotates it")
	}
	if verdict(t, m, other, belt) {
		t.Fatalf("belt must not conform to the other document's namesake Safety")
	}
	ownType, otherType := m.CompileElementFilter(own).Type, m.CompileElementFilter(other).Type
	if ownType == nil || otherType == nil || symbols.SameElement(ownType, otherType) {
		t.Fatalf("the two conditions must name two distinct elements, got %v and %v", ownType, otherType)
	}
	for _, typ := range []*symbols.Symbol{ownType, otherType} {
		for _, a := range m.annotationsOf(belt) {
			if allocs := testing.AllocsPerRun(20, func() { m.annotationConforms(a, typ, "Meta::Safety") }); allocs != 0 {
				t.Errorf("a repeated conformance check against %s allocates %.0f objects, want 0", typ.DocName, allocs)
			}
		}
	}
}

// A condition naming a library type is not satisfied by a workspace namesake of
// that type, while a second library copy of the same declaration still is.
func TestFilterNamingALibraryTypeTellsAWorkspaceNamesakeApart(t *testing.T) {
	idx := symbols.NewIndex()
	addTestDoc(t, idx, "lib.sysml", `package Meta { metadata def Safety; }`)
	addTestDoc(t, idx, "lib2.sysml", `package Meta { metadata def Safety; }
		package LibB { #Meta::Safety part strap; }`)
	addTestDoc(t, idx, "ws.sysml", `package Meta { metadata def Safety; }
		package LibA { #Meta::Safety part belt; }`)
	idx.MarkLibrary("lib.sysml")
	idx.MarkLibrary("lib2.sysml")
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	belt := idx.LookupQualified("LibA::belt")[0]
	strap := idx.LookupQualified("LibB::strap")[0]

	f := filterIn(t, m, "lib.sysml", "@Meta::Safety")
	if typ := m.CompileElementFilter(f).Type; typ == nil || typ.DocName != "lib.sysml" {
		t.Fatalf("the condition must name the library's Safety, got %v", typ)
	}
	if verdict(t, m, f, belt) {
		t.Fatalf("belt, annotated by the workspace's namesake Safety, must not conform to the library's")
	}
	if !verdict(t, m, f, strap) {
		t.Fatalf("strap, annotated by another library copy of Safety, must conform to the library's")
	}
}

// Base::Anything is the top type only as the library declares it; a workspace
// type of that name admits no more than what it annotates (KerML 1.0 §9.2.2.2).
func TestFilterNamingAWorkspaceAnythingIsNotTheTopType(t *testing.T) {
	for _, library := range []bool{false, true} {
		idx := symbols.NewIndex()
		addTestDoc(t, idx, "base.sysml", `package Base { metadata def Anything; }`)
		addTestDoc(t, idx, "ws.sysml", `package Meta { metadata def Tag; }
			package LibA { #Meta::Tag part radio; }`)
		if library {
			idx.MarkLibrary("base.sysml")
		}
		idx.ExpandWildcardImports()
		r := resolve.New(idx)
		m := NewModel(r)
		r.SetModel(m)
		radio := idx.LookupQualified("LibA::radio")[0]
		f := filterIn(t, m, "ws.sysml", "@Base::Anything")
		if got := verdict(t, m, f, radio); got != library {
			t.Errorf("library=%v: a part annotated by an unrelated type conforms to Base::Anything: got %v, want %v", library, got, library)
		}
	}
}
