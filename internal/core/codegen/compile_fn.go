package codegen

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// funcValue is a function value the compiler fixes statically: the calc it is
// a value of, and for a library function the compiler implements its qualified
// name. A calc binding an `in calc` parameter is compiled once per distinct
// value it is applied to (monomorphization), so no function value exists at run time.
type funcValue struct {
	sym *symbols.Symbol
	lib string
}

// ident is the identifier a specialization over v carries; injective, as identOf is.
func (v funcValue) ident() string {
	if v.lib == "" {
		return identOf(v.sym)
	}
	var b strings.Builder
	b.WriteString("lib")
	encodeName(&b, v.lib)
	return b.String()
}

// specKey identifies a compiled function: a calc together with the function
// values its `in calc` parameters are bound to, in declaration order.
type specKey struct {
	sym   *symbols.Symbol
	fargs string
}

func specKeyOf(sym *symbols.Symbol, fargs []funcValue) specKey {
	ids := make([]string, len(fargs))
	for i, f := range fargs {
		ids[i] = f.ident()
	}
	return specKey{sym: sym, fargs: strings.Join(ids, ",")}
}

// specIdent is the C/Go identifier of sym specialized over fargs.
func specIdent(sym *symbols.Symbol, fargs []funcValue) string {
	id := identOf(sym)
	for _, f := range fargs {
		id += "_fn_" + f.ident()
	}
	return id
}

// paramDecl is one in-parameter of a calc: its name, whether it is an `in calc`
// parameter bound to a function value, and the calc such a parameter is typed by, if any.
type paramDecl struct {
	name string
	fn   bool
	typ  *symbols.Symbol
}

// calcParams is the in-parameters of the calc sym, in declaration order: its
// own, or those of the one calc a member-less specialization compiles to.
func (c *Compiler) calcParams(sym *symbols.Symbol) ([]paramDecl, error) {
	body, rels, err := calcDecl(sym.Decl)
	if err != nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: err.Error()}
	}
	if len(rels) > 0 {
		parent, err := c.inheritedCalc(sym, body, rels)
		if err != nil {
			return nil, err
		}
		return c.calcParams(parent)
	}
	var params []paramDecl
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok || (u.Direction != ast.DirIn && u.Direction != ast.DirInOut) {
			continue
		}
		name, _ := ast.EffectiveName(u)
		p := paramDecl{name: name, fn: u.Kind == ast.UsageCalc}
		if p.fn && hasTyping(u) {
			typ, why := c.typingOf(sym.Scope, u, name)
			if why != "" {
				return nil, &UnsupportedError{Calc: c.name(sym), What: why}
			}
			p.typ = typ
		}
		params = append(params, p)
	}
	return params, nil
}

// hasUnsuppliedInput reports an in-parameter of the calc usage sym that no
// read could bind: one with neither a default nor an optional multiplicity.
// Reading such a usage denotes a function value; reading any other computes it.
func (fc *funcCompiler) hasUnsuppliedInput(sym *symbols.Symbol) (bool, error) {
	body, rels, err := calcDecl(sym.Decl)
	if err != nil {
		return false, fc.unsupported(err.Error())
	}
	if len(rels) > 0 {
		parent, err := fc.c.inheritedCalc(sym, body, rels)
		if err != nil {
			return false, err
		}
		return fc.hasUnsuppliedInput(parent)
	}
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok || (u.Direction != ast.DirIn && u.Direction != ast.DirInOut) || u.Value != nil {
			continue
		}
		name, _ := ast.EffectiveName(u)
		m, err := fc.multOf(u, name)
		if err != nil {
			return false, err
		}
		if m.Lower != 0 {
			return true, nil
		}
	}
	return false, nil
}

// compileFuncArg is the function value the argument node names, or the typed
// refusal naming what it is instead; where names the binding for diagnostics.
func (fc *funcCompiler) compileFuncArg(node ast.Node, where string) (funcValue, error) {
	// A parenthesized expression parses as a body holding one expression.
	if b, ok := node.(*ast.BodyExpr); ok && len(b.Members) == 1 && b.Result == nil && len(b.Params) == 0 {
		if inner := unwrap(b.Members[0]); inner != nil {
			node = inner
		}
	}
	ref, ok := node.(*ast.FeatureReference)
	if !ok {
		if op, isOp := node.(*ast.OperatorExpr); isOp && op.Operator == ast.OpConditional {
			return funcValue{}, fc.unsupported(where + ": an `if` choosing a function value at run time; a function value must name a calc directly")
		}
		if _, isCall := node.(*ast.InvocationExpr); isCall {
			return funcValue{}, fc.unsupported(where + ": an invocation where a function value is expected; a function value must name a calc directly")
		}
		if _, isChain := node.(*ast.FeatureChainExpr); isChain {
			return funcValue{}, fc.unsupported(where + ": a calc read off an object through a feature chain, whose function value closes over that object")
		}
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s: expression %T where a function value is expected; a function value must name a calc directly", where, node))
	}
	qn := ref.Name
	if qn != nil && len(qn.Parts) == 1 && !qn.Global {
		if b, ok := fc.env.lookup(qn.Parts[0].Text); ok {
			switch {
			case b.fn != nil:
				return *b.fn, nil
			case b.sampled != nil:
				return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, a SampledFunction, where a function value is expected", where, qn.Parts[0].Text))
			}
			return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, a %s, where a function value is expected", where, qn.Parts[0].Text, b.t))
		}
	}
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, qn)
	if !ok {
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s does not resolve", where, qnText(qn)))
	}
	return fc.functionValueOf(sym, where)
}

