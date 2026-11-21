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
	// succession is the SuccessionAsUsage the `then` clause owns (SysML.xtext TransitionSuccessionMember).
	succession *symbols.Symbol
	// sourceChain is the chain Feature a dotted `first a.b` denotes, owned through sourceChainMembership
	// as the conversion writes it (transitionSourceMember); nil for a plain name.
	sourceChain           *symbols.Symbol
	sourceChainMembership *symbols.Symbol
	// linkParameters are the `in` ReferenceUsages a `transition` owns through a ParameterMembership
	// each, ahead of its trigger: one, and a second when it declares a trigger (EmptyParameterMember).
	linkParameters []*symbols.Symbol
	// transitionLink is the ReferenceUsage every transition with a target owns last, through a
	// FeatureMembership after its succession, as the pilot exports it.
	transitionLink *symbols.Symbol
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
	branch, isBranch := transitionBranchDecl(sym)
	if m == nil || !ok && !isBranch && !declaresTransition(sym) {
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
	if target := transitionTarget(sym); target != nil {
		parts.withSuccession(sym, target)
	}
	var payload *ast.Usage
	var event ast.Node
	if ok {
		payload, event = triggerPayload(trans)
	}
	// Every form owns one link parameter, a state body's `then x;` and a guarded branch too
	// (SysML.xtext TargetTransitionUsage, targetTransitionHeadMembers).
	for i := 0; i < 1+boolInt(payload != nil || event != nil); i++ {
		// A kind-less `in` reference, typed by Anything as a usage with no kind keyword is (SysML v2 §7.6.2).
		link := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirIn}
		link.NodeSpan = sym.DeclSpan
		parameter := transitionPart(sym, link, link.NodeSpan, symbols.SymbolReferenceUsage, sym, symbols.ExpressionTransitionLinkParameter)
		parameter.Expr.Index = i
		parts.linkParameters = append(parts.linkParameters, parameter)
	}
	if !ok {
		if isBranch {
			if branch.Guard != nil {
				parts.guard = transitionPart(sym, branch.Guard, branch.Guard.Span(), symbols.SymbolExpression, sym, symbols.ExpressionGuard)
			}
			parts.withSuccession(sym, branch.Target)
		}
		m.transitionParts[sym] = parts
		return parts
	}
	if trans.Source != nil && nameHasChain(trans.Source) {
		parts.withSourceChain(sym, trans.Source)
	}
	if payload != nil || event != nil {
		if payload != nil && payload.Value != nil {
			event = payload.Value
		}
		// The trigger's accept is an unnamed composite action of its own (SysML.xtext
		// TriggerActionMember); the trigger node itself is its payload or event.
		accept := &ast.Usage{Kind: ast.UsageAction}
		accept.NodeSpan = trans.TriggerSpan
		parts.accepter = transitionPart(sym, accept, trans.TriggerSpan, symbols.SymbolActionUsage, sym, symbols.ExpressionTriggerAction)
		var decl ast.Node = payload
		if payload == nil {
			decl = event
		}
		if declared := sym.Scope.MemberDeclaring(decl); payload != nil && declared != nil {
			// A named payload is the symbol the transition's scope declares for it, owned by the accept.
			parts.payload = declared
		} else {
			parts.payload = transitionPart(sym, decl, decl.Span(), symbols.SymbolReferenceUsage, parts.accepter, symbols.ExpressionPayload)
		}
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
			parts.receiver.Expr.Index = 1 // the accept's second parameter, so its identity is not the payload's
			parts.receiverValue = transitionPart(sym, ref, ref.NodeSpan, symbols.SymbolExpression, parts.receiver, symbols.ExpressionValue)
		}
	}
	if trans.Guard != nil {
		parts.guard = transitionPart(sym, trans.Guard, trans.Guard.Span(), symbols.SymbolExpression, sym, symbols.ExpressionGuard)
	}
	m.transitionParts[sym] = parts
	return parts
}

// withSuccession synthesizes the succession trans owns for `then target` (SysML.xtext TransitionSuccession), read
// in the transition's declaring scope and owning a scope for its two ends (SuccessionEndMembers); the transitionLink follows.
func (p *transitionParts) withSuccession(trans *symbols.Symbol, target *ast.QualifiedName) {
	if target == nil || p.succession != nil {
		return
	}
	p.succession = transitionPart(trans, trans.Decl, target.Span(), symbols.SymbolSuccessionUsage, trans, symbols.ExpressionTransitionSuccession)
	p.succession.OwnerScope = trans.OwnerScope
	p.succession.Scope = symbols.NewScope(trans.OwnerScope, trans.Decl)
	p.succession.Scope.SetOwner(p.succession)
	link := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true}
	// It spans the transition's end, so it sorts after every member the transition owns.
	link.NodeSpan = source.Span{Offset: trans.Decl.Span().End()}
	p.transitionLink = transitionPart(trans, link, link.NodeSpan, symbols.SymbolReferenceUsage, trans, symbols.ExpressionTransitionLink)
}

