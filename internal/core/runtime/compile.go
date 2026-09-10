package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CalcCompileEnvVar switches the compiled tier for pure calc bodies off when set
// to 0 (or false/off/no), leaving every calc to the reference evaluator.
const CalcCompileEnvVar = "OPENSYSML_CALC_COMPILE"

// CalcCompileFromEnv reports whether the environment leaves the compiled calc
// tier on, which it does unless CalcCompileEnvVar (or its legacy SYSML_ name)
// switches it off.
func CalcCompileFromEnv() bool {
	return calcCompileFromValue(envvar.Lookup(CalcCompileEnvVar))
}

// calcCompileFromValue reads the switch's value: unset or empty leaves it on.
func calcCompileFromValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// SetCalcCompile turns the compiled tier on or off for this context.
func (ctx *Context) SetCalcCompile(on bool) { ctx.compileCalcs = on }

// CalcCompile reports whether this context runs eligible calc bodies compiled.
func (ctx *Context) CalcCompile() bool { return ctx.compileCalcs }

// compileState is where a shape stands with the compiled tier.
type compileState uint8

const (
	compileUndecided compileState = iota
	compileInProgress
	compileEligible
	compileIneligible
)

// maxCompiledParams bounds the parameters of a compiled calc, whose bound
// arguments an invocation records in one paramSet.
const maxCompiledParams = 64

// paramSet is a set of parameter slots below maxCompiledParams, one bit each;
// every slot it takes is checked against that bound when the calc compiles.
type paramSet uint64

func (s *paramSet) add(i int) { *s |= 1 << uint(i) } // #nosec G115 -- slot below maxCompiledParams

func (s paramSet) has(i int) bool { return s&(1<<uint(i)) != 0 } // #nosec G115 -- slot below maxCompiledParams

// allParams is the set of the first n parameters.
func allParams(n int) paramSet { return paramSet(1)<<uint(n) - 1 } // #nosec G115 -- n below maxCompiledParams

// compiledCalc is a calc body in the compiled tier: parameters and body locals
// in the slots of one scalar frame, the body a tree of closures over it.
type compiledCalc struct {
	kind   string
	name   string
	params []compiledParam
	// required has a bit set for every parameter without a default, which an
	// invocation must bind.
	required paramSet
	// frameSize is the slots an invocation's frame holds: the parameters, then
	// the widest set of body locals in force at once.
	frameSize int
	// result checks what the body yields where the body is a bound result
	// expression; a body of statements checks at its return.
	result *scalarCheck
	body   compiledExpr
	// bodyErr prefixes an error the body raises and resultWhat names the result
	// in a refusal, each worded as the evaluator does for this body form.
	bodyErr    string
	resultWhat string
	// readsLibrary says the body reads a library constant by a bare name, which
	// the evaluator lets a feature of the bound object answer first.
	readsLibrary bool
}

// compiledParam is one parameter: its compiled default, if any, and the check
// on the value bound to it.
type compiledParam struct {
	name  string
	dflt  compiledExpr
	check scalarCheck
}

// scalarCheck is a parameter's or result's declaration, decided for a scalar
// without boxing where the scalar lattice decides it.
type scalarCheck struct {
	decl    *calcMemberDecl
	countOK bool
	// Whether the declared type holds a value of each lattice type a scalar has.
	boolOK, naturalOK, integerOK, rationalOK, realOK bool
	// least is the smallest integer a Natural-holding declaration takes: 1 for Positive.
	least int64
}

// accepts reports whether the declaration surely holds v, deciding on the
// scalar lattice alone; a value it declines is left to refuse.
func (c *scalarCheck) accepts(v scalar) bool {
	if !c.countOK {
		return false
	}
	switch v.kind {
	case scalarInt:
		return c.integerOK || (c.naturalOK && v.int() >= c.least)
	case scalarBool:
		return c.boolOK
	}
	return c.acceptsReal(v)
}

// acceptsReal places a Real on the lattice by its representation, as the evaluator does.
func (c *scalarCheck) acceptsReal(v scalar) bool {
	switch representationPrim(v.boxed()) {
	case semantics.PrimRational:
		return c.rationalOK
	case semantics.PrimReal:
		return c.realOK
	}
	return false
}

