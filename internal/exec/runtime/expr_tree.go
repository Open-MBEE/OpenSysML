package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// invocation is the tree of a call, which the semantic layer is asked about.
func (x lowered) invocation() *ast.InvocationExpr {
	n, _ := x.entry.Tree.(*ast.InvocationExpr)
	return n
}

// operands are a call's arguments as lowered: those bound by position in the
// order written, a receiver first, and then those bound by name.
type operands struct {
	positional, named []*lower.ExprEntry
	labels            []ast.NamedArg
}

// operands are the arguments of the call or construction x.
func (x lowered) operands() operands {
	expr := x.expr()
	first := 0
	if expr.Applied {
		first = 2
	}
	named := len(x.entry.Operands) - len(expr.Named)
	args := operands{positional: x.entry.Operands[first:named], named: x.entry.Operands[named:]}
	switch n := x.entry.Tree.(type) {
	case *ast.InvocationExpr:
		args.labels = n.NamedArgs
	case *ast.ConstructorExpr:
		args.labels = n.NamedArgs
	}
	return args
}

// count is how many arguments are written.
func (o operands) count() int { return len(o.positional) + len(o.named) }

// at is the i-th argument written, those bound by position counted first.
func (o operands) at(i int) lowered {
	if i < len(o.positional) {
		return lowered{entry: o.positional[i]}
	}
	return lowered{entry: o.named[i-len(o.positional)]}
}

// label is the name the j-th named argument is written with.
func (o operands) label(j int) *ast.QualifiedName { return o.labels[j].Name }

// Eval evaluates an expression node, charging a step per evaluation against ctx.maxSteps;
// a traced context records it after its sub-expressions, whose order the trace thus holds.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	return ec.evaluateTree(node, false)
}

// evalHeld is Eval leaving the required members the value ends in unmade, for an
// operation reading only its size or one position (required.go).
func (ec *EvalContext) evalHeld(node ast.Node) (Value, error) {
	return ec.evaluateTree(node, true)
}

// evaluateTree lowers node for this context's scope and evaluates it.
func (ec *EvalContext) evaluateTree(node ast.Node, keep bool) (Value, error) {
	return ec.evaluate(ec.lowered(node), keep)
}

// lowered is node lowered for evaluation here: in this context's scope.
func (ec *EvalContext) lowered(node ast.Node) lowered {
	return ec.ctx.model.loweredExpr(ec.scope, node)
}

// tree is the syntax the expression was lowered from.
func (x lowered) tree() ast.Node { return x.entry.Tree }

// operator is the tree of an operator application, which the semantic layer is asked about.
func (x lowered) operator() *ast.OperatorExpr {
	n, _ := x.entry.Tree.(*ast.OperatorExpr)
	return n
}

// boundParameterNames is the parameter each named argument binds, as callee's signature spells it,
// beside the error where callee has none; without a callee every label is kept as written.
func (ctx *Context) boundParameterNames(scope *symbols.Scope, callee *symbols.Symbol, named []ast.NamedArg) ([]string, []error) {
	names := make([]string, len(named))
	unbound := make([]error, len(named))
	for i, arg := range named {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			continue
		}
		names[i] = arg.Name.Text()
		if callee == nil || ctx.model.semantics == nil {
			continue
		}
		if name, ok := ctx.model.semantics.BoundParameter(scope, callee, arg.Name); ok {
			names[i] = name
		} else {
			unbound[i] = fmt.Errorf("%w: %s has no parameter named %q",
				errUnknownParameter, ctx.QualifiedName(callee), names[i])
		}
	}
	return names, unbound
}

// unresolvedInvocation reports a call to a name denoting nothing with the validator's "did you
// mean" hint: for a qualified name, the member the segment past the deepest resolved may start.
func (ec *EvalContext) unresolvedInvocation(qn *ast.QualifiedName, written string) error {
	if qn == nil || ec.ctx.model.resolver == nil {
		return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
	}
	if len(qn.Parts) == 1 && !qn.Global {
		return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedName(ec.scope, written, qn))
	}
	reading := ec.ctx.readQualified(ec.scope, qn)
	for i := len(qn.Parts) - 2; i >= 0; i-- {
		if owner, ok := reading.Part(i); ok {
			return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedMember(ec.scope, qn, owner, i+1))
		}
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// chainText spells a feature chain as written, `holder.scale`.
func chainText(n ast.Node) string {
	switch c := n.(type) {
	case *ast.FeatureChainExpr:
		return chainText(c.Operand) + "." + qualifiedNameToString(c.Member)
	case *ast.FeatureReference:
		return qualifiedNameToString(c.Name)
	}
	return traceLabel(n)
}

