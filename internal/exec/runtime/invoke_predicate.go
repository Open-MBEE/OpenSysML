package runtime

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A constraint or requirement is a predicate (KerML §7.4.9; SysML v2 §7.19, §7.21):
// invoked as an expression, it answers whether its conditions hold for the
// arguments bound to its subject and input parameters. A parameter left unbound
// takes its default, evaluated where the predicate is declared.

// isPredicateDecl reports a declaration an invocation applies as a predicate: a
// constraint or requirement definition or usage, an objective among the latter.
func isPredicateDecl(decl ast.Node) bool {
	if _, ok := ast.OwnedConstraintOf(decl); ok {
		return true
	}
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefConstraint || d.Kind == ast.DefRequirement
	case *ast.Usage:
		return d.Kind == ast.UsageConstraint || d.Kind == ast.UsageRequirement || d.Kind == ast.UsageObjective
	}
	return false
}

// InvokePredicate applies a constraint or requirement to positional arguments
// and reports whether its conditions hold.
func (ctx *Context) InvokePredicate(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (bool, error) {
	defer ctx.beginRun()()
	if sym == nil || !isPredicateDecl(sym.Decl) {
		name := "<nil>"
		if sym != nil {
			name = ctx.QualifiedName(sym)
		}
		return false, fmt.Errorf("%s is not a predicate", name)
	}
	result, err := newEvalContext(ctx, scope).invokePredicate(sym, calcArgs{positional: args})
	if err != nil {
		return false, err
	}
	if !result.isBool() {
		return false, fmt.Errorf("%w: predicate %s returned %s, want Boolean",
			ErrTypeMismatch, ctx.QualifiedName(sym), DescribeValue(result))
	}
	return result.Const.Bool, nil
}

// predicateKind names the kind of predicate sym declares, as verdicts name it.
func predicateKind(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefConstraint {
			return "constraint"
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageConstraint:
			return "constraint"
		case ast.UsageObjective:
			return "objective"
		}
	}
	return "requirement"
}

// predicateShapeOf is the memoized invocation interface of the predicate sym:
// its parameters, the subject first; its conditions stand for a body.
func (ctx *Context) predicateShapeOf(sym *symbols.Symbol) *calcShape {
	if cached, ok := ctx.model.predicateShapes[sym]; ok {
		return cached
	}
	supers := ctx.model.semantics.MemberSources(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && isPredicateDecl(supers[i].Decl) && !ctx.frameDeclared(supers[i]) {
			chain = append(chain, supers[i])
		}
	}
	chain = append(chain, sym)
	name := ctx.QualifiedName(sym)
	if sym.Name == "" {
		// An anonymous usage answers to the name it redefines (`objective : MinimizeObjective;`).
		name += ctx.model.semantics.EffectiveNameOf(sym)
	}
	shape := &calcShape{Sym: sym, Name: name, Kind: predicateKind(sym), Label: predicateKind(sym) + " " + name}
	shape.Params = ctx.calcParameters(chain, &shape.Aliases)
	shape.ParamNames = make([]string, len(shape.Params))
	for i, param := range shape.Params {
		shape.ParamNames[i] = param.Name
	}
	ctx.model.predicateShapes[sym] = shape
	return shape
}

// invokePredicate applies the predicate sym to args and answers whether its
// conditions hold, reading an enclosing run's bindings as a nested calc does.
func (ec *EvalContext) invokePredicate(sym *symbols.Symbol, args calcArgs) (Value, error) {
	ctx := ec.ctx
	if val, ok, err := ec.invokeConstraintCollection(sym, args); ok {
		return val, err
	}
	shape := ctx.predicateShapeOf(sym)
	if predicateOrderAware(ctx, sym) {
		return ctx.invokeOrderedPredicate(shape, args, ec.self, func() (Value, error) {
			return ec.invokePredicateDirect(sym, args)
		})
	}
	return ec.invokePredicateDirect(sym, args)
}

func (ec *EvalContext) invokePredicateDirect(sym *symbols.Symbol, args calcArgs) (Value, error) {
	ctx := ec.ctx
	shape := ctx.predicateShapeOf(sym)
	if err := shape.checkArgs(args); err != nil {
		return Value{}, err
	}
	if err := ctx.enterCalc(shape.Name); err != nil {
		return Value{}, err
	}
	defer ctx.leaveCalc()

	activation := ctx.newActivation()
	defer ctx.endActivation(activation)

	// The conditions are evaluated over one frame, so the enclosing run's bindings
	// and the parameters share it, an inner binding shadowing an outer one.
	env := flattenFrames(ec.enclosingRun(shape))
	env.aliases = mergeAliases(env.aliases, shape.Aliases)
	env.owner, env.run = shape, ctx.newRun()
	binder := &EvalContext{
		ctx:        ctx,
		scope:      ec.scope,
		self:       ec.self,
		frames:     []frame{env},
		trace:      ctx.trace,
		activation: activation,
	}
	if binder.trace != nil {
		binder.trace.recordCalculationEnter(shape.Kind, shape.Name)
	}
	if err := ctx.bindCalcParameters(shape, binder, args, ec.scope, env, calcBindingSite{}); err != nil {
		if binder.trace != nil {
			binder.trace.recordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return Value{}, err
	}

	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:      sym,
		kind:     shape.Kind,
		what:     "require condition",
		element:  ctx.model.semantics.EffectiveNameOf(sym),
		self:     ec.self,
		bindings: env,
		negated:  NegatedDecl(sym),
	}, ctx.conditionsOf(sym, ctx.chainMembers(sym, sym.OwnerScope)))
	if errors.Is(err, ErrViolated) {
		holds, err = false, nil
	}
	if binder.trace != nil {
		if err != nil {
			binder.trace.recordCalculationExitError(shape.Kind, shape.Name, err)
		} else {
			binder.trace.recordCalculationExit(shape.Kind, shape.Name, boolValue(holds))
		}
	}
	if err != nil {
		return Value{}, calcFrame(shape.Kind, shape.Name, err)
	}
	return boolValue(holds), nil
}