// refuse is the evaluator's verdict on a value accepts declined, so a refusal
// carries the reference's message and a value it would take is taken.
func (c *scalarCheck) refuse(ctx *Context, v scalar, what func() string) error {
	boxed := v.boxed()
	return c.decl.check(ctx, &boxed, what)
}

// compiledCalcOf answers the compiled body of shape, compiling it and every
// calc it invokes on first ask, or nil when the shape is ineligible.
func (ctx *Context) compiledCalcOf(shape *calcShape) *compiledCalc {
	switch shape.compileState {
	case compileEligible:
		return shape.compiled
	case compileIneligible:
		return nil
	}
	batch := &compileBatch{ctx: ctx}
	batch.compile(shape)
	batch.settle()
	if shape.compileState == compileEligible {
		return shape.compiled
	}
	return nil
}

// compileBatch is one compilation and the calc call graph it pulls in. A shape
// calling one still being compiled (a recursion or a cycle) takes its cell on
// trust, so the batch settles every call once each shape has an answer.
type compileBatch struct {
	ctx *Context
	// callees records the shapes each member compiled a call to.
	callees map[*calcShape][]*calcShape
}

// compile decides shape, compiling its callees first.
func (b *compileBatch) compile(shape *calcShape) {
	shape.compileState = compileInProgress
	shape.compiled = &compiledCalc{kind: shape.Kind, name: shape.Name}
	c := &calcCompiler{batch: b, ctx: b.ctx, shape: shape}
	if err := c.compile(shape.compiled); err != nil {
		shape.withdraw(err.Error())
		return
	}
	shape.compileState = compileEligible
}

// withdraw keeps shape on the evaluator for good, for the reason given.
func (shape *calcShape) withdraw(why string) {
	shape.compileState = compileIneligible
	shape.compiled = nil
	shape.ineligibleWhy = why
}

// call records that member compiled a call to callee.
func (b *compileBatch) call(member, callee *calcShape) {
	if b.callees == nil {
		b.callees = map[*calcShape][]*calcShape{}
	}
	b.callees[member] = append(b.callees[member], callee)
}

// settle withdraws eligibility from every member calling an ineligible shape,
// to a fixpoint, and marks a member reading a library constant via a callee.
func (b *compileBatch) settle() {
	for changed := true; changed; {
		changed = false
		for member, callees := range b.callees {
			if member.compileState != compileEligible {
				continue
			}
			for _, callee := range callees {
				if callee.compileState != compileEligible {
					member.withdraw("callee " + callee.Name + " is ineligible")
					changed = true
					break
				}
				if callee.compiled.readsLibrary && !member.compiled.readsLibrary {
					member.compiled.readsLibrary = true
					changed = true
				}
			}
		}
	}
}

// ineligible says why a body stays with the evaluator; the reason is recorded
// on the shape and never reaches a user.
func ineligible(why string) error { return errors.New(why) }

// calcCompiler compiles one shape's parameters and body.
type calcCompiler struct {
	batch *compileBatch
	ctx   *Context
	shape *calcShape
	cell  *compiledCalc
}

// compile fills cell with the shape's compiled form, or says why it cannot.
func (c *calcCompiler) compile(cell *compiledCalc) error {
	shape := c.shape
	c.cell = cell
	for i := range shape.Outputs {
		if !shape.Outputs[i].IsResult {
			return ineligible(fmt.Sprintf("output feature %q beside the result", shape.Outputs[i].Name))
		}
	}
	if len(shape.Params) > maxCompiledParams {
		return ineligible(fmt.Sprintf("%d parameters", len(shape.Params)))
	}
	cell.params = make([]compiledParam, len(shape.Params))
	for i := range shape.Params {
		param := &shape.Params[i]
		if param.IsCalc {
			return ineligible(fmt.Sprintf("parameter %q binds a function value", param.Name))
		}
		check, ok := c.scalarCheckFor(&param.Decl)
		if !ok {
			return ineligible(fmt.Sprintf("parameter %q declares a type outside the scalar lattice", param.Name))
		}
		cell.params[i] = compiledParam{name: param.Name, check: check}
		if param.Default == nil {
			cell.required.add(i)
			continue
		}
		scope := c.ctx.calcScope(param.Owner, shape.Sym, nil)
		dflt, err := c.compileNode(param.Default, scope, newFrameLayout(shape.ParamNames, shape.Aliases, i))
		if err != nil {
			return err
		}
		if !dflt.isConst {
			return ineligible(fmt.Sprintf("default of %q is not a literal", param.Name))
		}
		cell.params[i].dflt = dflt.expr()
	}
	var result *scalarCheck
	if out := shape.resultOutput(); out != nil {
		check, ok := c.scalarCheckFor(&out.Decl)
		if !ok {
			return ineligible("result declares a type outside the scalar lattice")
		}
		result = &check
	}
	layout := newFrameLayout(shape.ParamNames, shape.Aliases, len(shape.Params))
	if len(shape.Steps) == 0 {
		if err := c.compileResultBinding(cell, layout, result); err != nil {
			return err
		}
	} else if err := c.compileBody(cell, layout, result); err != nil {
		return err
	}
	cell.frameSize = layout.size
	return nil
}