// functionValueOf is the declaration sym read as a function value, as the
// interpreter reads it, when the compiler can fix that value statically: a calc
// def or a calc usage with an unsupplied input owned by packages alone, or a
// library function the compiler implements. A calc owned by a part or declared
// in a behavior body closes over the object or run it is read in and is refused.
func (fc *funcCompiler) functionValueOf(sym *symbols.Symbol, where string) (funcValue, error) {
	name := fc.c.name(sym)
	if fc.c.resolver.Index().Library(sym) {
		if _, _, ok := seqOpByName(name); ok {
			return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s binds its arguments unevaluated and cannot be read as a value", where, name))
		}
		if _, ok := runtime.LibraryFunctionParams(name); ok {
			return funcValue{sym: sym, lib: name}, nil
		}
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s: library function %s is not compiled", where, name))
	}
	if !isCalc(sym.Decl) {
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, which is not a calc, where a function value is expected", where, name))
	}
	if u, ok := sym.Decl.(*ast.Usage); ok {
		if u.Direction != ast.DirNone {
			return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, an `in calc` parameter, read outside the body of the calc declaring it", where, name))
		}
		unsupplied, err := fc.hasUnsuppliedInput(sym)
		if err != nil {
			return funcValue{}, err
		}
		if !unsupplied {
			return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, a calc usage with no unsupplied input, which reads as its result rather than as a function value", where, name))
		}
	}
	for owner := ownerOf(sym); owner != nil; owner = ownerOf(owner) {
		switch owner.Kind {
		case symbols.SymbolPackage, symbols.SymbolNamespace:
			continue
		case symbols.SymbolCalcDef, symbols.SymbolCalcUsage, symbols.SymbolActionDef, symbols.SymbolActionUsage:
			return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, a calc declared in the body of %s, whose function value closes over that run's bindings", where, name, fc.c.name(owner)))
		}
		return funcValue{}, fc.unsupported(fmt.Sprintf("%s: %s, a calc owned by %s, whose function value closes over that object", where, name, fc.c.name(owner)))
	}
	return funcValue{sym: sym}, nil
}

// checkFuncArgType refuses the function value f bound to the typed `in calc`
// parameter p unless f's calc, model or library, conforms to the calc p is
// typed by: the relation the interpreter binds the value under.
func (fc *funcCompiler) checkFuncArgType(p paramDecl, f funcValue) error {
	if p.typ == nil || fc.c.model.Conforms(f.sym, p.typ) {
		return nil
	}
	return fc.unsupported(fmt.Sprintf("%s: cannot bind the function value %s to a parameter typed by %s", paramWhere(p.name), fc.c.name(f.sym), fc.c.name(p.typ)))
}

func ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// functionValueRead is the description of node when it names a function value
// where a plain value is expected; false when it is anything else.
func (fc *funcCompiler) functionValueRead(node ast.Node) (string, bool) {
	ref, ok := node.(*ast.FeatureReference)
	if !ok {
		return "", false
	}
	return fc.functionValueName(ref.Name)
}

// functionValueName is the description of qn when it names a function value
// the interpreter would read: an `in calc` parameter, a calc def, or a calc
// usage with an unsupplied input; false when it names anything else.
func (fc *funcCompiler) functionValueName(qn *ast.QualifiedName) (string, bool) {
	if qn == nil {
		return "", false
	}
	if len(qn.Parts) == 1 && !qn.Global {
		if b, ok := fc.env.lookup(qn.Parts[0].Text); ok {
			if b.fn != nil {
				return fmt.Sprintf("the function value %s", qn.Parts[0].Text), true
			}
			return "", false
		}
	}
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, qn)
	if !ok || !isCalc(sym.Decl) {
		return "", false
	}
	if fc.c.resolver.Index().Library(sym) {
		return "", false
	}
	if u, isUsage := sym.Decl.(*ast.Usage); isUsage {
		if u.Direction != ast.DirNone {
			return "", false
		}
		if unsupplied, err := fc.hasUnsuppliedInput(sym); err != nil || !unsupplied {
			return "", false
		}
	}
	return fmt.Sprintf("the function value %s", fc.c.name(sym)), true
}