// valueArgument types an evaluated argument for overload selection: a scalar as the literal
// spelling it, an object by every type classifying it, a collection by what its elements share.
func (ec *EvalContext) valueArgument(val Value, name *ast.QualifiedName) semantics.Argument {
	arg := semantics.Argument{Name: name}
	elements := elementsOf(val)
	if len(elements) == 0 {
		arg.Empty = true
		return arg
	}
	if prim := spelledPrim(elements); prim != semantics.PrimUnknown {
		arg.Prim, arg.Exact = prim, true
		return arg
	}
	var common []*symbols.Symbol
	for i, el := range elements {
		types, err := ec.ctx.valueTypes(ec.scope, el)
		if err != nil || len(types) == 0 {
			return arg
		}
		if i == 0 {
			common = types
			continue
		}
		if common = ec.sharedTypes(common, types); len(common) == 0 {
			return arg
		}
	}
	arg.Type, arg.Also = common[0], common[1:]
	arg.Prim = ec.ctx.model.semantics.PrimTypeOf(common[0])
	return arg
}

// qualifiedNameToString converts a QualifiedName AST node to "Package::Name" format.
func qualifiedNameToString(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		if seg.Text != "" {
			parts = append(parts, seg.Text)
		}
	}
	return strings.Join(parts, "::")
}

// subjectName is the qualified name a classification's subject is written as, or
// nil for a subject that is no name.
func subjectName(n ast.Node) *ast.QualifiedName {
	switch subject := n.(type) {
	case *ast.FeatureReference:
		return subject.Name
	case *ast.QualifiedName:
		return subject
	default:
		return nil
	}
}

// isSelfName reports whether a subject is written as `self`, which names the
// object being evaluated — the same subject the notation leaves out.
func isSelfName(n ast.Node) bool {
	qn := subjectName(n)
	return qn != nil && len(qn.Parts) == 1 && qn.Parts[0].Text == "self"
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
// Resolves names from the root scope.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	defer ctx.beginRun()()

	// Use resolver's root scope for name resolution
	// (In a full implementation, this would track evaluation context scope)
	ec := newEvalContext(ctx, nil)
	return ec.Eval(node)
}

// EvalWithScope evaluates an expression with a given scope context for name resolution.
func (ctx *Context) EvalWithScope(node ast.Node, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	ec := newEvalContext(ctx, scope)
	return ec.Eval(node)
}

// EvalArguments evaluates an invocation's whole argument list in one run, so a seeded draw
// continues the one before it; the i-th value is the i-th argument's, or an ArgumentError.
func (ctx *Context) EvalArguments(scope *symbols.Scope, args []ast.Node) ([]Value, error) {
	defer ctx.beginRun()()

	values := make([]Value, len(args))
	for i, arg := range args {
		value, err := newEvalContext(ctx, scope).Eval(arg)
		if err != nil {
			return nil, &ArgumentError{Index: i, Err: err}
		}
		values[i] = value
	}
	return values, nil
}

// EvalWithScopeOn evaluates an expression against a concrete instance, whose feature values the
// names read; it brackets one run, as EvalWithScope does, bounding it by the step budget.
func (ctx *Context) EvalWithScopeOn(node ast.Node, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	return NewEvalContextIn(ctx, scope, self).Eval(node)
}

// name is the label of the k-th argument written, nil for one bound by position,
// and whether the argument is one a parameter can bind: a named one states its label.
func (o operands) name(k int) (*ast.QualifiedName, bool) {
	if k < len(o.positional) {
		return nil, true
	}
	name := o.labels[k-len(o.positional)].Name
	return name, name != nil && len(name.Parts) > 0
}

