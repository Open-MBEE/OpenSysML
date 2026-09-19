package model

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// DeclaredViews returns the views declared in scope and its nested scopes,
// outermost first and in declaration order.
func DeclaredViews(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	walkScope(scope, func(sym *symbols.Symbol) {
		if semantics.IsView(sym) {
			out = append(out, sym)
		}
	})
	return out
}

// DeclaredDocumentDefinitions returns the document definitions declared in
// scope and its nested scopes, outermost first and in declaration order.
func DeclaredDocumentDefinitions(index *symbols.Index, sem *semantics.Model, scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	walkScope(scope, func(sym *symbols.Symbol) {
		if docplan.IsDocumentDefinition(index, sem, sym) {
			out = append(out, sym)
		}
	})
	return out
}

// SiblingDocumentPlans compiles the document definitions the workspace
// documents declare other than entry, skipping any whose plan does not
// compile, so a single-document render can emit the anchors incoming
// cross-document references expect.
func SiblingDocumentPlans(index *symbols.Index, sem *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) []*docplan.Plan {
	var plans []*docplan.Plan
	for _, doc := range index.WorkspaceDocuments() {
		for _, sym := range DeclaredDocumentDefinitions(index, sem, index.DocumentRoot(doc)) {
			if sym == entry {
				continue
			}
			plan, err := docplan.Compile(index, sem, resolver, sym)
			if err != nil {
				continue
			}
			plans = append(plans, plan)
		}
	}
	return plans
}

// TopLevelDeclarations returns root declarations, expanding packages one level.
func TopLevelDeclarations(root *symbols.Scope) []*symbols.Symbol {
	if root == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, sym := range scopeMembers(root) {
		if sym.Kind == symbols.SymbolPackage {
			out = append(out, scopeMembers(sym.Scope)...)
			continue
		}
		out = append(out, sym)
	}
	return out
}

func walkScope(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if scope == nil {
		return
	}
	for _, sym := range scopeMembers(scope) {
		visit(sym)
		walkScope(sym.Scope, visit)
	}
}

func scopeMembers(scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	members := scope.AllMembers()
	out := make([]*symbols.Symbol, 0, len(members))
	// AllMembers indexes a short-named declaration under both names.
	seen := make(map[*symbols.Symbol]bool, len(members))
	for _, sym := range members {
		// Aliases and imports name declarations owned by another scope.
		if sym == nil || seen[sym] || sym.Kind == symbols.SymbolAlias || sym.DocName != scope.DocName() {
			continue
		}
		seen[sym] = true
		out = append(out, sym)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}
