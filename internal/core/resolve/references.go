package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// References walks a document's scope tree and AST, gathering every
// QualifiedName reference with the scope it resolves in. It mirrors
// document.go's traversal: a form handled there but not here is a reference a
// consumer walking the document — the editor's navigation and highlighting —
// cannot find.
func References(root *ast.RootNamespace, rootScope *symbols.Scope) []Reference {
	c := &refCollector{}
	if root != nil && rootScope != nil {
		c.walkMembers(rootScope, root.Members)
	}
	return c.refs
}

type refCollector struct {
	refs []Reference
	// condition is set while the names of an element-filter condition are walked.
	condition bool
	// member is the declaration whose text is being walked.
	member ast.Node
	// head is set while a head relationship of a declaration with a scope of
	// its own is walked (Reference.Head).
	head *HeadRelationship
}

// push records a reference, marking it as a filter condition's own name when one
// is being walked: those names resolve unfiltered, as in resolve/document.go.
func (c *refCollector) push(ref Reference) {
	ref.Condition = c.condition
	ref.Member = c.member
	ref.Head = c.head
	// The outermost chain member is the name decided on.
	if ref.Head != nil && ref.Head.Member == ref.QN {
		ref.Head = &HeadRelationship{Scope: ref.Head.Scope, Kind: ref.Head.Kind}
	}
	c.refs = append(c.refs, ref)
}

// conditionExpr walks the names of a filter condition.
func (c *refCollector) conditionExpr(scope *symbols.Scope, e ast.Node) {
	prev := c.condition
	c.condition = true
	defer func() { c.condition = prev }()
	c.expr(scope, e)
}

func (c *refCollector) add(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn})
	}
}

// addReference records the target of a reference subsetting owned by decl.
func (c *refCollector) addReference(scope *symbols.Scope, decl ast.Node, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn, Referrer: decl})
	}
}

// constraintCondition records the names a require/assume member's condition
// uses; a lone name is the reference form, recorded as decl's reference target.
func (c *refCollector) constraintCondition(scope *symbols.Scope, decl ast.Node, expr ast.Node) {
	if ref := ast.ConditionReference(decl); ref != nil {
		c.referenceTarget(scope, decl, ref)
		return
	}
	c.expr(scope, expr)
}

// addRedefinition records a redefinition's target, which names a feature of the
// owning scope's generals rather than a member of the scope itself, past the
// name decl itself borrows from it.
func (c *refCollector) addRedefinition(scope *symbols.Scope, decl ast.Node, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn, Referrer: decl, Redefines: true})
	}
}

// addConstructed records a constructor argument's label, which names a feature
// of the instantiated type rather than an element of scope.
func (c *refCollector) addConstructed(scope *symbols.Scope, typ, label *ast.QualifiedName) {
	if typ != nil && label != nil {
		c.push(Reference{Scope: scope, QN: label, Constructed: typ})
	}
}

// addEndpoint records a transition endpoint, which names a vertex of the
// enclosing machine ahead of anything else the name reaches.
func (c *refCollector) addEndpoint(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn, Endpoint: true})
	}
}

// edgeEnd records a succession or control-flow end the author named; one bound
// by position is no reference (see resolveEdgeEnd).
func (c *refCollector) edgeEnd(scope *symbols.Scope, qn *ast.QualifiedName, member ast.Node, implied bool) {
	if qn == nil || len(qn.Parts) == 0 || member != nil || implied {
		return
	}
	if inStateMachine(scope) {
		c.addEndpoint(scope, qn)
		return
	}
	c.add(scope, qn)
}

// addChainMember records the member segments of a feature chain, which name
// members of the operand rather than elements of scope.
func (c *refCollector) addChainMember(scope *symbols.Scope, decl ast.Node, chain *ast.FeatureChainExpr) {
	if chain != nil && chain.Member != nil {
		c.push(Reference{Scope: scope, QN: chain.Member, Referrer: decl, Chain: chain})
	}
}

func (c *refCollector) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, ch := range scope.Children() {
		if ch.Node() == decl {
			return ch
		}
	}
	return nil
}

