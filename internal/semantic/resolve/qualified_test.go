package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func indexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	for name, src := range docs {
		idx.AddDocument(name, parsedRoot(t, name, src))
	}
	return idx
}

// libraryIndexOf indexes docs as library content, the shape a library load
// produces: parsed declarations marked as library.
func libraryIndexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := indexOf(t, docs)
	for name := range docs {
		idx.MarkLibrary(name)
	}
	return idx
}

// parsedRoot parses src as the document called name, failing the test on any
// parse diagnostic.
func parsedRoot(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	return root
}

func qn(global bool, parts ...string) *ast.QualifiedName {
	q := &ast.QualifiedName{Global: global}
	for _, p := range parts {
		q.Parts = append(q.Parts, ast.NameSegment{Text: p})
	}
	return q
}

func TestResolveQualifiedFromRoot(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q { namespace N; } }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	sym, ok := r.ResolveQualified(root, qn(false, "P", "Q", "N"))
	if !ok {
		t.Fatalf("P::Q::N unresolved; diagnostics: %v", r.Diagnostics)
	}
	if sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", sym.Kind)
	}
}

// A segment is looked up under the name the index registered the namespace
// walked so far under, not under the path the reference spells: an inner
// namespace does not borrow a same-named top-level one's members.
func TestResolveQualifiedDoesNotReachASameNamedOuterNamespace(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Mid { package Inner { part def Thing; } }",
		"app.sysml": `package Outer {
			package Mid { package Inner; }
		}`,
	})
	outer := scopeOf(t, idx.DocumentRoot("app.sysml"), "Outer")

	r := New(idx)
	if _, ok := r.ResolveQualified(outer, qn(false, "Mid", "Inner", "Thing")); ok {
		t.Error("Outer::Mid::Inner declares no Thing, and the unrelated Mid::Inner::Thing is not it")
	}
}

func TestResolveQualifiedMissingSegment(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	if _, ok := r.ResolveQualified(root, qn(false, "P", "Missing")); ok {
		t.Fatalf("P::Missing should be unresolved")
	}
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic")
	}
}

func TestResolveQualifiedGlobal(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace N; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	// $::P::N resolves from the document root regardless of scope.
	sym, ok := r.ResolveQualified(root, qn(true, "P", "N"))
	if !ok || sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("$::P::N unresolved or wrong kind: %v ok=%v", sym, ok)
	}
}

func TestResolveQualifiedLeadingLocalShadowsImportedNamespace(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"app.sysml": "package Parts { part def parts; } package App { package Parts; }",
	})
	parts := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "Parts")

	r := New(idx)
	if _, ok := r.ResolveQualified(parts, qn(false, "Parts", "parts")); ok {
		t.Fatal("the innermost local Parts namespace must shadow the library Parts")
	}
}

func TestResolveQualifiedExplicitGlobalReachesLibraryNamespace(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"app.sysml": "package Parts { part def parts; } package App { package Parts; }",
	})
	parts := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "Parts")

	r := New(idx)
	if _, ok := r.ResolveQualified(parts, qn(true, "Parts", "parts")); !ok {
		t.Fatalf("$::Parts::parts should reach the library namespace; diagnostics: %v", r.Diagnostics)
	}
}

// A name a namespace holds only through a private wildcard import is not a
// visible member of it, so a qualified reference from another package does not
// reach it — while the same reference made inside that namespace does
// (KerML 8.2.3.3).
func TestResolveQualifiedRejectsAPrivatelyImportedName(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; }",
		"app.sysml":  "package App { }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	if sym, ok := r.ResolveQualified(app, qn(false, "Mid", "Hidden")); ok {
		t.Fatalf("Mid::Hidden resolved to %q from App: Mid imported Base privately",
			sym.Name)
	}

	mid := scopeOf(t, idx.DocumentRoot("mid.sysml"), "Mid")
	if _, ok := r.ResolveQualified(mid, qn(false, "Mid", "Hidden")); !ok {
		t.Fatalf("Mid::Hidden unresolved inside Mid, where the private import is "+
			"visible; diagnostics: %v", r.Diagnostics)
	}
}

