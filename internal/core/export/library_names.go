package export

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// libraryDocument is the bundled library document a graph is a version of: the
// one declaring every root, each a top-level package the norm names and ids as
// the graph does. A graph rooted anywhere else — in a nested library element,
// or in several documents — is none, and is read beside the library.
func libraryDocument(roots []*element) string {
	var read []identity.LibraryRoot
	for _, root := range roots {
		read = append(read, identity.LibraryRoot{QName: root.qname, ID: root.elementID, Pkg: root.metaclass == "Package"})
	}
	return identity.LibraryCatalog(libs.NewModelIndex()).DocumentRootedAt(read)
}

// documentLibrary is the bundled library document a parsed file is a version of,
// read over the library alone in the language it was parsed as (see identity.LibraryVersion).
func documentLibrary(file *source.SourceFile, root *ast.RootNamespace) string {
	idx := libs.NewModelIndex()
	if !identity.LibraryCatalog(idx).NamesEveryRoot(root) {
		return ""
	}
	name := file.Name()
	idx.AddDocumentWithKind(name, root, file.Kind())
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	return identity.LibraryVersion(model, res, name)
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
	data, err := libs.BundledSource().Read(doc)
	if err != nil {
		return err
	}
	file := source.New(doc, data)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		lines := file.Lines()
		messages := make([]string, 0, len(p.Diagnostics))
		for _, diag := range p.Diagnostics {
			pos := lines.PosAt(diag.Span.Offset)
			messages = append(messages, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, diag.Message))
		}
		return fmt.Errorf("%s: %d syntax error(s):\n  %s", doc, len(messages), strings.Join(messages, "\n  "))
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
