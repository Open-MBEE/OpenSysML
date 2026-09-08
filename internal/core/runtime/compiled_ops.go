package runtime

import (
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// scalar is an unboxed Integer, Real or Boolean, the values the compiled calc
// tier computes over; it is boxed into a Value only at an invocation boundary.
type scalar struct {
	kind scalarKind
	bits uint64
}

type scalarKind uint8

const (
	scalarInt scalarKind = iota
	scalarReal
	scalarBool
)

// #nosec G115 -- the Integer's two's-complement bits are stored, not its magnitude.
func intScalar(i int64) scalar { return scalar{kind: scalarInt, bits: uint64(i)} }

func realScalar(f float64) scalar { return scalar{kind: scalarReal, bits: math.Float64bits(f)} }

func boolScalar(b bool) scalar {
	if b {
		return scalar{kind: scalarBool, bits: 1}
	}
	return scalar{kind: scalarBool, bits: 0}
}

// #nosec G115 -- the inverse of intScalar: the same bits read back as an Integer.
func (s scalar) int() int64 { return int64(s.bits) }

func (s scalar) real() float64 { return math.Float64frombits(s.bits) }

func (s scalar) truth() bool { return s.bits != 0 }

// semantic is the constant the scalar stands for.
func (s scalar) semantic() semantics.Value {
	switch s.kind {
	case scalarInt:
		return semantics.Value{Kind: semantics.ValInt, Int: s.int()}
	case scalarReal:
		return semantics.Value{Kind: semantics.ValReal, Real: s.real()}
	default:
		return semantics.Value{Kind: semantics.ValBool, Bool: s.truth()}
	}
}

// boxed is the scalar as the evaluator's Value.
func (s scalar) boxed() Value {
	return Value{Kind: ValConst, Const: s.semantic()}
}

// scalarOfConst unboxes a scalar constant, declining any other constant kind.
func scalarOfConst(c semantics.Value) (scalar, bool) {
	switch c.Kind {
	case semantics.ValInt:
		return intScalar(c.Int), true
	case semantics.ValReal:
		return realScalar(c.Real), true
	case semantics.ValBool:
		return boolScalar(c.Bool), true
	}
	return scalar{}, false
}

// scalarOf unboxes a Value holding a scalar constant, declining any other value.
func scalarOf(v Value) (scalar, bool) {
	if v.Kind != ValConst {
		return scalar{}, false
	}
	return scalarOfConst(v.Const)
}

// compiledExpr evaluates one compiled expression over the invocation's
// parameters, charging the evaluator's step for each node it stands for.
type compiledExpr func(ctx *Context, args []scalar) (scalar, error)

// chargeSteps spends n evaluation steps at once, for a subtree of nodes none of
// which can fail. It leaves the counter where the evaluator's would stop.
func (ctx *Context) chargeSteps(n int64) error {
	ctx.run.steps += n
	if ctx.run.steps > ctx.maxSteps {
		ctx.run.steps = ctx.maxSteps + 1
		return ctx.stepLimitExceeded()
	}
	return nil
}

// binaryOp combines two evaluated operands.
type binaryOp func(l, r scalar) (scalar, error)

// arithmeticOp is constArithmetic for one operator, the Integer sum and
// difference answered without boxing.
func arithmeticOp(op ast.OperatorKind) binaryOp {
	switch op {
	case ast.OpAdd:
		return addScalars
	case ast.OpSub:
		return subScalars
	}
	return func(l, r scalar) (scalar, error) { return arithScalars(op, l, r) }
}

func addScalars(l, r scalar) (scalar, error) {
	if l.kind == scalarInt && r.kind == scalarInt {
		a, b := l.int(), r.int()
		if res := a + b; (b <= 0 || res > a) && (b >= 0 || res < a) {
			return intScalar(res), nil
		}
		return scalar{}, semantics.IntegerOverflow(ast.OpAdd, a, b)
	}
	return arithScalars(ast.OpAdd, l, r)
}

func subScalars(l, r scalar) (scalar, error) {
	if l.kind == scalarInt && r.kind == scalarInt {
		a, b := l.int(), r.int()
		if res := a - b; (b >= 0 || res > a) && (b <= 0 || res < a) {
			return intScalar(res), nil
		}
		return scalar{}, semantics.IntegerOverflow(ast.OpSub, a, b)
	}
	return arithScalars(ast.OpSub, l, r)
}

// arithScalars is constArithmetic over scalars.
func arithScalars(op ast.OperatorKind, l, r scalar) (scalar, error) {
	res, err := constArithmetic(op, l.semantic(), r.semantic())
	if err != nil {
		return scalar{}, err
	}
	return scalarResult(op, res)
}

// scalarResult unboxes an operator's result, which constArithmetic and
// constUnary keep on the scalar lattice.
func scalarResult(op ast.OperatorKind, res semantics.Value) (scalar, error) {
	out, ok := scalarOfConst(res)
	if !ok {
		return scalar{}, fmt.Errorf("compiled '%s' produced a non-scalar %s", op,
			describeValue(Value{Kind: ValConst, Const: res}))
	}
	return out, nil
}

// comparisonOp is constComparison for one operator, two Integers answered
// without boxing.
func comparisonOp(op ast.OperatorKind) binaryOp {
	switch op {
	case ast.OpLt:
		return func(l, r scalar) (scalar, error) {
			if l.kind == scalarInt && r.kind == scalarInt {
				return boolScalar(l.int() < r.int()), nil
			}
			return compareScalars(op, l, r)
		}
	case ast.OpLe:
		return func(l, r scalar) (scalar, error) {
			if l.kind == scalarInt && r.kind == scalarInt {
				return boolScalar(l.int() <= r.int()), nil
			}
			return compareScalars(op, l, r)
		}
	case ast.OpGt:
		return func(l, r scalar) (scalar, error) {
			if l.kind == scalarInt && r.kind == scalarInt {
				return boolScalar(l.int() > r.int()), nil
			}
			return compareScalars(op, l, r)
		}
	case ast.OpGe:
		return func(l, r scalar) (scalar, error) {
			if l.kind == scalarInt && r.kind == scalarInt {
				return boolScalar(l.int() >= r.int()), nil
			}
			return compareScalars(op, l, r)
		}
	}
	return func(l, r scalar) (scalar, error) { return compareScalars(op, l, r) }
}

// compareScalars is constComparison over scalars.
func compareScalars(op ast.OperatorKind, l, r scalar) (scalar, error) {
	res, err := constComparison(op, l.semantic(), r.semantic())
	if err != nil {
		return scalar{}, err
	}
	return boolScalar(res), nil
}

// equalScalars is the evaluator's `==` over two scalar constants.
func equalScalars(l, r scalar) bool {
	return valueEqual(l.boxed(), r.boxed())
}

// identicalScalars is the evaluator's `===` over two scalar constants.
func identicalScalars(l, r scalar) bool {
	return valueIdentical(l.boxed(), r.boxed())
}

// unaryScalar is constUnary over a scalar.
func unaryScalar(op ast.OperatorKind, v scalar) (scalar, error) {
	res, err := constUnary(op, v.semantic())
	if err != nil {
		return scalar{}, err
	}
	return scalarResult(op, res)
}

// scalarTruth reads a Boolean operand, reporting any other scalar as the
// evaluator's boolOperand does.
func scalarTruth(what string, v scalar) (bool, error) {
	if v.kind != scalarBool {
		return boolOperand(what, v.boxed())
	}
	return v.truth(), nil
}

// constNode is a literal or a folded constant.
func constNode(v scalar) *cnode {
	n := &cnode{prefix: 1, infallible: true, leaf: true, isConst: true, constant: v}
	n.emit = func(precharged bool) compiledExpr {
		if precharged {
			return func(*Context, []scalar) (scalar, error) { return v, nil }
		}
		return func(ctx *Context, _ []scalar) (scalar, error) {
			if err := ctx.incrementStep(); err != nil {
				return scalar{}, err
			}
			return v, nil
		}
	}
	return n
}

// slotNode reads frame slot i: a parameter, or a body local its declaration
// has bound by the time the read runs.
func slotNode(i int) *cnode {
	n := &cnode{prefix: 1, infallible: true, leaf: true, slot: i}
	n.emit = func(precharged bool) compiledExpr {
		if precharged {
			return func(_ *Context, args []scalar) (scalar, error) { return args[i], nil }
		}
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.incrementStep(); err != nil {
				return scalar{}, err
			}
			return args[i], nil
		}
	}
	return n
}

