package model

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DocumentDefinition is one native document definition the workspace holds: a
// part def specializing DocumentQueries::Document, and the file declaring it.
type DocumentDefinition struct {
	FQN string
	Doc string
}

// DocumentDefinitions lists the document definitions declared across the
// workspace's own documents, in qualified-name order.
func (w *Workspace) DocumentDefinitions() []DocumentDefinition {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []DocumentDefinition{}
	for name := range w.docs {
		w.queryLocked(name, func(_ *resolve.Resolver, sem *semantics.Model) {
			walkScope(w.index.DocumentRoot(name), func(sym *symbols.Symbol) {
				if docplan.IsDocumentDefinition(w.index, sem, sym) {
					out = append(out, DocumentDefinition{FQN: notationFQN(w.index, sym), Doc: name})
				}
			})
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FQN < out[j].FQN })
	return out
}

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the workspace model, and renders the result as Markdown,
// linking the other documents by the files a Markdown set writes them to.
func (w *Workspace) RenderDocumentMarkdown(fqn string, opts docrender.MarkdownOptions) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	matches := symbols.PreferDeclared(w.index.LookupQualified(fqn))
	if len(matches) == 0 {
		return "", fmt.Errorf("no element named %s", fqn)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%s names %d elements; rename one so the name is unambiguous", fqn, len(matches))
	}
	sym := matches[0]
	var out string
	var err error
	w.queryLocked(sym.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
		if !docplan.IsDocumentDefinition(w.index, sem, sym) {
			err = fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", fqn)
			return
		}
		var plan *docplan.Plan
		if plan, err = docplan.Compile(w.index, sem, resolver, sym); err != nil {
			return
		}
		if opts.Files, err = DocumentFiles(DocumentNames(w.index, sem), ".md"); err != nil {
			return
		}
		var document *docir.Document
		document, err = docir.EvaluateLinked(plan,
			SiblingDocumentPlans(w.index, sem, resolver, sym),
			queryexec.Context{Index: w.index, Resolver: resolver, Model: sem},
			queryexec.Options{}, w.sourceText())
		if err != nil {
			return
		}
		out, err = docrender.Markdown(document, opts)
	})
	return out, err
}

// QueryBindingParameter resolves the parameter a document query binding names:
// for an `in` member of a calc usage typed by a query definition, the matching
// parameter declaration on that definition.
func (w *Workspace) QueryBindingParameter(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym == nil || sym.OwnerScope == nil {
		return nil, false
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Direction != ast.DirIn {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out *symbols.Symbol
	w.queryLocked(sym.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
		target := docplan.QueryTarget(w.index, sem, resolver, sym.OwnerScope.Owner())
		if target == nil {
			return
		}
		for _, member := range w.memberSymbolsLocked(resolver, sem, sym.OwnerScope, target) {
			if member == nil || member.Name != sym.Name {
				continue
			}
			if md, ok := member.Decl.(*ast.Usage); ok && md.Direction == ast.DirIn {
				out = member
				return
			}
		}
	})
	return out, out != nil
}

// QueryUsageParameters lists the `in` parameters of the query definition a calc
// usage is typed by; false when the usage is not typed by one.
func (w *Workspace) QueryUsageParameters(usage *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if usage == nil {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []*symbols.Symbol
	typed := false
	w.queryLocked(usage.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
		target := docplan.QueryTarget(w.index, sem, resolver, usage)
		if target == nil {
			return
		}
		typed = true
		scope := usage.OwnerScope
		if usage.Scope != nil {
			scope = usage.Scope
		}
		for _, member := range w.memberSymbolsLocked(resolver, sem, scope, target) {
			if member == nil {
				continue
			}
			if md, ok := member.Decl.(*ast.Usage); ok && md.Direction == ast.DirIn {
				out = append(out, member)
			}
		}
	})
	return out, typed
}

// IsDocumentDefinition reports whether sym is a native document definition: a
// part def specializing DocumentQueries::Document.
func (w *Workspace) IsDocumentDefinition(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out bool
	w.queryLocked(sym.DocName, func(_ *resolve.Resolver, sem *semantics.Model) {
		out = docplan.IsDocumentDefinition(w.index, sem, sym)
	})
	return out
}

// QueryTypeCandidate pairs a visible spelling with the element it reaches, so
// an alias completes under the name it is reachable by, not its target's.
type QueryTypeCandidate struct {
	Name string
	Sym  *symbols.Symbol
}

// QueryTypeCandidates lists what a calc usage's type position may name: the
// query definitions reachable by a single name from scope — imports, aliases
// and inheritance included — and the namespaces a qualified name may start with.
func (w *Workspace) QueryTypeCandidates(scope *symbols.Scope) []QueryTypeCandidate {
	var out []QueryTypeCandidate
	seen := map[string]bool{}
	for _, vn := range w.VisibleNames(scope, VisibleNamesOptions{MaxDepth: 1}) {
		if seen[vn.Name] {
			continue
		}
		matches := symbols.PreferDeclared(w.LookupQualified(vn.FQN))
		if len(matches) == 0 || matches[0] == nil {
			continue
		}
		sym := matches[0]
		switch {
		case sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace,
			len(w.QueryDefinitions([]*symbols.Symbol{sym})) > 0:
			seen[vn.Name] = true
			out = append(out, QueryTypeCandidate{Name: vn.Name, Sym: sym})
		}
	}
	return out
}

// QueryDefinitions filters syms to the query definitions among them: the calc
// defs specializing DocumentQueries::Query.
func (w *Workspace) QueryDefinitions(syms []*symbols.Symbol) []*symbols.Symbol {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []*symbols.Symbol
	for _, sym := range syms {
		if sym == nil {
			continue
		}
		w.queryLocked(sym.DocName, func(_ *resolve.Resolver, sem *semantics.Model) {
			if queryplan.IsQueryDefinition(w.index, sem, sym) {
				out = append(out, sym)
			}
		})
	}
	return out
}
