package codegen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrUnsupported reports notation outside the compiled subset. The message
// names the construct so the model can be adjusted or run interpreted.
var ErrUnsupported = errors.New("not compilable")

// UnsupportedError is an ErrUnsupported with the construct and calc it was met in.
type UnsupportedError struct {
	Calc string
	What string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s: in calc %s: %s", ErrUnsupported, e.Calc, e.What)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// Compiler lowers calc definitions to the codegen IR. One Compiler compiles one
// Program; the functions it produces are shared across calls within it.
type Compiler struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	funcs    map[*symbols.Symbol]*Func
	order    []*Func
	// collections is set once any compiled value is a collection.
	collections bool
}

// New returns a Compiler resolving names through resolver and typing through model.
func New(model *semantics.Model, resolver *resolve.Resolver) *Compiler {
	return &Compiler{model: model, resolver: resolver, funcs: map[*symbols.Symbol]*Func{}}
}

// Compile compiles entry and every calc it invokes, transitively.
func (c *Compiler) Compile(entry *symbols.Symbol) (*Program, error) {
	fn, err := c.compileCalc(entry)
	if err != nil {
		return nil, err
	}
	return &Program{Funcs: c.order, Entry: fn, Collections: c.collections}, nil
}

// env is the lexical environment of a body: parameters and body-local variables,
// innermost block last.
type env struct {
	frames []map[string]binding
}

// binding is the declared type of a variable, the range its elements are
// narrowed to and, for a collection, the multiplicity a write must satisfy.
// A body-expression local is read on demand, so inline names its initializer.
type binding struct {
	t      Type
	r      Range
	m      Mult
	inline Expr
}

func (e *env) push()                    { e.frames = append(e.frames, map[string]binding{}) }
func (e *env) pop()                     { e.frames = e.frames[:len(e.frames)-1] }
func (e *env) bind(n string, b binding) { e.frames[len(e.frames)-1][n] = b }
func (e *env) lookup(n string) (binding, bool) {
	for i := len(e.frames) - 1; i >= 0; i-- {
		if b, ok := e.frames[i][n]; ok {
			return b, true
		}
	}
	return binding{}, false
}

// funcCompiler compiles one calc body.
type funcCompiler struct {
	c     *Compiler
	fn    *Func
	scope *symbols.Scope
	env   env
	// result is the return binding once a declaration or a return has fixed it.
	result binding
	temps  int
}

// resultWhere names the result in a multiplicity diagnostic.
const resultWhere = "result"

// paramWhere names a parameter's binding in a multiplicity diagnostic.
func paramWhere(name string) string { return fmt.Sprintf("argument for parameter %q", name) }

func (c *Compiler) compileCalc(sym *symbols.Symbol) (*Func, error) {
	if fn, ok := c.funcs[sym]; ok {
		return fn, nil
	}
	if sym == nil || sym.Decl == nil {
		return nil, &UnsupportedError{Calc: "?", What: "no declaration"}
	}
	body, rels, err := calcDecl(sym.Decl)
	if err != nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: err.Error()}
	}
	if len(rels) > 0 {
		return c.compileInheriting(sym, body, rels)
	}

	fn := &Func{Name: c.name(sym), Ident: identOf(sym)}
	// Registered before the body is compiled so recursion finds it; the result
	// type of a recursive call is fixed by an earlier return (see compileCall).
	c.funcs[sym] = fn
	c.order = append(c.order, fn)

	fc := &funcCompiler{c: c, fn: fn, scope: sym.Scope}
	fc.env.push()
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		if u.Direction != ast.DirIn && u.Direction != ast.DirInOut {
			continue
		}
		name, _ := ast.EffectiveName(u)
		if name == "" {
			return nil, fc.unsupported("an unnamed parameter")
		}
		if u.Value != nil {
			return nil, fc.unsupported(fmt.Sprintf("parameter %s has a default value", name))
		}
		if u.Kind == ast.UsageCalc {
			return nil, fc.unsupported(fmt.Sprintf("parameter %s binds a function value", name))
		}
		b, err := fc.declaredBinding(sym.Scope, u, name)
		if err != nil {
			return nil, err
		}
		fn.Params = append(fn.Params, Param{Name: name, Type: b.t, Range: b.r, Mult: b.m})
		fc.env.bind(name, b)
	}

	stmts := lower.CalcBody(sym.Decl, body, sym.Scope)
	if !lower.Returns(stmts) {
		return nil, fc.unsupported("a calc that binds `out` features rather than returning a value")
	}
	if declared, err := fc.declaredResult(sym, body); err != nil {
		return nil, err
	} else if declared.t != TypeInvalid {
		fc.result = declared
		fn.Result = declared.t
		if declared.t.Scalar() {
			fn.ResultRange = declared.r
		}
	}
	compiled, err := fc.compileBlock(stmts)
	if err != nil {
		return nil, err
	}
	if fc.result.t == TypeInvalid {
		return nil, fc.unsupported("the result type could not be inferred")
	}
	fn.Result = fc.result.t
	fn.Body = compiled
	return fn, nil
}