// applyFunction compiles an invocation of the function value f with n's arguments.
func (fc *funcCompiler) applyFunction(f funcValue, n *ast.InvocationExpr) (Expr, error) {
	if f.lib != "" {
		return fc.compileLibCall(n, f.lib)
	}
	return fc.callCalc(f.sym, n)
}

// applyToOne compiles the application of f to the one argument x, as
// `calculation(x)` inside SampledFunctions::Sample applies it.
func (fc *funcCompiler) applyToOne(f funcValue, x Expr) (Expr, error) {
	args := []Arg{{Param: 0, Value: x}}
	if f.lib != "" {
		params, err := fc.sampledLibParams(f)
		if err != nil {
			return nil, err
		}
		return fc.finishLibCall(f.lib, params, args)
	}
	callee, err := fc.sampledCalc(f)
	if err != nil {
		return nil, err
	}
	return fc.finishCalcCall(callee, args)
}

// sampledLibParams is the one parameter of the library function f, which Sample applies.
func (fc *funcCompiler) sampledLibParams(f funcValue) ([]string, error) {
	params, _ := runtime.LibraryFunctionParams(f.lib)
	if len(params) != 1 {
		return nil, fc.unsupported(fmt.Sprintf("Sample of %s, which takes %d arguments", f.lib, len(params)))
	}
	return params, nil
}

// sampledCalc compiles the calc f, which Sample applies to one value argument.
func (fc *funcCompiler) sampledCalc(f funcValue) (*Func, error) {
	params, err := fc.c.calcParams(f.sym)
	if err != nil {
		return nil, err
	}
	if len(params) != 1 || params[0].fn {
		return nil, fc.unsupported(fmt.Sprintf("Sample of %s, which does not take one value argument", fc.c.name(f.sym)))
	}
	return fc.c.compileCalcWith(f.sym, nil)
}

// sampledElemType is the element type of a domain sampled over f whose values
// fix none: the type of f's one parameter; for a library function, the operand
// type of its operation over a Real (its widest kind).
func (fc *funcCompiler) sampledElemType(f funcValue) (Type, error) {
	if f.lib != "" {
		if _, err := fc.sampledLibParams(f); err != nil {
			return TypeInvalid, err
		}
		op, why := libOpFor(f.lib, []Type{TypeReal})
		if why != "" {
			return TypeInvalid, fc.unsupported(why)
		}
		return op.Operands()[0], nil
	}
	callee, err := fc.sampledCalc(f)
	if err != nil {
		return TypeInvalid, err
	}
	return callee.Params[0].Type.Elem(), nil
}

// sampledFn is a SampledFunction held as two hidden locals: the domain values
// and the sampled calc's result at each, computed when the sample was taken.
type sampledFn struct {
	dom, rng   string
	domT, rngT Type
}

const (
	sampledFunctionType = "SampledFunctions::SampledFunction"
	sampleCalc          = "SampledFunctions::Sample"
	sampleDomain        = "SampledFunctions::Domain"
	sampleRange         = "SampledFunctions::Range"
)

// sampleCall is node as an invocation of SampledFunctions::Sample, if it is one.
func (fc *funcCompiler) sampleCall(node ast.Node) (*ast.InvocationExpr, bool) {
	n, ok := node.(*ast.InvocationExpr)
	if !ok || n.Type == nil || n.Operand != nil {
		return nil, false
	}
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, n.Type)
	if !ok || !fc.c.resolver.Index().Library(sym) || fc.c.name(sym) != sampleCalc {
		return nil, false
	}
	return n, true
}

// compileSample compiles `Sample(calculation, domainValues)` into a Sample of
// two fresh locals: the domain is the sequence of samples taken (so null
// samples to `[]`, as the library's collect does) and the range the
// calculation at each, in order, failing at the first element that fails.
func (fc *funcCompiler) compileSample(n *ast.InvocationExpr) (Sample, error) {
	args, fargs, err := fc.bindArgs(n, sampleCalc, []paramDecl{{name: "calculation", fn: true}, {name: "domainValues"}})
	if err != nil {
		return Sample{}, err
	}
	dom := args[0].Value
	if dom.Type() == TypeNull {
		elem, err := fc.sampledElemType(fargs[0])
		if err != nil {
			return Sample{}, err
		}
		dom = fc.retype(dom, elem.Seq())
	}
	if dom, err = fc.toMany(dom, dom.Type().Seq(), "argument for parameter \"domainValues\""); err != nil {
		return Sample{}, err
	}
	x := fc.temp(dom.Type().Elem())
	at, err := fc.applyToOne(fargs[0], Var{Name: x.Name, T: x.Type})
	if err != nil {
		return Sample{}, err
	}
	if !at.Type().Scalar() {
		return Sample{}, fc.unsupported(fmt.Sprintf("Sample of a calc whose result is a %s, not a scalar", at.Type()))
	}
	fc.c.collections = true
	return Sample{Dom: fc.temp(dom.Type()).Name, Rng: fc.temp(at.Type().Seq()).Name, Seq: dom, Body: Lambda{Params: []Param{x}, Body: at}}, nil
}

