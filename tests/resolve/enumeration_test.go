package resolve_test

import (
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Completion enumerates names rather than resolving them, so the enumeration
// paths must apply the same filters and visibility a lookup does.
const filteredLibrary = `metadata def Critical;
package Kit {
	#Critical part def Brake;
	part def Trim;
	private part def Internal;
}
package Mid {
	public import Kit::*;
	filter @Critical;
}`

// A document that borrows Kit's names privately: they are members of it, not of
// any other document.
const borrowingClient = `private import Kit::*;`

const filteredClient = `package App {
	private import Kit::*[@Critical];
	private import Mid::*;
	private import Kit::Trim;
}`

// The elements an import surfaces are the ones a lookup through it reaches, for
// the wildcard form and the membership form alike.
func TestImportedElementsAreTheOnesTheImportSurfaces(t *testing.T) {
	r, _, scope := filteredWorkspace(t)
	imports := importsOf(t, scope)
	if len(imports) != 3 {
		t.Fatalf("the client declares %d imports, want 3", len(imports))
	}
	// The import's own filter clause; a namespace's filter members, which gate
	// what it re-exports; and a membership import, which names one element.
	for i, want := range [][]string{{"Brake"}, {"Brake"}, {"Trim"}} {
		if got := namesOf(r.ImportedElements(scope, imports[i])); !equalNames(got, want) {
			t.Errorf("import %d surfaces %v, want %v", i, got, want)
		}
	}
}

// An enumeration of a namespace's members offers exactly the names a lookup in the
// same scope resolves: a filtered-out or private member is neither.
func TestAdmittedChildrenMatchWhatResolves(t *testing.T) {
	r, idx, scope := filteredWorkspace(t)
	from := r.ReferringNamespaceFQN(scope)
	children := idx.LookupDirectChildrenFrom("Kit", from)
	if len(children) < 2 {
		t.Fatalf("the index holds %d children of Kit, want the declared ones", len(children))
	}
	offered := map[string]bool{}
	for _, sym := range r.AdmittedChildrenOf(scope, "Kit", children) {
		offered[sym.Name] = true
	}
	for _, child := range children {
		_, reachable := r.ResolveQualified(scope, qualifiedName("Kit", child.Name))
		if offered[child.Name] != reachable {
			t.Errorf("Kit::%s offered = %v, but resolving it = %v", child.Name, offered[child.Name], reachable)
		}
	}
	if !offered["Brake"] {
		t.Error("Kit::Brake is neither offered nor reachable, so the filter admits nothing")
	}
}

// A name borrowed through another document's private root-level import belongs to
// that document alone, so enumerating this document's root must not offer it.
func TestAdmittedTopLevelExcludesAnotherDocumentsPrivateImport(t *testing.T) {
	r, idx, _ := filteredWorkspace(t)
	borrowed := namesOf(r.AdmittedTopLevel("borrow.sysml", idx.TopLevelBindings("borrow.sysml")))
	if !contains(borrowed, "Brake") {
		t.Errorf("borrow.sysml offers %v at the root, want its privately imported Brake", borrowed)
	}
	offered := namesOf(r.AdmittedTopLevel("app.sysml", idx.TopLevelBindings("app.sysml")))
	if !contains(offered, "Kit") {
		t.Errorf("app.sysml offers %v at the root, want another document's public Kit", offered)
	}
	if contains(offered, "Brake") {
		t.Errorf("app.sysml offers %v at the root: Brake is another document's private import", offered)
	}
}

// filteredWorkspace indexes filteredLibrary and filteredClient as two documents
// of one workspace and returns a resolver over them with the client's root scope.
func filteredWorkspace(t *testing.T) (*resolve.Resolver, *symbols.Index, *symbols.Scope) {
	t.Helper()
	idx := symbols.NewIndex()
	for name, src := range map[string]string{
		"kit.sysml":    filteredLibrary,
		"borrow.sysml": borrowingClient,
		"app.sysml":    filteredClient,
	} {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s parse diagnostics: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	app := idx.LookupQualified("App")
	if len(app) != 1 || app[0].Scope == nil {
		t.Fatalf("App names %d symbols with a scope", len(app))
	}
	return r, idx, app[0].Scope
}

// importsOf are the import declarations written in scope's owning namespace.
func importsOf(t *testing.T, scope *symbols.Scope) []*ast.Import {
	t.Helper()
	pkg, ok := scope.Node().(*ast.Package)
	if !ok {
		t.Fatalf("the scope's node is %T, want a package", scope.Node())
	}
	var out []*ast.Import
	for _, member := range pkg.Members {
		if imp, isImport := member.(*ast.Import); isImport {
			out = append(out, imp)
		}
	}
	return out
}

// qualifiedName builds the name a qualified reference to segments would parse as.
func qualifiedName(segments ...string) *ast.QualifiedName {
	qn := &ast.QualifiedName{}
	for _, segment := range segments {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: segment})
	}
	return qn
}

func namesOf(syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, sym.Name)
	}
	sort.Strings(out)
	return out
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