// binaryNode evaluates both operands in order, then applies op; opInfallible
// says op itself cannot fail on any two scalars.
func binaryNode(kids []*cnode, op binaryOp, opInfallible bool) *cnode {
	l, r := kids[0], kids[1]
	n := &cnode{prefix: 1 + l.prefix, infallible: opInfallible && l.infallible && r.infallible}
	if l.infallible {
		n.prefix += r.prefix
	}
	n.emit = func(precharged bool) compiledExpr {
		var pre int64
		if !precharged {
			pre = n.prefix
		}
		if l.leaf && r.leaf {
			return leafPair(pre, l, r, op)
		}
		le, re := l.emit(true), r.emit(l.infallible)
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			lv, err := le(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			rv, err := re(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			return op(lv, rv)
		}
	}
	return n
}

// leafPair is a binary operator over two leaves, reading them in place.
func leafPair(pre int64, l, r *cnode, op binaryOp) compiledExpr {
	switch {
	case !l.isConst && r.isConst:
		i, c := l.slot, r.constant
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			return op(args[i], c)
		}
	case !l.isConst && !r.isConst:
		i, j := l.slot, r.slot
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			return op(args[i], args[j])
		}
	case l.isConst && !r.isConst:
		c, j := l.constant, r.slot
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			return op(c, args[j])
		}
	default:
		c, d := l.constant, r.constant
		return func(ctx *Context, _ []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			return op(c, d)
		}
	}
}

