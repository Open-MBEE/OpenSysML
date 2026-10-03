package codegen

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// selfParam is the parameter through which a record's calc reads the record it
// is applied to; a NUL cannot occur in a source name.
const selfParam = "\x00self"

// recordOf is the compiled record of the model attribute definition typ, its
// features those semantics.Model.ShapeFeatures gives an object of typ.
func (fc *funcCompiler) recordOf(typ *symbols.Symbol) (*Record, string) {
	c := fc.c
	r, ok := c.records[typ]
	if ok && r.pass == c.pass {
		return r, r.why
	}
	if !ok {
		r = &Record{Name: c.name(typ), Short: typ.Name, sym: typ}
		c.records[typ] = r
	}
	r.pass, r.why, r.Fields, r.ID = c.pass, "", nil, len(c.recOrder)
	c.recOrder = append(c.recOrder, r)
	r.why = fc.recordFields(r)
	return r, r.why
}

// recordFields fills in r's features, or says why r is not compiled.
func (fc *funcCompiler) recordFields(r *Record) string {
	c := fc.c
	def, ok := r.sym.Decl.(*ast.Definition)
	if !ok {
		return fmt.Sprintf("type %s is not a definition", r.Name)
	}
	for _, rel := range def.Relationships {
		if rel != nil && rel.Kind == ast.RelSpecializes {
			return fmt.Sprintf("type %s specializes another definition, whose features a compiled record does not inherit", r.Name)
		}
	}
	for _, sf := range c.model.ShapeFeatures(r.sym) {
		u, ok := sf.Symbol.Decl.(*ast.Usage)
		if !ok {
			return fmt.Sprintf("%s.%s is not a usage", r.Short, sf.Name)
		}
		if u.Kind == ast.UsageCalc {
			continue
		}
		if u.Kind != ast.UsageAttribute || sf.Symbol != sf.Declared || sf.Symbol.Owner() != r.sym || inheritsShape(u) || u.Direction != ast.DirNone {
			return fmt.Sprintf("%s.%s is not an attribute %s declares itself", r.Short, sf.Name, r.Short)
		}
		b, err := fc.declaredBinding(r.sym.Scope, u, sf.Name)
		if err != nil {
			return whyOf(err)
		}
		switch e := b.t.Elem(); {
		case e.IsFn():
			return fmt.Sprintf("%s.%s holds a function value", r.Short, sf.Name)
		case c.target == TargetC && (b.t.Many() || e == TypeString):
			return fmt.Sprintf("%s.%s, a %s, for the C target: a C record cannot keep a String or a collection past the statement making it (the Go target computes it)", r.Short, sf.Name, b.t)
		case b.t.Many() && b.unique && e.IsRec():
			return fmt.Sprintf("%s.%s, a unique collection of records, whose uniqueness violation names an object by its number", r.Short, sf.Name)
		}
		b = c.slotted(b, slot{key: specKey{sym: r.sym}, name: sf.Name, decl: u})
		f := Field{Name: sf.Name, T: b.t, b: b}
		if u.Value != nil {
			if !u.ValueIsDefault || u.ValueIsInitial {
				return fmt.Sprintf("%s.%s is bound to its value rather than given a default", r.Short, sf.Name)
			}
			v, ok := c.model.EvalWithin(unwrap(u.Value), runtime.DefaultMaxIntegerBits)
			switch {
			case !ok || b.t.Many():
				return fmt.Sprintf("the default of %s.%s is computed as the record is made", r.Short, sf.Name)
			case v.Kind == semantics.ValInt:
				f.def = intLit(v)
			case v.Kind == semantics.ValReal:
				f.def = RealLit{Value: v.Real}
			case v.Kind == semantics.ValBool:
				f.def = BoolLit{Value: v.Bool}
			default:
				return fmt.Sprintf("the default of %s.%s is not a number or a Boolean", r.Short, sf.Name)
			}
		}
		r.Fields = append(r.Fields, f)
	}
	for _, s := range c.model.ConstructibleFeatures(r.sym) {
		if r.Field(s.Name) < 0 {
			return fmt.Sprintf("%s binds %s, which is not one of its attributes", r.Short, s.Name)
		}
	}
	return ""
}

// whyOf is what a refusal says, without the calc it names.
func whyOf(err error) string {
	var u *UnsupportedError
	if errors.As(err, &u) {
		return u.What
	}
	return err.Error()
}

