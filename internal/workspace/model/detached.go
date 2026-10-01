package model

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Detached is the workspace's documents as they stood when it was taken, on an
// index of its own that later edits do not touch: what a runtime is built over.
type Detached struct {
	index      *symbols.Index
	resolver   *resolve.Resolver
	semantics  *semantics.Model              // passes.NewTypedModel's result type
	names      []string                      // held documents, sorted
	sources    map[string]*source.SourceFile // by document name
	text       source.Lookup
	versions   map[string]int
	generation uint64
}

// Detach takes the workspace's current documents on an index of their own.
func (w *Workspace) Detach() (*Detached, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.detachLocked()
}

// detachLocked takes the workspace's documents under the write lock.
func (w *Workspace) detachLocked() (*Detached, error) {
	idx, err := w.privateIndexLocked()
	if err != nil {
		return nil, err
	}
	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	resolver.SetModel(sem)
	sem.SetSourceText(w.heldTextLocked())
	d := &Detached{
		index:      idx,
		resolver:   resolver,
		semantics:  sem,
		names:      w.sortedDocNamesLocked(),
		sources:    make(map[string]*source.SourceFile, len(w.docs)),
		versions:   make(map[string]int, len(w.docs)),
		generation: w.generation,
	}
	files := make(map[string]*source.SourceFile, len(d.names))
	for _, name := range d.names {
		doc := w.docs[name]
		d.sources[name] = doc.sf
		files[doc.sf.Name()] = doc.sf
		d.versions[name] = doc.Version
	}
	d.text = source.TextOf(files, sem.SourceText())
	return d, nil
}

// Index is the detached model's index.
func (d *Detached) Index() *symbols.Index { return d.index }

// Resolver is the resolver over the detached model's index.
func (d *Detached) Resolver() *resolve.Resolver { return d.resolver }

// Semantics is the typed semantic model over the detached model's index.
func (d *Detached) Semantics() *semantics.Model { return d.semantics }

// Documents are the sorted names of the held documents and must not be modified.
func (d *Detached) Documents() []string { return d.names }

// Source is the held source file for doc, or nil when doc is not held.
func (d *Detached) Source(doc string) *source.SourceFile { return d.sources[doc] }

// heldTextLocked reads notation from the documents held now, then the library
// files: a detached model outlives the lock, so it keeps the documents it was built from.
func (w *Workspace) heldTextLocked() source.Lookup {
	files := make(map[string]*source.SourceFile, len(w.docs))
	for name, d := range w.docs {
		files[name] = d.sf
	}
	return source.TextOf(files, libs.Text(w.libSource))
}

// privateIndexLocked indexes the workspace's documents on an index the workspace
// does not write to, holding everything else the workspace's index holds; a
// version standing in for a bundled file displaces it there as well. A detached
// model is built from trees, never evaluated against a record: recorded documents
// are hydrated first, dependents invalidated as by an edit.
func (w *Workspace) privateIndexLocked() (*symbols.Index, error) {
	w.hydrateAllLocked()
	for _, name := range w.sortedDocNamesLocked() {
		if w.docs[name].AST == nil {
			return nil, fmt.Errorf("%s: document has no parse tree", name)
		}
	}
	idx := w.detachedIndexLocked()
	w.addDocumentsLocked(idx, "")
	idx.ExpandWildcardImports()
	return idx, nil
}

// sortedDocNamesLocked names the workspace's documents in a fixed order, so two
// detached models over one workspace index the same documents the same way.
func (w *Workspace) sortedDocNamesLocked() []string {
	names := make([]string, 0, len(w.docs))
	for name := range w.docs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Generation is the workspace's generation the detached model was built from;
// the workspace still holds the same documents while Workspace.Generation equals it.
func (d *Detached) Generation() uint64 { return d.generation }

// Version is the version of the named document the detached model was built from,
// and false for a document it does not hold.
func (d *Detached) Version(doc string) (int, bool) {
	v, ok := d.versions[doc]
	return v, ok
}

// Declared is the element fqn names in doc as the detached model's own index has it,
// by qualified name or among the document's declarations; build executors from it.
func (d *Detached) Declared(doc, fqn string) *symbols.Symbol {
	return declaredIn(d.index, doc, fqn)
}

// Named is the element fqn names from doc: doc's own, else the one another held
// document declares by qualified name; an error names the documents when several do.
func (d *Detached) Named(doc, fqn string) (*symbols.Symbol, error) {
	return namedFrom(d.index, func(doc string) bool { _, ok := d.versions[doc]; return ok }, doc, fqn)
}

// FQN spells sym's fully qualified name as the detached model's index holds it.
func (d *Detached) FQN(sym *symbols.Symbol) string {
	return d.index.GetFQN(sym)
}
