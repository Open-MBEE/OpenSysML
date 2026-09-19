package identity

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// LibraryRoot is one root of a document, as library recognition reads it: its
// qualified name, the id it states ("" for none), and how it is declared.
type LibraryRoot struct {
	QName, ID string
	// Pkg reports a root written as a package; Library and Standard are its keywords.
	Pkg, Library, Standard bool
}

// DocumentRootedAt is the library document declaring every root as a top-level
// package under its name and its id, or with its package keywords where none
// is stated; "" when the roots are not one library document's.
func (c *Catalog) DocumentRootedAt(roots []LibraryRoot) string {
	doc := ""
	for _, root := range roots {
		el, ok := c.RootNamed(root.QName)
		if !ok || !topLevelPackage(root, el) || (doc != "" && el.Symbol.DocName != doc) {
			return ""
		}
		pkg, ok := el.Symbol.Decl.(*ast.Package)
		if !ok || root.ID != "" && root.ID != el.ID ||
			root.ID == "" && (root.Library != pkg.IsLibrary || root.Standard != pkg.IsStandard) {
			return ""
		}
		doc = el.Symbol.DocName
	}
	return doc
}

// topLevelPackage reports whether root is written as a package and el is one
// its document declares at the top: its owner scope is the document root.
func topLevelPackage(root LibraryRoot, el *LibraryElement) bool {
	if !root.Pkg || el.Symbol.Kind != symbols.SymbolPackage {
		return false
	}
	scope := el.Symbol.OwnerScope
	return scope != nil && scope.Parent() == nil && scope.Owner() == nil
}

// NamesEveryRoot reports whether every root of the parsed document is a package
// named as a library document's top-level package: the cheap test a document
// must pass before DocumentRootedAt is worth asking, and one a user file rarely does.
func (c *Catalog) NamesEveryRoot(root *ast.RootNamespace) bool {
	names, ok := RootPackageNames(root)
	if !ok {
		return false
	}
	for _, name := range names {
		if _, ok := c.RootNamed(name); !ok {
			return false
		}
	}
	return true
}

// RootPackageNames lists the declared names of the parsed document's roots when
// every one is a package; ok is false for no document or a root that is not one.
func RootPackageNames(root *ast.RootNamespace) (names []string, ok bool) {
	if root == nil {
		return nil, false
	}
	for _, member := range root.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			return nil, false
		}
		pkg, ok := m.Member.(*ast.Package)
		if !ok {
			return nil, false
		}
		name, _ := pkg.Ident.DeclaredName()
		names = append(names, name)
	}
	return names, true
}

// LibraryVersion is the bundled library document the named indexed document is
// a version of, judged against the library the index holds apart from it: over a
// base, a copy indexed under a library file's own name is judged against that file.
func LibraryVersion(model *semantics.Model, res *resolve.Resolver, name string) string {
	return libraryApart(res.Index(), name).VersionOf(model, res, name)
}

// VersionOf is the catalogued library document the named indexed document is a
// version of (see DocumentRootedAt); a root's id is the one an annotation
// declares, since an unannotated package states none. The document must be
// indexed unmarked, so its roots read as the user's declarations, and in the
// library document's language, which is what its text was parsed as.
func (c *Catalog) VersionOf(model *semantics.Model, res *resolve.Resolver, name string) string {
	idx := res.Index()
	rs := idx.DocumentRoot(name)
	if rs == nil {
		return ""
	}
	if root, ok := rs.Node().(*ast.RootNamespace); !ok || !c.NamesEveryRoot(root) {
		return ""
	}
	var roots []LibraryRoot
	for _, sym := range rs.Members() {
		read := LibraryRoot{QName: idx.GetFQN(sym)}
		if pkg, ok := sym.Decl.(*ast.Package); ok {
			read.Pkg, read.Library, read.Standard = true, pkg.IsLibrary, pkg.IsStandard
		}
		if info, ok := Of(model, res, sym); ok && info.Source == SourceDeclared {
			read.ID = info.EffectiveID
		}
		roots = append(roots, read)
	}
	doc := c.DocumentRootedAt(roots)
	if doc != "" && idx.DocumentKind(name) != c.DocumentKind(doc) {
		return ""
	}
	return doc
}
