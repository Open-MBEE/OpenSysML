package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// FlowMintsEnds reports whether a flow usage owns two FlowEnds, as the conversion mints them and the pilot
// exports them: a `flow` (no `message`) stating both its `from` and its `to` (SysML.xtext FlowEndMember).
func FlowMintsEnds(usage *ast.Usage) bool {
	return usage != nil && usage.Kind == ast.UsageFlow && usage.Keyword != "message" && usage.FlowEnds != nil &&
		usage.FlowEnds.From != nil && usage.FlowEnds.To != nil
}

// FlowEndMembers are the source and target FlowEnds a flow owns through an EndFeatureMembership each
// (FlowMintsEnds), nameless and named by the Flows::Flow end they redefine.
func (m *Model) FlowEndMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Recorded() || sym.Scope == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !FlowMintsEnds(usage) {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.flowEnds[sym]; ok {
		return cached
	}
	journal(m, m.flowEnds, sym, sym.Decl)
	forms := [2]symbols.ImpliedForm{symbols.ImpliedFlowSourceEnd, symbols.ImpliedFlowTargetEnd}
	out := make([]*symbols.Symbol, len(forms))
	for i, form := range forms {
		out[i] = &symbols.Symbol{
			Kind:       symbols.SymbolConnectorEnd,
			DocName:    sym.DocName,
			DeclSpan:   source.Span{Offset: flowEndNode(usage, i).Span().Offset},
			Visibility: ast.VisibilityDefault,
			OwnerScope: sym.Scope,
			Decl:       &symbols.ImpliedDecl{Of: sym.Decl, Form: form},
		}
	}
	m.flowEnds[sym] = out
	return out
}

// flowEndNode is the `from` (0) or `to` (1) target a flow usage writes.
func flowEndNode(usage *ast.Usage, i int) ast.Node {
	if i == 0 {
		return usage.FlowEnds.From
	}
	return usage.FlowEnds.To
}

// flowEndOwner is the flow whose end sym implies, with the end's position; nil for any other symbol.
func flowEndOwner(sym *symbols.Symbol) (flow *symbols.Symbol, usage *ast.Usage, i int) {
	i, ok := symbols.ImpliedFlowEndIndex(sym)
	if !ok || sym.OwnerScope == nil {
		return nil, nil, 0
	}
	flow = sym.OwnerScope.Owner()
	if flow == nil {
		return nil, nil, 0
	}
	usage, _ = flow.Decl.(*ast.Usage)
	if usage == nil || usage.FlowEnds == nil {
		return nil, nil, 0
	}
	return flow, usage, i
}

// FlowEndFeatureMember is the ReferenceUsage a flow's end owns through a FeatureMembership, redefining the
// last feature the end names (SysML.xtext FlowFeatureMember); nil for any other symbol.
func (m *Model) FlowEndFeatureMember(end *symbols.Symbol) *symbols.Symbol {
	flow, usage, i := flowEndOwner(end)
	if flow == nil {
		return nil
	}
	defer m.own(flow).LeaveDoc()
	if cached, ok := m.flowEndFeatures[end]; ok {
		return cached
	}
	journal(m, m.flowEndFeatures, end, flow.Decl)
	scope := symbols.NewScope(flow.Scope, nil)
	scope.SetOwner(end)
	forms := [2]symbols.ImpliedForm{symbols.ImpliedFlowSourceEndFeature, symbols.ImpliedFlowTargetEndFeature}
	feature := &symbols.Symbol{
		Kind:       symbols.SymbolReferenceUsage,
		DocName:    flow.DocName,
		DeclSpan:   source.Span{Offset: flowEndNode(usage, i).Span().Offset},
		Visibility: ast.VisibilityDefault,
		OwnerScope: scope,
		Decl:       &symbols.ImpliedDecl{Of: flow.Decl, Form: forms[i]},
	}
	m.flowEndFeatures[end] = feature
	return feature
}