// invokeConstraintCollection invokes a constraint feature of the object being
// evaluated that holds many objects — a requirement's `assumptions()`, each member the
// object of a constraint usage subsetting it — as the sequence of the members' results
// (Requirements.sysml `allTrue(assumptions()) implies allTrue(constraints())`), one
// member's undetermined result leaving the whole undetermined. Not ok for a scalar
// constraint, one taking arguments, or one that is no feature of the object.
func (ec *EvalContext) invokeConstraintCollection(sym *symbols.Symbol, args calcArgs) (Value, bool, error) {
	self := ec.self
	if self == nil || sym == nil || sym.Kind != symbols.SymbolConstraintUsage || len(args.positional) > 0 || len(args.named) > 0 {
		return Value{}, false, nil
	}
	name, ok := ec.ctx.featureNamed(self, sym)
	if !ok || self.FeatureValues[name].Feature.Scalar() {
		return Value{}, false, nil
	}
	fv, err := self.GetFeatureValue(ec.ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	held := fv.HeldValue()
	if held.Kind == ValUndetermined {
		return held, true, nil
	}
	members, err := ec.ctx.HeldElements(held)
	if err != nil {
		return Value{}, true, err
	}
	results := make([]Value, 0, len(members))
	for _, member := range members {
		obj, ok := ec.ctx.Instance(member.Instance)
		if member.Kind != ValInstance || !ok {
			return Value{}, true, fmt.Errorf("invoke %s: member %s is no object", name, FormatValue(member))
		}
		result, err := ec.ctx.objectResult(obj)
		if err != nil {
			return Value{}, true, fmt.Errorf("invoke %s: %w", name, err)
		}
		if result.Kind == ValUndetermined {
			return result, true, nil
		}
		results = append(results, result)
	}
	return sequenceOf(results), true, nil
}

// featureNamed is the name under which inst holds the feature sym: the one of its
// shape whose symbol is sym, else sym's own name where a redefinition restates it.
func (ctx *Context) featureNamed(inst *Instance, sym *symbols.Symbol) (string, bool) {
	for name, fv := range inst.FeatureValues {
		if fv.Feature != nil && fv.Feature.Symbol == sym {
			return name, true
		}
	}
	if fv, ok := inst.FeatureValues[sym.Name]; ok && fv.Feature != nil && slices.Contains(ctx.redefinedFeatures(fv.Feature.Symbol, inst.Type), sym) {
		return sym.Name, true
	}
	return "", false
}

// objectResult is the value of the result parameter the object's type declares or
// inherits (KerML 8.4.4.6), read as a member of the object.
func (ctx *Context) objectResult(inst *Instance) (Value, error) {
	for _, m := range ctx.model.semantics.MembersOf(inst.Type) {
		if !resultParameter(m) {
			continue
		}
		fv, err := inst.MemberValue(ctx, m.Name)
		if err != nil {
			return Value{}, err
		}
		return fv.HeldValue(), nil
	}
	return Value{}, fmt.Errorf("%w: %s declares no result", ErrNoSuchFeature, SymbolText(inst.Type))
}

// flattenFrames is frames as one frame of its own storage, an inner binding
// shadowing an outer one of the same name.
func flattenFrames(frames []frame) frame {
	out := frame{vars: make(map[string]Value)}
	for _, f := range frames {
		f.each(func(name string, value Value) { out.vars[name] = value })
		out.aliases = mergeAliases(out.aliases, f.aliases)
		if f.perf != nil {
			out.perf = f.perf
		}
		if f.firing != nil {
			out.firing = f.firing
		}
		if f.machine != nil {
			out.machine = f.machine
		}
		if run := f.running(); run != nil {
			out.merged = append(out.merged, run)
		}
		out.merged = append(out.merged, f.merged...)
	}
	return out
}

// mergeAliases is into with each alias of from added, from's winning on a name.
func mergeAliases(into, from map[string]string) map[string]string {
	if len(from) == 0 {
		return into
	}
	if into == nil {
		into = make(map[string]string, len(from))
	}
	for name, alias := range from {
		into[name] = alias
	}
	return into
}
