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
	if _, done := w.displaced[name]; done {
		return
	}
	if file, ok := w.library[name]; ok {
		w.displaced[name] = file.record
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
	if file, ok := w.library[library]; ok {
		w.index.AddDocumentWithKind(library, file.root, file.kind)
		w.index.MarkLibraryDocument(library, record)
		w.index.ExpandWildcardImports()
	}
}

// indexed applies standInLocked's rule to idx once it holds sf as root: a library
// version displaces the bundled file, a document that stopped being one restores it.
// The edit's own stand-ins are consulted, as an earlier rewrite may have moved them.
func (e *editIndex) indexed(idx *symbols.Index, sf *source.SourceFile, root *ast.RootNamespace) {
	w := e.w
	name, library := sf.Name(), ""
	if w.namesLibraryRoots(root) {
		resolver, sem := w.resolverOver(idx)
		_, catalog := w.libraryAlone()
		library = catalog.VersionOf(sem, resolver, name)
	}
	if previous := e.standIns[name]; previous != "" && previous != library {
		delete(e.standIns, name)
		e.restore(idx, previous)
	}
	if library == "" {
		return
	}
	if library != name && w.docs[library] == nil {
		idx.RemoveDocument(library)
	}
	e.standIns[name] = library
	idx.MarkLibraryDocument(name, symbols.LibraryDocument{
		Tier:   w.library[library].record.Tier,
		Digest: symbols.TextDigest(sf.Bytes()),
	})
}

// restore re-indexes the bundled file library in idx, marked, unless it is
// already there or another of the edit's documents stands in for it.
func (e *editIndex) restore(idx *symbols.Index, library string) {
	w := e.w
	if idx.DocumentRoot(library) != nil {
		return
	}
	for _, stoodFor := range e.standIns {
		if stoodFor == library {
			return
		}
	}
	if file, ok := w.library[library]; ok {
		idx.AddDocumentWithKind(library, file.root, file.kind)
		idx.MarkLibraryDocument(library, file.record)
		idx.ExpandWildcardImports()
	}
}

// libraryVersionLocked is the library file the indexed doc is a version of, ""
// for the workspace's own file. Caller holds the lock.
func (w *Workspace) libraryVersionLocked(name string, doc *Document) string {
	if !w.namesLibraryRoots(doc.AST) {
		return ""
	}
	resolver, sem := w.resolverOver(w.index)
	_, catalog := w.libraryAlone()
	return catalog.VersionOf(sem, resolver, name)
}

// namesLibraryRoots reports whether every root of the parsed document is a
// package named as a top-level package of the workspace's library: the cheap
// test a document passes before the library's catalog is consulted, or built.
func (w *Workspace) namesLibraryRoots(root *ast.RootNamespace) bool {
	names, ok := identity.RootPackageNames(root)
	if !ok || len(names) == 0 {
		return false
	}
	for _, name := range names {
		if !w.libraryRoots[name] {
			return false
		}
	}
	return true
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
