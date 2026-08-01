package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context             // runtime context
	scope  *symbols.Scope       // scope context for name resolution
	frames []map[string]Value   // stack of local bindings (innermost = frames[len-1])
}

// NewEvalContext creates an evaluation context with an empty frame stack.
func NewEvalContext(ctx *Context, scope *symbols.Scope) *EvalContext {
	return &EvalContext{
		ctx:    ctx,
		scope:  scope,
		frames: nil,
	}
}

// Push adds a new frame to the stack (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value) {
	ec.frames = append(ec.frames, bindings)
}

// Pop removes the top frame from the stack (on return, lambda exit).
func (ec *EvalContext) Pop() {
	if len(ec.frames) > 0 {
		ec.frames = ec.frames[:len(ec.frames)-1]
	}
}

// Lookup searches for a name in the frame stack (innermost first).
func (ec *EvalContext) Lookup(name string) (Value, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if val, ok := ec.frames[i][name]; ok {
			return val, true
		}
	}
	return Value{}, false
}

// Eval evaluates an expression node. Returns a Value or an error.
// Increments ctx.steps on each eval call; errors when ctx.steps >= ctx.maxSteps.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	// Step counter
	if err := ec.ctx.incrementStep(); err != nil {
		return Value{}, err
	}
	
	// Dispatch by node type (scaffolding; full implementation in later tasks)
	switch n := node.(type) {
	case *ast.LiteralInteger:
		return ec.evalLiteralInteger(n)
	case *ast.LiteralReal:
		return ec.evalLiteralReal(n)
	case *ast.LiteralBool:
		return ec.evalLiteralBool(n)
	case *ast.LiteralString:
		return ec.evalLiteralString(n)
	case *ast.NullExpr:
		return ec.evalNull(n)
	case *ast.FeatureReference:
		return ec.evalFeatureReference(n)
	case *ast.OperatorExpr:
		return ec.evalOperator(n)
	case *ast.SequenceExpr:
		return ec.evalSequenceExpr(n)
	case *ast.CollectExpr:
		return ec.evalCollectExpr(n)
	case *ast.SelectExpr:
		return ec.evalSelectExpr(n)
	case *ast.InvocationExpr:
		return ec.evalInvocation(n)
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
// Resolves names from the root scope.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	// Use resolver's root scope for name resolution
	// (In a full implementation, this would track evaluation context scope)
	ec := NewEvalContext(ctx, nil)
	return ec.Eval(node)
}

// EvalWithScope evaluates an expression with a given scope context for name resolution.
func (ctx *Context) EvalWithScope(node ast.Node, scope *symbols.Scope) (Value, error) {
	ec := NewEvalContext(ctx, scope)
	return ec.Eval(node)
}

// evalLiteralInteger evaluates an integer literal.
func (ec *EvalContext) evalLiteralInteger(n *ast.LiteralInteger) (Value, error) {
	val, _ := strconv.ParseInt(n.Value, 10, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
}

// evalLiteralReal evaluates a real literal.
func (ec *EvalContext) evalLiteralReal(n *ast.LiteralReal) (Value, error) {
	val, _ := strconv.ParseFloat(n.Value, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: val}}, nil
}

// evalLiteralBool evaluates a boolean literal.
func (ec *EvalContext) evalLiteralBool(n *ast.LiteralBool) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: n.Value}}, nil
}

// evalLiteralString evaluates a string literal.
func (ec *EvalContext) evalLiteralString(n *ast.LiteralString) (Value, error) {
	// Strip quotes
	str := n.Value
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return Value{Kind: ValString, Str: str}, nil
}

// evalNull evaluates a null expression.
func (ec *EvalContext) evalNull(n *ast.NullExpr) (Value, error) {
	return Value{Kind: ValNull}, nil
}

// evalFeatureReference evaluates a feature reference (variable lookup).
func (ec *EvalContext) evalFeatureReference(n *ast.FeatureReference) (Value, error) {
	if n.Name == nil || len(n.Name.Parts) == 0 {
		return Value{}, fmt.Errorf("empty feature reference")
	}
	
	// Simple case: single-part name lookup in frame stack
	if len(n.Name.Parts) == 1 {
		name := n.Name.Parts[0].Text
		if val, ok := ec.Lookup(name); ok {
			return val, nil
		}
		return Value{}, fmt.Errorf("unresolved feature: %s", name)
	}
	
	// Multi-part qualified names (e.g., "Vehicle::speed") not yet supported
	return Value{}, fmt.Errorf("qualified feature reference not yet supported: %s", qualifiedNameToString(n.Name))
}