// bodyScope is the scope the body of an action node resolves against: its own
// where the builder gave it one, and the enclosing scope otherwise.
func (c *refCollector) bodyScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	if child := c.childScope(scope, decl); child != nil {
		return child
	}
	return scope
}

func (c *refCollector) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl := m
		switch v := m.(type) {
		case *ast.Membership:
			decl = v.Member
		}
		c.resolveDecl(scope, decl)
	}
}

func (c *refCollector) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	prev := c.member
	c.member = decl
	defer func() { c.member = prev }()
	switch {
	case c.namespaceDecl(scope, decl):
	case c.typeDecl(scope, decl):
	case c.behaviorDecl(scope, decl):
	default:
		// A bare expression member is the body's result, as in a calc body
		// whose result is its last expression.
		c.expr(scope, decl)
	}
}

// namespaceDecl collects the references of a namespace-level declaration, reporting whether
// decl was one.
func (c *refCollector) namespaceDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Package:
		c.prefixes(scope, d, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Namespace:
		c.prefixes(scope, d, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Import:
		if d.Imported != nil {
			c.push(Reference{Scope: scope, QN: d.Imported, Import: d})
		}
		c.conditionExpr(scope, d.FilterExpr)
		return true
	case *ast.Alias:
		c.add(scope, d.For)
		return true
	case *ast.RelationshipMember:
		c.target(scope, d.Source)
		c.target(scope, d.Target)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Dependency:
		c.prefixes(scope, d, d.Prefixes)
		for _, cl := range d.Clients {
			c.add(scope, cl)
		}
		for _, sp := range d.Suppliers {
			c.add(scope, sp)
		}
		return true
	case *ast.MultiplicityDecl:
		c.multiplicity(scope, d.Range)
		c.add(scope, d.Subsets)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Comment:
		for _, a := range d.About {
			c.add(scope, a)
		}
		return true
	case *ast.PrefixMetadata:
		c.metadataPrefix(scope, scope, d)
		return true
	case *ast.FilterMember:
		c.conditionExpr(scope, d.Condition)
		return true
	default:
		return false
	}
}

// typeDecl collects the references of a definition, usage or constraint member, reporting whether
// decl was one.
func (c *refCollector) typeDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		c.prefixes(scope, d, d.Prefixes)
		child := c.childScope(scope, d)
		c.headerRelationships(scope, child, d, d.Relationships)
		if child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Usage:
		c.prefixes(scope, d, d.Prefixes)
		child := c.childScope(scope, d)
		c.headerRelationships(scope, child, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		if d.CrossFeature != nil {
			c.crossFeature(scope, child, d)
		}
		// An accept node keeps its trigger in the usage's value.
		if d.IsAccept {
			c.trigger(scope, d.Value)
		} else if inv := d.PerformedInvocation(); inv != nil {
			c.invocation(scope, inv, true)
		} else {
			c.expr(scope, d.Value)
		}
		for _, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			// An end that reference-subsets what it attaches to declares its
			// own name, so that name is a declaration, not a reference. A `:>>`
			// on it names an end of the connector's type and so resolves in the
			// connector's scope; everything else resolves in the enclosing one.
			_, declaresName := end.DeclaredName()
			endScope := scope
			if child != nil {
				endScope = child
			}
			redefines, others := ast.SplitRedefinitions(end.Relationships)
			c.relationships(endScope, end, redefines)
			c.relationships(scope, end, others)
			if !declaresName {
				c.target(scope, end.Target)
			}
			c.target(scope, end.Reference)
		}
		if d.FlowEnds != nil {
			c.expr(scope, d.FlowEnds.From)
			c.expr(scope, d.FlowEnds.To)
			// A declared payload (`of name : Type`) names a member of the flow
			// itself, not an element of the enclosing scope.
			payloadScope := scope
			if d.FlowEnds.PayloadDecl != nil && child != nil {
				payloadScope = child
			}
			c.expr(payloadScope, d.FlowEnds.Payload)
		}
		if child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.SubjectMember:
		c.prefixes(scope, d, d.Prefixes)
		c.add(scope, d.TypeRef)
		c.multiplicity(scope, d.Multiplicity)
		c.relationships(scope, d, d.Relationships)
		c.expr(scope, d.BindingExpr)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Body)
		}
		return true
	case *ast.InitialNode:
		// The node's own name is a label, not a reference.
		c.edgeEnd(scope, d.Successor, nil, false)
		c.expr(scope, d.Guard)
		c.walkMembers(c.bodyScope(scope, d), d.Members)
		return true
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walkMembers(c.bodyScope(scope, d), ast.NodeBodyMembers(d))
		return true
	case *ast.ConstraintMember:
		c.expr(scope, d.Expression)
		c.walkMembers(symbols.ConstraintBodyScope(scope, d), d.Body)
		return true
	case *ast.AssumeMember:
		c.prefixes(scope, d, d.Prefixes)
		c.constraintCondition(scope, d, d.Expression)
		c.addReference(scope, d, d.Reference)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.Value)
		c.walkMembers(symbols.ConstraintBodyScope(scope, d), d.Body)
		return true
	case *ast.RequireMember:
		c.prefixes(scope, d, d.Prefixes)
		c.constraintCondition(scope, d, d.Expression)
		c.addReference(scope, d, d.Reference)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.Value)
		c.walkMembers(symbols.ConstraintBodyScope(scope, d), d.Body)
		return true
	default:
		return false
	}
}

