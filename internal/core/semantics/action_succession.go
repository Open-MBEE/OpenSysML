package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ActionSuccession is one Succession an action's body declares or inherits,
// with each end resolved to the member it attaches to. It is the KerML view a
// Feature's sourceConnector/targetConnector give of the successions written as
// `succession s first a then b`, `first a then b`, `then b`, a guarded
// `if`/`else` edge or `succession s first a if g then b` (a transition owning a
// succession), or `succession flow from a.x to b.y` (a flow that is a succession).
type ActionSuccession struct {
	// Decl is the syntax declaring the succession.
	Decl ast.Node
	// Owner is the action whose body declares Decl; nil for a body-local block
	// such as a loop or branch, or for an action queried by body alone.
	Owner *symbols.Symbol
	// Source and Target are the two ends, in connector-end order.
	Source, Target ActionSuccessionEnd
}

// ActionSuccessionEnd is one end of an ActionSuccession.
type ActionSuccessionEnd struct {
	// Node is the declaration the end attaches to; nil when it names none the
	// model declares (an implied `start`/`done`, or an unresolved name).
	Node ast.Node
	// Symbol declares Node when the end reached it by name; nil for an end
	// bound by position, whose Node is a member of the succession's owner.
	Symbol *symbols.Symbol
	// Multiplicity is the multiplicity the end writes, nil when it writes none.
	Multiplicity *ast.Multiplicity
	// Span is the reference the end was resolved from; empty when the end is
	// bound by position or implied, and so rests on no reference.
	Span source.Span
}

// ActionSuccessions returns the successions of an action definition or usage:
// those inherited from what it specializes or is typed by, most general first,
// then its own in declaration order. A redefined inherited succession is masked.
func (m *Model) ActionSuccessions(sym *symbols.Symbol) []ActionSuccession {
	if sym == nil || sym.Decl == nil {
		return nil
	}
	var out []ActionSuccession
	sources := m.MemberSources(sym)
	for i := len(sources) - 1; i >= 0; i-- {
		src := sources[i]
		if src == nil || src.Scope == nil {
			continue
		}
		for _, s := range m.DeclaredSuccessions(src.Scope, src, actionBodyMembers(src)) {
			if named, ok := namedSuccession(src.Scope, s.Decl); ok && !m.HasMember(sym, named) {
				continue
			}
			out = append(out, s)
		}
	}
	return append(out, m.DeclaredSuccessions(sym.Scope, sym, actionBodyMembers(sym))...)
}

// namedSuccession is the symbol scope registers for a `succession s` usage. An
// inherited one is sym's only while MembersOf still lists it: a closer
// declaration of its name, or a redefinition of it, masks it. The shorthand
// forms have no name of their own and are never masked.
func namedSuccession(scope *symbols.Scope, decl ast.Node) (*symbols.Symbol, bool) {
	usage, ok := decl.(*ast.Usage)
	if !ok || usage.Ident.Name == "" {
		return nil, false
	}
	for _, member := range scope.AllMembers() {
		if member.Decl == decl {
			return member, true
		}
	}
	return nil, false
}

// actionBodyMembers is the body of an action definition, usage, or control node.
func actionBodyMembers(sym *symbols.Symbol) []ast.Node {
	if members := declMembers(sym); members != nil {
		return members
	}
	return ast.NodeBodyMembers(sym.Decl)
}

// DeclaredSuccessions returns the successions members declare, in order, with
// end names resolved in scope; owner, which may be nil, is recorded on each.
func (m *Model) DeclaredSuccessions(scope *symbols.Scope, owner *symbols.Symbol, members []ast.Node) []ActionSuccession {
	var out []ActionSuccession
	for _, member := range members {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		switch n := member.(type) {
		case *ast.Usage:
			switch {
			case n.Kind == ast.UsageSuccession && len(n.ConnectorEnds) == 2:
				out = append(out, ActionSuccession{
					Decl:   n,
					Owner:  owner,
					Source: m.connectorEndOf(scope, owner, n.ConnectorEnds[0]),
					Target: m.connectorEndOf(scope, owner, n.ConnectorEnds[1]),
				})
			case n.IsSuccessionFlow() && n.FlowEnds != nil:
				out = append(out, ActionSuccession{
					Decl:   n,
					Owner:  owner,
					Source: m.flowEndOf(scope, owner, n.FlowEnds.From),
					Target: m.flowEndOf(scope, owner, n.FlowEnds.To),
				})
			}
		case *ast.InitialNode:
			if n.Successor == nil {
				continue
			}
			out = append(out, ActionSuccession{
				Decl:   n,
				Owner:  owner,
				Source: m.referenceEnd(scope, owner, n.First),
				Target: m.referenceEnd(scope, owner, n.Successor),
			})
		case *ast.SuccessionEdge:
			out = append(out, ActionSuccession{
				Decl:   n,
				Owner:  owner,
				Source: m.edgeEnd(scope, owner, n.Source, n.SourceMember),
				Target: m.edgeEnd(scope, owner, n.Target, n.TargetMember),
			})
		case *ast.ControlFlowEdge:
			out = append(out, ActionSuccession{
				Decl:   n,
				Owner:  owner,
				Source: m.edgeEnd(scope, owner, n.Source, n.SourceMember),
				Target: m.edgeEnd(scope, owner, n.Target, n.TargetMember),
			})
		case *ast.TransitionMember:
			// A guarded succession (`succession s first a if g then b`) is a
			// transition usage owning a succession from a to b.
			if n.Target == nil {
				continue
			}
			var source ActionSuccessionEnd
			if n.Source != nil {
				source = m.referenceEnd(scope, owner, n.Source)
			}
			out = append(out, ActionSuccession{
				Decl:   n,
				Owner:  owner,
				Source: source,
				Target: m.referenceEnd(scope, owner, n.Target),
			})
		}
	}
	return out
}