// compileResultBinding compiles a statement-less body: the expression bound to
// the result parameter, whose errors the evaluator words as an output feature's.
func (c *calcCompiler) compileResultBinding(cell *compiledCalc, layout *frameLayout, result *scalarCheck) error {
	shape := c.shape
	if shape.ResultExpr == nil {
		return ineligible("body without a result expression")
	}
	scope := c.ctx.calcScope(shape.Sym, shape.Sym, nil)
	if err := plainScope(scope); err != nil {
		return err
	}
	body, err := c.compileNode(shape.ResultExpr, scope, layout)
	if err != nil {
		return err
	}
	cell.body = body.expr()
	cell.result = result
	cell.bodyErr = fmt.Sprintf("calc %s: output result: ", shape.Name)
	cell.resultWhat = fmt.Sprintf("calc %s: output result", shape.Name)
	return nil
}

// compileBody compiles a body of statements, which must return on every path;
// one that may run off its end reads its result as an output feature, so declines.
func (c *calcCompiler) compileBody(cell *compiledCalc, layout *frameLayout, result *scalarCheck) error {
	if ret, ok := c.singleReturn(); ok {
		return c.compileReturnBody(cell, ret, layout, result)
	}
	stmts, returns, err := c.compileStatements(c.shape.Steps, layout, result)
	if err != nil {
		return err
	}
	if !returns {
		return ineligible("body may end without returning")
	}
	cell.body = bodyStmt(stmts)
	return nil
}

// singleReturn answers the body's one statement when it is a valued return.
func (c *calcCompiler) singleReturn() (lower.Return, bool) {
	if len(c.shape.Steps) != 1 {
		return lower.Return{}, false
	}
	ret, ok := c.shape.Steps[0].(lower.Return)
	return ret, ok && ret.Value != nil
}

// compileReturnBody compiles a body that is one return as the expression itself,
// sparing the statement dispatch; the result is held to its declaration on exit.
func (c *calcCompiler) compileReturnBody(cell *compiledCalc, ret lower.Return, layout *frameLayout, result *scalarCheck) error {
	if err := plainScope(ret.Scope); err != nil {
		return err
	}
	body, err := c.compileNode(ret.Value, ret.Scope, layout)
	if err != nil {
		return err
	}
	cell.body = body.expr()
	cell.result = result
	cell.bodyErr = "evaluating the returned expression: "
	cell.resultWhat = "result"
	return nil
}

// scalarCheckFor decides a declaration for scalars, declining a declared type
// the scalar lattice does not place. A declaration stating no type, or one that
// does not resolve, holds any scalar, as it does on the evaluator; a non-scalar
// argument never reaches the compiled tier.
func (c *calcCompiler) scalarCheckFor(decl *calcMemberDecl) (scalarCheck, bool) {
	check := scalarCheck{decl: decl, countOK: true,
		boolOK: true, naturalOK: true, integerOK: true, rationalOK: true, realOK: true}
	if decl.Target == nil {
		return check, true
	}
	one := Value{Kind: ValConst}
	check.countOK = c.ctx.writeCountRefusal(decl.Target, &one) == ""
	if decl.Target.typ == nil {
		return check, true
	}
	prim := c.ctx.model.PrimTypeOf(decl.Target.typ)
	if prim == semantics.PrimUnknown {
		return scalarCheck{}, false
	}
	holds := func(from semantics.PrimType) bool { return semantics.PrimConforms(from, prim) }
	check.boolOK = holds(semantics.PrimBoolean)
	check.naturalOK = holds(semantics.PrimNatural)
	check.integerOK = holds(semantics.PrimInteger)
	check.rationalOK = holds(semantics.PrimRational)
	check.realOK = holds(semantics.PrimReal)
	if c.ctx.positiveScalar(decl.Target.typ) {
		check.least = 1
	}
	return check, true
}

