package codegen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// fnTables are the function values of a compilation, kept across its passes:
// the cases met, in order, and the interned sets of cases.
type fnTables struct {
	cases map[string]*FnCase
	order []*FnCase
	sets  map[string]*FnSet
	// open are the closures whose bodies are being compiled.
	open map[*Func]bool
}

func newFnTables() fnTables {
	return fnTables{cases: map[string]*FnCase{}, sets: map[string]*FnSet{}, open: map[*Func]bool{}}
}

// runPrefix begins the name of the variable holding a body's run identity; a
// NUL cannot occur in a source name.
const runPrefix = "\x00run "

// fnCase is the case of the function value v; closure is the compiled body of
// a body-local calc, whose case is one per enclosing specialization.
func (c *Compiler) fnCase(v funcValue, closure *Func) *FnCase {
	key := v.ident()
	if closure != nil {
		key = "closure " + closure.Ident
	}
	k, ok := c.fns.cases[key]
	if !ok {
		k = &FnCase{Name: c.name(v.sym), ID: len(c.fns.order), val: v}
		c.fns.cases[key] = k
		c.fns.order = append(c.fns.order, k)
	}
	if closure != nil {
		k.Closure, k.fn, k.pass = true, closure, c.pass
		k.Env = closure.Params[len(closure.Params)-closure.Captured:]
	}
	return k
}

// fnSet is the interned set of cases.
func (c *Compiler) fnSet(cases []*FnCase) *FnSet {
	cases = slices.Clone(cases)
	slices.SortFunc(cases, func(a, b *FnCase) int { return a.ID - b.ID })
	cases = slices.Compact(cases)
	ids := make([]string, len(cases))
	for i, k := range cases {
		ids[i] = strconv.Itoa(k.ID)
	}
	key := strings.Join(ids, ",")
	if s, ok := c.fns.sets[key]; ok {
		return s
	}
	s := &FnSet{Cases: cases, ID: len(c.fns.sets)}
	c.fns.sets[key] = s
	return s
}

// fnUnion is the function type, a collection if either is, over the cases of
// both function types a and b.
func (c *Compiler) fnUnion(a, b Type) Type {
	t := FnType(c.fnSet(append(slices.Clone(a.Fns.Cases), b.Fns.Cases...)))
	if a.Many() || b.Many() {
		return t.Seq()
	}
	return t
}

// fnSubset reports whether every function the function type a ranges over is
// one b ranges over.
func fnSubset(a, b Type) bool {
	for _, k := range a.Fns.Cases {
		if !slices.Contains(b.Fns.Cases, k) {
			return false
		}
	}
	return true
}

// fnRead is a read of the function value f.
func (fc *funcCompiler) fnRead(f funcValue) Expr {
	if f.dyn.IsFn() {
		return f.read
	}
	k := fc.c.fnCase(f, nil)
	return FnLit{Case: k, T: FnType(fc.c.fnSet([]*FnCase{k}))}
}

// declaring is the compiler of the body, this one or one enclosing it, that
// declares the body-local calc sym; nil when none does.
func (fc *funcCompiler) declaring(sym *symbols.Symbol) *funcCompiler {
	for o := fc; o != nil; o = o.outer {
		if o.sym == sym.Owner() {
			return o
		}
	}
	return nil
}

// closureOf compiles the body-local calc sym, over the bindings it reads from
// the body declaring it.
func (fc *funcCompiler) closureOf(sym *symbols.Symbol) (*Func, *funcCompiler, error) {
	owner := fc.declaring(sym)
	if owner == nil {
		return nil, nil, fc.unsupported(fmt.Sprintf("%s, a calc declared in the body of %s, read from the body of %s",
			fc.c.name(sym), fc.c.name(sym.Owner()), fc.fn.Name))
	}
	if fn, ok := fc.c.funcs[closureKey(sym, owner.fn)]; ok {
		return fn, owner, nil
	}
	fn, err := fc.c.compileBody(sym, nil, owner)
	return fn, owner, err
}

func closureKey(sym *symbols.Symbol, outer *Func) specKey {
	return specKey{sym: sym, fargs: "\x00in " + outer.Ident}
}

// runOf is, read in this body, the identity of the run of the body owner
// compiles, which a closure it declares carries.
func (fc *funcCompiler) runOf(owner *funcCompiler) (Expr, error) {
	name := runPrefix + owner.fn.Ident
	if owner.fn.Run == "" {
		owner.fn.Run = name
		owner.env.frames[0][name] = binding{t: TypeRun}
	}
	b, ok := fc.env.lookup(name)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("the run of %s, read where it is not bound", owner.fn.Name))
	}
	return fc.readBinding(name, b)
}