// isCalcUsage reports a calc usage, which an object carries as a behavior
// rather than holds as a value.
func isCalcUsage(sym *symbols.Symbol) bool {
	u, ok := sym.Decl.(*ast.Usage)
	return ok && u.Kind == ast.UsageCalc
}

// compileConstructor is `new T(…)` of a record type T, as the interpreter's
// evalConstructor: the arity checked, the arguments evaluated in source order,
// then the object made (a step) and its features written in name order.
func (fc *funcCompiler) compileConstructor(n *ast.ConstructorExpr) (Expr, error) {
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, n.Type)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("new %s: the type does not resolve", qnText(n.Type)))
	}
	t, _, why := fc.valueType(sym)
	if why == "" && !t.IsRec() {
		why = fmt.Sprintf("%s is not a record type", fc.c.name(sym))
	}
	if why != "" {
		return nil, fc.unsupported(fmt.Sprintf("new %s: %s", qnText(n.Type), why))
	}
	r := t.Rec
	what := "new " + sym.Name
	slots := fc.c.model.ConstructibleFeatures(sym)
	fail := func(lets []Let, msg string) Expr {
		return wrapLets(lets, Refusal{Parts: []string{what + ": " + msg}, T: t})
	}
	if len(n.Args) > len(slots) {
		return fail(nil, fmt.Sprintf("new %s takes %d argument(s), found %d", sym.Name, len(slots), len(n.Args))), nil
	}
	var lets []Let
	payload := map[string]Expr{}
	arg := func(node ast.Node, name string) error {
		v, err := fc.compileExpr(node)
		if err != nil {
			return err
		}
		tmp := fc.temp(v.Type())
		lets = append(lets, Let{Name: tmp.Name, Value: v})
		payload[name] = Var{Name: tmp.Name, T: v.Type()}
		return nil
	}
	bound := map[*symbols.Symbol]string{}
	for i, a := range n.Args {
		bound[slots[i]] = slots[i].Name
		if err := arg(a, slots[i].Name); err != nil {
			return nil, err
		}
	}
	for _, a := range n.NamedArgs {
		name, msg := fc.constructorLabel(sym, n.Type, a.Name, bound)
		if _, twice := payload[name]; msg == "" && twice {
			msg = qnText(a.Name) + " is bound twice"
		}
		if msg != "" {
			return fail(lets, msg), nil
		}
		if err := arg(a.Value, name); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(payload))
	for name := range payload {
		names = append(names, name)
	}
	sort.Strings(names)
	var writes []Let
	written := map[string]Expr{}
	for _, name := range names {
		f := r.Fields[r.Field(name)]
		label := fmt.Sprintf("%s: feature value %s.%s", what, r.Short, name)
		v, err := fc.bind(payload[name], f.b, label, label)
		if err != nil {
			return nil, err
		}
		if f.b.t.Scalar() && f.b.r != RangeAny {
			v = Narrowed{X: v, R: f.b.r, Where: label}
		}
		if pure(v) {
			written[name] = v
			continue
		}
		tmp := fc.temp(v.Type())
		writes = append(writes, Let{Name: tmp.Name, Value: v})
		written[name] = Var{Name: tmp.Name, T: v.Type()}
	}
	fields := make([]Expr, len(r.Fields))
	for i, f := range r.Fields {
		if v, ok := written[f.Name]; ok {
			fields[i] = v
			continue
		}
		switch {
		case f.def != nil:
			v, err := fc.bind(f.def, f.b, what, what)
			if err != nil {
				return nil, err
			}
			fields[i] = v
		case f.T.Many():
			fields[i] = NullLit{T: f.T}
		default:
			return nil, fc.unsupported(fmt.Sprintf("%s binds no value to %s, which has no default", what, f.Name))
		}
	}
	return wrapLets(lets, Steps{N: 1, X: wrapLets(writes, RecNew{Rec: r, Fields: fields})}), nil
}

// wrapLets is x evaluated after lets, in order.
func wrapLets(lets []Let, x Expr) Expr {
	for i := len(lets) - 1; i >= 0; i-- {
		x = Let{Name: lets[i].Name, Value: lets[i].Value, In: x}
	}
	return x
}