// synthesizedSuccessionEnds are the ends of the succession sym owns though its scope declares none: a
// `then` succession's, a body's `first a then b;` and a transition's (SuccessionEndMembers).
func (m *Model) synthesizedSuccessionEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if _, ok := sym.Decl.(*ast.SuccessionEdge); ok && (sym.Expr == nil || sym.Expr.Role != symbols.ExpressionConnectorEnd) {
		return m.SuccessionEndMembers(sym)
	}
	if _, ok := sym.Decl.(*ast.InitialNode); ok && sym.Expr == nil && sym.Implicit == nil {
		return m.SuccessionEndMembers(sym)
	}
	if transitionSuccessionPart(sym) {
		return m.SuccessionEndMembers(sym)
	}
	return nil
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

// transitionBranchDecl is the decision branch sym is synthesized for (`if g then b;`,
// `else b;`), a transition usage that owns a guard and a succession but no trigger.
func transitionBranchDecl(sym *symbols.Symbol) (*ast.ControlFlowEdge, bool) {
	if sym == nil || sym.Implicit != nil || sym.Expr != nil {
		return nil, false
	}
	branch, ok := sym.Decl.(*ast.ControlFlowEdge)
	return branch, ok
}

// transitionDecl is the transition member sym declares, a `succession first a if g then b;` included
// (SysML.xtext TransitionUsage owns the same parts, transitionHeadMembers).
func transitionDecl(sym *symbols.Symbol) (*ast.TransitionMember, bool) {
	if sym == nil || sym.Implicit != nil || sym.Expr != nil || !sym.DeclaresTransition() {
		return nil, false
	}
	trans, ok := sym.Decl.(*ast.TransitionMember)
	return trans, ok
}

// declaresTransition reports whether sym is a TransitionUsage written as a transition member or as a
// state body's `then x;` after a state (SysML.xtext TargetTransitionUsage), not a `succession … if g then`.
func declaresTransition(sym *symbols.Symbol) bool {
	if sym == nil || sym.Implicit != nil || sym.Expr != nil || !sym.DeclaresTransition() {
		return false
	}
	trans, ok := sym.Decl.(*ast.TransitionMember)
	return !ok || !trans.IsSuccession
}

// transitionTarget is the reference a transition's `then` names, nil where it names none.
func transitionTarget(sym *symbols.Symbol) *ast.QualifiedName {
	switch d := sym.Decl.(type) {
	case *ast.TransitionMember:
		return d.Target
	case *ast.SuccessionEdge:
		return d.Target
	}
	return nil
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
	for _, part := range append(append([]*symbols.Symbol{}, parts.linkParameters...), parts.accepter, parts.guard, parts.succession, parts.transitionLink) {
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
	out := make([]*symbols.Symbol, 0, len(members)+len(parts))
	for _, member := range members {
		// A named trigger payload is declared in the transition's scope and owned by the trigger's accept.
		if !m.isTriggerPayload(sym, member) {
			out = append(out, member)
		}
	}
	out = append(out, parts...)
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
		if owner := sym.Expr.Owner; owner != nil && owner.Expr == nil {
			return m.acceptActionParts(owner)
		}
		return m.acceptParts(sym.Expr.Owner)
	}
	return nil
}

// acceptActionParts are the parts of an accept action written with `via p`: its receiver parameter,
// valued by the FeatureReferenceExpression to p, as the conversion writes them (acceptReceiver); nil otherwise.
func (m *Model) acceptActionParts(sym *symbols.Symbol) *transitionParts {
	if m == nil || sym == nil || sym.Expr != nil || sym.Implicit != nil || sym.Recorded() {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAction {
		return nil
	}
	var via *ast.QualifiedName
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelVia {
			via, _ = rel.Target.(*ast.QualifiedName)
		}
	}
	if via == nil {
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
	parts := &transitionParts{accepter: sym}
	ref := &ast.FeatureReference{Name: via}
	ref.NodeSpan = via.Span()
	receiver := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirIn, Value: ref}
	receiver.NodeSpan = via.Span()
	parts.receiver = transitionPart(sym, receiver, receiver.NodeSpan, symbols.SymbolReferenceUsage, sym, symbols.ExpressionPayload)
	parts.receiver.OwnerScope = sym.OwnerScope
	parts.receiverValue = transitionPart(sym, ref, ref.NodeSpan, symbols.SymbolExpression, parts.receiver, symbols.ExpressionValue)
	m.transitionParts[sym] = parts
	return parts
}