// evalOperator evaluates an operator expression.
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}

	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNeq:
		return ec.evalEquality(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpOr:
		return ec.evalLogical(n)
	case ast.OpNeg:
		return ec.evalNeg(n)
	case ast.OpNot:
		return ec.evalNot(n)
	default:
		return Value{}, fmt.Errorf("unsupported operator: %v", n.Operator)
	}
}

// evalArithmetic evaluates arithmetic operators (+, -, *, /).
func (ec *EvalContext) evalArithmetic(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) < 2 {
		return Value{}, fmt.Errorf("arithmetic operator requires 2 operands")
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}

	// Simplified: assume both are ValConst int/real
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, ErrTypeMismatch
	}

	// Integer arithmetic
	if left.Const.Kind == semantics.ValInt && right.Const.Kind == semantics.ValInt {
		var result int64
		switch n.Operator {
		case ast.OpAdd:
			result = left.Const.Int + right.Const.Int
		case ast.OpSub:
			result = left.Const.Int - right.Const.Int
		case ast.OpMul:
			result = left.Const.Int * right.Const.Int
		case ast.OpDiv:
			if right.Const.Int == 0 {
				return Value{}, fmt.Errorf("division by zero")
			}
			result = left.Const.Int / right.Const.Int
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: result}}, nil
	}

	// Real arithmetic (coerce int to real if needed)
	leftReal := toReal(left.Const)
	rightReal := toReal(right.Const)
	var result float64
	switch n.Operator {
	case ast.OpAdd:
		result = leftReal + rightReal
	case ast.OpSub:
		result = leftReal - rightReal
	case ast.OpMul:
		result = leftReal * rightReal
	case ast.OpDiv:
		result = leftReal / rightReal
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: result}}, nil
}

// toReal converts a semantics.Value to float64.
func toReal(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// evalEquality evaluates equality operators (==, !=).
func (ec *EvalContext) evalEquality(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("equality not yet implemented")
}

// evalComparison evaluates comparison operators (<, <=, >, >=).
func (ec *EvalContext) evalComparison(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("comparison requires 2 operands, got %d", len(n.Operands))
	}
	
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	
	// Both must be ValConst
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, fmt.Errorf("comparison operands must be constants")
	}
	
	// Compare integers
	if left.Const.Kind == semantics.ValInt && right.Const.Kind == semantics.ValInt {
		var result bool
		switch n.Operator {
		case ast.OpLt:
			result = left.Const.Int < right.Const.Int
		case ast.OpLe:
			result = left.Const.Int <= right.Const.Int
		case ast.OpGt:
			result = left.Const.Int > right.Const.Int
		case ast.OpGe:
			result = left.Const.Int >= right.Const.Int
		default:
			return Value{}, fmt.Errorf("unknown comparison operator: %v", n.Operator)
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
	}
	
	// Compare reals (coerce int to real)
	leftReal := toReal(left.Const)
	rightReal := toReal(right.Const)
	var result bool
	switch n.Operator {
	case ast.OpLt:
		result = leftReal < rightReal
	case ast.OpLe:
		result = leftReal <= rightReal
	case ast.OpGt:
		result = leftReal > rightReal
	case ast.OpGe:
		result = leftReal >= rightReal
	default:
		return Value{}, fmt.Errorf("unknown comparison operator: %v", n.Operator)
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
}

// evalLogical evaluates logical operators (&&, ||).
func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("logical not yet implemented")
}

// evalNeg evaluates unary negation (-).
func (ec *EvalContext) evalNeg(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("negation not yet implemented")
}

// evalNot evaluates logical not (!).
func (ec *EvalContext) evalNot(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("not not yet implemented")
}