// operandsIn are arguments written outside an expression, each lowered for evaluation here.
func (ec *EvalContext) operandsIn(args []ast.Node, named []ast.NamedArg) operands {
	lowered := operands{labels: named}
	if len(args) > 0 {
		lowered.positional = make([]*lower.ExprEntry, len(args))
	}
	for i, arg := range args {
		lowered.positional[i] = ec.lowered(arg).entry
	}
	if len(named) > 0 {
		lowered.named = make([]*lower.ExprEntry, len(named))
	}
	for i, arg := range named {
		lowered.named[i] = ec.lowered(arg.Value).entry
	}
	return lowered
}

// appliedChain is the chain a call `x.f(a)` applies, as written.
func (x lowered) appliedChain() *ast.FeatureChainExpr {
	n, _ := x.entry.Operands[0].Tree.(*ast.FeatureChainExpr)
	return n
}

// chain is the base and members of the chain x was written as.
func (x lowered) chain() (ast.Node, []ast.NameSegment) {
	n, _ := x.entry.Tree.(*ast.FeatureChainExpr)
	return ast.FeatureChainBase(n)
}

// chainOfMember is a chain of the one member name.
func chainOfMember(name string) []ast.NameSegment { return []ast.NameSegment{{Text: name}} }

// constructor is the tree of a construction, whose type and labels the model resolves.
func (x lowered) constructor() *ast.ConstructorExpr {
	n, _ := x.entry.Tree.(*ast.ConstructorExpr)
	return n
}

// evalBody is the function a body expression denotes, closed over this evaluation.
func (ec *EvalContext) evalBody(x lowered) Value {
	return newExprValue(x.entry.Tree, ec.closure())
}

// compileNode compiles n written in scope, reading names from the frame slots
// the layout describes; a name outside it is admitted only as a library constant.
func (c *calcCompiler) compileNode(n ast.Node, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	return c.compileExpr(c.ctx.model.loweredExpr(scope, n), scope, layout)
}

// compileName resolves a name as the evaluator does: a frame binding first,
// then what the scope resolves it to, read only where it is a library constant.
func (c *calcCompiler) compileName(qn *ast.QualifiedName, expr *program.Expr, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if qn == nil || len(qn.Parts) == 0 {
		return nil, ineligible("empty feature reference")
	}
	if scope == nil {
		return nil, ineligible("name without a scope")
	}
	if len(qn.Parts) > 1 {
		return c.compileQualifiedName(qn, scope)
	}
	name := qn.Parts[0].Text
	if slot, ok := layout.lookup(name); ok {
		return slotNode(slot), nil
	}
	if name == thatName || name == thisName {
		return nil, ineligible(fmt.Sprintf("name %q reads the bound object", name))
	}
	sym, ok := c.ctx.loweredName(scope, &expr.Ref)
	if !ok || sym == nil {
		return nil, ineligible(fmt.Sprintf("name %q is not bound in the frame", name))
	}
	if c.ctx.readsAsFunction(sym) {
		return nil, ineligible(fmt.Sprintf("name %q is a function value", name))
	}
	node, err := c.libraryConstant(sym, name)
	if err != nil {
		return nil, err
	}
	c.cell.readsLibrary = true
	return node, nil
}

// compileQualifiedName resolves `A::B::x` as the evaluator does — first part in
// scope, the rest as members — and reads it where it is a library constant.
func (c *calcCompiler) compileQualifiedName(qn *ast.QualifiedName, scope *symbols.Scope) (*cnode, error) {
	firstQN := &ast.QualifiedName{Global: qn.Global, Parts: []ast.NameSegment{qn.Parts[0]}}
	firstQN.NodeBase = qn.NodeBase
	sym, ok := c.ctx.resolveQualified(scope, firstQN)
	if !ok || sym == nil {
		return nil, ineligible(fmt.Sprintf("qualified name %s does not resolve", qualifiedNameToString(qn)))
	}
	for _, part := range qn.Parts[1:] {
		if lower.IsCalcUsageSymbol(sym) {
			return nil, ineligible(fmt.Sprintf("qualified name %s reads a calc usage", qualifiedNameToString(qn)))
		}
		next, found := c.ctx.model.semantics.LookupMember(sym, part.Text)
		if !found {
			return nil, ineligible(fmt.Sprintf("qualified name %s does not resolve", qualifiedNameToString(qn)))
		}
		sym = next
	}
	return c.libraryConstant(sym, qualifiedNameToString(qn))
}
