package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// ExprWalker walks declarations and their expressions down to every member list
// they own (a declaration's body, a `{ … }` value's members), each with its scope.
type ExprWalker struct {
	// Body returns the scope a body expression's members and result resolve in.
	Body func(parent *Scope, body *ast.BodyExpr) *Scope
	// Members is called for each member list reached, with its scope.
	Members func(scope *Scope, members []ast.Node)
	// Applied, when set, is called for each body expression written as the argument
	// of an operation (`xs.{…}`, `xs.?{…}`, `xs->f {…}`), with the operation's scope.
	Applied func(scope *Scope, op ast.Node, body *ast.BodyExpr)
}

// buildBodyScopes links the scope each body expression declares its parameters
// into (`c->forAll { in i : Positive; f(i) }`) into the document scope tree.
// Body expressions sit inside expressions, which the declaration builder does
// not walk, so this second pass walks them once the declarations exist.
func buildBodyScopes(scope *Scope, members []ast.Node) {
	ExprWalker{Body: newBodyExprScope, Members: buildBodyScopes}.WalkMembers(scope, members)
}

// WalkMembers walks the declaration each member wraps, in scope.
func (w ExprWalker) WalkMembers(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapMember(m)
		if decl == nil {
			continue
		}
		w.Decl(scope, decl)
	}
}

// BodyExprScope returns the scope holding body's parameters. A body without
// parameters declares nothing, so its result resolves in parent.
func BodyExprScope(parent *Scope, body *ast.BodyExpr) *Scope {
	if parent == nil {
		return nil
	}
	for _, ch := range parent.Children() {
		if ch.Node() == body {
			return ch
		}
	}
	return parent
}

// newBodyExprScope creates and links the scope body's parameters declare into.
func newBodyExprScope(parent *Scope, body *ast.BodyExpr) *Scope {
	if len(body.Params) == 0 && len(body.Members) == 0 {
		return parent
	}
	scope := NewScope(parent, body)
	scope.markBodyLocal()
	for i := range body.Params {
		p := &body.Params[i]
		if p.Name == "" {
			continue
		}
		// The parameter's own span, not the whole body's: the editor renames
		// through NameSpan and jumps to DeclSpan.
		scope.Define(p.Name, &Symbol{
			Name:       p.Name,
			Kind:       SymbolAttributeUsage,
			Decl:       body,
			DeclSpan:   p.Span,
			NameSpan:   p.Span,
			OwnerScope: scope,
		})
	}
	buildMembers(scope, body.Members)
	parent.AddChild(scope)
	return scope
}

// bodyScopeChild returns the child scope declared by decl, or nil.
func bodyScopeChild(scope *Scope, decl ast.Node) *Scope {
	return scope.ChildFor(decl)
}

// nodeBodyScope returns the scope the body of an action node resolves against:
// its own where the builder gave it one, and the enclosing scope otherwise.
func nodeBodyScope(scope *Scope, node ast.Node) *Scope {
	if child := bodyScopeChild(scope, node); child != nil {
		return child
	}
	return scope
}

