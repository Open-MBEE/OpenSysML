package codegen

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// bind coerces v to the shape of binding b at where: a scalar binding takes the
// one value v holds, a collection binding views a scalar as one element and
// checks multiplicity and range.
func (fc *funcCompiler) bind(v Expr, b binding, where string) (Expr, error) {
	if b.t.Elem() == TypeReal && v.Type().Elem() == TypeInt {
		v = ToReal{X: v}
	}
	vt := v.Type()
	if b.t.Scalar() {
		switch {
		case vt == b.t:
			return v, nil
		case vt == TypeNull:
			return ToOne{X: fc.retype(v, b.t.Seq()), Where: where}, nil
		case vt.Many() && vt.Elem() == b.t:
			fc.c.collections = true
			return ToOne{X: v, Where: where}, nil
		}
		return nil, fc.unsupported(fmt.Sprintf("a %s bound at %s, which holds %s", vt, where, b.t))
	}
	fc.c.collections = true
	many, err := fc.toMany(v, b.t, where)
	if err != nil {
		return nil, err
	}
	if b.m == MultAny && b.r == RangeAny {
		return many, nil
	}
	return Checked{X: many, M: b.m, R: b.r, Where: where}, nil
}

// toMany views v as a collection of type t.
func (fc *funcCompiler) toMany(v Expr, t Type, what string) (Expr, error) {
	fc.c.collections = true
	switch vt := v.Type(); {
	case vt == TypeNull:
		return fc.retype(v, t), nil
	case vt == t:
		return v, nil
	case vt == t.Elem():
		return ToMany{X: v}, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("a %s at %s, which holds %s", v.Type(), what, t))
}

// retype gives an expression of TypeNull (null, or a sequence of nulls) the
// collection type t.
func (fc *funcCompiler) retype(v Expr, t Type) Expr {
	switch v := v.(type) {
	case NullLit:
		return NullLit{T: t}
	case SeqLit:
		return SeqLit{Elems: v.Elems, T: t}
	}
	return v
}

// scalarOperand is v as the left or right operand of the binary operator op,
// other being the operand across from it: a collection yields the one value it
// holds and fails otherwise, worded as the interpreter's operator does.
func (fc *funcCompiler) scalarOperand(v, other Expr, op ast.OperatorKind, left bool) (Expr, error) {
	if !v.Type().Many() {
		return fc.scalarOperandWith(v, "", false, fmt.Sprintf("'%s'", op))
	}
	fc.c.collections = true
	side := "right"
	if left {
		side = "left"
	}
	switch op {
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return fc.pairOperand(v, other, left, "comparison operands must be constants, got %s and %s", true, "constant"), nil
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		return ToOne{X: v, Fail: fmt.Sprintf("type mismatch: %s operand of '%s' must be Boolean, got %%s", side, op), Bare: true}, nil
	}
	return fc.pairOperand(v, other, left, fmt.Sprintf("type mismatch: operator '%s' is not defined for %%s and %%s", op), false, ""), nil
}

// pairOperand is the ToOne of v whose message names both operands. A scalar
// other reads as fixed, or by its type (an Integer) unless bare; a collection
// other still to be checked has its description filled at run time.
func (fc *funcCompiler) pairOperand(v, other Expr, left bool, fail string, bare bool, fixed string) Expr {
	desc := fixed
	if desc == "" {
		desc = article(other.Type().Elem())
	}
	if left && other.Type().Many() {
		return ToOne{X: v, Fail: fail, Bare: bare, Other: other, OtherOne: desc}
	}
	if left {
		return ToOne{X: v, Fail: strings.Replace(fail, " and %s", " and "+desc, 1), Bare: bare}
	}
	return ToOne{X: v, Fail: strings.Replace(fail, "%s and ", desc+" and ", 1), Bare: bare}
}

// failStmtCondition is the interpreter's wording for a non-Boolean condition of
// the if, while or until statement kw.
func failStmtCondition(kw string) string {
	return fmt.Sprintf("condition of '%s' must evaluate to a Boolean, got %%s", kw)
}

// scalarOperandOf is v as the argument for param of the scalar library function fqn.
func (fc *funcCompiler) scalarOperandOf(v Expr, fqn, param string) (Expr, error) {
	return fc.scalarOperandWith(v, fmt.Sprintf("type mismatch: function %s parameter %q requires a numeric value", fqn, param), false, fqn)
}

