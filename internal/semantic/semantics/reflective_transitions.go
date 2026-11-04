package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// transitionParts are the trigger accept (with its payload and that payload's value) and the guard
// a transition owns through TransitionFeatureMemberships but its scope declares no symbol for (SysML v2.1 §8.3.18.9).
type transitionParts struct {
	accepter     *symbols.Symbol
	payload      *symbols.Symbol
	payloadValue *symbols.Symbol
	// receiver is the `via` port's parameter, valued by the FeatureReferenceExpression receiverValue.
	receiver      *symbols.Symbol
	receiverValue *symbols.Symbol
	guard         *symbols.Symbol
}

// acceptParameters are the payload and receiver parameters of the trigger accept sym, in that
// order; nil for any other symbol (SysML v2.1 §8.3.17.2 AcceptActionUsage).
func (m *Model) acceptParameters(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionTriggerAction {
		return nil
	}
	parts := m.acceptParts(sym)
	var out []*symbols.Symbol
	for _, param := range []*symbols.Symbol{parts.payload, parts.receiver} {
		if param != nil {
			out = append(out, param)
		}
	}
	return out
}

// parameterValue is the value of the accept's payload or receiver parameter param.
func (p *transitionParts) parameterValue(param *symbols.Symbol) *symbols.Symbol {
	if param == p.receiver {
		return p.receiverValue
	}
	return p.payloadValue
}

// transitionPartsOf are the parts of the transition sym, synthesized once per model so every
// read answers the same identities; nil for any other symbol.
func (m *Model) transitionPartsOf(sym *symbols.Symbol) *transitionParts {
	trans, ok := transitionDecl(sym)
	if !ok || m == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if parts, ok := m.transitionParts[sym]; ok {
		return parts
	}
	if m.transitionParts == nil {
		m.transitionParts = make(map[*symbols.Symbol]*transitionParts)
	}
	journal(m, m.transitionParts, sym, sym.Decl)
	parts := &transitionParts{}
	if payload, event := triggerPayload(trans); payload != nil || event != nil {
		if payload != nil && payload.Value != nil {
			event = payload.Value
		}
		parts.accepter = transitionPart(sym, trans.Trigger, trans.TriggerSpan, symbols.SymbolActionUsage, sym, symbols.ExpressionTriggerAction)
		var decl ast.Node = payload
		if payload == nil {
			decl = event
		}
		parts.payload = transitionPart(sym, decl, decl.Span(), symbols.SymbolReferenceUsage, parts.accepter, symbols.ExpressionPayload)
		if event != nil {
			parts.payloadValue = transitionPart(sym, event, event.Span(), symbols.SymbolExpression, parts.payload, symbols.ExpressionValue)
		}
		if trans.Via != nil {
			// Bind the port here too, so the receiver's referent reads where no document walk ran.
			m.resolver.ResolveReference(resolve.Reference{Scope: sym.OwnerScope, QN: trans.Via, Via: true})
			ref := &ast.FeatureReference{Name: trans.Via}
			ref.NodeSpan = trans.Via.Span()
			receiver := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirIn, Value: ref}
			receiver.NodeSpan = trans.Via.Span()
			parts.receiver = transitionPart(sym, receiver, receiver.NodeSpan, symbols.SymbolReferenceUsage, parts.accepter, symbols.ExpressionPayload)
			parts.receiverValue = transitionPart(sym, ref, ref.NodeSpan, symbols.SymbolExpression, parts.receiver, symbols.ExpressionValue)
		}
	}
	if trans.Guard != nil {
		parts.guard = transitionPart(sym, trans.Guard, trans.Guard.Span(), symbols.SymbolExpression, sym, symbols.ExpressionGuard)
	}
	m.transitionParts[sym] = parts
	return parts
}

func transitionPart(trans *symbols.Symbol, decl ast.Node, span source.Span, kind symbols.SymbolKind,
	owner *symbols.Symbol, role symbols.ExpressionRole) *symbols.Symbol {
	return &symbols.Symbol{
		Kind:       kind,
		DocName:    trans.DocName,
		Decl:       decl,
		DeclSpan:   span,
		Visibility: trans.Visibility,
		Expr:       &symbols.ExpressionElement{Owner: owner, Role: role},
	}
}

func transitionDecl(sym *symbols.Symbol) (*ast.TransitionMember, bool) {
	if sym == nil || sym.Implicit != nil || sym.Expr != nil {
		return nil, false
	}
	trans, ok := sym.Decl.(*ast.TransitionMember)
	return trans, ok && !trans.IsSuccession
}

