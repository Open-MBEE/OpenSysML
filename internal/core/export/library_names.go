package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// NormativeSubject reports whether id is the id the norm fixes for the subject a
// graph names qualifiedName: the library element the encoder writes under that
// name, or its owning membership, which names nothing. A user element that
// carries a library uuid is not the library element, so its id stays its own.
func NormativeSubject(id, qualifiedName string) bool {
	catalog := identity.LibraryCatalog(libs.NewModelIndex())
	el, ok := catalog.Element(id)
	if !ok {
		_, om := catalog.OwningMembership(id)
		return om && qualifiedName == ""
	}
	if el.FQN == qualifiedName {
		return true
	}
	written, err := libraryGraphName(el)
	return err == nil && written == qualifiedName
}

// libraryRoot is one root of a document, as library recognition reads it: its
// qualified name, the id it states ("" for none), and how it is declared.
type libraryRoot struct {
	qname, id string
	// pkg reports a root written as a package; library and standard are its keywords.
	pkg, library, standard bool
}

// libraryDocument is the bundled library document a graph is a version of: the
// one declaring every root, each a top-level package the norm names and ids as
// the graph does. A graph rooted anywhere else — in a nested library element,
// or in several documents — is none, and is read beside the library.
func libraryDocument(roots []*element) string {
	var read []libraryRoot
	for _, root := range roots {
		read = append(read, libraryRoot{qname: root.qname, id: root.elementID, pkg: root.metaclass == "Package"})
	}
	return libraryDocumentOf(read)
}

// documentLibrary is the bundled library document a parsed document is a version
// of; its roots' ids are read beside the library, where only an annotation states one.
func documentLibrary(name string, root *ast.RootNamespace) string {
	idx := libs.NewModelIndex()
	catalog := identity.LibraryCatalog(idx)
	for _, member := range root.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			return ""
		}
		pkg, ok := m.Member.(*ast.Package)
		if !ok {
			return ""
		}
		if _, ok := catalog.ElementNamed(pkg.Ident.Name); !ok {
			return ""
		}
	}
	idx.AddDocument(name, root)
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	var roots []libraryRoot
	for _, sym := range idx.DocumentRoot(name).Members() {
		read := libraryRoot{qname: idx.GetFQN(sym)}
		if pkg, ok := sym.Decl.(*ast.Package); ok {
			read.pkg, read.library, read.standard = true, pkg.IsLibrary, pkg.IsStandard
		}
		if info, ok := identity.Of(model, res, sym); ok && info.Source == identity.SourceDeclared {
			read.id = info.EffectiveID
		}
		roots = append(roots, read)
	}
	return libraryDocumentOf(roots)
}

// libraryDocumentOf is the bundled library document declaring every root as a top-level
// package under its name and its id, or with its package keywords where none is stated.
func libraryDocumentOf(roots []libraryRoot) string {
	catalog := identity.LibraryCatalog(libs.NewModelIndex())
	doc := ""
	for _, root := range roots {
		el, ok := catalog.ElementNamed(root.qname)
		if !ok || !topLevelPackage(root, el) || (doc != "" && el.Symbol.DocName != doc) {
			return ""
		}
		pkg, ok := el.Symbol.Decl.(*ast.Package)
		if !ok || root.id != "" && root.id != el.ID ||
			root.id == "" && (root.library != pkg.IsLibrary || root.standard != pkg.IsStandard) {
			return ""
		}
		doc = el.Symbol.DocName
	}
	return doc
}

// topLevelPackage reports whether root is written as a package and el is one
// its document declares at the top: its owner scope is the document root.
func topLevelPackage(root libraryRoot, el *identity.LibraryElement) bool {
	if !root.pkg || el.Symbol.Kind != symbols.SymbolPackage {
		return false
	}
	scope := el.Symbol.OwnerScope
	return scope != nil && scope.Parent() == nil && scope.Owner() == nil
}

// libraryGraphNames memoizes, per bundled library document, the qualified name
// the encoder writes for each element the norm fixes an id for, keyed by that id.
var libraryGraphNames sync.Map // document name → map[string]string, or error

// libraryGraphName is the qualified name the encoder writes el under, which
// positions an effectively named member (`@n`) where the symbol table names it.
func libraryGraphName(el *identity.LibraryElement) (string, error) {
	doc := el.Symbol.DocName
	cached, ok := libraryGraphNames.Load(doc)
	if !ok {
		cached, _ = libraryGraphNames.LoadOrStore(doc, encodeLibraryNames(doc))
	}
	switch names := cached.(type) {
	case map[string]string:
		if name, ok := names[el.ID]; ok {
			return name, nil
		}
		return "", fmt.Errorf("library element %s is not written by %s", el.FQN, doc)
	case error:
		return "", names
	}
	return "", fmt.Errorf("library document %s: unexpected cache entry %T", doc, cached)
}

// encodeLibraryNames runs the encoder's naming over one bundled library
// document, so the names it answers with are the ones it writes.
func encodeLibraryNames(doc string) any {
	data, err := libs.EmbeddedSource().Read(doc)
	if err != nil {
		return err
	}
	file := source.New(doc, data)
	p := parser.New(file)
	root := p.ParseFile()
	if err := syntaxError(doc, file, p); err != nil {
		return err
	}
	e, err := newEncoder(file, root, doc)
	if err != nil {
		return err
	}
	names := make(map[string]string)
	for node, el := range e.ids.byNode {
		if el.source != identity.SourceNormative {
			continue
		}
		if fqn, ok := e.fqn[node]; ok {
			names[el.id] = fqn
		}
	}
	return names
}