// closureValue is the body-local calc sym read as a function value.
func (fc *funcCompiler) closureValue(sym *symbols.Symbol) (funcValue, error) {
	callee, owner, err := fc.closureOf(sym)
	if err != nil {
		return funcValue{}, err
	}
	if fc.c.fns.open[callee] {
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s read as a function value within its own body", callee.Name))
	}
	k := fc.c.fnCase(funcValue{sym: sym}, callee)
	run, err := fc.runOf(owner)
	if err != nil {
		return funcValue{}, err
	}
	env, err := fc.captureValues(callee)
	if err != nil {
		return funcValue{}, err
	}
	t := FnType(fc.c.fnSet([]*FnCase{k}))
	return funcValue{sym: sym, dyn: t, read: FnLit{Case: k, Run: run, Env: env, T: t}}, nil
}

// captureValues reads, at this site, the bindings the closure callee captures.
func (fc *funcCompiler) captureValues(callee *Func) ([]Expr, error) {
	captured := callee.Params[len(callee.Params)-callee.Captured:]
	out := make([]Expr, len(captured))
	for i, p := range captured {
		b, ok := fc.env.lookup(p.Name)
		if !ok {
			return nil, fc.unsupported(fmt.Sprintf("%s, which %s reads, is not bound where %[2]s is read", p.Name, callee.Name))
		}
		v, err := fc.readBinding(p.Name, b)
		if err != nil {
			return nil, err
		}
		if out[i], err = fc.coerce(v, p.Type, "binding "+p.Name+" captured by "+callee.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// capture binds, in the closure being compiled, the binding b of name its
// enclosing body holds: a run-time value becomes a parameter the closure's
// value carries; a function value fixed statically is bound as it is.
func (fc *funcCompiler) capture(name string, b binding) (binding, bool) {
	if b.fn != nil && !b.fn.dyn.IsFn() {
		fc.env.frames[0][name] = b
		return b, true
	}
	if b.t == TypeRun {
		captured := binding{t: TypeRun}
		fc.fn.Params = append(fc.fn.Params, Param{Name: name, Type: TypeRun, Mult: MultOne})
		fc.fn.Captured++
		fc.captured[name] = true
		fc.env.frames[0][name] = captured
		return captured, true
	}
	if b.inline != nil || b.sampled != nil {
		fc.noteCapture(fmt.Sprintf("%s, a calc declared in a body, reading %s, which that body binds to no stored value", fc.fn.Name, name))
		return binding{}, false
	}
	t := b.t
	if fc.c.target == TargetC && (t == TypeString || t.Many() || t.IsFn()) {
		fc.noteCapture(fmt.Sprintf("%s capturing %s, a %s, for the C target: a C closure holds its captures inline and cannot keep one past the statement reading it (the Go target computes it)", fc.fn.Name, name, t))
		return binding{}, false
	}
	captured := binding{t: t, m: MultAny}
	if b.fn != nil {
		captured.fn = &funcValue{dyn: t, read: Var{Name: name, T: t}}
	}
	if t.Many() {
		captured.m = b.m
	}
	fc.fn.Params = append(fc.fn.Params, Param{Name: name, Type: t, Mult: MultAny})
	fc.fn.Captured++
	fc.captured[name] = true
	fc.env.frames[0][name] = captured
	return captured, true
}

// noteCapture records the first capture the closure cannot make.
func (fc *funcCompiler) noteCapture(what string) {
	if fc.captureErr == nil {
		fc.captureErr = fc.unsupported(what)
	}
}

// callClosure compiles a call of the closure callee of the calc sym, its
// captured bindings supplied by env.
func (fc *funcCompiler) callClosure(sym *symbols.Symbol, callee *Func, n *ast.InvocationExpr, env func() ([]Expr, error)) (Expr, error) {
	params, err := fc.c.calcParams(sym)
	if err != nil {
		return nil, err
	}
	args, fargs, trailing, err := fc.bindArgs(n, fc.c.name(sym), params)
	if err != nil {
		return nil, err
	}
	if len(fargs) > 0 {
		return nil, fc.unsupported(fmt.Sprintf("%s, a calc declared in a body, taking a function value", callee.Name))
	}
	if fc.c.fns.open[callee] {
		// A closure calling itself passes on what it has captured so far; its body
		// is compiled again should it capture more.
		fc.selfCalls = true
	}
	values, err := env()
	if err != nil {
		return nil, err
	}
	base := len(callee.Params) - callee.Captured
	for i, v := range values {
		args = append(args, Arg{Param: base + i, Value: v})
	}
	return fc.finishCall(callee, args, trailing)
}

// dispatch compiles an invocation, with n's arguments, of the function value
// fx: an application of each function fx may hold, chosen by the one it holds.
func (fc *funcCompiler) dispatch(fx Expr, n *ast.InvocationExpr) (Expr, error) {
	if n.Operand != nil {
		return nil, fc.unsupported("an invocation of a function value with a receiver (`x->f()`)")
	}
	t := fc.temp(fx.Type())
	held := Var{Name: t.Name, T: t.Type}
	arms := make([]Expr, len(fx.Type().Fns.Cases))
	result := TypeInvalid
	for i, k := range fx.Type().Fns.Cases {
		arm, err := fc.applyCase(k, held, n)
		if err != nil {
			return nil, err
		}
		arms[i] = arm
		if i == 0 {
			result = arm.Type()
		} else if result, err = fc.unify(Var{T: result}, arm, "results of the functions "+fc.fnNames(fx.Type())); err != nil {
			return nil, err
		}
	}
	if result == TypeNull {
		return nil, fc.unsupported("an invocation of functions that all return null")
	}
	for i, arm := range arms {
		var err error
		if arms[i], err = fc.coerce(arm, result, "result of "+fx.Type().Fns.Cases[i].Name); err != nil {
			return nil, err
		}
	}
	return FnDispatch{Name: t.Name, F: fx, Cases: arms, T: result}, nil
}

// applyCase is the application, with n's arguments, of case k held in held.
func (fc *funcCompiler) applyCase(k *FnCase, held Var, n *ast.InvocationExpr) (Expr, error) {
	if !k.Closure {
		return fc.applyFunction(k.val, n)
	}
	callee := k.fn
	if k.pass != fc.c.pass {
		// The case was met in an earlier pass; its body is compiled anew.
		var err error
		if callee, _, err = fc.closureOf(k.val.sym); err != nil {
			return nil, err
		}
	}
	return fc.callClosure(k.val.sym, callee, n, func() ([]Expr, error) {
		env := make([]Expr, len(k.Env))
		for i, p := range k.Env {
			env[i] = FnEnv{Name: held.Name, Case: k, I: i, T: p.Type}
		}
		return env, nil
	})
}

// fnNames lists the functions the function type t ranges over.
func (fc *funcCompiler) fnNames(t Type) string {
	names := make([]string, len(t.Fns.Cases))
	for i, k := range t.Fns.Cases {
		names[i] = k.Name
	}
	return strings.Join(names, ", ")
}

// fnOperator is the binary operator op over operands l and r one of which is
// a function, which no library function declares op for: a failure, worded as
// the interpreter's, once both are evaluated.
func (fc *funcCompiler) fnOperator(op ast.OperatorKind, l, r Expr) (Expr, bool, error) {
	lt, rt := l.Type(), r.Type()
	if !lt.IsFn() && !rt.IsFn() {
		return nil, false, nil
	}
	for _, t := range []Type{lt, rt} {
		if !t.Scalar() || t == TypeString {
			return nil, true, fc.unsupported(fmt.Sprintf("'%s' over %s and %s", op, lt, rt))
		}
	}
	prefix := fmt.Sprintf("type mismatch: operator '%s' is not defined for ", op)
	t := lt
	if t.IsFn() {
		t = rt
	}
	switch op {
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		// The first operand no DataValue explains the failure.
		var lets []Let
		l, lets = fc.hoist(l, lets)
		r, lets = fc.hoist(r, lets)
		gap := l
		if lt.IsEnum() {
			return nil, false, nil
		}
		if !lt.IsFn() {
			gap = r
		}
		var x Expr = Refusal{Operands: []Expr{l, r, gap}, Parts: []string{prefix, " and ", fmt.Sprintf("; DataFunctions::'%s' takes DataValue operands and ", op), " is none"}, Describe: true, T: TypeBool}
		for i := len(lets) - 1; i >= 0; i-- {
			x = Let{Name: lets[i].Name, Value: lets[i].Value, In: x}
		}
		fc.c.collections = true
		return x, true, nil
	}
	if t.IsEnum() {
		return nil, false, nil
	}
	fc.c.collections = true
	return Refusal{Operands: []Expr{l, r}, Parts: []string{prefix, " and ", ""}, Describe: true, T: t}, true, nil
}
