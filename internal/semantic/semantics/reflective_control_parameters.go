package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Qualified names of the features an assignment's target owns (SysML v2 Table 33,
// checkAssignmentActionUsageStartingAtRedefinition and ...AccessedFeatureRedefinition).
const (
	assignmentStartingAtFQN      = "FeatureReferencingPerformances::FeatureAccessPerformance::onOccurrence::startingAt"
	assignmentAccessedFeatureFQN = assignmentStartingAtFQN + "::accessedFeature"
)

// controlParts are the parameters the notation of an assignment, loop, conditional or terminate action
// implies (SysML.xtext AssignmentNode to TerminateNode), in order, with the values the bound ones own.
type controlParts struct {
	params []*symbols.Symbol
	values map[*symbols.Symbol]*symbols.Symbol
	// names are the library features the parameters redefine, each by its name in the action's generals.
	names map[*symbols.Symbol]string
	// startingAt and accessedFeature are the features an assignment's target owns, the
	// second nested in the first; referent is the name of the feature assigned.
	startingAt, accessedFeature *symbols.Symbol
	referent                    *ast.QualifiedName
}

// controlPartsOf are the parts of the control action sym, synthesized once per model so every
// read answers the same identities; nil for any other symbol.
func (m *Model) controlPartsOf(sym *symbols.Symbol) *controlParts {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() || sym.Kind == symbols.SymbolReferenceUsage {
		return nil
	}
	statement := controlStatementOf(sym.Decl)
	if statement == nil {
		statement = terminateStatementOf(sym.Decl)
	}
	accepted := symbols.AcceptPayloadOf(sym.Decl)
	if statement == nil && accepted == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if parts, ok := m.controlParts[sym]; ok {
		return parts
	}
	if m.controlParts == nil {
		m.controlParts = make(map[*symbols.Symbol]*controlParts)
	}
	journal(m, m.controlParts, sym, sym.Decl)
	parts := &controlParts{values: map[*symbols.Symbol]*symbols.Symbol{}, names: map[*symbols.Symbol]string{}}
	m.controlParts[sym] = parts
	operand := func(slot string) *symbols.Symbol {
		for _, candidate := range m.statementOperands(sym) {
			if candidate.Expr.Slot == slot {
				return candidate
			}
		}
		return nil
	}
	// reference adds a ReferenceUsage parameter at span, valued by the expression written at slot.
	reference := func(name string, span source.Span, slot string, value ast.Node) *symbols.Symbol {
		decl := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirIn, Value: value}
		decl.NodeSpan = span
		param := transitionPart(sym, decl, span, symbols.SymbolReferenceUsage, sym, symbols.ExpressionPayload)
		param.Expr.Index = len(parts.params)
		parts.names[param] = name
		parts.params = append(parts.params, param)
		if value != nil {
			m.bindArgumentReference(operandScope(sym), value)
			expr := transitionPart(sym, value, value.Span(), symbols.SymbolExpression, param, symbols.ExpressionValue)
			expr.Expr.Slot = slot
			parts.values[param] = expr
		}
		return param
	}
	add := func(name string, param *symbols.Symbol) {
		if param != nil {
			parts.names[param] = name
			parts.params = append(parts.params, param)
		}
	}
	if accepted != nil {
		m.withAcceptParts(parts, sym, accepted)
		return parts
	}
	switch d := statement.(type) {
	case *ast.AssignmentActionNode:
		// The target is the occurrence the assigned feature is accessed on: the operand of a
		// chain, else the featuring instance, which no expression states.
		var base ast.Node
		span := source.Span{Offset: d.Span().Offset}
		if d.Target != nil {
			span = source.Span{Offset: d.Target.Span().Offset}
		}
		if chain, ok := d.Target.(*ast.FeatureChainExpr); ok {
			base, parts.referent = chain.Operand, chain.Member
		} else {
			_, parts.referent, _ = expressionReference(d.Target)
		}
		target := reference("target", span, "target", base)
		nested := func(owner *symbols.Symbol) *symbols.Symbol {
			decl := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true}
			decl.NodeSpan = span
			return transitionPart(sym, decl, span, symbols.SymbolReferenceUsage, owner, symbols.ExpressionAccess)
		}
		parts.startingAt = nested(target)
		parts.accessedFeature = nested(parts.startingAt)
		if d.Value != nil {
			reference("replacementValues", d.Value.Span(), "value", d.Value)
		}
	case *ast.WhileLoopActionNode:
		body := m.loopBody(sym, d)
		switch d.Kind {
		case ast.LoopFor:
			add("var", m.loopVariable(sym, d))
			if d.Collection != nil {
				reference("seq", d.Collection.Span(), "collection", d.Collection)
			}
			add("body", body)
		case ast.LoopUntil:
			// `loop` tests nothing before its body: an empty whileTest stands ahead of it.
			reference("whileTest", source.Span{Offset: d.Span().Offset}, "", nil)
			add("body", body)
			add("untilTest", operand("untilCondition"))
		default:
			add("whileTest", operand("whileCondition"))
			add("body", body)
			add("untilTest", operand("untilCondition"))
		}
	case *ast.IfActionNode:
		add("ifTest", operand("condition"))
		for i, branch := range m.statementNodes(sym) {
			add([]string{"thenClause", "elseClause"}[min(i, 1)], branch)
		}
	case *ast.TerminateStatement:
		if d.Target != nil {
			reference("terminatedOccurrence", d.Target.Span(), "expression", d.Target)
		}
	}
	return parts
}

