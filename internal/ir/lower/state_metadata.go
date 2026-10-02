package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// pseudostateMetadataFQN maps each pseudostate metadata definition to the kind
// of pseudostate an annotated state usage declares.
var pseudostateMetadataFQN = map[string]ast.PseudostateKind{
	"StateMachines::ChoiceMetadata":         ast.PseudostateChoice,
	"StateMachines::JunctionMetadata":       ast.PseudostateJunction,
	"StateMachines::ShallowHistoryMetadata": ast.PseudostateShallowHistory,
	"StateMachines::DeepHistoryMetadata":    ast.PseudostateDeepHistory,
}

// deferredKeeperMetadataFQN is the metadata definition marking the accept of
// the standard deferred-signal encoding that keeps the signal for the state.
const deferredKeeperMetadataFQN = "MigrationMetadata::DeferredKeeper"

// annotationSymbol resolves the metadata definition an annotation names in the
// annotated usage's own scope, then its member's, alias-tolerantly; nil without
// a resolver or a resolution.
func annotationSymbol(resolver *resolve.Resolver, scope *symbols.Scope, usage *ast.Usage, a semantics.MetadataAnnotation) *symbols.Symbol {
	// The annotated usage owns the annotation, so its own scope reads before
	// the enclosing one, as the semantics tier reads it.
	scopes := []*symbols.Scope{childScope(scope, usage)}
	if scopes[0] != scope {
		scopes = append(scopes, scope)
	}
	return annotationSymbolIn(resolver, scopes, a)
}

// annotationSymbolIn resolves the metadata definition an annotation names in
// the first of scopes that resolves it, alias-tolerantly.
func annotationSymbolIn(resolver *resolve.Resolver, scopes []*symbols.Scope, a semantics.MetadataAnnotation) *symbols.Symbol {
	if resolver == nil || a.Node == nil || a.Node.Type == nil {
		return nil
	}
	for _, s := range scopes {
		sym, ok := resolver.ReadQualified(s, a.Node.Type).Symbol()
		if !ok || sym == nil {
			continue
		}
		if target, ok := resolver.ResolveAliasTarget(sym); ok && target != nil {
			sym = target
		}
		return sym
	}
	return nil
}

// PseudostateMetadata reports the pseudostate kind usage declares by carrying a
// StateMachines pseudostate annotation (`#choice state pick;`), for the member
// scope it was written in. Detection is by resolved annotation type, never by
// the annotation's spelling.
func PseudostateMetadata(resolver *resolve.Resolver, scope *symbols.Scope, usage *ast.Usage) (ast.PseudostateKind, bool) {
	for _, a := range semantics.MetadataAnnotationsOf(usage) {
		sym := annotationSymbol(resolver, scope, usage, a)
		if kind, ok := pseudostateMetadataFQN[symbols.FQNOf(sym)]; ok {
			return kind, true
		}
	}
	return 0, false
}

// IsStateSourceDecl is IsStateSource for a declaration as written: a state
// usage carrying a StateMachines pseudostate annotation is no transition
// source, as the pseudostate it lowers to is not.
func IsStateSourceDecl(resolver *resolve.Resolver, scope *symbols.Scope, source ast.Node) bool {
	if usage, ok := source.(*ast.Usage); ok {
		if _, annotated := PseudostateMetadata(resolver, scope, usage); annotated {
			return false
		}
	}
	return IsStateSource(source)
}

// pseudostateKindOf is PseudostateMetadata through the graph's own resolver.
func (g *StateGraph) pseudostateKindOf(usage *ast.Usage, scope *symbols.Scope) (ast.PseudostateKind, bool) {
	return PseudostateMetadata(g.resolver, scope, usage)
}

// pseudostateFromUsage synthesizes the pseudostate a metadata-annotated state
// usage stands for, keeping the usage's name and span.
func pseudostateFromUsage(usage *ast.Usage, kind ast.PseudostateKind) *ast.PseudostateNode {
	name, _ := ast.EffectiveName(usage)
	keyword := "#" + pseudostateAnnotationKeyword(kind) + " state"
	ps := &ast.PseudostateNode{
		Kind:    kind,
		Name:    name,
		Keyword: keyword,
	}
	ps.NodeSpan = usage.NodeSpan
	return ps
}

// pseudostateAnnotationKeyword spells the annotation of a pseudostate kind, for
// the synthesized node's Keyword.
func pseudostateAnnotationKeyword(kind ast.PseudostateKind) string {
	switch kind {
	case ast.PseudostateJunction:
		return "junction"
	case ast.PseudostateShallowHistory:
		return "shallowHistory"
	case ast.PseudostateDeepHistory:
		return "deepHistory"
	default:
		return "choice"
	}
}

// isDeferredKeeper reports whether an accept node carries a DeferredKeeper
// annotation, read in the scope the node was written in. Detection is by
// resolved annotation type, never by the annotation's spelling.
func isDeferredKeeper(resolver *resolve.Resolver, scope *symbols.Scope, node *ast.Usage) bool {
	for _, a := range semantics.MetadataAnnotationsOf(node) {
		if symbols.FQNOf(annotationSymbol(resolver, scope, node, a)) == deferredKeeperMetadataFQN {
			return true
		}
	}
	return false
}