// scalarOperandWith is v as a scalar; a collection fails with fail (%s the shape
// found, bare without its article) and null is outside the subset.
func (fc *funcCompiler) scalarOperandWith(v Expr, fail string, bare bool, what string) (Expr, error) {
	switch vt := v.Type(); {
	case vt.Scalar():
		return v, nil
	case vt.Many():
		fc.c.collections = true
		return ToOne{X: v, Fail: fail, Bare: bare}, nil
	}
	return nil, fc.unsupported("null as an operand of " + what)
}

// compileSequence is `(a, b, …)`: each operand contributes its elements, so
// nested sequences flatten and null contributes nothing.
func (fc *funcCompiler) compileSequence(n *ast.SequenceExpr) (Expr, error) {
	fc.c.collections = true
	elems := make([]Expr, len(n.Elements))
	elem := TypeInvalid
	for i, e := range n.Elements {
		v, err := fc.compileExpr(e)
		if err != nil {
			return nil, err
		}
		if t := v.Type(); t != TypeNull {
			if elem != TypeInvalid && elem != t.Elem() {
				return nil, fc.unsupported(fmt.Sprintf("a sequence mixing %s and %s elements", elem, t.Elem()))
			}
			elem = t.Elem()
		}
		elems[i] = v
	}
	if elem == TypeInvalid {
		return SeqLit{Elems: elems, T: TypeNull}, nil
	}
	return SeqLit{Elems: elems, T: elem.Seq()}, nil
}

// compileIndex is `x#(i)`, one-based; `x[i]` is a quantity, not an index.
func (fc *funcCompiler) compileIndex(n *ast.IndexExpr) (Expr, error) {
	if n.Bracket {
		return nil, fc.unsupported("a quantity expression `x[unit]`")
	}
	x, err := fc.compileExpr(n.Operand)
	if err != nil {
		return nil, err
	}
	seq, err := fc.toMany(x, x.Type().Seq(), "the operand of '#'")
	if err != nil {
		return nil, err
	}
	if seq.Type() == TypeNull {
		seq = fc.retype(seq, TypeSeqInt)
	}
	i, err := fc.compileExpr(n.Index)
	if err != nil {
		return nil, err
	}
	if i, err = fc.scalarOperandWith(i, "type mismatch: sequence index requires an Integer index, got %s", true, "'#'"); err != nil {
		return nil, err
	}
	if i.Type() != TypeInt {
		return nil, fc.unsupported(fmt.Sprintf("a %s as a sequence index", i.Type()))
	}
	return Index{Seq: seq, I: i}, nil
}

// compileRange is `lo..hi`, the Integers from lo to hi inclusive.
func (fc *funcCompiler) compileRange(n *ast.OperatorExpr) (Expr, error) {
	l, r, wrap, err := fc.binaryOperands(n)
	if err != nil {
		return nil, err
	}
	if l.Type() != TypeInt || r.Type() != TypeInt {
		return nil, fc.unsupported(fmt.Sprintf("'..' over %s and %s", l.Type(), r.Type()))
	}
	fc.c.collections = true
	return wrap(RangeExpr{Lo: l, Hi: r}), nil
}

// compileCoalesce is `l ?? r`: l unless it is null.
func (fc *funcCompiler) compileCoalesce(n *ast.OperatorExpr) (Expr, error) {
	l, r, err := fc.rawOperands(n)
	if err != nil {
		return nil, err
	}
	if l.Type().Scalar() {
		return l, nil
	}
	t := l.Type()
	if t == TypeNull {
		t = r.Type().Seq()
	}
	if t == TypeNull {
		return nil, fc.unsupported("'??' between two nulls")
	}
	if l, err = fc.toMany(l, t, "the left operand of '??'"); err != nil {
		return nil, err
	}
	if r, err = fc.toMany(r, t, "the right operand of '??'"); err != nil {
		return nil, err
	}
	return Coalesce{L: l, R: r, T: t}, nil
}