// withAcceptParts adds to parts the payload the accept action sym declares and the receiver its
// `via` names (SysML.xtext AcceptNodeDeclaration, PayloadParameterMember and NodeParameterMember).
func (m *Model) withAcceptParts(parts *controlParts, sym *symbols.Symbol, payload *ast.Usage) {
	if sym.Scope != nil {
		if declared := sym.Scope.MemberDeclaring(payload); declared != nil {
			parts.names[declared] = "payload"
			parts.params = append(parts.params, declared)
		}
	}
	if accept := m.acceptActionParts(sym); accept != nil {
		parts.names[accept.receiver] = "receiver"
		parts.params = append(parts.params, accept.receiver)
		parts.values[accept.receiver] = accept.receiverValue
	}
}

// terminateStatementOf is the terminate statement decl is written as, or the one a terminate
// action usage names its occurrence with; nil for any other declaration.
func terminateStatementOf(decl ast.Node) ast.Node {
	switch d := decl.(type) {
	case *ast.TerminateStatement:
		return d
	case *ast.Usage:
		if d.IsTerminate && d.Terminates != nil {
			return d.Terminates
		}
	}
	return nil
}

// operandScope is the scope enclosing the control action sym, where the operands evaluated before its
// body is entered resolve (resolve/document.go): an assignment's, a `for` loop's collection.
func operandScope(sym *symbols.Symbol) *symbols.Scope {
	if _, usage := sym.Decl.(*ast.Usage); usage && sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// statementScope is the scope the expressions of the control action sym are written in.
func (m *Model) statementScope(sym *symbols.Symbol) *symbols.Scope {
	scope := sym.Scope
	if scope == nil {
		return sym.OwnerScope
	}
	if statement := nodeStatementOf(sym.Decl); statement != nil {
		if child := scope.ChildFor(statement); child != nil {
			return child
		}
	}
	return scope
}

// loopVariable is the iteration variable the `for` loop sym declares in its body scope.
func (m *Model) loopVariable(sym *symbols.Symbol, loop *ast.WhileLoopActionNode) *symbols.Symbol {
	scope := m.statementScope(sym)
	if scope == nil {
		return nil
	}
	var variable *symbols.Symbol
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Decl == ast.Node(loop) && IsLoopVariable(member) {
			variable = member
		}
		return variable == nil
	})
	return variable
}