// behaviorDecl collects the references of a behavioral member — a state, transition or action node, reporting whether
// decl was one.
func (c *refCollector) behaviorDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.EntryMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.DoMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.ExitMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.DeferMember:
		for _, trigger := range d.Triggers {
			c.trigger(scope, trigger)
		}
		return true
	case *ast.StateNode:
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.walkMembers(body, d.Entry)
		c.walkMembers(body, d.Do)
		c.walkMembers(body, d.Exit)
		c.walkMembers(body, d.Substates)
		for _, region := range d.Regions {
			c.resolveDecl(body, region)
		}
		return true
	case *ast.StateRegion:
		states := scope
		if child := c.childScope(scope, d); child != nil {
			states = child
		}
		c.walkMembers(states, d.States)
		return true
	case *ast.TransitionMember:
		// Ends, trigger and guard scopes as in resolveBehaviorDecl.
		c.addEndpoint(scope, d.Source)
		c.addEndpoint(scope, d.Target)
		c.trigger(scope, d.Trigger)
		c.add(scope, d.Via)
		body := symbols.TriggerScope(scope, d)
		c.expr(body, d.Guard)
		c.walkMembers(body, d.Effect)
		c.walkMembers(body, d.Members)
		return true
	case *ast.InitialNode:
		c.edgeEnd(scope, d.Successor, nil, false)
		c.expr(scope, d.Guard)
		c.walkMembers(c.bodyScope(scope, d), d.Members)
		return true
	case *ast.SuccessionEdge:
		c.edgeEnd(scope, d.Source, d.SourceMember, d.SourceImplied)
		c.edgeEnd(scope, d.Target, d.TargetMember, d.TargetImplied)
		c.walkMembers(c.bodyScope(scope, d), d.Members)
		return true
	case *ast.ControlFlowEdge:
		c.edgeEnd(scope, d.Source, d.SourceMember, d.SourceImplied)
		c.edgeEnd(scope, d.Target, d.TargetMember, d.TargetImplied)
		c.expr(scope, d.Guard)
		return true
	case *ast.SendStatement:
		c.expr(scope, d.Message)
		c.expr(scope, d.Target)
		c.expr(scope, d.Receiver)
		c.walkMembers(c.bodyScope(scope, d), d.Members)
		return true
	case *ast.TerminateStatement:
		c.expr(scope, d.Target)
		return true
	case *ast.AssignmentActionNode:
		c.expr(scope, d.Target)
		c.expr(scope, d.Value)
		return true
	case *ast.ActionExecutionNode:
		c.add(scope, d.ActionRef)
		c.expr(scope, d.Expression)
		return true
	case *ast.PerformActionNode:
		if inv := d.PerformedInvocation(); inv != nil {
			c.invocation(scope, inv, true)
		} else {
			c.expr(scope, d.ActionRef)
		}
		return true
	case *ast.WhileLoopActionNode:
		// A loop owns the scope its body declares into (see symbols.buildDecl).
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.expr(scope, d.Collection)
		c.expr(body, d.Condition)
		c.expr(body, d.Until)
		c.walkMembers(body, d.Body)
		return true
	case *ast.IfActionNode:
		c.expr(scope, d.Condition)
		for _, branch := range d.Branches() {
			c.resolveDecl(scope, branch)
		}
		return true
	case *ast.IfBranchNode:
		// A branch owns the scope its body declares into (see symbols.buildDecl).
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.walkMembers(body, d.Body)
		return true
	default:
		return false
	}
}

