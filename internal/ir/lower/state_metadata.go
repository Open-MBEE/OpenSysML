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

// deferredEventMetadataFQN is the metadata definition a migration annotates a
// state with for each signal the source state deferred.
const deferredEventMetadataFQN = "MigrationMetadata::DeferredEvent"

// DeferredSignal is one signal a state defers, as a DeferredEvent annotation on
// the state names it: `@MigrationMetadata::DeferredEvent { ref :>> signal : Sig; }`.
type DeferredSignal struct {
	// Type names the signal definition, as the annotation writes it.
	Type *ast.QualifiedName
	// Scope is where Type resolves: the annotated state's body.
	Scope *symbols.Scope
}

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

// recordDeferred records the signals a state's declaration defers through its
// DeferredEvent annotations, read in the state's body scope, where they were written.
func (g *StateGraph) recordDeferred(state *ast.StateNode, scope *symbols.Scope) {
	usage, ok := g.declOf[state].(*ast.Usage)
	if !ok {
		return
	}
	if deferred := deferredSignalsOf(g.resolver, scope, usage); len(deferred) > 0 {
		g.Deferred[state] = deferred
	}
}

// deferredSignalsOf reads the signals a state usage defers: one per DeferredEvent
// annotation whose body types its `signal`, in declaration order. Detection is
// by resolved annotation type, never by the annotation's spelling.
func deferredSignalsOf(resolver *resolve.Resolver, scope *symbols.Scope, usage *ast.Usage) []DeferredSignal {
	var deferred []DeferredSignal
	for _, a := range semantics.MetadataAnnotationsOf(usage) {
		sym := annotationSymbolIn(resolver, []*symbols.Scope{scope}, a)
		if symbols.FQNOf(sym) != deferredEventMetadataFQN {
			continue
		}
		if signal := deferredSignalType(a.Node); signal != nil {
			deferred = append(deferred, DeferredSignal{Type: signal, Scope: scope})
		}
	}
	return deferred
}

// deferredSignalType is the type the annotation body's redefinition of `signal`
// states, nil where the body states none.
func deferredSignalType(node *ast.PrefixMetadata) *ast.QualifiedName {
	for _, member := range node.Body {
		u, ok := unwrapMembership(member).(*ast.Usage)
		if !ok {
			continue
		}
		if name, _ := ast.EffectiveName(u); name != "signal" {
			continue
		}
		for _, rel := range u.Relationships {
			if rel == nil || rel.Kind != ast.RelTyping {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				return qn
			}
		}
	}
	return nil
}
