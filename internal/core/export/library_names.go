package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
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
// one declaring every root, each a package the norm names and ids as the graph
// does. A graph rooted anywhere else, or in several documents, is none.
func libraryDocument(roots []*element) string {
	catalog := identity.LibraryCatalog(libs.NewModelIndex())
	doc := ""
	for _, root := range roots {
		el, ok := catalog.Element(root.elementID)
		if !ok || el.FQN != root.qname || (doc != "" && el.Symbol.DocName != doc) {
			return ""
		}
		doc = el.Symbol.DocName
	}
	return doc
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