// calcDecl is the body and the declared relationships of a calc def or a calc
// usage; any other declaration is not a calc.
func calcDecl(decl ast.Node) ([]ast.Node, []*ast.Relationship, error) {
	switch d := decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefCalc {
			return d.Members, d.Relationships, nil
		}
	case *ast.Usage:
		if d.Kind == ast.UsageCalc {
			return d.Members, d.Relationships, nil
		}
	}
	return nil, nil, errors.New("not a calc def or calc usage")
}

// compileInheriting compiles a calc that specializes or is typed by another.
// The interpreter flattens parameters and body along the chain of calc
// supertypes; a calc adding no member of its own is that chain's one other
// calc, so it compiles to that calc's function. Anything richer is refused.
func (c *Compiler) compileInheriting(sym *symbols.Symbol, body []ast.Node, rels []*ast.Relationship) (*Func, error) {
	var parent *symbols.Symbol
	for _, super := range c.model.DirectSupertypes(sym) {
		if super == nil || !isCalc(super.Decl) || c.resolver.Index().Library(super) {
			continue
		}
		if parent != nil && parent != super {
			return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc inheriting from several calcs"}
		}
		parent = super
	}
	if parent == nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc declaring `" + rels[0].Kind.String() + "` of something that is not a compiled calc"}
	}
	if len(unwrapped(body)) > 0 {
		return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc declaring `" + rels[0].Kind.String() + "` and members of its own; redefinition of inherited parameters and body is not compiled"}
	}
	fn, err := c.compileCalc(parent)
	if err != nil {
		return nil, err
	}
	c.funcs[sym] = fn
	return fn, nil
}

func isCalc(decl ast.Node) bool {
	_, _, err := calcDecl(decl)
	return err == nil
}

// declaredResult is the binding the `return` parameter declares, of type
// TypeInvalid when it declares none.
func (fc *funcCompiler) declaredResult(sym *symbols.Symbol, body []ast.Node) (binding, error) {
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok || !(u.IsResult || u.Direction == ast.DirOut) {
			continue
		}
		if u.Direction == ast.DirOut && !u.IsResult {
			return binding{}, fc.unsupported("an `out` parameter")
		}
		name, _ := ast.EffectiveName(u)
		if !hasTyping(u) {
			if m, err := fc.multOf(u, "the result"); err != nil {
				return binding{}, err
			} else if m != MultOne {
				return binding{}, fc.unsupported("an untyped result declaring a multiplicity")
			}
			return binding{}, nil
		}
		return fc.declaredBinding(sym.Scope, u, name)
	}
	return binding{}, nil
}

// multOf is the multiplicity a usage declares, `[1]` when it declares none.
func (fc *funcCompiler) multOf(u *ast.Usage, what string) (Mult, error) {
	rng, ok := fc.c.model.RangeOf(u.Multiplicity)
	if !ok {
		return MultOne, nil
	}
	bound := func(b semantics.Bound) (int64, error) {
		switch {
		case b.Infinite:
			return -1, nil
		case !b.Known:
			return 0, fc.unsupported(what + " declares a multiplicity that is not a literal")
		}
		return b.Value, nil
	}
	lo, err := bound(rng.Lower)
	if err != nil {
		return Mult{}, err
	}
	hi, err := bound(rng.Upper)
	if err != nil {
		return Mult{}, err
	}
	if lo < 0 {
		return Mult{}, fc.unsupported(what + " declares an infinite lower bound")
	}
	return Mult{Lower: lo, Upper: hi}, nil
}

// declaredBinding is the binding a typed usage declares: a scalar for `[1]`,
// otherwise a collection of the declared multiplicity.
func (fc *funcCompiler) declaredBinding(scope *symbols.Scope, u *ast.Usage, name string) (binding, error) {
	t, r, err := fc.declaredType(scope, u, name)
	if err != nil {
		return binding{}, err
	}
	m, err := fc.multOf(u, name)
	if err != nil {
		return binding{}, err
	}
	if m == MultOne {
		return binding{t: t, r: r, m: m}, nil
	}
	fc.c.collections = true
	return binding{t: t.Seq(), r: r, m: m}, nil
}