func arithmeticNode(op ast.OperatorKind, kids []*cnode) *cnode {
	return binaryNode(kids, arithmeticOp(op), false)
}

func comparisonNode(op ast.OperatorKind, kids []*cnode) *cnode {
	return binaryNode(kids, comparisonOp(op), true)
}

func equalityNode(op ast.OperatorKind, kids []*cnode) *cnode {
	negate := op == ast.OpNeq
	return binaryNode(kids, func(l, r scalar) (scalar, error) {
		return boolScalar(equalScalars(l, r) != negate), nil
	}, true)
}

func identityNode(op ast.OperatorKind, kids []*cnode) *cnode {
	negate := op == ast.OpNeqEqEq
	return binaryNode(kids, func(l, r scalar) (scalar, error) {
		return boolScalar(identicalScalars(l, r) != negate), nil
	}, true)
}

// unaryNode applies `-`, `+` or `not` to its operand.
func unaryNode(op ast.OperatorKind, kids []*cnode) *cnode {
	k := kids[0]
	n := &cnode{prefix: 1 + k.prefix}
	n.emit = func(precharged bool) compiledExpr {
		var pre int64
		if !precharged {
			pre = n.prefix
		}
		ke := k.emit(true)
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			v, err := ke(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			return unaryScalar(op, v)
		}
	}
	return n
}

// logicalNode is `and`, `or`, `xor` and `implies`, deciding on the left
// operand alone where the evaluator does.
func logicalNode(op ast.OperatorKind, kids []*cnode) *cnode {
	l, r := kids[0], kids[1]
	n := &cnode{prefix: 1 + l.prefix}
	leftWhat := fmt.Sprintf("left operand of '%s'", op)
	rightWhat := fmt.Sprintf("right operand of '%s'", op)
	var short func(l bool) (bool, bool)
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		short = func(l bool) (bool, bool) { return false, !l }
	case ast.OpOr, ast.OpConditionalOr:
		short = func(l bool) (bool, bool) { return true, l }
	case ast.OpImplies:
		short = func(l bool) (bool, bool) { return true, !l }
	default:
		short = func(bool) (bool, bool) { return false, false }
	}
	combine := func(l, r bool) bool {
		switch op {
		case ast.OpAnd, ast.OpConditionalAnd:
			return l && r
		case ast.OpOr, ast.OpConditionalOr:
			return l || r
		case ast.OpXor:
			return l != r
		default:
			return !l || r
		}
	}
	n.emit = func(precharged bool) compiledExpr {
		var pre int64
		if !precharged {
			pre = n.prefix
		}
		le, re := l.emit(true), r.emit(false)
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			lv, err := le(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			lb, err := scalarTruth(leftWhat, lv)
			if err != nil {
				return scalar{}, err
			}
			if res, decided := short(lb); decided {
				return boolScalar(res), nil
			}
			rv, err := re(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			rb, err := scalarTruth(rightWhat, rv)
			if err != nil {
				return scalar{}, err
			}
			return boolScalar(combine(lb, rb)), nil
		}
	}
	return n
}