// declaresAcceptPayload reports the payload parameter an accept declares (SysML.xtext PayloadParameter).
func declaresAcceptPayload(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsAccept
}

// controlParameters are the parameters of the control action sym in order; nil for any other symbol.
func (m *Model) controlParameters(sym *symbols.Symbol) []*symbols.Symbol {
	if parts := m.controlPartsOf(sym); parts != nil {
		return parts.params
	}
	return nil
}

// controlPartsOwning are the parts of the control action that owns the synthesized part sym.
func (m *Model) controlPartsOwning(sym *symbols.Symbol) *controlParts {
	for at := sym; at != nil && at.Expr != nil; at = at.Expr.Owner {
		if at.Expr.Role != symbols.ExpressionPayload && at.Expr.Role != symbols.ExpressionAccess {
			return nil
		}
		if at.Expr.Owner != nil && at.Expr.Owner.Expr == nil {
			return m.controlPartsOf(at.Expr.Owner)
		}
	}
	return nil
}

// controlParameterValue is the value of the control action parameter param, nil when it has none.
func (m *Model) controlParameterValue(param *symbols.Symbol) *symbols.Symbol {
	if parts := m.controlPartsOwning(param); parts != nil {
		return parts.values[param]
	}
	return nil
}

// accessFeatures are the features an assignment's target parameter, or its startingAt, owns.
func (m *Model) accessFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	parts := m.controlPartsOwning(sym)
	if parts == nil || parts.startingAt == nil {
		return nil
	}
	switch {
	case parts.startingAt.Expr.Owner == sym:
		return []*symbols.Symbol{parts.startingAt}
	case parts.startingAt == sym:
		return []*symbols.Symbol{parts.accessedFeature}
	}
	return nil
}

// accessRedefinitions are the features an assignment's startingAt and accessedFeature redefine:
// the library's, and for the accessed feature the referent the assignment names (SysML v2 Table 33).
func (m *Model) accessRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionAccess {
		return nil
	}
	parts := m.controlPartsOwning(sym)
	if parts == nil {
		return nil
	}
	var out []*symbols.Symbol
	add := func(target *symbols.Symbol) {
		if target != nil {
			out = append(out, target)
		}
	}
	if sym == parts.startingAt {
		add(m.symbolByFQN(assignmentStartingAtFQN))
		return out
	}
	add(m.symbolByFQN(assignmentAccessedFeatureFQN))
	add(m.referencedSymbol(parts.startingAt.Expr.Owner, parts.referent))
	return out
}

// loopBody is the action usage a loop's body is (SysML.xtext ActionBodyParameter), a parameter
// of the loop sym.
func (m *Model) loopBody(sym *symbols.Symbol, loop *ast.WhileLoopActionNode) *symbols.Symbol {
	if declared := m.declaredLoopBody(sym, loop); declared != nil {
		return declared
	}
	decl := &ast.Usage{Kind: ast.UsageAction, Direction: ast.DirIn, IsBodyParameter: true, Members: loop.Body}
	span := source.Span{Offset: loop.Span().End()}
	if len(loop.Body) > 0 {
		span = source.Span{Offset: loop.Body[0].Span().Offset}
	}
	decl.NodeSpan = span
	body := transitionPart(sym, decl, span, symbols.SymbolActionUsage, sym, symbols.ExpressionPayload)
	body.Expr.Index = 2
	return body
}

// declaredLoopBody is the action usage the loop sym writes as its body (`while c action b { … }`).
func (m *Model) declaredLoopBody(sym *symbols.Symbol, loop *ast.WhileLoopActionNode) *symbols.Symbol {
	scope := m.statementScope(sym)
	if scope == nil {
		return nil
	}
	for _, member := range loop.Body {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if usage, ok := member.(*ast.Usage); ok && usage.IsBodyParameter {
			return scope.MemberDeclaring(usage)
		}
	}
	return nil
}

// isDeclaredBody reports an action usage written as the body of a loop (SysML.xtext ActionBodyParameter).
func isDeclaredBody(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsBodyParameter
}