// compileEquality is `==`, `!=`, `===` and `!==`. Scalars compare as numbers
// (Integer and Real alike, except under `===`); anything else compares as
// collections: same shape, same elements in order.
func (fc *funcCompiler) compileEquality(n *ast.OperatorExpr) (Expr, error) {
	l, r, err := fc.rawOperands(n)
	if err != nil {
		return nil, err
	}
	neq := n.Operator == ast.OpNeq || n.Operator == ast.OpNeqEqEq
	ident := n.Operator == ast.OpEqEqEq || n.Operator == ast.OpNeqEqEq
	lt, rt := l.Type(), r.Type()
	if (lt == TypeBool) != (rt == TypeBool) && lt != TypeNull && rt != TypeNull {
		return nil, fc.unsupported("equality between a Boolean and a number")
	}
	if lt.Scalar() && rt.Scalar() {
		op := ast.OpEq
		if neq {
			op = ast.OpNeq
		}
		if lt != rt {
			if ident {
				return nil, fc.unsupported(fmt.Sprintf("'%s' between %s and %s", n.Operator, lt, rt))
			}
			t, _ := fc.unify(l, r, "")
			l, _ = fc.coerce(l, t, "")
			r, _ = fc.coerce(r, t, "")
		}
		return Binary{Op: op, L: l, R: r, T: TypeBool}, nil
	}
	fc.c.collections = true
	le, re := lt.Elem(), rt.Elem()
	switch {
	case lt == TypeNull && rt == TypeNull:
		le, re = TypeInt, TypeInt
	case lt == TypeNull:
		le = re
	case rt == TypeNull:
		re = le
	}
	if le != re {
		if ident {
			return nil, fc.unsupported(fmt.Sprintf("'%s' between %s and %s", n.Operator, lt, rt))
		}
		le, re = TypeReal, TypeReal
	}
	if l, err = fc.toMany(l, le.Seq(), "the left operand of '"+n.Operator.String()+"'"); err != nil {
		return nil, err
	}
	if r, err = fc.toMany(r, re.Seq(), "the right operand of '"+n.Operator.String()+"'"); err != nil {
		return nil, err
	}
	return SeqEq{L: l, R: r, Neq: neq, Ident: ident}, nil
}

// widen is v as a collection of Reals.
func widen(v Expr) Expr {
	if v.Type() == TypeSeqReal {
		return v
	}
	return ToReal{X: v}
}