// flowEndPath is the features the end at position i of flow names, outermost first (`src.op` resolves to
// [src op]); nil when one of them resolves to nothing.
func (m *Model) flowEndPath(flow *symbols.Symbol, usage *ast.Usage, i int) []*symbols.Symbol {
	return m.attachmentPath(flow.OwnerScope, ast.EndTarget(flowEndNode(usage, i)))
}

// flowEndReference is the feature a flow's end reference-subsets: the one feature before the last it names,
// or the chain Feature for more (flowEndChainFeature); nil, and stated false, for a lone name.
func (m *Model) flowEndReference(end *symbols.Symbol) (target *symbols.Symbol, stated bool) {
	flow, usage, i := flowEndOwner(end)
	if flow == nil {
		return nil, false
	}
	if chain := m.flowEndChainFeature(flow, i); chain != nil {
		return chain, true
	}
	path := m.flowEndPath(flow, usage, i)
	switch {
	case ast.FlowEndChainPrefix(usage, i) != nil:
		// A chain whose prefix resolves to nothing: the subsetting is stated, its target underived.
		return nil, true
	case len(path) == 2:
		return path[0], true
	case len(path) > 2:
		return nil, true
	}
	if _, chained := ast.EndTarget(flowEndNode(usage, i)).(*ast.FeatureChainExpr); chained {
		return nil, true
	}
	return nil, false
}

// flowEndFeatureRedefinition is the feature the ReferenceUsage a flow's end owns redefines: the last feature
// the end names; nil when it resolves to nothing.
func (m *Model) flowEndFeatureRedefinition(feature *symbols.Symbol) *symbols.Symbol {
	i, ok := symbols.ImpliedFlowEndFeatureIndex(feature)
	if !ok || feature.OwnerScope == nil {
		return nil
	}
	end := feature.OwnerScope.Owner()
	flow, usage, _ := flowEndOwner(end)
	if flow == nil {
		return nil
	}
	path := m.flowEndPath(flow, usage, i)
	if len(path) == 0 {
		return nil
	}
	return path[len(path)-1]
}

// flowEndFeatureDirection is the direction of the ReferenceUsage a flow's end owns: that of the feature it
// redefines as the feature the end references inherits it (KerML.xtext FlowFeature, note 1).
func (m *Model) flowEndFeatureDirection(feature *symbols.Symbol) (ast.FeatureDirection, bool) {
	i, ok := symbols.ImpliedFlowEndFeatureIndex(feature)
	if !ok || feature.OwnerScope == nil {
		return ast.DirNone, false
	}
	flow, usage, _ := flowEndOwner(feature.OwnerScope.Owner())
	if flow == nil {
		return ast.DirNone, false
	}
	path := m.flowEndPath(flow, usage, i)
	if len(path) < 2 {
		return ast.DirNone, true
	}
	return m.directionThrough(path[len(path)-2], path[len(path)-1]), true
}

// withFlowEndFeature adds the ReferenceUsage a flow's end owns to the end's members (ownedMember, member,
// ownedFeature); an end declares no body of its own.
func (m *Model) withFlowEndFeature(sym *symbols.Symbol, members []*symbols.Symbol) []*symbols.Symbol {
	feature := m.FlowEndFeatureMember(sym)
	if feature == nil {
		return members
	}
	return append(append([]*symbols.Symbol{}, members...), feature)
}

// flowEndChainReference is the ReferenceSubsetting through which a flow's end owns chain, the chain Feature
// it references (`a.b` of `flow a.b.x to c.y`); nil when the flow owns no end at chain's position.
func (m *Model) flowEndChainReference(chain *symbols.Symbol) *symbols.Symbol {
	ends := m.FlowEndMembers(chain.Chain.Flow)
	if chain.Chain.End < 0 || chain.Chain.End >= len(ends) {
		return nil
	}
	for _, rel := range m.ImplicitRelationships(ends[chain.Chain.End]) {
		if rel.Implicit != nil && rel.Implicit.Form == symbols.ImplicitSuccessionEndReference && rel.Implicit.Member == chain {
			return rel
		}
	}
	return nil
}