func hasTyping(u *ast.Usage) bool {
	for _, r := range u.Relationships {
		if r != nil && r.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// declaredType resolves the scalar type a usage is typed by, and its range.
func (fc *funcCompiler) declaredType(scope *symbols.Scope, u *ast.Usage, name string) (Type, Range, error) {
	var typ *symbols.Symbol
	for _, r := range u.Relationships {
		if r == nil || r.Kind != ast.RelTyping || r.Target == nil {
			continue
		}
		target := r.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: typing by an expression", name))
		}
		resolved, ok := fc.c.resolver.ResolveQualified(scope, qn)
		if !ok {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: type %s does not resolve", name, qnText(qn)))
		}
		if typ != nil {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s is typed more than once", name))
		}
		typ = resolved
	}
	if typ == nil {
		return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s declares no type", name))
	}
	t, r, ok := scalarType(fc.c.name(typ))
	if !ok {
		return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: type %s is not Integer, Real or Boolean", name, fc.c.name(typ)))
	}
	return t, r, nil
}

// scalarType maps a library data type to the compiled representation and the
// range a write to it is checked against.
func scalarType(fqn string) (Type, Range, bool) {
	switch fqn {
	case "ScalarValues::Integer":
		return TypeInt, RangeAny, true
	case "ScalarValues::Natural":
		return TypeInt, RangeNatural, true
	case "ScalarValues::Positive":
		return TypeInt, RangePositive, true
	case "ScalarValues::Real", "ScalarValues::Rational":
		return TypeReal, RangeAny, true
	case "ScalarValues::Boolean":
		return TypeBool, RangeAny, true
	}
	return TypeInvalid, RangeAny, false
}

func (fc *funcCompiler) compileBlock(stmts []lower.Statement) ([]Stmt, error) {
	var out []Stmt
	for _, s := range stmts {
		compiled, err := fc.compileStmt(s)
		if err != nil {
			return nil, err
		}
		if compiled != nil {
			out = append(out, compiled)
		}
	}
	return out, nil
}

func (fc *funcCompiler) compileStmt(s lower.Statement) (Stmt, error) {
	switch s := s.(type) {
	case lower.Declare:
		return fc.compileDeclare(s)
	case lower.Assign:
		if s.Chain != nil {
			return nil, fc.unsupported("assignment through a feature chain")
		}
		b, ok := fc.env.lookup(s.Target)
		if !ok {
			return nil, fc.unsupported(fmt.Sprintf("assignment to %s, which the body does not declare", s.Target))
		}
		v, err := fc.compileExpr(s.Value)
		if err != nil {
			return nil, err
		}
		v, err = fc.bind(v, b, s.Target)
		if err != nil {
			return nil, err
		}
		if b.t.Many() {
			return Assign{Name: s.Target, Value: v}, nil
		}
		return Assign{Name: s.Target, Range: b.r, Value: v}, nil
	case lower.If:
		cond, err := fc.compileBool(s.Condition, "condition of if", failStmtCondition("if"))
		if err != nil {
			return nil, err
		}
		fc.env.push()
		then, err := fc.compileBlock(s.Then.Steps())
		fc.env.pop()
		if err != nil {
			return nil, err
		}
		var els []Stmt
		if s.Else != nil {
			fc.env.push()
			els, err = fc.compileBlock(s.Else.Steps())
			fc.env.pop()
			if err != nil {
				return nil, err
			}
		}
		return If{Cond: cond, Then: then, Else: els}, nil
	case lower.Loop:
		return fc.compileLoop(s)
	case lower.Return:
		v, err := fc.compileExpr(s.Value)
		if err != nil {
			return nil, err
		}
		if fc.result.t == TypeInvalid {
			if v.Type() == TypeNull {
				return nil, fc.unsupported("a return of null from a calc declaring no result type")
			}
			fc.result = binding{t: v.Type(), m: MultAny}
			fc.fn.Result = v.Type()
		}
		v, err = fc.bind(v, fc.result, resultWhere)
		if err != nil {
			return nil, err
		}
		return Return{Value: v}, nil
	case lower.Block:
		fc.env.push()
		body, err := fc.compileBlock(s.Steps())
		fc.env.pop()
		if err != nil {
			return nil, err
		}
		return If{Cond: BoolLit{Value: true}, Then: body}, nil
	case lower.Unsupported:
		return nil, fc.unsupported(s.Description)
	case lower.DeclareUsage:
		return nil, fc.unsupported("a body-local calc usage")
	case lower.Effect:
		return nil, fc.unsupported("a statement acting outside the calc")
	default:
		return nil, fc.unsupported(fmt.Sprintf("statement %T", s))
	}
}

// compileDeclare declares a body-local attribute. Its binding is a scalar only
// when it is declared `[1]` and initialized with a scalar: an unbound attribute
// is null, and an initializer's shape is kept whatever the declaration says.
func (fc *funcCompiler) compileDeclare(s lower.Declare) (Stmt, error) {
	u, _ := s.Node.(*ast.Usage)
	var declared binding
	if u != nil && hasTyping(u) {
		b, err := fc.declaredBinding(s.Scope, u, s.Name)
		if err != nil {
			return nil, err
		}
		declared = b
	}
	if s.Value == nil {
		if declared.t == TypeInvalid {
			return nil, fc.unsupported(fmt.Sprintf("attribute %s has neither a type nor a value", s.Name))
		}
		declared.t = declared.t.Seq()
		fc.c.collections = true
		fc.env.bind(s.Name, declared)
		return Declare{Name: s.Name, T: declared.t}, nil
	}
	v, err := fc.compileExpr(s.Value)
	if err != nil {
		return nil, err
	}
	if declared.t == TypeInvalid {
		if v.Type() == TypeNull {
			return nil, fc.unsupported(fmt.Sprintf("attribute %s is null and declares no type", s.Name))
		}
		declared = binding{t: v.Type(), m: MultAny}
		if v.Type().Many() {
			declared.m = MultOne
		}
	}
	if declared.t.Scalar() && !v.Type().Scalar() {
		declared.t = declared.t.Seq()
		fc.c.collections = true
	}
	init, err := fc.bind(v, binding{t: declared.t, m: MultAny}, "")
	if err != nil {
		return nil, fc.unsupported(fmt.Sprintf("a %s bound to %s, which is %s", v.Type(), s.Name, declared.t))
	}
	fc.env.bind(s.Name, declared)
	return Declare{Name: s.Name, T: declared.t, Init: init}, nil
}

func (fc *funcCompiler) compileLoop(s lower.Loop) (Stmt, error) {
	if s.Kind == ast.LoopFor {
		return fc.compileForEach(s)
	}
	// `loop { … } until c` keeps its post-condition in Condition; a `while`
	// loop's optional `until` clause is in Until (see lower.Loop).
	cond, until := s.Condition, s.Until
	if s.Kind == ast.LoopUntil {
		cond, until = nil, s.Condition
	}
	if cond == nil && until == nil {
		return nil, fc.unsupported("a loop with neither a while nor an until condition")
	}
	loop := While{Cond: BoolLit{Value: true}}
	if cond != nil {
		c, err := fc.compileBool(cond, "condition of while", failStmtCondition("while"))
		if err != nil {
			return nil, err
		}
		loop.Cond = c
	}
	// The post-condition is tested after the body, in its scope, so it can
	// read attributes the body declares.
	fc.env.push()
	defer fc.env.pop()
	body, err := fc.compileBlock(s.Body.Steps())
	if err != nil {
		return nil, err
	}
	loop.Body = body
	if until != nil {
		u, err := fc.compileBool(until, "condition of until", failStmtCondition("until"))
		if err != nil {
			return nil, err
		}
		loop.Until = u
	}
	return loop, nil
}

// compileBool compiles a Boolean operand; fail is the run-time message for a
// collection or null there (%s the shape found), what names it at compile time.
func (fc *funcCompiler) compileBool(n ast.Node, what, fail string) (Expr, error) {
	v, err := fc.compileExpr(n)
	if err != nil {
		return nil, err
	}
	if v.Type() == TypeSeqBool {
		v = ToOne{X: v, Fail: fail, Bare: true}
	}
	if v.Type() != TypeBool {
		return nil, fc.unsupported(fmt.Sprintf("%s is %s, not Boolean", what, v.Type()))
	}
	return v, nil
}

// coerce widens an Integer operand to a Real where a Real is expected; any
// other mismatch is outside the subset.
func (fc *funcCompiler) coerce(v Expr, t Type, what string) (Expr, error) {
	switch {
	case v.Type() == t:
		return v, nil
	case v.Type() == TypeInt && t == TypeReal, v.Type() == TypeSeqInt && t == TypeSeqReal:
		return ToReal{X: v}, nil
	case v.Type() == TypeNull && t.Many():
		return fc.retype(v, t), nil
	case v.Type().Scalar() && t.Many():
		return fc.toMany(v, t, what)
	}
	return nil, fc.unsupported(fmt.Sprintf("a %s %s where %s is expected", v.Type(), what, t))
}

func (fc *funcCompiler) compileExpr(n ast.Node) (Expr, error) {
	switch n := n.(type) {
	case *ast.LiteralInteger:
		v, err := strconv.ParseInt(n.Value, 10, 64)
		if err != nil {
			return nil, fc.unsupported(fmt.Sprintf("literal %s is outside the Integer range", n.Value))
		}
		return IntLit{Value: v}, nil
	case *ast.LiteralReal:
		v, err := semantics.ParseReal(n.Value)
		if err != nil {
			return nil, fc.unsupported(fmt.Sprintf("literal %s is outside the Real range", n.Value))
		}
		return RealLit{Value: v}, nil
	case *ast.LiteralBool:
		return BoolLit{Value: n.Value}, nil
	case *ast.FeatureReference:
		return fc.compileName(n.Name)
	case *ast.OperatorExpr:
		return fc.compileOperator(n)
	case *ast.InvocationExpr:
		return fc.compileCall(n)
	case *ast.BodyExpr:
		// A parenthesized expression parses as a body holding one expression.
		if len(n.Members) == 1 {
			if inner := unwrap(n.Members[0]); inner != nil {
				if _, decl := inner.(*ast.Usage); !decl {
					return fc.compileExpr(inner)
				}
			}
		}
		return nil, fc.unsupported("an expression body outside a collection operation")
	case *ast.NullExpr:
		return NullLit{T: TypeNull}, nil
	case *ast.SequenceExpr:
		return fc.compileSequence(n)
	case *ast.IndexExpr:
		return fc.compileIndex(n)
	case *ast.CollectExpr:
		return fc.compileBodyOp(SeqCollect, n.Operand, n.Body)
	case *ast.SelectExpr:
		return fc.compileBodyOp(SeqSelect, n.Operand, n.Body)
	}
	return nil, fc.unsupported(fmt.Sprintf("expression %T", n))
}

func unwrap(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func (fc *funcCompiler) compileName(qn *ast.QualifiedName) (Expr, error) {
	if qn != nil && len(qn.Parts) == 1 {
		if b, ok := fc.env.lookup(qn.Parts[0].Text); ok {
			if b.inline != nil {
				return b.inline, nil
			}
			return Var{Name: qn.Parts[0].Text, T: b.t}, nil
		}
	}
	// A library constant reads as its value, as the interpreter's feature seam gives it.
	if qn != nil {
		if sym, ok := fc.c.resolver.ResolveQualified(fc.scope, qn); ok && fc.c.resolver.Index().Library(sym) {
			if v, ok := libFeatureValue(fc.c.name(sym)); ok {
				return v, nil
			}
		}
	}
	return nil, fc.unsupported(fmt.Sprintf("reference to %s, which is not a parameter, body-local attribute or compiled library constant", qnText(qn)))
}

func (fc *funcCompiler) compileOperator(n *ast.OperatorExpr) (Expr, error) {
	switch n.Operator {
	case ast.OpConditional:
		if len(n.Operands) != 3 {
			return nil, fc.unsupported("an `if` with no else")
		}
		cond, err := fc.compileBool(n.Operands[0], "condition of if", "type mismatch: condition of 'if' must be Boolean, got %s")
		if err != nil {
			return nil, err
		}
		then, err := fc.compileExpr(n.Operands[1])
		if err != nil {
			return nil, err
		}
		els, err := fc.compileExpr(n.Operands[2])
		if err != nil {
			return nil, err
		}
		t, err := fc.unify(then, els, "branches of if")
		if err != nil {
			return nil, err
		}
		if t == TypeNull {
			return nil, fc.unsupported("an `if` whose branches are both null")
		}
		if then, err = fc.coerce(then, t, "branch of if"); err != nil {
			return nil, err
		}
		if els, err = fc.coerce(els, t, "branch of if"); err != nil {
			return nil, err
		}
		return Cond{C: cond, Then: then, Else: els, T: t}, nil
	case ast.OpNullCoalesce:
		return fc.compileCoalesce(n)
	case ast.OpRange:
		return fc.compileRange(n)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		l, r, t, wrap, err := fc.numericOperands(n)
		if err != nil {
			return nil, err
		}
		if n.Operator == ast.OpDiv {
			// A quotient of Integers is a Rational.
			t = TypeReal
		}
		if n.Operator == ast.OpPow && t == TypeInt {
			// Integer ** Integer is an Integer only for a non-negative exponent,
			// a distinction a static type cannot make of a run-time exponent.
			lit, isLit := r.(IntLit)
			switch {
			case isLit && lit.Value >= 0:
			case isLit:
				t = TypeReal
				l, r = ToReal{X: l}, ToReal{X: r}
			default:
				return nil, fc.unsupported("`**` of an Integer by a non-literal Integer exponent (write the exponent as a literal, or make the base Real)")
			}
		}
		return wrap(Binary{Op: n.Operator, L: l, R: r, T: t}), nil
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		l, r, _, wrap, err := fc.numericOperands(n)
		if err != nil {
			return nil, err
		}
		return wrap(Binary{Op: n.Operator, L: l, R: r, T: TypeBool}), nil
	case ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq:
		return fc.compileEquality(n)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		// The right operand is checked only after the left has been read and,
		// for the lazy operators, evaluated only if the left does not decide.
		l, r, err := fc.rawOperands(n)
		if err != nil {
			return nil, err
		}
		if l, err = fc.scalarOperand(l, r, n.Operator, true); err != nil {
			return nil, err
		}
		if r, err = fc.scalarOperand(r, l, n.Operator, false); err != nil {
			return nil, err
		}
		if l.Type() != TypeBool || r.Type() != TypeBool {
			return nil, fc.unsupported(fmt.Sprintf("'%s' over non-Boolean operands", n.Operator))
		}
		return Binary{Op: n.Operator, L: l, R: r, T: TypeBool}, nil
	case ast.OpNeg, ast.OpPos:
		if len(n.Operands) != 1 {
			return nil, fc.unsupported("a unary operator with two operands")
		}
		// The least Integer is only writable as a negated literal.
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok && n.Operator == ast.OpNeg {
			if v, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
				return IntLit{Value: v}, nil
			}
		}
		x, err := fc.compileExpr(n.Operands[0])
		if err != nil {
			return nil, err
		}
		if x, err = fc.scalarOperandWith(x, fmt.Sprintf("type mismatch: unary '%s' requires numeric operand, got %%s", n.Operator), true, fmt.Sprintf("'%s'", n.Operator)); err != nil {
			return nil, err
		}
		if x.Type() == TypeBool {
			return nil, fc.unsupported(fmt.Sprintf("'%s' over a Boolean", n.Operator))
		}
		return Unary{Op: n.Operator, X: x, T: x.Type()}, nil
	case ast.OpNot:
		if len(n.Operands) != 1 {
			return nil, fc.unsupported("a unary operator with two operands")
		}
		x, err := fc.compileBool(n.Operands[0], "operand of not", "type mismatch: logical not requires bool operand, got %s")
		if err != nil {
			return nil, err
		}
		return Unary{Op: n.Operator, X: x, T: TypeBool}, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("operator '%s'", n.Operator))
}

// binaryOperands compiles both operands of a strict operator n as scalars: a
// collection operand is taken as the one value it holds, failing as the
// interpreter's operator does. Two collection operands are evaluated once each
// into temporaries the returned wrap binds around the operation, so a failure
// can describe both.
func (fc *funcCompiler) binaryOperands(n *ast.OperatorExpr) (Expr, Expr, func(Expr) Expr, error) {
	l, r, err := fc.rawOperands(n)
	if err != nil {
		return nil, nil, nil, err
	}
	wrap := func(x Expr) Expr { return x }
	if l.Type().Many() && r.Type().Many() {
		var lets []Let
		l, lets = fc.hoist(l, lets)
		r, lets = fc.hoist(r, lets)
		wrap = func(x Expr) Expr {
			for i := len(lets) - 1; i >= 0; i-- {
				x = Let{Name: lets[i].Name, Value: lets[i].Value, In: x}
			}
			return x
		}
	}
	if l, err = fc.scalarOperand(l, r, n.Operator, true); err != nil {
		return nil, nil, nil, err
	}
	if r, err = fc.scalarOperand(r, l, n.Operator, false); err != nil {
		return nil, nil, nil, err
	}
	return l, r, wrap, nil
}

// hoist evaluates an impure v into a fresh temporary, appended to lets, and
// returns the Var reading it; a pure v is returned as is.
func (fc *funcCompiler) hoist(v Expr, lets []Let) (Expr, []Let) {
	if pure(v) {
		return v, lets
	}
	fc.temps++
	// A NUL cannot occur in a source name, so the temporary shadows nothing.
	name := fmt.Sprintf("\x00%d", fc.temps)
	return Var{Name: name, T: v.Type()}, append(lets, Let{Name: name, Value: v})
}

func (fc *funcCompiler) rawOperands(n *ast.OperatorExpr) (Expr, Expr, error) {
	if len(n.Operands) != 2 {
		return nil, nil, fc.unsupported(fmt.Sprintf("'%s' with %d operands", n.Operator, len(n.Operands)))
	}
	l, err := fc.compileExpr(n.Operands[0])
	if err != nil {
		return nil, nil, err
	}
	r, err := fc.compileExpr(n.Operands[1])
	if err != nil {
		return nil, nil, err
	}
	return l, r, nil
}

// numericOperands compiles both operands and widens them to a common numeric type.
func (fc *funcCompiler) numericOperands(n *ast.OperatorExpr) (Expr, Expr, Type, func(Expr) Expr, error) {
	l, r, wrap, err := fc.binaryOperands(n)
	if err != nil {
		return nil, nil, TypeInvalid, nil, err
	}
	if l.Type() == TypeBool || r.Type() == TypeBool {
		return nil, nil, TypeInvalid, nil, fc.unsupported(fmt.Sprintf("'%s' over a Boolean", n.Operator))
	}
	t, _ := fc.unify(l, r, "")
	l, _ = fc.coerce(l, t, "")
	r, _ = fc.coerce(r, t, "")
	return l, r, t, wrap, nil
}

// unify is the common type of two numeric expressions: Real if either is.
func (fc *funcCompiler) unify(a, b Expr, what string) (Type, error) {
	at, bt := a.Type(), b.Type()
	switch {
	case at == bt:
		return at, nil
	case at == TypeNull:
		return bt.Seq(), nil
	case bt == TypeNull:
		return at.Seq(), nil
	case at.Elem() == bt.Elem():
		return at.Seq(), nil
	case at.Elem() == TypeBool || bt.Elem() == TypeBool:
		return TypeInvalid, fc.unsupported(fmt.Sprintf("%s are %s and %s", what, at, bt))
	}
	if at.Scalar() && bt.Scalar() {
		return TypeReal, nil
	}
	return TypeSeqReal, nil
}

func (fc *funcCompiler) compileCall(n *ast.InvocationExpr) (Expr, error) {
	if n.Type == nil {
		return nil, fc.unsupported("an invocation naming no calc")
	}
	// The declaration the checker and interpreter select by the arguments' types.
	sel := passes.SelectInvocation(fc.c.resolver, fc.c.model, fc.scope, n, semantics.PerformsBehavior)
	if sel.Ambiguous {
		names := make([]string, len(sel.Tied))
		for i, tied := range sel.Tied {
			names[i] = fc.c.name(tied)
		}
		return nil, fc.unsupported(fmt.Sprintf("invocation of %s, which is ambiguous between %s",
			qnText(n.Type), strings.Join(names, ", ")))
	}
	sym := sel.Called()
	if sym == nil {
		// As the interpreter: a name that denotes nothing is not the library
		// function of that name.
		written := qnText(n.Type)
		if len(n.Type.Parts) == 1 && !n.Type.Global {
			written = fc.c.resolver.UnresolvedName(fc.scope, written, n.Type)
		}
		return nil, fc.unsupported(fmt.Sprintf("invocation of %s, which does not resolve", written))
	}
	if !isCalc(sym.Decl) {
		return nil, fc.unsupported(fmt.Sprintf("invocation of %s, which is not a calc", fc.c.name(sym)))
	}
	if fc.c.resolver.Index().Library(sym) {
		return fc.compileLibCall(n, fc.c.name(sym))
	}
	if n.Operand != nil {
		return nil, fc.unsupported("an invocation of a model calc with a receiver (`x->F()`)")
	}
	callee, err := fc.c.compileCalc(sym)
	if err != nil {
		return nil, err
	}
	params := make([]string, len(callee.Params))
	for i, p := range callee.Params {
		params[i] = p.Name
	}
	args, err := fc.bindArgs(n, callee.Name, params)
	if err != nil {
		return nil, err
	}
	for i, a := range args {
		p := callee.Params[a.Param]
		// The callee checks multiplicity and range on entry; only the shape is bound here.
		if args[i].Value, err = fc.bind(a.Value, binding{t: p.Type, m: MultAny}, paramWhere(p.Name)); err != nil {
			return nil, err
		}
	}
	if callee.Result == TypeInvalid {
		// Reached only from inside callee's own body, before a return fixed it.
		if fc.fn != callee || fc.result.t == TypeInvalid {
			return nil, fc.unsupported(fmt.Sprintf("call of %s before a return fixes its result type; declare its return type", callee.Name))
		}
		callee.Result = fc.result.t
	}
	return Call{Fn: callee, Args: args}, nil
}

// compileLibCall compiles a call of the library function fqn; the operation is
// chosen from the operand types, as the interpreter's kind-preserving functions are.
func (fc *funcCompiler) compileLibCall(n *ast.InvocationExpr, fqn string) (Expr, error) {
	if op, realAgg, ok := seqOpByName(fqn); ok {
		return fc.compileSeqCall(n, op, realAgg)
	}
	if n.Operand != nil {
		return nil, fc.unsupported(fmt.Sprintf("an invocation of %s with a receiver (`x->F()`)", fqn))
	}
	params, ok := runtime.LibraryFunctionParams(fqn)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("library function %s is not compiled", fqn))
	}
	args, err := fc.bindArgs(n, fqn, params)
	if err != nil {
		return nil, err
	}
	types := make([]Type, len(params))
	for i, a := range args {
		if args[i].Value, err = fc.scalarOperandOf(a.Value, fqn, params[a.Param]); err != nil {
			return nil, err
		}
		types[a.Param] = args[i].Value.Type()
	}
	op, why := libOpFor(fqn, types)
	if why != "" {
		return nil, fc.unsupported(why)
	}
	for i, a := range args {
		if args[i].Value, err = fc.coerce(a.Value, op.Operands()[a.Param], fmt.Sprintf("argument for %s of %s", params[a.Param], fqn)); err != nil {
			return nil, err
		}
	}
	return LibCall{Op: op, Args: args}, nil
}