// compileSeqCall is a call of a collection operation. The receiver of
// `x->op(…)` is its first argument.
func (fc *funcCompiler) compileSeqCall(n *ast.InvocationExpr, op SeqOp, realAgg bool) (Expr, error) {
	if len(n.NamedArgs) > 0 {
		return nil, fc.unsupported(fmt.Sprintf("named arguments to %s", op.Name()))
	}
	nodes := n.Args
	if n.Operand != nil {
		nodes = append([]ast.Node{n.Operand}, n.Args...)
	}
	spec := op.spec()
	if op.Body() > 0 {
		if len(nodes) != len(spec.params)+1 {
			return nil, fc.unsupported(fmt.Sprintf("%s with %d arguments", op.Name(), len(nodes)))
		}
		body, ok := nodes[len(nodes)-1].(*ast.BodyExpr)
		if !ok {
			return nil, fc.unsupported(fmt.Sprintf("%s whose body is not an expression body", op.Name()))
		}
		return fc.compileBodyOp(op, nodes[0], body)
	}
	if len(nodes) < len(spec.params)-spec.optional || len(nodes) > len(spec.params) {
		return nil, fc.unsupported(fmt.Sprintf("%s with %d arguments", op.Name(), len(nodes)))
	}
	args := make([]Expr, len(nodes))
	for i, a := range nodes {
		v, err := fc.compileExpr(a)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	return fc.seqCall(op, realAgg, args)
}

// seqCall types a value operation: collection operands share one element
// type, Integer operands are scalars.
func (fc *funcCompiler) seqCall(op SeqOp, realAgg bool, args []Expr) (Expr, error) {
	fc.c.collections = true
	spec := op.spec()
	elem := TypeInvalid
	for i, a := range args {
		if spec.params[i] != paramSeq || a.Type() == TypeNull {
			continue
		}
		switch t := a.Type().Elem(); {
		case elem == TypeInvalid:
			elem = t
		case elem != t && (elem == TypeBool || t == TypeBool):
			return nil, fc.unsupported(fmt.Sprintf("%s over Boolean and numeric collections", op.Name()))
		case elem != t && !predicateOp(op):
			return nil, fc.unsupported(fmt.Sprintf("%s over %s and %s collections", op.Name(), elem, t))
		case elem != t:
			elem = TypeReal
		}
	}
	if elem == TypeInvalid {
		elem = TypeInt
	}
	if realAgg && elem == TypeInt {
		elem = TypeReal
	}
	var err error
	for i, a := range args {
		what := fmt.Sprintf("argument %d of %s", i+1, op.Name())
		if spec.params[i] == paramInt {
			if args[i], err = fc.scalarOperandWith(a, "type mismatch: "+op.Name()+" requires an Integer index, got %s", true, op.Name()); err != nil {
				return nil, err
			}
			if args[i].Type() != TypeInt {
				return nil, fc.unsupported(fmt.Sprintf("a %s as %s", a.Type(), what))
			}
			continue
		}
		if a.Type() != TypeNull && a.Type().Elem() != elem {
			a = widen(a)
		}
		if args[i], err = fc.toMany(a, elem.Seq(), what); err != nil {
			return nil, err
		}
	}
	t, err := fc.seqResult(op, elem)
	if err != nil {
		return nil, err
	}
	return SeqCall{Op: op, Args: args, T: t}, nil
}

// predicateOp reports whether an operation only compares its operands'
// elements, so Integers and Reals may be compared as numbers.
func predicateOp(op SeqOp) bool {
	switch op {
	case SeqIncludes, SeqIncludesOnly, SeqExcludes, SeqEquals:
		return true
	}
	return false
}

// seqResult is the type of a value operation over collections of elem.
func (fc *funcCompiler) seqResult(op SeqOp, elem Type) (Type, error) {
	switch op {
	case SeqSize:
		return TypeInt, nil
	case SeqIsEmpty, SeqNotEmpty, SeqIncludes, SeqIncludesOnly, SeqExcludes, SeqEquals, SeqSame:
		return TypeBool, nil
	case SeqAllTrue, SeqAnyTrue:
		if elem != TypeBool {
			return TypeInvalid, fc.unsupported(fmt.Sprintf("%s over %s elements", op.Name(), elem))
		}
		return TypeBool, nil
	case SeqSum, SeqProduct:
		if elem == TypeBool {
			return TypeInvalid, fc.unsupported(fmt.Sprintf("%s over Boolean elements", op.Name()))
		}
		return elem, nil
	}
	return elem.Seq(), nil
}

// compileBodyOp is an operation applying body per element of operand.
func (fc *funcCompiler) compileBodyOp(op SeqOp, operand ast.Node, body ast.Node) (Expr, error) {
	fc.c.collections = true
	b, ok := body.(*ast.BodyExpr)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("%s whose body is not an expression body", op.Name()))
	}
	x, err := fc.compileExpr(operand)
	if err != nil {
		return nil, err
	}
	seq, err := fc.toMany(x, x.Type().Seq(), "the operand of "+op.Name())
	if err != nil {
		return nil, err
	}
	if seq.Type() == TypeNull {
		seq = fc.retype(seq, TypeSeqInt)
	}
	elem := seq.Type().Elem()
	if op == SeqReduce && elem == TypeBool {
		return nil, fc.unsupported("reduce over Boolean elements")
	}
	paramTypes := make([]Type, op.Body())
	for i := range paramTypes {
		paramTypes[i] = elem
	}
	lambda, err := fc.compileLambda(op, b, paramTypes)
	if err != nil {
		return nil, err
	}
	rt := lambda.Body.Type()
	var t Type
	switch op {
	case SeqSelect, SeqReject, SeqSelectOne, SeqForAll, SeqExists:
		if rt != TypeBool {
			return nil, fc.unsupported(fmt.Sprintf("%s whose body yields %s, not a Boolean", op.Name(), rt))
		}
		t = elem.Seq()
		if op == SeqForAll || op == SeqExists {
			t = TypeBool
		}
	case SeqCollect:
		if rt == TypeNull {
			return nil, fc.unsupported("collect whose body yields null")
		}
		t = rt.Seq()
	case SeqReduce:
		if rt != elem {
			return nil, fc.unsupported(fmt.Sprintf("reduce over %s elements whose body yields %s", elem, rt))
		}
		t = elem.Seq()
	case SeqMinimize, SeqMaximize:
		if rt != TypeInt && rt != TypeReal {
			return nil, fc.unsupported(fmt.Sprintf("%s whose body yields %s, not a number", op.Name(), rt))
		}
		t = rt
	default:
		return nil, fc.unsupported(op.Name() + " with a body")
	}
	return Fold{Op: op, Seq: seq, Body: lambda, T: t}, nil
}