// A public wildcard import still re-exports: the qualified reference the private
// case rejects resolves here.
func TestResolveQualifiedReachesAPubliclyImportedName(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Shown; }",
		"mid.sysml":  "package Mid { public import Base::*; }",
		"app.sysml":  "package App { }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	if _, ok := r.ResolveQualified(app, qn(false, "Mid", "Shown")); !ok {
		t.Fatalf("Mid::Shown unresolved from App; diagnostics: %v", r.Diagnostics)
	}
}

// An unnamed element contributes no segment to a fully-qualified name, so a
// reference written inside one is still made from the enclosing namespace and
// still sees what that namespace imported privately.
func TestResolveQualifiedFromInsideAnUnnamedElement(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; part : Hidden { part inner; } }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	mid := scopeOf(t, idx.DocumentRoot("mid.sysml"), "Mid")
	unnamed := mid.Children()
	if len(unnamed) != 1 {
		t.Fatalf("Mid has %d child scopes, want the unnamed part's", len(unnamed))
	}
	if _, ok := r.ResolveQualified(unnamed[0], qn(false, "Mid", "Hidden")); !ok {
		t.Fatalf("Mid::Hidden unresolved inside Mid's unnamed part; diagnostics: %v",
			r.Diagnostics)
	}
	inner := unnamed[0].Children()
	if len(inner) != 1 {
		t.Fatalf("the unnamed part has %d child scopes, want inner's", len(inner))
	}
	if _, ok := r.ResolveQualified(inner[0], qn(false, "Mid", "Hidden")); !ok {
		t.Fatalf("Mid::Hidden unresolved inside Mid's unnamed part's inner part; "+
			"diagnostics: %v", r.Diagnostics)
	}
}

// directChildLookup is how a semantic model reaches the members of a symbol
// restored from the library cache, which carries no Scope: by enumerating the
// index's direct children under its FQN.
type directChildLookup struct{ idx *symbols.Index }

func (d directChildLookup) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, child := range d.idx.LookupDirectChildren(sym.Name) {
		leaf := child.Name
		if i := strings.LastIndex(leaf, "::"); i >= 0 {
			leaf = leaf[i+2:]
		}
		if leaf == name {
			return child, true
		}
	}
	return nil, false
}

func (d directChildLookup) LookupContributedMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

// The qualified walk falls back to an inheritance-aware member search when the
// index has nothing, and that search enumerates direct children without
// consulting the visibility marks. A privately imported name must not come back
// through it either — the stdlib, where private wildcard imports are pervasive,
// is loaded exactly this way.
func TestResolveQualifiedRejectsAPrivatelyImportedNameThroughMemberLookup(t *testing.T) {
	idx := libraryIndexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)
	r.SetModel(directChildLookup{idx: idx})

	if sym, ok := r.ResolveQualified(nil, qn(false, "Mid", "Hidden")); ok {
		t.Fatalf("Mid::Hidden resolved to %q from outside Mid: Mid imported Base "+
			"privately", sym.Name)
	}
	// The member route still reaches a name Mid holds publicly.
	if _, ok := r.ResolveQualified(nil, qn(false, "Base", "Hidden")); !ok {
		t.Fatalf("Base::Hidden unresolved; diagnostics: %v", r.Diagnostics)
	}
}

func TestResolveQualifiedSegmentIntoLeaf(t *testing.T) {
	// A leaf symbol (no child scope) cannot own further segments.
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { comment C /* x */ }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	if _, ok := r.ResolveQualified(root, qn(false, "P", "C", "Deeper")); ok {
		t.Fatalf("P::C::Deeper should fail past the leaf comment")
	}
}
