package model

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Runtime is a runtime model over the workspace's documents as they stood when
// it was built, on an index of its own that later edits do not touch.
type Runtime struct {
	model    *runtime.Model
	index    *symbols.Index
	resolver *resolve.Resolver
	// versions are the document versions the run was built from, by name.
	versions map[string]int
	// generation is the workspace's when the run was built.
	generation uint64
}

// NewRuntime builds a runtime over the workspace's current documents, on a fresh
// overlay of the library index (a bare index when the workspace has no base).
func (w *Workspace) NewRuntime() (*Runtime, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.newRuntimeLocked()
}

// newRuntimeLocked is NewRuntime under the read lock.
func (w *Workspace) newRuntimeLocked() (*Runtime, error) {
	idx, err := w.privateIndexLocked()
	if err != nil {
		return nil, err
	}
	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	resolver.SetModel(sem)
	sem.SetSourceText(w.heldTextLocked())
	model := runtime.NewModel(sem, resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	rt := &Runtime{model: model, index: idx, resolver: resolver, versions: make(map[string]int, len(w.docs)), generation: w.generation}
	for _, name := range w.sortedDocNamesLocked() {
		d := w.docs[name]
		model.RegisterSource(d.sf)
		model.RegisterScope(idx.DocumentRoot(name))
		rt.versions[name] = d.Version
	}
	return rt, nil
}

// heldTextLocked reads notation from the documents held now, then the library
// files: a runtime outlives the lock, so it keeps the documents it was built from.
func (w *Workspace) heldTextLocked() source.Lookup {
	files := make(map[string]*source.SourceFile, len(w.docs))
	for name, d := range w.docs {
		files[name] = d.sf
	}
	return source.TextOf(files, libs.Text(w.libSource))
}

// privateIndexLocked indexes the workspace's documents on an index the workspace
// does not write to, holding everything else the workspace's index holds; a
// version standing in for a bundled file displaces it there as well.
func (w *Workspace) privateIndexLocked() (*symbols.Index, error) {
	for _, name := range w.sortedDocNamesLocked() {
		if w.docs[name].Recorded() {
			return nil, &symbols.NeedsHydration{Doc: name, Question: "a runtime over the workspace"}
		}
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
// runtimes over one workspace index the same documents the same way.
func (w *Workspace) sortedDocNamesLocked() []string {
	names := make([]string, 0, len(w.docs))
	for name := range w.docs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Model is the runtime model executions are built over with runtime.NewContext.
func (r *Runtime) Model() *runtime.Model { return r.model }

// Generation is the workspace's generation the runtime was built from; the
// workspace still holds the same documents while Workspace.Generation equals it.
func (r *Runtime) Generation() uint64 { return r.generation }

// Version is the version of the named document the runtime was built from, and
// false for a document it does not hold.
func (r *Runtime) Version(doc string) (int, bool) {
	v, ok := r.versions[doc]
	return v, ok
}

// Declared is the element fqn names in doc as the runtime's own index has it, by
// qualified name or among the document's declarations; build executors from it.
func (r *Runtime) Declared(doc, fqn string) *symbols.Symbol {
	return declaredIn(r.index, doc, fqn)
}

// Named is the element fqn names from doc: doc's own, else the one another held
// document declares by qualified name; an error names the documents when several do.
func (r *Runtime) Named(doc, fqn string) (*symbols.Symbol, error) {
	return namedFrom(r.index, func(doc string) bool { _, ok := r.versions[doc]; return ok }, doc, fqn)
}

// FQN spells sym's fully qualified name as the runtime's index holds it.
func (r *Runtime) FQN(sym *symbols.Symbol) string {
	return r.index.GetFQN(sym)
}