// temp is a fresh hidden local of type t.
func (fc *funcCompiler) temp(t Type) Param {
	fc.temps++
	return Param{Name: fmt.Sprintf("\x00%d", fc.temps), Type: t, Mult: MultOne}
}

// projection compiles `Domain(fn)` or `Range(fn)` over the sample's values held
// in seq: the library calc collects them into a fresh sequence, one frame deeper.
func (fc *funcCompiler) projection(seq Var) Expr {
	x := fc.temp(seq.T.Elem())
	return Framed{X: Fold{Op: SeqCollect, Seq: seq, Body: Lambda{Params: []Param{x}, Body: Var{Name: x.Name, T: x.Type}}, T: seq.T}}
}

// compileSampledDeclare declares an attribute holding a SampledFunction, which
// must be taken by Sample right there: its domain and range become two locals.
func (fc *funcCompiler) compileSampledDeclare(s lower.Declare) ([]Stmt, error) {
	if s.Value == nil {
		return nil, fc.unsupported(fmt.Sprintf("attribute %s, a SampledFunction bound to no Sample", s.Name))
	}
	n, ok := fc.sampleCall(s.Value)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("attribute %s, a SampledFunction bound to something other than `Sample(…)`", s.Name))
	}
	if u, isUsage := s.Node.(*ast.Usage); isUsage {
		if m, err := fc.multOf(u, s.Name); err != nil {
			return nil, err
		} else if m != MultOne {
			return nil, fc.unsupported(fmt.Sprintf("attribute %s, a SampledFunction declaring a multiplicity", s.Name))
		}
	}
	sample, err := fc.compileSample(n)
	if err != nil {
		return nil, err
	}
	fc.env.bind(s.Name, binding{sampled: &sampledFn{dom: sample.Dom, rng: sample.Rng, domT: sample.DomType(), rngT: sample.RngType()}})
	return []Stmt{sample}, nil
}

// compileSampledRead compiles `Domain(s)` or `Range(s)` of a SampledFunction:
// one held by an attribute, or taken by `Sample(…)` in the argument itself.
func (fc *funcCompiler) compileSampledRead(n *ast.InvocationExpr, fqn string) (Expr, error) {
	if n.Operand != nil {
		return nil, fc.unsupported(fmt.Sprintf("an invocation of %s with a receiver (`x->F()`)", fqn))
	}
	var arg ast.Node
	switch {
	case len(n.NamedArgs) == 1 && paramIndex([]string{"fn"}, n.NamedArgs[0].Name) == 0:
		arg = n.NamedArgs[0].Value
	case len(n.NamedArgs) == 0 && len(n.Args) == 1:
		arg = n.Args[0]
	default:
		return nil, fc.unsupported(fmt.Sprintf("%s takes 1 argument, %d given", fqn, len(n.Args)+len(n.NamedArgs)))
	}
	rangeRead := fqn == sampleRange
	if ref, ok := arg.(*ast.FeatureReference); ok && ref.Name != nil && len(ref.Name.Parts) == 1 && !ref.Name.Global {
		if b, ok := fc.env.lookup(ref.Name.Parts[0].Text); ok {
			if b.sampled == nil {
				return nil, fc.unsupported(fmt.Sprintf("%s of %s, which is not a SampledFunction", fqn, ref.Name.Parts[0].Text))
			}
			if rangeRead {
				return fc.projection(Var{Name: b.sampled.rng, T: b.sampled.rngT}), nil
			}
			return fc.projection(Var{Name: b.sampled.dom, T: b.sampled.domT}), nil
		}
	}
	n, ok := fc.sampleCall(arg)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("%s of something other than an attribute holding a SampledFunction or `Sample(…)` itself", fqn))
	}
	sample, err := fc.compileSample(n)
	if err != nil {
		return nil, err
	}
	read := Var{Name: sample.Rng, T: sample.RngType()}
	if !rangeRead {
		read = Var{Name: sample.Dom, T: sample.DomType()}
	}
	return Sampled{S: sample, In: fc.projection(read)}, nil
}