// bindArgs compiles n's arguments in source order, each bound to one of params;
// every parameter must be bound, by position or by name.
func (fc *funcCompiler) bindArgs(n *ast.InvocationExpr, callee string, params []string) ([]Arg, error) {
	var args []Arg
	bound := make([]bool, len(params))
	if len(n.NamedArgs) > 0 {
		for _, na := range n.NamedArgs {
			i := paramIndex(params, na.Name)
			if i < 0 {
				return nil, fc.unsupported(fmt.Sprintf("%s has no parameter %s", callee, qnText(na.Name)))
			}
			if bound[i] {
				return nil, fc.unsupported(fmt.Sprintf("%s binds parameter %s twice", callee, params[i]))
			}
			v, err := fc.compileExpr(na.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, Arg{Param: i, Value: v})
			bound[i] = true
		}
	} else {
		if len(n.Args) != len(params) {
			return nil, fc.unsupported(fmt.Sprintf("%s takes %d arguments, %d given", callee, len(params), len(n.Args)))
		}
		for i, a := range n.Args {
			v, err := fc.compileExpr(a)
			if err != nil {
				return nil, err
			}
			args = append(args, Arg{Param: i, Value: v})
			bound[i] = true
		}
	}
	for i, b := range bound {
		if !b {
			return nil, fc.unsupported(fmt.Sprintf("%s: parameter %s is not bound", callee, params[i]))
		}
	}
	return args, nil
}