// triggerPayload is a trigger's payload parameter, or the event an unnamed payload's value is
// (SysML.xtext TriggerValuePart), as the conversion writes them; neither for a call or `when <name>`.
func triggerPayload(trans *ast.TransitionMember) (*ast.Usage, ast.Node) {
	switch t := trans.Trigger.(type) {
	case nil, *ast.ErrorNode, *ast.QualifiedName, *ast.FeatureReference, *ast.AcceptEvent, *ast.CallEvent:
		return nil, nil
	case *ast.Usage:
		if t.IsAccept {
			return t, nil
		}
		return nil, nil
	case *ast.TimeEvent, *ast.ChangeEvent:
		return nil, t
	default:
		// The `when <condition>` spelling is the change event of that condition.
		change := &ast.ChangeEvent{Condition: t}
		change.NodeSpan = trans.TriggerSpan
		return nil, change
	}
}

// transitionFeatureKind is the TransitionFeatureKind of the membership through which the
// transition owner owns member, ok false for a member it owns otherwise (SysML v2.1 §8.3.18.8).
func (m *Model) transitionFeatureKind(owner, member *symbols.Symbol) (string, bool) {
	if member != nil && member.Expr != nil && member.Expr.Owner == owner {
		switch member.Expr.Role {
		case symbols.ExpressionTriggerAction:
			return "trigger", true
		case symbols.ExpressionGuard:
			return "guard", true
		}
	}
	if effect := m.transitionEffect(owner); effect != nil && effect == member {
		return "effect", true
	}
	return "", false
}

// transitionEffect is the action a transition's `do` performs, nil where it writes none.
func (m *Model) transitionEffect(sym *symbols.Symbol) *symbols.Symbol {
	trans, ok := transitionDecl(sym)
	if !ok || len(trans.Effect) != 1 || sym.Scope == nil {
		return nil
	}
	decl := memberDecl(trans.Effect[0])
	var effect *symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member != nil && member.Decl == decl {
			effect = member
			return false
		}
		return true
	})
	return effect
}

func memberDecl(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// transitionMembers are the parts a transition owns besides its scope's members.
func (m *Model) transitionMembers(sym *symbols.Symbol) []*symbols.Symbol {
	parts := m.transitionPartsOf(sym)
	if parts == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, part := range []*symbols.Symbol{parts.accepter, parts.guard} {
		if part != nil {
			out = append(out, part)
		}
	}
	return out
}

// withTransitionParts places a transition's trigger accept and guard among its members, in
// source order; members of any other symbol are returned as they are.
func (m *Model) withTransitionParts(sym *symbols.Symbol, members []*symbols.Symbol) []*symbols.Symbol {
	parts := m.transitionMembers(sym)
	if len(parts) == 0 {
		return members
	}
	out := append(append([]*symbols.Symbol{}, members...), parts...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// acceptParts are the parts of the transition whose trigger accept, or its payload, sym is.
func (m *Model) acceptParts(sym *symbols.Symbol) *transitionParts {
	if sym == nil || sym.Expr == nil {
		return nil
	}
	switch sym.Expr.Role {
	case symbols.ExpressionTriggerAction:
		return m.transitionPartsOf(sym.Expr.Owner)
	case symbols.ExpressionPayload:
		return m.acceptParts(sym.Expr.Owner)
	}
	return nil
}

// transitionElements is ReflectiveElements for the features of a transition and of its
// trigger's accept (SysML v2.1 §8.3.18.9 TransitionUsage, §8.3.17.2 AcceptActionUsage).
func (m *Model) transitionElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	if parts := m.transitionPartsOf(sym); parts != nil {
		switch feature {
		case "triggerAction":
			return optionalElement(parts.accepter), true
		case "guardExpression":
			return optionalElement(parts.guard), true
		case "effectAction":
			return optionalElement(m.transitionEffect(sym)), true
		}
		return nil, false
	}
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionTriggerAction {
		return nil, false
	}
	parts := m.acceptParts(sym)
	switch feature {
	case "payloadParameter":
		return optionalElement(parts.payload), true
	case "payloadArgument":
		return optionalElement(parts.payloadValue), true
	case "receiverArgument":
		return optionalElement(parts.receiverValue), true
	}
	return nil, false
}

func optionalElement(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return []*symbols.Symbol{}
	}
	return []*symbols.Symbol{sym}
}

// triggerKind is the TriggerKind a time or change event's TriggerInvocationExpression has.
func triggerKind(node ast.Node) string {
	switch n := node.(type) {
	case *ast.ChangeEvent:
		return "when"
	case *ast.TimeEvent:
		if n.Absolute {
			return "at"
		}
		return "after"
	}
	return ""
}