// Decl walks the expressions a declaration carries and the member lists it
// owns, each in the child scope it resolves against.
func (w ExprWalker) Decl(scope *Scope, decl ast.Node) {
	for _, prefix := range prefixMetadataOf(decl) {
		if prefix != nil && len(prefix.Body) > 0 {
			w.Members(nodeBodyScope(scope, prefix), prefix.Body)
		}
	}
	switch d := decl.(type) {
	case *ast.Package:
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.Namespace:
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.FilterMember:
		w.Expr(scope, d.Condition)
	case *ast.PrefixMetadata:
		w.Members(nodeBodyScope(scope, d), d.Body)
	case *ast.Definition:
		w.relationships(scope, d.Relationships)
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.Usage:
		w.relationships(scope, d.Relationships)
		w.multiplicity(scope, d.Multiplicity)
		if d.CrossFeature != nil {
			w.relationships(scope, d.CrossFeature.Relationships)
			w.multiplicity(scope, d.CrossFeature.Multiplicity)
		}
		// An accept node keeps its trigger in the usage's value.
		if d.IsAccept {
			w.Trigger(scope, d.Value)
		} else {
			w.Expr(scope, d.Value)
		}
		for _, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			w.Expr(scope, end.Target)
			w.Expr(scope, end.Reference)
		}
		if d.FlowEnds != nil {
			w.Expr(scope, d.FlowEnds.From)
			w.Expr(scope, d.FlowEnds.To)
			w.Expr(scope, d.FlowEnds.Payload)
		}
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.SubjectMember:
		w.multiplicity(scope, d.Multiplicity)
		w.Expr(scope, d.BindingExpr)
		w.Members(nodeBodyScope(scope, d), d.Body)
	case *ast.InitialNode:
		w.Expr(scope, d.Guard)
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		w.Members(nodeBodyScope(scope, d), ast.NodeBodyMembers(d))
	case *ast.ConstraintMember:
		w.Expr(scope, d.Expression)
		w.Members(ConstraintBodyScope(scope, d), d.Body)
	case *ast.AssumeMember:
		w.Expr(scope, d.Expression)
		w.multiplicity(scope, d.Multiplicity)
		w.Expr(scope, d.Value)
		w.Members(ConstraintBodyScope(scope, d), d.Body)
	case *ast.RequireMember:
		w.Expr(scope, d.Expression)
		w.multiplicity(scope, d.Multiplicity)
		w.Expr(scope, d.Value)
		w.Members(ConstraintBodyScope(scope, d), d.Body)
	case *ast.EntryMember:
		w.Members(scope, d.Actions)
	case *ast.DoMember:
		w.Members(scope, d.Actions)
	case *ast.ExitMember:
		w.Members(scope, d.Actions)
	case *ast.DeferMember:
		for _, trigger := range d.Triggers {
			w.Trigger(scope, trigger)
		}
	case *ast.StateNode:
		body := nodeBodyScope(scope, d)
		w.Members(body, d.Entry)
		w.Members(body, d.Do)
		w.Members(body, d.Exit)
		w.Members(body, d.Substates)
		for _, region := range d.Regions {
			w.Decl(body, region)
		}
	case *ast.StateRegion:
		w.Members(nodeBodyScope(scope, d), d.States)
	case *ast.TransitionMember:
		w.Trigger(scope, d.Trigger)
		// The parameters a trigger declares are visible to the transition's own
		// guard, effect and body, and nowhere else.
		body := TriggerScope(scope, d)
		w.Expr(body, d.Guard)
		w.Members(body, d.Effect)
		w.Members(body, d.Members)
	case *ast.SendStatement:
		w.Expr(scope, d.Message)
		w.Expr(scope, d.Target)
		w.Expr(scope, d.Receiver)
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.SuccessionEdge:
		w.Members(nodeBodyScope(scope, d), d.Members)
	case *ast.TerminateStatement:
		w.Expr(scope, d.Target)
	case *ast.AssignmentActionNode:
		w.Expr(scope, d.Target)
		w.Expr(scope, d.Value)
	case *ast.ActionExecutionNode:
		w.Expr(scope, d.Expression)
	case *ast.PerformActionNode:
		w.Expr(scope, d.ActionRef)
	case *ast.WhileLoopActionNode:
		body := nodeBodyScope(scope, d)
		w.Expr(scope, d.Collection)
		w.Expr(body, d.Condition)
		w.Expr(body, d.Until)
		w.Members(body, d.Body)
	case *ast.IfActionNode:
		w.Expr(scope, d.Condition)
		for _, branch := range d.Branches() {
			w.Decl(scope, branch)
		}
	case *ast.IfBranchNode:
		// A branch owns the scope its body declares into (see builder.buildDecl).
		w.Members(nodeBodyScope(scope, d), d.Body)
	default:
		// Bare expression members can contain nested body expressions.
		w.Expr(scope, decl)
	}
}

// TriggerScope returns the innermost scope trans's guard and effect resolve
// against: the one holding the parameters its trigger declares when it declares
// any, the transition's own scope when it has one, and parent otherwise
// (builder.buildDecl builds both, with the effect in the innermost).
func TriggerScope(parent *Scope, trans *ast.TransitionMember) *Scope {
	if parent == nil {
		return nil
	}
	scope := parent
	if child := bodyScopeChild(parent, trans); child != nil {
		scope = child
		if trans.Trigger != nil {
			if params := bodyScopeChild(child, trans.Trigger); params != nil {
				scope = params
			}
		}
	}
	return scope
}