// compileLambda compiles a body `{in a; in b; …; result}` whose parameters
// take the given types, in a frame over the enclosing environment. A local the
// body declares is read on demand, as the interpreter reads it, so its
// initializer is inlined where the result names it.
func (fc *funcCompiler) compileLambda(op SeqOp, b *ast.BodyExpr, paramTypes []Type) (Lambda, error) {
	if len(b.Params) != len(paramTypes) {
		return Lambda{}, fc.unsupported(fmt.Sprintf("%s with a body of %d parameters, not %d", op.Name(), len(b.Params), len(paramTypes)))
	}
	fc.env.push()
	defer fc.env.pop()
	var lambda Lambda
	for i, p := range b.Params {
		if p.Name == "" {
			return Lambda{}, fc.unsupported("an unnamed body parameter")
		}
		if p.Value != nil || p.IsReference || p.Multiplicity != nil {
			return Lambda{}, fc.unsupported(fmt.Sprintf("body parameter %s with a default, `ref` or a multiplicity", p.Name))
		}
		if p.Type != nil {
			sym, ok := fc.c.resolver.ResolveQualified(fc.scope, p.Type)
			if !ok {
				return Lambda{}, fc.unsupported(fmt.Sprintf("body parameter %s: type %s does not resolve", p.Name, qnText(p.Type)))
			}
			if t, _, ok := scalarType(fc.c.name(sym)); !ok || t != paramTypes[i] {
				return Lambda{}, fc.unsupported(fmt.Sprintf("body parameter %s typed other than %s", p.Name, paramTypes[i]))
			}
		}
		fc.env.bind(p.Name, binding{t: paramTypes[i], m: MultOne})
		lambda.Params = append(lambda.Params, Param{Name: p.Name, Type: paramTypes[i], Mult: MultOne})
	}
	var result ast.Node
	for _, member := range b.Members {
		if _, ok := unwrap(member).(*ast.Documentation); ok {
			continue
		}
		if u, ok := unwrap(member).(*ast.Usage); ok {
			decls, err := fc.compileDeclare(lower.Declare{Name: usageName(u), Value: u.Value, Node: u, Scope: fc.scope})
			if err != nil {
				return Lambda{}, err
			}
			if len(decls) != 1 {
				return Lambda{}, fc.unsupported(fmt.Sprintf("attribute %s, a SampledFunction declared inside a body expression", usageName(u)))
			}
			d := decls[0].(Declare)
			local, _ := fc.env.lookup(d.Name)
			local.inline = d.Init
			if local.inline == nil {
				local.inline = NullLit{T: d.T}
			}
			fc.env.bind(d.Name, local)
			continue
		}
		if result != nil {
			return Lambda{}, fc.unsupported("a body with more than one result expression")
		}
		result = member
	}
	if b.Result != nil {
		if result != nil {
			return Lambda{}, fc.unsupported("a body with more than one result expression")
		}
		result = b.Result
	}
	if result == nil {
		return Lambda{}, fc.unsupported("a body without a result expression")
	}
	v, err := fc.compileExpr(result)
	if err != nil {
		return Lambda{}, err
	}
	lambda.Body = v
	return lambda, nil
}

func usageName(u *ast.Usage) string {
	name, _ := ast.EffectiveName(u)
	return name
}

// compileForEach is `for x in c { … }`: c is evaluated once and its elements
// bound to x in turn; a scalar is not iterable.
func (fc *funcCompiler) compileForEach(s lower.Loop) (Stmt, error) {
	if s.Variable == "" {
		return nil, fc.unsupported("a `for` loop declaring no iteration variable")
	}
	fc.c.collections = true
	c, err := fc.compileExpr(s.Collection)
	if err != nil {
		return nil, err
	}
	if c.Type().Scalar() {
		return nil, fc.unsupported("a `for` loop over a scalar, which is not a collection")
	}
	if c.Type() == TypeNull {
		c = fc.retype(c, TypeSeqInt)
	}
	fc.env.push()
	defer fc.env.pop()
	fc.env.bind(s.Variable, binding{t: c.Type().Elem(), m: MultOne})
	body, err := fc.compileBlock(s.Body.Steps())
	if err != nil {
		return nil, err
	}
	return ForEach{Var: s.Variable, Seq: c, Body: body}, nil
}