// conditionalNode is `if c ? t else e`, evaluating one branch.
func conditionalNode(_ ast.OperatorKind, kids []*cnode) *cnode {
	c, t, e := kids[0], kids[1], kids[2]
	n := &cnode{prefix: 1 + c.prefix}
	n.emit = func(precharged bool) compiledExpr {
		var pre int64
		if !precharged {
			pre = n.prefix
		}
		ce, te, ee := c.emit(true), t.emit(false), e.emit(false)
		return func(ctx *Context, args []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			cv, err := ce(ctx, args)
			if err != nil {
				return scalar{}, err
			}
			held, err := scalarTruth("condition of 'if'", cv)
			if err != nil {
				return scalar{}, err
			}
			if held {
				return te(ctx, args)
			}
			return ee(ctx, args)
		}
	}
	return n
}

// argumentPrefix folds the steps of the leading infallible arguments into one
// charge and says, per argument, whether its own steps are in it.
func argumentPrefix(args []*cnode) (prefix int64, precharged []bool) {
	prefix = 1
	precharged = make([]bool, len(args))
	fused := true
	for i, arg := range args {
		precharged[i] = fused
		if fused {
			prefix += arg.prefix
		}
		fused = fused && arg.infallible
	}
	return prefix, precharged
}

// emitArguments emits the argument closures, each charging its own steps
// unless the call's prefix already did.
func emitArguments(args []*cnode, precharged []bool) []compiledExpr {
	exprs := make([]compiledExpr, len(args))
	for i, arg := range args {
		exprs[i] = arg.emit(precharged[i])
	}
	return exprs
}

// callNode calls a compiled calc, evaluating args in source order into the
// slots they bind; bound marks the bound parameters, the callee defaults the rest.
func callNode(callee *compiledCalc, args []*cnode, slots []int, bound paramSet) *cnode {
	n := &cnode{}
	var precharged []bool
	n.prefix, precharged = argumentPrefix(args)
	positional := true
	for i, slot := range slots {
		positional = positional && slot == i
	}
	n.emit = func(precharged0 bool) compiledExpr {
		var pre int64
		if !precharged0 {
			pre = n.prefix
		}
		exprs := emitArguments(args, precharged)
		if positional && len(exprs) == 1 {
			arg := exprs[0]
			return func(ctx *Context, frame []scalar) (scalar, error) {
				if err := ctx.chargeSteps(pre); err != nil {
					return scalar{}, err
				}
				v, err := arg(ctx, frame)
				if err != nil {
					return scalar{}, err
				}
				base := len(ctx.scalarStack)
				ctx.scalarStack = append(ctx.scalarStack, v)
				res, err := callee.invoke(ctx, base, bound)
				ctx.scalarStack = ctx.scalarStack[:base]
				return res, err
			}
		}
		if positional {
			return func(ctx *Context, frame []scalar) (scalar, error) {
				if err := ctx.chargeSteps(pre); err != nil {
					return scalar{}, err
				}
				base := len(ctx.scalarStack)
				for _, arg := range exprs {
					v, err := arg(ctx, frame)
					if err != nil {
						ctx.scalarStack = ctx.scalarStack[:base]
						return scalar{}, err
					}
					ctx.scalarStack = append(ctx.scalarStack, v)
				}
				res, err := callee.invoke(ctx, base, bound)
				ctx.scalarStack = ctx.scalarStack[:base]
				return res, err
			}
		}
		// Named arguments land in their slots as they are evaluated, so the
		// parameters' slots are reserved before the first one runs.
		nParams := len(callee.params)
		return func(ctx *Context, frame []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			base := ctx.reserveScalars(nParams)
			for i, arg := range exprs {
				v, err := arg(ctx, frame)
				if err != nil {
					ctx.scalarStack = ctx.scalarStack[:base]
					return scalar{}, err
				}
				ctx.scalarStack[base+slots[i]] = v
			}
			res, err := callee.invoke(ctx, base, bound)
			ctx.scalarStack = ctx.scalarStack[:base]
			return res, err
		}
	}
	return n
}