// triggerParameterDefiner returns a function defining the parameters trigger
// declares, or nil when it declares none.
func triggerParameterDefiner(trigger ast.Node) func(*Scope) {
	switch t := trigger.(type) {
	case *ast.CallEvent:
		if len(t.Parameters) == 0 {
			return nil
		}
		return func(scope *Scope) {
			for _, param := range t.Parameters {
				scope.Define(param.Text, &Symbol{
					Name:       param.Text,
					Kind:       SymbolAttributeUsage,
					Decl:       t,
					DeclSpan:   param.Span,
					NameSpan:   param.Span,
					OwnerScope: scope,
				})
			}
		}
	case *ast.AcceptEvent:
		return payloadParameterDefiner(t.Payload)
	case *ast.Usage:
		// The parser gives an accept's payload parameter as the usage it was
		// written as (`accept w : Warning`); lowering classifies it. A usage
		// carrying a value carries a time or change event instead, which declares
		// no parameter.
		if t.Value != nil {
			return nil
		}
		return payloadParameterDefiner(t)
	default:
		return nil
	}
}

// payloadParameterDefiner returns a function defining the payload parameter an
// accept names, or nil when it names none. The parameter holds the received
// occurrence and is visible to the transition's guard and effect only.
func payloadParameterDefiner(payload *ast.Usage) func(*Scope) {
	if payload == nil {
		return nil
	}
	name, nameSpan := ast.EffectiveName(payload)
	if name == "" {
		return nil
	}
	return func(scope *Scope) {
		scope.Define(name, &Symbol{
			Name:       name,
			Kind:       SymbolItemUsage,
			Decl:       payload,
			DeclSpan:   payload.Span(),
			NameSpan:   nameSpan,
			OwnerScope: scope,
		})
	}
}

// Trigger walks the expression a trigger carries: only the payload expressions
// of time and change events can hold a body expression.
func (w ExprWalker) Trigger(scope *Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		w.Expr(scope, t.Duration)
	case *ast.ChangeEvent:
		w.Expr(scope, t.Condition)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.AcceptEvent, *ast.CallEvent:
		// Event names, not expressions.
	default:
		w.Expr(scope, trigger)
	}
}

func (w ExprWalker) relationships(scope *Scope, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil {
			w.Expr(scope, rel.Target)
		}
	}
}

func (w ExprWalker) multiplicity(scope *Scope, m *ast.Multiplicity) {
	if m == nil {
		return
	}
	w.Expr(scope, m.Lower)
	w.Expr(scope, m.Upper)
}

// Expr walks an expression subtree, entering every body expression it holds:
// the body's members through Members, its result in the scope Body returns.
func (w ExprWalker) Expr(scope *Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			w.Expr(scope, op)
		}
	case *ast.FeatureChainExpr:
		w.Expr(scope, v.Operand)
	case *ast.IndexExpr:
		w.Expr(scope, v.Operand)
		w.Expr(scope, v.Index)
	case *ast.InvocationExpr:
		w.Expr(scope, v.Operand)
		for _, a := range v.Args {
			w.applied(scope, v, a)
			w.Expr(scope, a)
		}
		for _, na := range v.NamedArgs {
			w.applied(scope, v, na.Value)
			w.Expr(scope, na.Value)
		}
	case *ast.CollectExpr:
		w.Expr(scope, v.Operand)
		w.applied(scope, v, v.Body)
		w.Expr(scope, v.Body)
	case *ast.SelectExpr:
		w.Expr(scope, v.Operand)
		w.applied(scope, v, v.Body)
		w.Expr(scope, v.Body)
	case *ast.ConstructorExpr:
		for _, a := range v.Args {
			w.Expr(scope, a)
		}
		for _, na := range v.NamedArgs {
			w.Expr(scope, na.Value)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			w.Expr(scope, v.Params[i].Value)
		}
		body := w.Body(scope, v)
		w.Members(body, v.Members)
		w.Expr(body, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			w.Expr(scope, el)
		}
	}
}

// applied reports arg to Applied when it is a body expression op is passed.
func (w ExprWalker) applied(scope *Scope, op ast.Node, arg ast.Node) {
	if body, ok := arg.(*ast.BodyExpr); ok && w.Applied != nil {
		w.Applied(scope, op, body)
	}
}
