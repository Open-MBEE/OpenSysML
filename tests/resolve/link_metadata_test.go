package resolve_test

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// linkedMetadataBodies covers how an annotation body finds its owner: in a
// definition, at the root, about an element, via an alias, nested, unresolved.
const linkedMetadataBodies = `package P {
	attribute def T { attribute b; }
	metadata def M { attribute a : T; attribute n; }
	alias N for M;
	part def C {
		@M { n = 1; a { b = 2; } }
		@N { n = 3; }
		@Missing { n = 4; }
	}
	@M about C { n = 5; }
	@N { n = 6; }
}`

// ownersOf renders the owner of every scope of the tree in tree order, "" for none.
func ownersOf(idx *symbols.Index, root *symbols.Scope) []string {
	var out []string
	var visit func(*symbols.Scope)
	visit = func(s *symbols.Scope) {
		owner := ""
		if s.Owner() != nil {
			owner = idx.GetFQN(s.Owner())
		}
		out = append(out, owner)
		for _, c := range s.Children() {
			visit(c)
		}
	}
	visit(root)
	return out
}

func indexedDoc(t *testing.T, name, src string) (*symbols.Index, *ast.RootNamespace, *resolve.Resolver) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	return idx, root, r
}

// Linking a document's metadata bodies sets exactly the owners resolving it sets,
// so resolving a linked document changes nothing and reports what it would have.
func TestLinkMetadataBodiesSetsWhatResolvingWould(t *testing.T) {
	const name = "linked.sysml"
	for _, src := range []string{linkedMetadataBodies, nestedMetadataBodies["nested.sysml"], nestedMetadataBodies["nested.kerml"]} {
		idx, root, r := indexedDoc(t, name, src)
		r.ResolveDocument(name, root)
		want := ownersOf(idx, idx.DocumentRoot(name))

		linkedIdx, linkedRoot, linker := indexedDoc(t, name, src)
		linker.LinkMetadataBodies(name)
		linked := ownersOf(linkedIdx, linkedIdx.DocumentRoot(name))
		if !reflect.DeepEqual(linked, want) {
			t.Errorf("linked owners\n%q\nwant those resolving sets\n%q", linked, want)
		}
		resolver := resolve.New(linkedIdx)
		resolver.SetModel(semantics.NewModel(resolver))
		resolver.ResolveDocument(name, linkedRoot)
		if got := ownersOf(linkedIdx, linkedIdx.DocumentRoot(name)); !reflect.DeepEqual(got, linked) {
			t.Errorf("resolving a linked document changed owners to\n%q\nfrom\n%q", got, linked)
		}
		if !reflect.DeepEqual(resolver.Diagnostics, r.Diagnostics) {
			t.Errorf("diagnostics after linking %v, want %v", resolver.Diagnostics, r.Diagnostics)
		}
	}
}

// A prefix carrying a body resolves its type from the annotated declaration's own
// scope; the linker does the same, so a type visible only there is found.
func TestLinkMetadataBodiesResolvesAPrefixBodyFromTheAnnotatedScope(t *testing.T) {
	const name = "prefixed.sysml"
	const src = `package P {
	metadata def M { attribute n; }
	part def C { alias Local for M; }
	@M { n = 1; }
}`
	build := func() (*symbols.Index, *ast.RootNamespace, *resolve.Resolver) {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		c := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		usage := pkg.Members[2].(*ast.Membership).Member.(*ast.PrefixMetadata)
		prefix := &ast.PrefixMetadata{Body: usage.Body, HasBody: true,
			Type: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "Local"}}}}
		c.Prefixes = []*ast.PrefixMetadata{prefix}
		pkg.Members = pkg.Members[:2]
		idx := symbols.NewIndexFromDoc(name, root)
		r := resolve.New(idx)
		r.SetModel(semantics.NewModel(r))
		return idx, root, r
	}
	idx, root, r := build()
	r.ResolveDocument(name, root)
	want := ownersOf(idx, idx.DocumentRoot(name))
	pkgScope := idx.DocumentRoot(name).Children()[0]
	c := pkgScope.Node().(*ast.Package).Members[1].(*ast.Membership).Member.(*ast.Definition)
	body := pkgScope.ChildFor(c.Prefixes[0])
	if body == nil || body.Owner() == nil || idx.GetFQN(body.Owner()) != "P::M" {
		t.Fatalf("resolving does not own the prefix body by P::M: %v", body)
	}
	linkedIdx, _, linker := build()
	linker.LinkMetadataBodies(name)
	if got := ownersOf(linkedIdx, linkedIdx.DocumentRoot(name)); !reflect.DeepEqual(got, want) {
		t.Errorf("linked owners %q, want %q", got, want)
	}
}