func (m *Model) connectorEndOf(scope *symbols.Scope, owner *symbols.Symbol, end *ast.ConnectorEnd) ActionSuccessionEnd {
	if end == nil {
		return ActionSuccessionEnd{}
	}
	e := m.referenceEnd(scope, owner, end.AttachedTarget())
	e.Multiplicity = end.Multiplicity
	return e
}

// flowEndOf is the participant a flow end relates: what its dot notation names
// ahead of the payload feature (`a.x` relates a). An end written without one
// subsets no feature and so relates none (KerML Connector::relatedFeature).
func (m *Model) flowEndOf(scope *symbols.Scope, owner *symbols.Symbol, target ast.Node) ActionSuccessionEnd {
	chain, ok := target.(*ast.FeatureChainExpr)
	if !ok {
		return ActionSuccessionEnd{}
	}
	return m.referenceEnd(scope, owner, chain.Operand)
}

// edgeEnd is an end of an edge member: bound to a neighbouring member by
// position (`then b` after `action a`), or named.
func (m *Model) edgeEnd(scope *symbols.Scope, owner *symbols.Symbol, ref *ast.QualifiedName, member ast.Node) ActionSuccessionEnd {
	if member != nil {
		return ActionSuccessionEnd{Node: member}
	}
	if ref == nil {
		return ActionSuccessionEnd{}
	}
	return m.referenceEnd(scope, owner, ref)
}

func (m *Model) referenceEnd(scope *symbols.Scope, owner *symbols.Symbol, target ast.Node) ActionSuccessionEnd {
	if target == nil {
		return ActionSuccessionEnd{}
	}
	e := ActionSuccessionEnd{Span: target.Span()}
	sym, ok := m.resolver.ResolveTarget(scope, target)
	if !ok {
		return e
	}
	if _, initial := sym.Decl.(*ast.InitialNode); initial {
		// A one-ended `first a;` registers a label under a's name; the end is
		// the node declared under that name.
		sym = m.actionSymbolNamed(sym.OwnerScope, owner, sym.Name)
	}
	if sym != nil {
		e.Node, e.Symbol = sym.Decl, sym
	}
	return e
}

// actionSymbolNamed is the symbol an action declares or inherits under name, as
// a one-ended `first a;` names it: the nearest declaration of that name that is
// not itself an initial-node marker.
func (m *Model) actionSymbolNamed(scope *symbols.Scope, owner *symbols.Symbol, name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	if scope != nil {
		if sym := nonInitialDecl(scope.LookupLocalAll(name)); sym != nil {
			return sym
		}
	}
	if owner == nil {
		return nil
	}
	var candidates []*symbols.Symbol
	for _, member := range m.MembersOf(owner) {
		if member.Name == name || member.ShortName == name {
			candidates = append(candidates, member)
		}
	}
	if sym := nonInitialDecl(candidates); sym != nil {
		return sym
	}
	// A `first j;` marker names j without declaring it, but its label masks the
	// inherited j in MembersOf; look the node up where it is declared.
	for _, src := range m.MemberSources(owner) {
		if src == nil || src.Scope == nil {
			continue
		}
		if sym := nonInitialDecl(src.Scope.LookupLocalAll(name)); sym != nil {
			return sym
		}
	}
	return nil
}

func nonInitialDecl(syms []*symbols.Symbol) *symbols.Symbol {
	for _, sym := range syms {
		if _, initial := sym.Decl.(*ast.InitialNode); !initial && sym.Decl != nil {
			return sym
		}
	}
	return nil
}

// ActionDeclaration reports whether decl declares an ActionDefinition or an
// ActionUsage (SysML v2 8.3.17), including the state, calculation, case,
// transition, and control-node kinds that specialize them.
func ActionDeclaration(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefAction, ast.DefState, ast.DefCalc, ast.DefCase,
			ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageAction, ast.UsageState, ast.UsageCalc, ast.UsageCase,
			ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
			ast.UsageTransition:
			return true
		}
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode,
		*ast.StateNode, *ast.TransitionMember, *ast.SendStatement,
		*ast.WhileLoopActionNode, *ast.IfBranchNode:
		return true
	case *ast.InitialNode:
		// `first a if c then b` is a guarded succession: a transition usage.
		return d.Guard != nil
	}
	return false
}