// evalSequenceExpr evaluates a sequence expression (1, 2, 3).
func (ec *EvalContext) evalSequenceExpr(n *ast.SequenceExpr) (Value, error) {
	seq := NewSequence()
	for _, elem := range n.Elements {
		val, err := ec.Eval(elem)
		if err != nil {
			return Value{}, err
		}
		seq.Append(val)
	}
	return Value{Kind: ValSequence, Sequence: seq}, nil
}

// evalCollectExpr evaluates `operand . body` — map over collection.
func (ec *EvalContext) evalCollectExpr(n *ast.CollectExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, fmt.Errorf("%w: collect operand must be collection", ErrTypeMismatch)
	}
	
	result := NewSequence()
	for _, elem := range elements {
		// Push 'it' binding for body
		ec.Push(map[string]Value{"it": elem})
		val, err := ec.Eval(n.Body)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		result.Append(val)
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}

// evalSelectExpr evaluates `operand .? body` — filter collection.
func (ec *EvalContext) evalSelectExpr(n *ast.SelectExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, fmt.Errorf("%w: select operand must be collection", ErrTypeMismatch)
	}
	
	result := NewSequence()
	for _, elem := range elements {
		ec.Push(map[string]Value{"it": elem})
		predVal, err := ec.Eval(n.Body)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		
		// Check if predicate is true (ValConst boolean)
		if predVal.Kind == ValConst && predVal.Const.Kind == semantics.ValBool && predVal.Const.Bool {
			result.Append(elem)
		}
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}

// evalInvocation evaluates a function/calc invocation.
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	// Build qualified name string for builtin lookup
	qualName := qualifiedNameToString(n.Type)
	
	// Eval args
	args := make([]Value, len(n.Args))
	for i, arg := range n.Args {
		val, err := ec.Eval(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}
	
	// Check builtin registry
	if fn, ok := builtins[qualName]; ok {
		return fn(ec, args)
	}
	
	// User-defined calc: resolve target symbol
	// Use evaluation context scope (or fallback to root if nil)
	calcSym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, n.Type)
	if !ok || calcSym == nil {
		return Value{}, fmt.Errorf("unresolved calc: %s", qualName)
	}
	
	// Ensure it's a calc definition or usage
	calcDef, isCalcDef := calcSym.Decl.(*ast.Definition)
	calcUsage, isCalcUsage := calcSym.Decl.(*ast.Usage)
	
	if !isCalcDef && !isCalcUsage {
		return Value{}, fmt.Errorf("not a calc: %s (%T)", qualName, calcSym.Decl)
	}
	
	var members []ast.Node
	if isCalcDef && calcDef.Kind == ast.DefCalc {
		members = calcDef.Members
	} else if isCalcUsage && calcUsage.Kind == ast.UsageCalc {
		members = calcUsage.Members
	} else {
		return Value{}, fmt.Errorf("symbol is not a calc: %s", qualName)
	}
	
	// Extract parameters (usages with 'in' direction) and result members
	params := []string{}
	var resultExprs []ast.Node
	
	for _, member := range members {
		// Unwrap Membership
		var node ast.Node = member
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		}
		
		// Check for parameters (usages with 'in' direction and name)
		if usage, ok := node.(*ast.Usage); ok {
			if usage.Ident.Name != "" && (usage.Direction == ast.DirIn || usage.Direction == ast.DirInOut) {
				params = append(params, usage.Ident.Name)
			}
		}
		
		// Check for ResultMember
		if resultMember, ok := node.(*ast.ResultMember); ok {
			resultExprs = append(resultExprs, resultMember.Expression)
		}
	}
	
	// Bind arguments to parameters
	if len(args) != len(params) {
		return Value{}, fmt.Errorf("calc %s: expected %d args, got %d", qualName, len(params), len(args))
	}
	
	bindings := make(map[string]Value)
	for i, paramName := range params {
		bindings[paramName] = args[i]
	}
	
	// Push new frame with parameter bindings
	ec.Push(bindings)
	defer ec.Pop()
	
	// Evaluate result expressions (return statements)
	if len(resultExprs) == 0 {
		return Value{}, fmt.Errorf("calc %s has no return statement", qualName)
	}
	
	// Evaluate the first return expression
	// (In SysML v2, calcs typically have one return; multiple would need aggregation)
	return ec.Eval(resultExprs[0])
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