// cnode is one compiled expression node. The evaluator charges a step per node
// as it reaches it; the compiled form charges the steps up to a subtree's first
// fallible operation at once, which leaves the counter identical at every point
// an error can be observed.
type cnode struct {
	// prefix is the steps charged before the subtree's first operation that can
	// fail; infallible says no operation in it can, so prefix is the whole.
	prefix     int64
	infallible bool
	// emit builds the closure; precharged says the node above charged prefix.
	emit func(precharged bool) compiledExpr
	// A leaf reads a frame slot or yields a constant.
	leaf     bool
	isConst  bool
	slot     int
	constant scalar
}

// expr is the node as a standalone expression charging its own steps.
func (n *cnode) expr() compiledExpr { return n.emit(false) }

// compileNode compiles n written in scope, reading names from the frame slots
// the layout describes; a name outside it is admitted only as a library constant.
func (c *calcCompiler) compileNode(n ast.Node, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		v, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			return nil, ineligible(fmt.Sprintf("integer literal %s outside the range", e.Value))
		}
		return constNode(intScalar(v)), nil
	case *ast.LiteralReal:
		v, err := semantics.ParseReal(e.Value)
		if err != nil {
			return nil, ineligible(fmt.Sprintf("real literal %s outside the range", e.Value))
		}
		return constNode(realScalar(v)), nil
	case *ast.LiteralBool:
		return constNode(boolScalar(e.Value)), nil
	case *ast.FeatureReference:
		return c.compileName(e.Name, scope, layout)
	case *ast.QualifiedName:
		return c.compileName(e, scope, layout)
	case *ast.OperatorExpr:
		return c.compileOperator(e, scope, layout)
	case *ast.InvocationExpr:
		return c.compileInvocation(e, scope, layout)
	default:
		return nil, ineligible(fmt.Sprintf("%T is outside the pure subset", n))
	}
}

// compileName resolves a name as the evaluator does: a frame binding first,
// then what the scope resolves it to, read only where it is a library constant.
func (c *calcCompiler) compileName(qn *ast.QualifiedName, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
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
	sym, ok := c.ctx.resolver.LookupName(scope, name)
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
	sym, ok := c.ctx.resolver.ResolveQualified(scope, firstQN)
	if !ok || sym == nil {
		return nil, ineligible(fmt.Sprintf("qualified name %s does not resolve", qualifiedNameToString(qn)))
	}
	for _, part := range qn.Parts[1:] {
		if isCalcUsageSymbol(sym) {
			return nil, ineligible(fmt.Sprintf("qualified name %s reads a calc usage", qualifiedNameToString(qn)))
		}
		next, found := c.ctx.model.LookupMember(sym, part.Text)
		if !found {
			return nil, ineligible(fmt.Sprintf("qualified name %s does not resolve", qualifiedNameToString(qn)))
		}
		sym = next
	}
	return c.libraryConstant(sym, qualifiedNameToString(qn))
}

// libraryConstant compiles a read of sym where it is a scalar constant the
// library seam supplies; anything else the name may denote keeps the evaluator.
func (c *calcCompiler) libraryConstant(sym *symbols.Symbol, name string) (*cnode, error) {
	if c.ctx.model.VariationPointOwning(sym) != nil || semantics.EnumerationOwning(sym) != nil {
		return nil, ineligible(fmt.Sprintf("name %q is a variant or an enumeration literal", name))
	}
	val, ok, err := c.ctx.libraryFeatureValue(sym)
	if !ok {
		return nil, ineligible(fmt.Sprintf("name %q is not bound in the frame", name))
	}
	if err != nil {
		return nil, ineligible(fmt.Sprintf("library feature %q has no value: %v", name, err))
	}
	v, ok := scalarOf(val)
	if !ok {
		return nil, ineligible(fmt.Sprintf("library feature %q is not a scalar", name))
	}
	return constNode(v), nil
}

