package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// stateMachineMetadataFQN are the qualified names of the StateMachines library
// metadata definitions (`OpenSysML Libraries/StateMachines.sysml`) that spell a
// state machine's pseudostates and deferred events as annotations.
const deferredEventsMetadataFQN = "StateMachines::DeferredMetadata"

// pseudostateMetadataFQN maps each pseudostate metadata definition to the kind
// of pseudostate an annotated state usage declares.
var pseudostateMetadataFQN = map[string]ast.PseudostateKind{
	"StateMachines::ChoiceMetadata":         ast.PseudostateChoice,
	"StateMachines::JunctionMetadata":       ast.PseudostateJunction,
	"StateMachines::ShallowHistoryMetadata": ast.PseudostateShallowHistory,
	"StateMachines::DeepHistoryMetadata":    ast.PseudostateDeepHistory,
}

// annotationSymbol resolves the metadata definition an annotation names, in the
// scope the annotated member was written in. Alias-tolerant: an alias naming the
// definition counts as the definition. nil when no resolver backs the lowering
// or the name resolves to nothing — an unresolved annotation is the
// name-resolution tier's finding, not lowering's.
func annotationSymbol(resolver *resolve.Resolver, scope *symbols.Scope, a semantics.MetadataAnnotation) *symbols.Symbol {
	if resolver == nil || a.Node == nil || a.Node.Type == nil {
		return nil
	}
	sym, ok := resolver.ReadQualified(scope, a.Node.Type).Symbol()
	if !ok || sym == nil {
		return nil
	}
	if target, ok := resolver.ResolveAliasTarget(sym); ok && target != nil {
		sym = target
	}
	return sym
}

// PseudostateMetadata reports the pseudostate kind usage declares by carrying a
// StateMachines pseudostate annotation (`#choice state pick;`), for the member
// scope it was written in. Detection is by resolved annotation type, never by
// the annotation's spelling.
func PseudostateMetadata(resolver *resolve.Resolver, scope *symbols.Scope, usage *ast.Usage) (ast.PseudostateKind, bool) {
	for _, a := range semantics.MetadataAnnotationsOf(usage) {
		sym := annotationSymbol(resolver, scope, a)
		if kind, ok := pseudostateMetadataFQN[symbols.FQNOf(sym)]; ok {
			return kind, true
		}
	}
	return 0, false
}

// DeferredMetadata reports whether usage carries the StateMachines deferred
// annotation (`#deferred ref : Ping;`).
func DeferredMetadata(resolver *resolve.Resolver, scope *symbols.Scope, usage *ast.Usage) bool {
	for _, a := range semantics.MetadataAnnotationsOf(usage) {
		if symbols.FQNOf(annotationSymbol(resolver, scope, a)) == deferredEventsMetadataFQN {
			return true
		}
	}
	return false
}

// pseudostateKindOf is PseudostateMetadata through the graph's own resolver.
func (g *StateGraph) pseudostateKindOf(usage *ast.Usage, scope *symbols.Scope) (ast.PseudostateKind, bool) {
	return PseudostateMetadata(g.resolver, scope, usage)
}

// deferredRefOf reports whether member is a `#deferred` reference usage, for the
// scope it was written in.
func (g *StateGraph) deferredRefOf(usage *ast.Usage, scope *symbols.Scope) bool {
	return DeferredMetadata(g.resolver, scope, usage)
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

// deferredSourceVertex returns the vertex a positional `then` sequences from
// when the member it would bind is a `#deferred` ref: like the `defer` member
// it replaces, the deferred ref is no feature a succession leaves, so the edge
// passes over it to the member before.
func (g *StateGraph) deferredSourceVertex(edge *ast.SuccessionEdge, body transitionBody, scope *symbols.Scope) ast.Node {
	var src ast.Node
	if edge.SourceMember != nil {
		src = unwrapMembership(edge.SourceMember)
	} else if edge.Source != nil {
		d, ok := g.endpoints.Endpoint(scope, edge.Source)
		if !ok {
			return nil
		}
		src = d
	}
	usage, ok := src.(*ast.Usage)
	if !ok || !g.deferredRefOf(usage, scope) {
		return nil
	}
	prev := precedingSuccessionSource(body.members, usage, g, scope)
	if prev == nil {
		return nil
	}
	node, _ := g.findVertex(prev)
	return node
}

// precedingSuccessionSource is the member before marker the succession a
// member-attached `then` binds actually leaves: the nearest preceding member a
// succession sequences from, passing over deferred members the way the parser
// passes over a `defer` member.
func precedingSuccessionSource(members []ast.Node, marker ast.Node, g *StateGraph, scope *symbols.Scope) ast.Node {
	var prev ast.Node
	for _, member := range members {
		actual := unwrapMembership(member)
		if member == marker || actual == marker {
			break
		}
		if !ast.IsSuccessionSource(actual) {
			continue
		}
		if u, ok := actual.(*ast.Usage); ok && g.deferredRefOf(u, scope) {
			continue
		}
		prev = actual
	}
	return prev
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

// deferredTrigger is the trigger a `#deferred ref : <event>;` member retains:
// the accepted occurrence named by the reference's typing, as a call event when
// the typing resolves to an action, else a signal.
func (g *StateGraph) deferredTrigger(usage *ast.Usage, scope *symbols.Scope) ast.Node {
	if qn := typingTarget(usage); qn != nil && g.resolvesToAction(scope, qn) {
		evt := &ast.CallEvent{Operation: qn}
		evt.NodeSpan = usage.NodeSpan
		return evt
	}
	return usage
}

// resolvesToAction reports whether qn names an action definition or usage in
// scope, which a deferred reference defers as a call event rather than a signal.
func (g *StateGraph) resolvesToAction(scope *symbols.Scope, qn *ast.QualifiedName) bool {
	if g.resolver == nil {
		return false
	}
	sym, ok := g.resolver.ReadQualified(scope, qn).Symbol()
	if !ok || sym == nil {
		return false
	}
	if target, ok := g.resolver.ResolveAliasTarget(sym); ok && target != nil {
		sym = target
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefAction
	case *ast.Usage:
		return decl.Kind == ast.UsageAction
	}
	return false
}