// Linking reaches every annotation body of the document: only the one typed by a
// name that does not resolve is left unowned.
func TestLinkMetadataBodiesReachesEveryBody(t *testing.T) {
	const name = "linked.sysml"
	idx, _, linker := indexedDoc(t, name, linkedMetadataBodies)
	linker.LinkMetadataBodies(name)
	var bodies, owned int
	var visit func(*symbols.Scope)
	visit = func(s *symbols.Scope) {
		if _, ok := s.Node().(*ast.PrefixMetadata); ok {
			bodies++
			if s.Owner() != nil {
				owned++
				if fqn := idx.GetFQN(s.Owner()); fqn != "P::M" && fqn != "P::M::a" {
					t.Errorf("body owned by %s, want P::M or P::M::a", fqn)
				}
			}
		}
		for _, c := range s.Children() {
			visit(c)
		}
	}
	visit(idx.DocumentRoot(name))
	if bodies != 5 || owned != 4 {
		t.Errorf("%d bodies, %d owned; want 5 and 4", bodies, owned)
	}
}

// parsedRoot parses src as name, failing the test on a parse diagnostic.
func parsedRoot(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("%s: parse diagnostics: %v", name, p.Diagnostics)
	}
	return root
}

// An annotation body owned by a definition of another document follows that
// document: once it is replaced, resolving or linking the unchanged annotating
// document owns the body by the definition now indexed, and once the definition
// is gone the body is owned by nothing.
func TestMetadataBodyOwnerFollowsTheDefinitionsDocument(t *testing.T) {
	const meta, use = "meta.sysml", "use.sysml"
	const useSrc = "package Use { private import Meta::*; part def C { @M { a = b; } } }"
	bodyOwner := func(idx *symbols.Index) *symbols.Symbol {
		body := idx.DocumentRoot(use).Children()[0].Children()[0].Children()[0]
		if _, ok := body.Node().(*ast.PrefixMetadata); !ok {
			t.Fatalf("C's first scope is a %T, want the annotation body", body.Node())
		}
		return body.Owner()
	}
	for _, relink := range []struct {
		name string
		do   func(r *resolve.Resolver, root *ast.RootNamespace)
	}{
		{"resolving", func(r *resolve.Resolver, root *ast.RootNamespace) { r.ResolveDocument(use, root) }},
		{"linking", func(r *resolve.Resolver, _ *ast.RootNamespace) { r.LinkMetadataBodies(use) }},
	} {
		idx := symbols.NewIndex()
		idx.AddDocument(meta, parsedRoot(t, meta, "package Meta { metadata def M { attribute a; attribute b; } }"))
		useRoot := parsedRoot(t, use, useSrc)
		idx.AddDocument(use, useRoot)
		idx.ExpandWildcardImports()
		r := resolve.New(idx)
		r.SetModel(semantics.NewModel(r))
		r.ResolveDocument(use, useRoot)
		first := bodyOwner(idx)
		if first == nil || idx.GetFQN(first) != "Meta::M" {
			t.Fatalf("%s: body owned by %v before the reload, want Meta::M", relink.name, first)
		}

		idx.AddDocument(meta, parsedRoot(t, meta, "package Meta { metadata def M { attribute a; attribute c; } }"))
		idx.ExpandWildcardImports()
		r = resolve.New(idx)
		r.SetModel(semantics.NewModel(r))
		relink.do(r, useRoot)
		if got := bodyOwner(idx); got == nil || idx.GetFQN(got) != "Meta::M" || got == first {
			t.Errorf("%s after the definition's document was replaced: body owned by %s, want the Meta::M now indexed", relink.name, fqnOrNone(idx, got))
		}
		if syms := idx.LookupQualified("Meta::M::b"); len(syms) != 0 {
			t.Fatalf("Meta::M::b should be gone from the index, found %d", len(syms))
		}

		idx.RemoveDocument(meta)
		r = resolve.New(idx)
		r.SetModel(semantics.NewModel(r))
		relink.do(r, useRoot)
		if got := bodyOwner(idx); got != nil {
			t.Errorf("%s after the definition's document was removed: body owned by %s, want nothing", relink.name, fqnOrNone(idx, got))
		}
	}
}

func fqnOrNone(idx *symbols.Index, sym *symbols.Symbol) string {
	if sym == nil {
		return "nothing"
	}
	if fqn := idx.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name + " of an earlier " + sym.DocName
}