// isNamedTriggerPayload reports whether sym is the payload a transition's trigger names (`accept x : T`):
// the symbol the transition's scope declares, owned by the trigger's accept (SysML.xtext PayloadParameterMember).
func isNamedTriggerPayload(sym *symbols.Symbol) bool {
	if sym == nil || sym.Decl == nil || sym.Expr != nil || sym.Implicit != nil || sym.Chain != nil || sym.OwnerScope == nil {
		return false
	}
	trans, ok := transitionDecl(sym.OwnerScope.Owner())
	if !ok {
		return false
	}
	payload, _ := triggerPayload(trans)
	return payload != nil && ast.Node(payload) == sym.Decl
}

// namedTriggerPayloadParts are the parts of the transition whose trigger names sym as its payload;
// nil for any other symbol.
func (m *Model) namedTriggerPayloadParts(sym *symbols.Symbol) *transitionParts {
	if !isNamedTriggerPayload(sym) {
		return nil
	}
	if parts := m.transitionPartsOf(sym.OwnerScope.Owner()); parts != nil && parts.payload == sym {
		return parts
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
		case "succession":
			return optionalElement(parts.succession), true
		case "source":
			return optionalElement(m.transitionSource(sym, parts)), true
		case "target":
			return optionalElement(m.transitionTarget(sym, parts)), true
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

// transitionSource is the ActionUsage the transition's source feature targets: the member of its source
// Membership, or the last feature of a chained source (SysML v2.1 §8.3.18.9 deriveTransitionUsageSource).
func (m *Model) transitionSource(sym *symbols.Symbol, parts *transitionParts) *symbols.Symbol {
	source, ok := m.transitionSourceMember(sym)
	if !ok {
		// A chained source is the first feature the succession relates, already the chain's featureTarget.
		source = m.transitionSuccessionEnd(parts, 0)
	}
	return m.actionUsageOrNil(source)
}

// transitionTarget is the ActionUsage the target feature of the transition's succession targets
// (SysML v2.1 §8.3.18.9 deriveTransitionUsageTarget).
func (m *Model) transitionTarget(sym *symbols.Symbol, parts *transitionParts) *symbols.Symbol {
	target := m.transitionSuccessionEnd(parts, 1)
	if target == nil {
		target = m.transitionTargetEndpoint(sym)
	}
	return m.actionUsageOrNil(target)
}

// transitionTargetEndpoint is the vertex a written target names by endpoint resolution, which reaches one nested
// in a sibling state as transitionSourceMember does for the source.
func (m *Model) transitionTargetEndpoint(sym *symbols.Symbol) *symbols.Symbol {
	if !declaresTransition(sym) || m.resolver == nil {
		return nil
	}
	var written *ast.QualifiedName
	switch d := sym.Decl.(type) {
	case *ast.TransitionMember:
		written = d.Target
	case *ast.SuccessionEdge:
		written = d.Target
	}
	if written == nil || ast.IsFeatureChain(written) {
		return nil
	}
	scope := sym.OwnerScope
	if scope == nil {
		scope = sym.Scope
	}
	target, _ := m.resolver.EndpointSymbol(scope, written)
	return target
}

// transitionSuccessionEnd is the featureTarget of the end at position i of the transition's succession, nil when unresolved.
func (m *Model) transitionSuccessionEnd(parts *transitionParts, i int) *symbols.Symbol {
	if parts.succession == nil {
		return nil
	}
	if targets, _ := m.successionEndFeatureTargets(parts.succession); len(targets) == 2 {
		return targets[i]
	}
	return nil
}

// actionUsageOrNil is sym when it is an ActionUsage, which a transition's source and target must be.
func (m *Model) actionUsageOrNil(sym *symbols.Symbol) *symbols.Symbol {
	if !m.reflectiveMetaclassConforms(sym, "ActionUsage") {
		return nil
	}
	return sym
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

func nameHasChain(name *ast.QualifiedName) bool {
	for _, part := range name.Parts {
		if part.Chained {
			return true
		}
	}
	return false
}

// withSourceChain gives trans the chain Feature its dotted `first a.b` denotes (SysML.xtext
// TransitionSourceMember) and the OwningMembership it owns the chain through (KerML 8.3.3.3.9).
func (p *transitionParts) withSourceChain(trans *symbols.Symbol, source *ast.QualifiedName) {
	node := chainNode(dottedTargetChain(source))
	if node == nil {
		return
	}
	membership := &symbols.Symbol{
		Kind:       symbols.SymbolRelationship,
		DocName:    trans.DocName,
		DeclSpan:   source.Span(),
		Visibility: trans.Visibility,
		Implicit:   &symbols.ImplicitRelationship{Owner: trans, Form: symbols.ImplicitMembership, Slot: "sourcemember"},
	}
	scope := trans.Scope
	if scope == nil {
		scope = symbols.NewScope(trans.OwnerScope, nil)
		scope.SetOwner(trans)
	}
	chain := &symbols.Symbol{
		Kind:       symbols.SymbolReferenceUsage,
		DocName:    trans.DocName,
		Decl:       node,
		DeclSpan:   node.Span(),
		Visibility: ast.VisibilityDefault,
		OwnerScope: scope,
		Chain:      &symbols.ChainingFeature{Relationship: membership, Node: node},
	}
	membership.Implicit.Member = chain
	p.sourceChain, p.sourceChainMembership = chain, membership
}

// dottedTargetChain is the feature chain a dotted transition target (`then a.b`) is
// written as (SysML.xtext TransitionSuccession), nested as the parser shapes one; a
// `::` qualifies the segment it follows (`then p::a.b` chains b from p::a).
func dottedTargetChain(target *ast.QualifiedName) ast.Node {
	if target == nil || !nameHasChain(target) {
		return target
	}
	var node ast.Node
	for start := 0; start < len(target.Parts); {
		end := start + 1
		for end < len(target.Parts) && !target.Parts[end].Chained {
			end++
		}
		last := target.Parts[end-1].Span
		segment := &ast.QualifiedName{Global: target.Global && start == 0, Parts: target.Parts[start:end:end]}
		segment.NodeSpan = source.Span{Offset: target.Parts[start].Span.Offset, Len: last.Offset + last.Len - target.Parts[start].Span.Offset}
		if node == nil {
			node = segment
		} else {
			chain := &ast.FeatureChainExpr{Operand: node, Member: segment}
			chain.NodeSpan = source.Span{Offset: target.Span().Offset, Len: last.Offset + last.Len - target.Span().Offset}
			node = chain
		}
		start = end
	}
	return node
}

// transitionSourceMember is the resolved, unchained source a transition, a guarded `succession first a if g then b`
// or a decision's branch names: the member of the Membership it owns ahead of its parameters (transitionHeadMembers).
func (m *Model) transitionSourceMember(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	_, isTransition := transitionDecl(sym)
	_, isBranch := transitionBranchDecl(sym)
	if m.resolver == nil || !isTransition && !isBranch && !declaresTransition(sym) {
		return nil, false
	}
	scope := sym.OwnerScope
	if scope == nil {
		scope = sym.Scope
	}
	var source, sourceMember ast.Node
	switch d := sym.Decl.(type) {
	case *ast.TransitionMember:
		if d.Source == nil {
			// A sourceless transition leaves the state before it (SysML v2 §7.18.3), which the conversion
			// names through a Membership where that state is a substate member (transitionSourceTerm).
			if before := ast.ImplicitTransitionSource(bodyMembers(m.ownerOf(sym)), d); ast.LeavableByTransition(before) {
				sourceMember = before
				break
			}
			return nil, false
		}
		source = d.Source
	case *ast.SuccessionEdge:
		// A state body's `then x;` leaves the member before it (SysML.xtext TargetTransitionUsage): the
		// state it names, or the nameless member it reaches by position.
		source, sourceMember = d.Source, d.SourceMember
	case *ast.ControlFlowEdge:
		source, sourceMember = d.Source, d.SourceMember
	default:
		return nil, false
	}
	if sourceMember != nil {
		member, ok := m.SymbolDeclaring(scope, sourceMember)
		return member, ok && member != nil
	}
	name, ok := source.(*ast.QualifiedName)
	if !ok || name == nil || ast.IsFeatureChain(name) {
		return nil, false
	}
	// The endpoint resolution, which reaches a vertex nested in a sibling state as the conversion's does.
	member, ok := m.resolver.EndpointSymbol(scope, name)
	return member, ok && member != nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isTriggerPayload reports whether member is the payload parameter the trigger of the transition sym
// names (`accept x : T`), which the trigger's accept owns (SysML.xtext TriggerAction, PayloadParameterMember).
func (m *Model) isTriggerPayload(sym, member *symbols.Symbol) bool {
	parts := m.transitionPartsOf(sym)
	return parts != nil && parts.payload == member
}