func paramIndex(params []string, name *ast.QualifiedName) int {
	if name == nil || len(name.Parts) != 1 {
		return -1
	}
	for i, p := range params {
		if p == name.Parts[0].Text {
			return i
		}
	}
	return -1
}

func (fc *funcCompiler) unsupported(what string) error {
	return &UnsupportedError{Calc: fc.fn.Name, What: what}
}

func (c *Compiler) name(sym *symbols.Symbol) string {
	if idx := c.resolver.Index(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}

func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	return strings.Join(parts, "::")
}

// unwrapped is members with each membership wrapper removed.
func unwrapped(members []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(members))
	for _, m := range members {
		if inner := unwrap(m); inner != nil {
			out = append(out, inner)
		}
	}
	return out
}

// identOf gives a symbol a C/Go identifier, injectively: names of the owner
// chain are joined by `_s_`, non-alphanumeric runes (`_` too) become `_hex_`.
func identOf(sym *symbols.Symbol) string {
	var names []string
	for s := sym; s != nil; {
		names = append(names, s.Name)
		if s.OwnerScope == nil {
			break
		}
		s = s.OwnerScope.Owner()
	}
	var b strings.Builder
	b.WriteString("calc")
	for i := len(names) - 1; i >= 0; i-- {
		b.WriteString("_s_")
		encodeName(&b, names[i])
	}
	return b.String()
}

func encodeName(b *strings.Builder, name string) {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			fmt.Fprintf(b, "_%x_", r)
		}
	}
}
