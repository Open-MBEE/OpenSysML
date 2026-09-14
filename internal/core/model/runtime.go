package model

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Runtime is a runtime model over the workspace's documents as they stood when
// it was built, on an index of its own that later edits do not touch.
type Runtime struct {
	model *runtime.Model
	index *symbols.Index
	// versions are the document versions the run was built from, by name.
	versions map[string]int
}

// NewRuntime builds a runtime over the workspace's current documents, on a fresh
// overlay of the library index (a bare index when the workspace has no base).
func (w *Workspace) NewRuntime() (*Runtime, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	idx, err := w.privateIndexLocked()
	if err != nil {
		return nil, err
	}
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	sem.SetArgumentTyper(passes.NewArgumentTyper(resolver, sem))
	sem.SetSourceText(w.sourceTextLocked())
	model := runtime.NewModel(sem, resolver)
	rt := &Runtime{model: model, index: idx, versions: make(map[string]int, len(w.docs))}
	for _, name := range w.sortedDocNamesLocked() {
		d := w.docs[name]
		model.RegisterSource(source.New(name, d.Content))
		model.RegisterScope(idx.DocumentRoot(name))
		rt.versions[name] = d.Version
	}
	return rt, nil
}

// privateIndexLocked indexes the workspace's documents on an index the workspace
// does not write to, over the same library base when there is one.
func (w *Workspace) privateIndexLocked() (*symbols.Index, error) {
	var idx *symbols.Index
	switch {
	case w.libBase != nil:
		idx = symbols.NewOverlay(w.libBase)
	case w.index.Base() != nil:
		idx = symbols.NewOverlay(w.index.Base())
	default:
		idx = symbols.NewIndex()
	}
	for _, name := range w.sortedDocNamesLocked() {
		d := w.docs[name]
		if d.AST == nil {
			return nil, fmt.Errorf("%s: document has no parse tree", name)
		}
		idx.AddDocumentWithKind(name, d.AST, w.index.DocumentKind(name))
	}
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

// Lookup is the runtime's own symbols under the fully qualified name fqn.
func (r *Runtime) Lookup(fqn string) []*symbols.Symbol {
	return r.index.LookupQualified(fqn)
}

// FQN spells sym's fully qualified name as the runtime's index holds it.
func (r *Runtime) FQN(sym *symbols.Symbol) string {
	return r.index.GetFQN(sym)
}

// Text is the source text of sym's declaration as the runtime holds it, "" for
// a symbol declared nowhere the runtime reads.
func (r *Runtime) Text(sym *symbols.Symbol) string {
	if sym == nil || sym.Decl == nil {
		return ""
	}
	return r.model.Text()(sym.DocName, sym.Decl.Span())
}