// reserveScalars pushes n slots on the scalar stack, to be written by slot,
// and answers the index of the first.
func (ctx *Context) reserveScalars(n int) int {
	base := len(ctx.scalarStack)
	ctx.scalarStack = slices.Grow(ctx.scalarStack, n)[:base+n]
	return base
}

// libraryArity bounds the parameters of a scalar library function the tier
// calls, so a call's arguments fit a fixed buffer.
const libraryArity = 4

// libraryCallNode calls a scalar library function: args evaluated in source
// order into slots, boxed for the very function the evaluator applies, unboxed back.
func libraryCallNode(fn *libraryFunction, args []*cnode, slots []int) *cnode {
	n := &cnode{}
	var precharged []bool
	n.prefix, precharged = argumentPrefix(args)
	nParams := len(fn.params)
	n.emit = func(precharged0 bool) compiledExpr {
		var pre int64
		if !precharged0 {
			pre = n.prefix
		}
		exprs := emitArguments(args, precharged)
		return func(ctx *Context, frame []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			var got [libraryArity]scalar
			for i, arg := range exprs {
				v, err := arg(ctx, frame)
				if err != nil {
					return scalar{}, err
				}
				got[slots[i]] = v
			}
			values := ctx.libraryArgs(nParams)
			for i := range values {
				values[i] = got[i].boxed()
			}
			res, err := fn.apply(fn.name, ctx, values)
			if err != nil {
				return scalar{}, err
			}
			return scalarOfResult(fn.name, res)
		}
	}
	return n
}

// aggregateNode calls a collection builtin over its one scalar argument, which
// the builtin reads as the one-element sequence the evaluator makes of it.
func aggregateNode(name string, fn builtinFunc, arg *cnode) *cnode {
	n := &cnode{prefix: 1 + arg.prefix}
	n.emit = func(precharged bool) compiledExpr {
		var pre int64
		if !precharged {
			pre = n.prefix
		}
		arg := arg.emit(true)
		return func(ctx *Context, frame []scalar) (scalar, error) {
			if err := ctx.chargeSteps(pre); err != nil {
				return scalar{}, err
			}
			v, err := arg(ctx, frame)
			if err != nil {
				return scalar{}, err
			}
			values := ctx.libraryArgs(1)
			values[0] = v.boxed()
			ec := &ctx.libraryEval
			*ec = EvalContext{ctx: ctx}
			res, err := fn(ec, values)
			if err != nil {
				return scalar{}, err
			}
			return scalarOfResult(name, res)
		}
	}
	return n
}

// libraryArgs is the shared buffer of one library call's boxed arguments, filled
// only after the arguments are evaluated and free again once the call returns.
func (ctx *Context) libraryArgs(n int) []Value {
	if cap(ctx.libraryArgBuf) < n {
		ctx.libraryArgBuf = make([]Value, n)
	}
	return ctx.libraryArgBuf[:n]
}

// scalarOfResult unboxes a library function's result, which a function the
// tier calls keeps on the scalar lattice.
func scalarOfResult(name string, res Value) (scalar, error) {
	out, ok := scalarOf(res)
	if !ok {
		return scalar{}, fmt.Errorf("compiled call of %s produced a non-scalar %s", name, describeValue(res))
	}
	return out, nil
}