// constructorLabel is the feature of typ the named argument qn binds, or the
// interpreter's message refusing it.
func (fc *funcCompiler) constructorLabel(typ *symbols.Symbol, typeRef, qn *ast.QualifiedName, bound map[*symbols.Symbol]string) (string, string) {
	label := qnText(qn)
	if label == "" {
		return "", "an argument is named by nothing"
	}
	m := fc.c.model
	feature, ok := fc.c.resolver.ResolveReference(resolve.Reference{Scope: fc.scope, QN: qn, Constructed: typeRef})
	if !ok || feature == nil || !slices.Contains(m.MembersOf(typ), feature) {
		return "", fmt.Sprintf("%s is not a feature of %s", label, typ.Name)
	}
	shape := m.ShapeFeatures(typ)
	i := slices.IndexFunc(shape, func(f semantics.ShapeFeature) bool { return f.Declared == feature || f.Symbol == feature })
	if i < 0 {
		return "", fmt.Sprintf("%s is not a feature of %s", label, typ.Name)
	}
	s := m.ConstructibleFeatureFor(typ, shape[i].Declared)
	if s == nil {
		return "", fmt.Sprintf("%s is not a feature a constructor of %s binds", label, typ.Name)
	}
	name := shape[i].Name
	if earlier, twice := bound[s]; twice {
		if earlier == name {
			return "", name + " is bound twice"
		}
		return "", fmt.Sprintf("%s and %s are one feature, bound twice", earlier, name)
	}
	bound[s] = name
	return name, ""
}

// compileChain is `x.f`: a feature of the record x, or a calc of it read as a
// function value over x. A chain `x.f.g` reads each feature in turn.
func (fc *funcCompiler) compileChain(n *ast.FeatureChainExpr) (Expr, error) {
	if n.Member == nil || len(n.Member.Parts) == 0 {
		return nil, fc.unsupported("a feature chain naming no feature")
	}
	x, err := fc.compileExpr(n.Operand)
	if err != nil {
		return nil, err
	}
	for _, part := range n.Member.Parts {
		if x, err = fc.member(x, part.Text, chainText(n)); err != nil {
			return nil, err
		}
	}
	return x, nil
}

// member is the feature name of the record x.
func (fc *funcCompiler) member(x Expr, name, written string) (Expr, error) {
	t := x.Type()
	if !t.Scalar() || !t.IsRec() {
		return nil, fc.unsupported(fmt.Sprintf("%s: a feature chain reading %s off a %s", written, name, t))
	}
	r := t.Rec
	if i := r.Field(name); i >= 0 {
		return RecGet{X: x, Field: i, T: r.Fields[i].T}, nil
	}
	sym, ok := fc.c.model.LookupMember(r.sym, name)
	if !ok || !isCalcUsage(sym) {
		return nil, fc.unsupported(fmt.Sprintf("%s: %s is not a feature of %s", written, name, r.Short))
	}
	unsupplied, err := fc.hasUnsuppliedInput(sym)
	if err != nil {
		return nil, err
	}
	if !unsupplied {
		return nil, fc.unsupported(fmt.Sprintf("%s: %s, a calc with no unsupplied input, which reads as its result rather than as a function value", written, fc.c.name(sym)))
	}
	k := fc.c.selfCase(sym, r)
	return FnLit{Case: k, Self: x, T: FnType(fc.c.fnSet([]*FnCase{k}))}, nil
}

// selfCase is the case of the calc sym of record r, read off a record.
func (c *Compiler) selfCase(sym *symbols.Symbol, r *Record) *FnCase {
	key := "self " + identOf(sym)
	k, ok := c.fns.cases[key]
	if !ok {
		k = &FnCase{Name: c.name(sym), ID: len(c.fns.order), Self: r, val: funcValue{sym: sym}}
		c.fns.cases[key] = k
		c.fns.order = append(c.fns.order, k)
	}
	return k
}

// chainText spells a feature chain as written, `holder.scale`.
func chainText(n ast.Node) string {
	switch n := n.(type) {
	case *ast.FeatureChainExpr:
		return chainText(n.Operand) + "." + qnText(n.Member)
	case *ast.FeatureReference:
		return qnText(n.Name)
	}
	return "an expression"
}