// trigger collects the references a transition trigger carries. Bare signal and
// call event names are not model references (see resolve/document.go), so they
// are skipped here too: renaming a declaration must not rewrite them.
func (c *refCollector) trigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		c.expr(scope, t.Duration)
	case *ast.ChangeEvent:
		c.expr(scope, t.Condition)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.AcceptEvent, *ast.CallEvent:
		// Event names, not model elements.
	default:
		c.expr(scope, trigger)
	}
}

// headerRelationships collects the head relationships of decl, whose own scope
// is header, where a target may resolve first (resolveHeaderRelationships).
func (c *refCollector) headerRelationships(scope, header *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	if header == nil || header == scope {
		c.relationships(scope, decl, rels)
		return
	}
	prev := c.head
	defer func() { c.head = prev }()
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		switch target := target.(type) {
		case *ast.QualifiedName:
			c.head = &HeadRelationship{Scope: header, Kind: rel.Kind}
		case *ast.FeatureChainExpr:
			c.head = &HeadRelationship{Scope: header, Kind: rel.Kind, Member: target.Member}
		default:
			c.head = nil
		}
		c.relationships(scope, decl, []*ast.Relationship{rel})
	}
}

// relationships collects the targets of typings, specializations, subsettings
// and redefinitions (`: T`, `:> T`, `:>> T`) owned by decl. A reference
// subsetting records decl as the referrer, so it resolves past decl's own
// binding of the name it references.
func (c *refCollector) relationships(scope *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if ast.IsReferenceSubsetting(decl, rel) {
			c.referenceTarget(scope, decl, rel.Target)
			continue
		}
		// A target parsed as an expression wraps the name it denotes.
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		// A subsetting other than of decl itself reaches a sibling redefinition
		// or resolves as a redefinition does, as in resolveRelationships.
		if qn, ok := target.(*ast.QualifiedName); ok && rel.Kind == ast.RelSubsets {
			c.push(Reference{Scope: scope, Subsetting: decl}.Spelled(qn))
			continue
		}
		if rel.Kind == ast.RelRedefines {
			c.redefinitionTarget(scope, decl, target)
			continue
		}
		c.target(scope, target)
	}
}

// redefinitionTarget collects a redefinition's target; a chain's leading name
// and its member segments are both read as the redefinition rule reads them.
func (c *refCollector) redefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	switch target := target.(type) {
	case *ast.QualifiedName:
		c.addRedefinition(scope, decl, target)
	case *ast.FeatureChainExpr:
		c.redefinitionTarget(scope, decl, target.Operand)
		if target.Member != nil {
			c.push(Reference{Scope: scope, QN: target.Member, Referrer: decl, Chain: target, Redefines: true})
		}
	default:
		c.expr(scope, target)
	}
}

// referenceTarget collects a reference subsetting's target, tagging the leading
// qualified name with the declaration that refers to it.
func (c *refCollector) referenceTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	if qn, ok := target.(*ast.QualifiedName); ok {
		c.addReference(scope, decl, qn)
		return
	}
	if chain, ok := target.(*ast.FeatureChainExpr); ok {
		c.referenceTarget(scope, decl, chain.Operand)
		c.addChainMember(scope, decl, chain)
		return
	}
	c.expr(scope, target)
}

// target collects a node that names something, whether it was parsed as a
// qualified name or wrapped in an expression.
func (c *refCollector) target(scope *symbols.Scope, target ast.Node) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	if qn, ok := target.(*ast.QualifiedName); ok {
		c.add(scope, qn)
		return
	}
	c.expr(scope, target)
}

