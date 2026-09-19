package model

import (
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Dependency is a declaration a behavior's run reads, in a document the
// workspace holds: named by document and qualified name, with its text.
type Dependency struct {
	Doc  string
	FQN  string
	Text string
}

// Dependencies lists the declarations a run from roots reads: the roots, what
// their text names, what that names in turn — in the runtime's documents alone,
// as no edit reaches a library — sorted by document then name, each listed as
// the nearest declaration its document names (see dependencyWalk.declaration).
func (r *Runtime) Dependencies(roots ...*symbols.Symbol) []Dependency {
	held := func(doc string) bool { _, ok := r.versions[doc]; return ok }
	return newDependencyWalk(r.index, r.resolver, held, r.model.Text()).closure(roots)
}

// DeclarationOf is the dependency sym is listed as: the nearest declaration its
// document names around it; false when the workspace does not hold its document
// or names nothing around it.
func (r *Runtime) DeclarationOf(sym *symbols.Symbol) (Dependency, bool) {
	if sym == nil {
		return Dependency{}, false
	}
	if _, held := r.versions[sym.DocName]; !held {
		return Dependency{}, false
	}
	held := func(doc string) bool { _, ok := r.versions[doc]; return ok }
	dep, _ := newDependencyWalk(r.index, r.resolver, held, r.model.Text()).declaration(sym)
	return dep, dep.FQN != ""
}

// Referenced is the symbols expr's names reach when resolved in scope, as the
// runtime's index has them.
func (r *Runtime) Referenced(scope *symbols.Scope, expr ast.Node) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, ref := range resolve.ExpressionReferences(scope, expr) {
		if sym, ok := r.resolver.ResolveReference(ref); ok {
			out = append(out, sym)
		}
	}
	return out
}

// Dependencies is Runtime.Dependencies over the documents as read.
func (r *Reading) Dependencies(roots ...*symbols.Symbol) []Dependency {
	held := func(doc string) bool { return r.w.docs[doc] != nil }
	resolver, _ := r.w.semanticsLocked()
	return newDependencyWalk(r.w.index, resolver, held, r.TextAt).closure(roots)
}

// dependencyKey identifies a dependency apart from its text.
type dependencyKey struct{ doc, fqn string }

func (dep Dependency) key() dependencyKey { return dependencyKey{dep.Doc, dep.FQN} }

// Name is what a message calls the dependency: its qualified name, or its
// document when it is one.
func (dep Dependency) Name() string {
	if dep.FQN == "" {
		return dep.Doc
	}
	return dep.FQN
}

// dependencyWalk gathers the declarations reachable by reference on one index.
type dependencyWalk struct {
	idx      *symbols.Index
	resolver *resolve.Resolver
	held     func(doc string) bool
	text     func(doc string, span source.Span) string
	// refs are each document's references sorted by offset, gathered on first need.
	refs map[string][]resolve.Reference
}

func newDependencyWalk(idx *symbols.Index, resolver *resolve.Resolver, held func(string) bool, text func(string, source.Span) string) *dependencyWalk {
	return &dependencyWalk{idx: idx, resolver: resolver, held: held, text: text, refs: make(map[string][]resolve.Reference)}
}

func (d *dependencyWalk) closure(roots []*symbols.Symbol) []Dependency {
	seen := make(map[dependencyKey]bool)
	var out []Dependency
	queue := slices.Clone(roots)
	for len(queue) > 0 {
		sym := queue[0]
		queue = queue[1:]
		if sym == nil || !d.held(sym.DocName) {
			continue
		}
		dep, span := d.declaration(sym)
		if seen[dep.key()] {
			continue
		}
		seen[dep.key()] = true
		out = append(out, dep)
		// A document's references resolve as a query owned by it.
		d.resolver.Query(sym.DocName, func() {
			for _, ref := range d.within(sym.DocName, span) {
				if target, ok := d.resolver.ResolveReference(ref); ok {
					queue = append(queue, target)
				}
			}
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Doc != out[j].Doc {
			return out[i].Doc < out[j].Doc
		}
		return out[i].FQN < out[j].FQN
	})
	return out
}

// declaration is the dependency sym is listed as, and the span its text covers:
// sym's nearest enclosing declaration that its document names — one an edit
// can be told apart by — or its whole document when none is.
func (d *dependencyWalk) declaration(sym *symbols.Symbol) (Dependency, source.Span) {
	for cur := sym; cur != nil; cur = enclosingSymbol(cur) {
		if cur.Name == "" || cur.Decl == nil {
			continue
		}
		fqn := d.idx.GetFQN(cur)
		if declaredIn(d.idx, cur.DocName, fqn) != cur {
			continue
		}
		span := cur.Decl.Span()
		return Dependency{Doc: cur.DocName, FQN: fqn, Text: declarationText(d.text, cur.DocName, span)}, span
	}
	span := d.idx.DocumentRoot(sym.DocName).Node().Span()
	return Dependency{Doc: sym.DocName, Text: declarationText(d.text, sym.DocName, span)}, span
}

// enclosingSymbol is the symbol whose declaration sym is written in, nil at a
// document's root.
func enclosingSymbol(sym *symbols.Symbol) *symbols.Symbol {
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if scope.Owner() != nil {
			return scope.Owner()
		}
	}
	return nil
}

// within is doc's references written inside span.
func (d *dependencyWalk) within(doc string, span source.Span) []resolve.Reference {
	refs, ok := d.refs[doc]
	if !ok {
		scope := d.idx.DocumentRoot(doc)
		if root, isRoot := scope.Node().(*ast.RootNamespace); isRoot {
			refs = resolve.RunReferences(root, scope)
		}
		sort.SliceStable(refs, func(i, j int) bool { return refs[i].QN.Span().Offset < refs[j].QN.Span().Offset })
		d.refs[doc] = refs
	}
	from := sort.Search(len(refs), func(i int) bool { return refs[i].QN.Span().Offset >= span.Offset })
	to := sort.Search(len(refs), func(i int) bool { return refs[i].QN.Span().Offset >= span.End() })
	return refs[from:to]
}