// nodeParameterOwner is the send or control action whose notation implies the parameter sym; nil for
// any other symbol.
func (m *Model) nodeParameterOwner(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Recorded() {
		return nil
	}
	var owner *symbols.Symbol
	switch {
	case sym.Expr != nil:
		owner = sym.Expr.Owner
	case IsLoopVariable(sym), declaresAcceptPayload(sym), isDeclaredBody(sym):
		owner = m.ownerOf(sym)
	default:
		// A branch is registered with its conditional when synthesized; nothing is forced for any other symbol.
		owner = m.statementOwners[sym]
	}
	if owner == nil || owner.Expr != nil {
		return nil
	}
	if m.nodeParameterName(owner, sym) != "" {
		return owner
	}
	return nil
}

// OwnNodeParameter is this model's parameter where sym is the one another model over the same
// symbols synthesized for a send or control action; sym itself otherwise.
func (m *Model) OwnNodeParameter(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Expr == nil || sym.Expr.Owner == nil || sym.Expr.Owner.Expr != nil {
		return sym
	}
	for _, own := range m.nodeParameters(sym.Expr.Owner) {
		if own.Expr != nil && own.Expr.Role == sym.Expr.Role && own.Kind == sym.Kind && own.DeclSpan == sym.DeclSpan {
			return own
		}
	}
	return sym
}

// sendParameterNames are the features a send action's parameters redefine, in argument order.
var sendParameterNames = [...]string{sendPayload: "payload", sendSender: "sender", sendReceiver: "receiver"}

// nodeParameterName is the name of the library feature the parameter sym of the send or control
// action owner redefines; empty when sym is no such parameter.
func (m *Model) nodeParameterName(owner, sym *symbols.Symbol) string {
	if parts := m.sendPartsOf(owner); parts != nil {
		for i, param := range parts.params {
			if param == sym {
				return sendParameterNames[i]
			}
		}
		return ""
	}
	if parts := m.controlPartsOf(owner); parts != nil {
		return parts.names[sym]
	}
	return ""
}

// controlParameterRedefinitions are the features a node parameter redefines: its role's in each general of the
// action (KerML checkFeatureParameterRedefinition), and SysML Table 33's for an assignment target's features.
func (m *Model) controlParameterRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if access := m.accessRedefinitions(sym); access != nil {
		return access
	}
	owner := m.nodeParameterOwner(sym)
	if owner == nil {
		return nil
	}
	name := m.nodeParameterName(owner, sym)
	generals := slices.Clone(m.DirectSupertypes(owner))
	for _, implied := range m.ImplicitSubsettings(owner) {
		if implied != nil && !slices.Contains(generals, implied) {
			generals = append(generals, implied)
		}
	}
	var out []*symbols.Symbol
	for _, general := range generals {
		// A general another one specializes is redundant (KerML §8.4.2): the parameter of the more specific is the one redefined.
		if slices.ContainsFunc(generals, func(other *symbols.Symbol) bool { return other != general && m.Conforms(other, general) }) {
			continue
		}
		if !behaviorLike(general) {
			continue
		}
		if found, ok := m.LookupMember(general, name); ok && found != sym && !slices.Contains(out, found) {
			out = append(out, found)
		}
	}
	return out
}

// NamedByNodeParameterRole reports whether sym declares no name and takes one from the parameter of
// its action's kind it redefines (KerML 8.3.3.3.4): a name the default conversion does not state.
func (m *Model) NamedByNodeParameterRole(sym *symbols.Symbol) bool {
	return m != nil && sym != nil && !sym.Named() && m.nodeParameterOwner(sym) != nil
}

// IsNodeParameter reports whether sym is a parameter the notation of a send or control action states:
// an element no scope declares, qualified by its action.
func (m *Model) IsNodeParameter(sym *symbols.Symbol) bool {
	return m.nodeParameterOwner(sym) != nil
}