func (c *refCollector) multiplicity(scope *symbols.Scope, m *ast.Multiplicity) {
	if m == nil {
		return
	}
	c.expr(scope, m.Lower)
	c.expr(scope, m.Upper)
}

// prefixes collects the prefix annotations of decl, a member of scope, in the
// scopes the resolver resolves them in (see resolvePrefixes).
func (c *refCollector) prefixes(scope *symbols.Scope, decl ast.Node, prefixes []*ast.PrefixMetadata) {
	names := c.bodyScope(scope, decl)
	for _, p := range prefixes {
		if p != nil {
			c.metadataPrefix(names, scope, p)
		}
	}
}

// crossFeature collects the references the cross feature an end declares ahead
// of itself writes, as that feature's; they resolve where the end's do.
func (c *refCollector) crossFeature(scope, header *symbols.Scope, u *ast.Usage) {
	prev := c.member
	c.member = u.CrossFeature
	defer func() { c.member = prev }()
	c.headerRelationships(scope, header, u.CrossFeature, u.CrossFeature.Relationships)
	c.multiplicity(scope, u.CrossFeature.Multiplicity)
}

// metadataPrefix collects an annotation's metaclass name and the elements it is
// about in names, and the references its body carries in the body's own scope,
// hung off parent — the same scopes the resolver resolves them in (see
// resolveMetadataPrefix). The annotation is the member its own names are
// written in, prefix or not.
func (c *refCollector) metadataPrefix(names, parent *symbols.Scope, p *ast.PrefixMetadata) {
	prev := c.member
	c.member = p
	defer func() { c.member = prev }()
	c.add(names, p.Type)
	for _, a := range p.About {
		c.add(names, a)
	}
	if len(p.Body) == 0 {
		return
	}
	if body := c.childScope(parent, p); body != nil {
		c.walkMembers(body, p.Body)
	}
}

// invocation collects a call's receiver, name and arguments; performed marks the
// call an action usage runs as its value.
func (c *refCollector) invocation(scope *symbols.Scope, v *ast.InvocationExpr, performed bool) {
	c.expr(scope, v.Operand)
	if v.Type != nil {
		c.push(Reference{Scope: scope, QN: v.Type, Invocation: v, Performed: performed})
	}
	for _, a := range v.Args {
		c.expr(scope, a)
	}
	for _, na := range v.NamedArgs {
		c.add(scope, na.Name)
		c.expr(scope, na.Value)
	}
}

func (c *refCollector) expr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		c.add(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			c.expr(scope, op)
		}
		c.add(scope, v.TypeRef)
	case *ast.FeatureChainExpr:
		c.expr(scope, v.Operand)
		c.addChainMember(scope, nil, v)
	case *ast.IndexExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Index)
	case *ast.InvocationExpr:
		c.invocation(scope, v, false)
	case *ast.CollectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.SelectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.ConstructorExpr:
		c.add(scope, v.Type)
		for _, a := range v.Args {
			c.expr(scope, a)
		}
		for _, na := range v.NamedArgs {
			c.addConstructed(scope, v.Type, na.Name)
			c.expr(scope, na.Value)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			p := &v.Params[i]
			c.add(scope, p.Type)
			c.relationships(scope, v, p.Relationships)
			c.multiplicity(scope, p.Multiplicity)
			c.expr(scope, p.Value)
		}
		// The same scope the resolver uses, so a reference to a parameter or a
		// body declaration and its declaration denote one symbol.
		inner := symbols.BodyExprScope(scope, v)
		c.walkMembers(inner, v.Members)
		c.expr(inner, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			c.expr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		c.add(scope, v.Ref)
	case *ast.CastExpr:
		c.add(scope, v.TargetType)
		c.multiplicity(scope, v.Multiplicity)
	case *ast.QualifiedName:
		// A bare name in expression position parses straight to a qualified
		// name rather than to a FeatureReference wrapper: `return r = speed;`
		// and `return (speed);` reach here, while `return speed + 1;` arrives
		// as an operand of an OperatorExpr.
		c.add(scope, v)
	}
}