// compileChainCall is `x.f(a)`, as the interpreter's evalChainInvocation: a
// calc of the record x applied over it, else the function value x.f holds.
func (fc *funcCompiler) compileChainCall(n *ast.InvocationExpr, chain *ast.FeatureChainExpr) (Expr, error) {
	call := *n
	call.Operand = nil
	if chain.Member == nil || len(chain.Member.Parts) != 1 {
		fx, err := fc.compileExpr(chain)
		if err != nil {
			return nil, err
		}
		return fc.dispatchChain(fx, &call, chainText(chain))
	}
	recv, err := fc.compileExpr(chain.Operand)
	if err != nil {
		return nil, err
	}
	name := chain.Member.Parts[0].Text
	if t := recv.Type(); t.Scalar() && t.IsRec() && t.Rec.Field(name) < 0 {
		if sym, ok := fc.c.model.LookupMember(t.Rec.sym, name); ok && isCalcUsage(sym) {
			tmp := fc.temp(t)
			x, err := fc.callRecordCalc(sym, t.Rec, Var{Name: tmp.Name, T: t}, &call)
			if err != nil {
				return nil, err
			}
			return Let{Name: tmp.Name, Value: recv, In: x}, nil
		}
	}
	fx, err := fc.member(recv, name, chainText(chain))
	if err != nil {
		return nil, err
	}
	return fc.dispatchChain(fx, &call, chainText(chain))
}

func (fc *funcCompiler) dispatchChain(fx Expr, n *ast.InvocationExpr, written string) (Expr, error) {
	if t := fx.Type(); !t.Scalar() || !t.IsFn() {
		return nil, fc.unsupported(fmt.Sprintf("%s is a %s, not a function", written, t))
	}
	return fc.dispatch(fx, n)
}

// callRecordCalc compiles a call, with n's arguments, of the calc sym of record
// r applied over the record self.
func (fc *funcCompiler) callRecordCalc(sym *symbols.Symbol, r *Record, self Expr, n *ast.InvocationExpr) (Expr, error) {
	params, err := fc.c.calcParams(sym)
	if err != nil {
		return nil, err
	}
	args, fargs, trailing, err := fc.bindArgs(n, fc.c.name(sym), params)
	if err != nil {
		return nil, err
	}
	callee, err := fc.c.compileSelfCalc(sym, r, fargs)
	if err != nil {
		return nil, err
	}
	args = append(passFunctions(args, params, fargs), Arg{Param: len(callee.Params) - 1, Value: self})
	return fc.finishCall(callee, args, trailing)
}

// compileSelfCalc compiles the calc sym of record r over fargs, taking the
// record it is applied over as its last parameter.
func (c *Compiler) compileSelfCalc(sym *symbols.Symbol, r *Record, fargs []funcValue) (*Func, error) {
	key := specKeyOf(sym, fargs)
	key.fargs += "\x00self"
	if fn, ok := c.funcs[key]; ok {
		return fn, nil
	}
	body, rels, err := calcDecl(sym.Decl)
	if err != nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: err.Error()}
	}
	if len(rels) > 0 {
		return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc of a record that specializes another calc"}
	}
	fn := &Func{Name: c.name(sym), Ident: specIdent(sym, fargs) + "_self", self: r}
	c.funcs[key] = fn
	c.keys[fn] = key
	c.order = append(c.order, fn)
	return fn, c.compileFunc(fn, key, sym, body, fargs, nil, nil)
}

// selfFeature is, in the body of a record's calc, the read of the feature qn
// names of the record the calc is applied over.
func (fc *funcCompiler) selfFeature(qn *ast.QualifiedName) (Expr, bool) {
	r := fc.fn.self
	if r == nil || qn == nil {
		return nil, false
	}
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, qn)
	if !ok {
		return nil, false
	}
	for _, sf := range fc.c.model.ShapeFeatures(r.sym) {
		if sf.Symbol != sym && sf.Declared != sym {
			continue
		}
		i := r.Field(sf.Name)
		if i < 0 {
			return nil, false
		}
		self, ok := fc.env.lookup(selfParam)
		if !ok {
			return nil, false
		}
		v, err := fc.readBinding(selfParam, self)
		if err != nil {
			return nil, false
		}
		return RecGet{X: v, Field: i, T: r.Fields[i].T}, true
	}
	return nil, false
}

// callSelfCalc is, in the body of a record's calc, a call of another calc of
// that record, applied over the same record; ok is false for any other call.
func (fc *funcCompiler) callSelfCalc(sym *symbols.Symbol, n *ast.InvocationExpr) (Expr, bool, error) {
	r := fc.fn.self
	if r == nil || n.Operand != nil {
		return nil, false, nil
	}
	if member, ok := fc.c.model.LookupMember(r.sym, sym.Name); !ok || member != sym || !isCalcUsage(sym) {
		return nil, false, nil
	}
	self, ok := fc.env.lookup(selfParam)
	if !ok {
		return nil, false, nil
	}
	v, err := fc.readBinding(selfParam, self)
	if err != nil {
		return nil, true, err
	}
	x, err := fc.callRecordCalc(sym, r, v, n)
	return x, true, err
}