// compileOperator compiles an operator application, folding it as the
// evaluator does before it looks at the operands.
func (c *calcCompiler) compileOperator(n *ast.OperatorExpr, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if folded, ok := c.ctx.model.Eval(n); ok {
		v, ok := scalarOfConst(folded)
		if !ok {
			return nil, ineligible("folds to a non-scalar constant")
		}
		return constNode(v), nil
	}
	switch n.Operator {
	case ast.OpConditional:
		if len(n.Operands) != 3 {
			return nil, ineligible(fmt.Sprintf("conditional with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, conditionalNode)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("arithmetic with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, arithmeticNode)
	case ast.OpEq, ast.OpNeq:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("equality with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, equalityNode)
	case ast.OpEqEqEq, ast.OpNeqEqEq:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("identity with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, identityNode)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("comparison with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, comparisonNode)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("logical operator with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, layout, logicalNode)
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(n.Operands) != 1 {
			return nil, ineligible(fmt.Sprintf("unary operator with %d operands", len(n.Operands)))
		}
		// The least Integer is read with its sign, as the evaluator reads it.
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok && n.Operator == ast.OpNeg {
			if _, err := strconv.ParseInt(lit.Value, 10, 64); err != nil {
				if v, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
					return constNode(intScalar(v)), nil
				}
			}
		}
		return c.compileOperands(n, scope, layout, unaryNode)
	default:
		return nil, ineligible(fmt.Sprintf("operator '%s' is outside the pure subset", n.Operator))
	}
}

// compileOperands compiles n's operands in order and builds the node over them.
func (c *calcCompiler) compileOperands(n *ast.OperatorExpr, scope *symbols.Scope, layout *frameLayout,
	build func(op ast.OperatorKind, kids []*cnode) *cnode) (*cnode, error) {
	kids := make([]*cnode, len(n.Operands))
	for i, operand := range n.Operands {
		kid, err := c.compileNode(operand, scope, layout)
		if err != nil {
			return nil, err
		}
		kids[i] = kid
	}
	return build(n.Operator, kids), nil
}

// callArguments are an invocation's arguments as written: the expressions in
// source order, a receiver first, and the parameter name each named one binds.
type callArguments struct {
	exprs []ast.Node
	names []string
}

// callArgumentsOf reads an invocation's arguments, declining the forms the
// evaluator reports at run time: a receiver or positional beside named ones.
// names is the parameter each named argument binds, as the target resolved them.
func callArgumentsOf(n *ast.InvocationExpr, names []string, unbound []error) (callArguments, error) {
	var args callArguments
	if n.Operand != nil {
		if len(n.NamedArgs) > 0 {
			return args, ineligible("invocation with a receiver and named arguments")
		}
		args.exprs = append(args.exprs, n.Operand)
	}
	args.exprs = append(args.exprs, n.Args...)
	if len(n.NamedArgs) == 0 {
		return args, nil
	}
	if len(args.exprs) > 0 {
		return args, ineligible("invocation with positional and named arguments")
	}
	for i, arg := range n.NamedArgs {
		if names[i] == "" {
			return args, ineligible("invocation with an unnamed argument")
		}
		if err := unbound[i]; err != nil {
			return args, ineligible(err.Error())
		}
		args.exprs = append(args.exprs, arg.Value)
		args.names = append(args.names, names[i])
	}
	return args, nil
}

// slotsFor places the arguments in parameter slots by position or by name; a
// name written twice is left to the evaluator's report.
func (a *callArguments) slotsFor(params []string) ([]int, paramSet, error) {
	slots := make([]int, len(a.exprs))
	var bound paramSet
	for i := range a.exprs {
		slot := i
		if a.names != nil {
			slot = -1
			for j, param := range params {
				if param == a.names[i] {
					slot = j
					break
				}
			}
			if slot < 0 {
				return nil, 0, ineligible(fmt.Sprintf("no parameter %q to bind by name", a.names[i]))
			}
			if bound.has(slot) {
				return nil, 0, ineligible(fmt.Sprintf("parameter %q bound by name twice", a.names[i]))
			}
		}
		slots[i] = slot
		bound.add(slot)
	}
	return slots, bound, nil
}

// compileArguments compiles the argument expressions in source order.
func (c *calcCompiler) compileArguments(args *callArguments, scope *symbols.Scope, layout *frameLayout) ([]*cnode, error) {
	kids := make([]*cnode, len(args.exprs))
	for i, arg := range args.exprs {
		kid, err := c.compileNode(arg, scope, layout)
		if err != nil {
			return nil, err
		}
		kids[i] = kid
	}
	return kids, nil
}

// scalarAggregates are the collection functions a scalar argument makes a
// one-element aggregation of, whose implementations the tier calls as they are.
var scalarAggregates = map[string]bool{
	"NumericalFunctions::sum": true, "NumericalFunctions::product": true,
	"IntegerFunctions::sum": true, "IntegerFunctions::product": true,
	"RationalFunctions::sum": true, "RationalFunctions::product": true,
	"RealFunctions::sum": true, "RealFunctions::product": true,
}

// compileInvocation compiles a call dispatched as the evaluator dispatches it:
// what the name resolves to in scope. A name that resolves to nothing is left
// to the evaluator, which reports it unresolved.
func (c *calcCompiler) compileInvocation(n *ast.InvocationExpr, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if scope == nil {
		return nil, ineligible("invocation without a scope")
	}
	target := (&EvalContext{ctx: c.ctx, scope: scope}).invocationTarget(n)
	args, err := callArgumentsOf(n, target.names, target.unbound)
	if err != nil {
		return nil, err
	}
	switch {
	case len(target.ambiguous) > 0:
		return nil, ineligible(fmt.Sprintf("%s is ambiguous", target.qualName))
	case target.builtin != nil:
		return c.compileAggregate(target.builtinName, target.builtin, &args, scope, layout)
	case target.library != nil:
		return c.compileLibraryCall(target.library, &args, scope, layout)
	case target.calc == nil:
		return nil, ineligible(fmt.Sprintf("%s does not resolve to a calc", target.qualName))
	case target.shape == nil:
		return nil, ineligible(fmt.Sprintf("%s is not a calc with a shape", target.qualName))
	}
	return c.compileCalcCall(target.shape, &args, scope, layout)
}

// compileCalcCall compiles a call of another eligible calc, the arguments
// placed in its slots and its arity checked here rather than per call.
func (c *calcCompiler) compileCalcCall(callee *calcShape, args *callArguments, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if callee.compileState == compileUndecided {
		c.batch.compile(callee)
	}
	if callee.compileState == compileIneligible {
		return nil, ineligible(fmt.Sprintf("callee %s is ineligible", callee.Name))
	}
	c.batch.call(c.shape, callee)
	if len(args.exprs) > len(callee.Params) {
		return nil, ineligible(fmt.Sprintf("%s called with %d arguments for %d parameters", callee.Name, len(args.exprs), len(callee.Params)))
	}
	slots, bound, err := args.slotsFor(callee.ParamNames)
	if err != nil {
		return nil, ineligible(fmt.Sprintf("%s: %v", callee.Name, err))
	}
	for i := range callee.Params {
		if !bound.has(i) && callee.Params[i].Default == nil {
			return nil, ineligible(fmt.Sprintf("%s called without an argument for %q", callee.Name, callee.Params[i].Name))
		}
	}
	kids, err := c.compileArguments(args, scope, layout)
	if err != nil {
		return nil, err
	}
	return callNode(callee.compiled, kids, slots, bound), nil
}

// compileLibraryCall compiles a call of a scalar library function binding every
// parameter; any other arity keeps the evaluator's own report.
func (c *calcCompiler) compileLibraryCall(fn *libraryFunction, args *callArguments, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if !fn.scalar {
		return nil, ineligible(fmt.Sprintf("library function %s is not over scalars alone", fn.name))
	}
	if len(fn.params) > libraryArity || fn.hasOptional() {
		return nil, ineligible(fmt.Sprintf("library function %s has optional or many parameters", fn.name))
	}
	if len(args.exprs) != len(fn.params) {
		return nil, ineligible(fmt.Sprintf("%s called with %d arguments for %d parameters", fn.name, len(args.exprs), len(fn.params)))
	}
	slots, bound, err := args.slotsFor(fn.paramNames())
	if err != nil {
		return nil, ineligible(fmt.Sprintf("%s: %v", fn.name, err))
	}
	if bound != allParams(len(fn.params)) {
		return nil, ineligible(fmt.Sprintf("%s called without an argument for every parameter", fn.name))
	}
	kids, err := c.compileArguments(args, scope, layout)
	if err != nil {
		return nil, err
	}
	return libraryCallNode(fn, kids, slots), nil
}

// compileAggregate compiles a sum or product of one scalar argument, which the
// evaluator's built-in aggregates as a one-element collection.
func (c *calcCompiler) compileAggregate(name string, fn builtinFunc, args *callArguments, scope *symbols.Scope, layout *frameLayout) (*cnode, error) {
	if !scalarAggregates[name] {
		return nil, ineligible(fmt.Sprintf("collection function %s", name))
	}
	if args.names != nil || len(args.exprs) != 1 {
		return nil, ineligible(fmt.Sprintf("%s called other than with one positional argument", name))
	}
	arg, err := c.compileNode(args.exprs[0], scope, layout)
	if err != nil {
		return nil, err
	}
	return aggregateNode(name, fn, arg), nil
}

// invoke runs the body in the frame at base on the scalar stack; bound says which
// parameter slots hold arguments, the rest take their defaults.
func (c *compiledCalc) invoke(ctx *Context, base int, bound paramSet) (scalar, error) {
	if err := ctx.enterCalc(c.name); err != nil {
		return scalar{}, err
	}
	frame := ctx.frameAt(base, c.frameSize)
	for i := range c.params {
		p := &c.params[i]
		v := frame[i]
		source := "argument"
		if !bound.has(i) {
			var err error
			if v, err = p.dflt(ctx, frame); err != nil {
				ctx.leaveCalc()
				return scalar{}, calcDefaultError(c.kind, c.name, p.name, err)
			}
			source = "default"
			frame[i] = v
		}
		if !p.check.accepts(v) {
			err := p.check.refuse(ctx, v, func() string {
				return fmt.Sprintf("%s %s: %s for parameter %q", c.kind, c.name, source, p.name)
			})
			if err != nil {
				ctx.leaveCalc()
				return scalar{}, err
			}
		}
	}
	result, err := c.body(ctx, frame)
	ctx.leaveCalc()
	if err != nil {
		return scalar{}, calcFrame(c.kind, c.name, fmt.Errorf("%s%w", c.bodyErr, err))
	}
	if c.result != nil && !c.result.accepts(result) {
		if err := c.result.refuse(ctx, result, func() string { return c.resultWhat }); err != nil {
			return scalar{}, calcFrame(c.kind, c.name, err)
		}
	}
	return result, nil
}

// frameAt extends the scalar stack to hold a frame of size slots at base; the
// local slots are written by their declarations before any read.
func (ctx *Context) frameAt(base, size int) []scalar {
	if top := base + size; len(ctx.scalarStack) < top {
		ctx.reserveScalars(top - len(ctx.scalarStack))
	}
	return ctx.scalarStack[base : base+size]
}

// invokeBoxed unboxes the arguments into a frame — by position, else by name —
// and runs the body; it declines a non-scalar argument or an unbound parameter.
func (c *compiledCalc) invokeBoxed(ctx *Context, args calcArgs) (Value, bool, error) {
	if len(args.positional) > len(c.params) {
		return Value{}, false, nil
	}
	base := len(ctx.scalarStack)
	var bound paramSet
	if args.named == nil {
		for i, arg := range args.positional {
			s, ok := scalarOf(arg)
			if !ok {
				ctx.scalarStack = ctx.scalarStack[:base]
				return Value{}, false, nil
			}
			ctx.scalarStack = append(ctx.scalarStack, s)
			bound.add(i)
		}
	} else {
		ctx.reserveScalars(len(c.params))
		for i := range c.params {
			arg, ok := args.named[c.params[i].name]
			if i < len(args.positional) {
				arg, ok = args.positional[i], true
			}
			if !ok {
				continue
			}
			s, ok := scalarOf(arg)
			if !ok {
				ctx.scalarStack = ctx.scalarStack[:base]
				return Value{}, false, nil
			}
			ctx.scalarStack[base+i] = s
			bound.add(i)
		}
	}
	if bound&c.required != c.required {
		ctx.scalarStack = ctx.scalarStack[:base]
		return Value{}, false, nil
	}
	result, err := c.invoke(ctx, base, bound)
	ctx.scalarStack = ctx.scalarStack[:base]
	if err != nil {
		return Value{}, true, err
	}
	return result.boxed(), true, nil
}
