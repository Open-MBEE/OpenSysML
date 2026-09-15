package model

import (
	"bytes"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// displaceLocked keeps the mark of the bundled library file named name, which the
// workspace document about to be indexed under that name takes the place of.
// Caller holds the write lock.
func (w *Workspace) displaceLocked(name string) {
	if _, done := w.displaced[name]; done || w.libBase == nil {
		return
	}
	if record := w.index.LibraryDocumentOf(name); record.Tier.Library() {
		w.displaced[name] = record
	}
}

// standInLocked puts a version of a bundled library file in that file's place: the bundled
// document leaves the index (a workspace document holding its name stays) and the version carries its tier.
func (w *Workspace) standInLocked(name string, doc *Document) {
	library := w.libraryVersionLocked(name, doc)
	if previous, ok := w.standIns[name]; ok && previous != library {
		w.releaseStandInLocked(name)
	}
	if library == "" {
		return
	}
	if library != name {
		w.displaceLocked(library)
		if w.docs[library] == nil {
			w.index.RemoveDocument(library)
		}
	}
	w.standIns[name] = library
	w.index.MarkLibraryDocument(name, symbols.LibraryDocument{
		Tier:   w.displaced[library].Tier,
		Digest: symbols.TextDigest(doc.Content),
	})
}

// releaseStandInLocked forgets that name stands in for a bundled library file
// and puts the file back where nothing else displaces it. Caller holds the lock.
func (w *Workspace) releaseStandInLocked(name string) {
	library, ok := w.standIns[name]
	if !ok {
		return
	}
	delete(w.standIns, name)
	w.restoreLocked(library)
}

// restoreLocked indexes the bundled library file named library again, under the
// mark it had, once no workspace document stands in for it or holds its name.
func (w *Workspace) restoreLocked(library string) {
	record, ok := w.displaced[library]
	if !ok || w.docs[library] != nil {
		return
	}
	for _, other := range w.standIns {
		if other == library {
			return
		}
	}
	delete(w.displaced, library)
	scope := w.libBase.DocumentRoot(library)
	if scope == nil {
		return
	}
	if root, ok := scope.Node().(*ast.RootNamespace); ok {
		w.index.AddDocument(library, root)
		w.index.MarkLibraryDocument(library, record)
		w.index.ExpandWildcardImports()
	}
}

// standInOverLocked applies standInLocked's rule to idx once it holds sf as root:
// a library version displaces the bundled file, a document that stopped being one restores it.
func (w *Workspace) standInOverLocked(idx *symbols.Index, sf *source.SourceFile, root *ast.RootNamespace) {
	if w.libBase == nil {
		return
	}
	name, library := sf.Name(), ""
	if identity.LibraryCatalog(idx).NamesEveryRoot(root) {
		resolver, sem := w.resolverOver(idx)
		library = identity.LibraryVersion(sem, resolver, name)
	}
	if previous := w.standIns[name]; previous != "" && previous != library {
		w.restoreOverLocked(idx, name, previous)
	}
	if library == "" {
		return
	}
	if library != name && w.docs[library] == nil {
		idx.RemoveDocument(library)
	}
	idx.MarkLibraryDocument(name, symbols.LibraryDocument{
		Tier:   w.libBase.LibraryDocumentOf(library).Tier,
		Digest: symbols.TextDigest(sf.Bytes()),
	})
}

// restoreOverLocked re-indexes the bundled file library in idx, marked, unless
// it is already there or another stand-in for it is.
func (w *Workspace) restoreOverLocked(idx *symbols.Index, name, library string) {
	if idx.DocumentRoot(library) != nil {
		return
	}
	for other, stoodFor := range w.standIns {
		if other != name && stoodFor == library && idx.DocumentRoot(other) != nil {
			return
		}
	}
	scope := w.libBase.DocumentRoot(library)
	if scope == nil {
		return
	}
	if root, ok := scope.Node().(*ast.RootNamespace); ok {
		idx.AddDocument(library, root)
		idx.MarkLibraryDocument(library, w.libBase.LibraryDocumentOf(library))
		idx.ExpandWildcardImports()
	}
}

// libraryVersionLocked is the bundled library document the indexed doc is a
// version of, "" for the workspace's own file; only an index over the bundled
// library has versions to recognise. Caller holds the lock.
func (w *Workspace) libraryVersionLocked(name string, doc *Document) string {
	if w.libBase == nil || !identity.LibraryCatalog(w.index).NamesEveryRoot(doc.AST) {
		return ""
	}
	resolver, sem := w.newResolver()
	return identity.LibraryVersion(sem, resolver, name)
}

// StandsInFor is the bundled library document name is a version of and stands
// in for in the index, "" for the workspace's own file.
func (w *Workspace) StandsInFor(name string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.standIns[name]
}

// libraryDocumentLocked is the named bundled library file parsed from the library
// source and cached; nil without a source or the file. Caller holds the write lock.
func (w *Workspace) libraryDocumentLocked(name string) *Document {
	if doc, ok := w.libDocs[name]; ok {
		return doc
	}
	if w.libSource == nil {
		return nil
	}
	content, err := w.libSource.Read(name)
	if err != nil {
		return nil
	}
	doc := newDocument(name, bytes.Clone(content), 0)
	w.libDocs[name] = doc
	return doc
}
