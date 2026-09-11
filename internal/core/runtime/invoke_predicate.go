package runtime

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A constraint or requirement is a predicate (KerML §7.4.9; SysML v2 §7.19, §7.21):
// invoked as an expression, it answers whether its conditions hold for the
// arguments bound to its subject and input parameters. A parameter left unbound
// takes its default, evaluated where the predicate is declared.

// isPredicateDecl reports a declaration an invocation applies as a predicate: a
// constraint or requirement definition or usage, an objective among the latter.
func isPredicateDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefConstraint || d.Kind == ast.DefRequirement
	case *ast.Usage:
		return d.Kind == ast.UsageConstraint || d.Kind == ast.UsageRequirement || d.Kind == ast.UsageObjective
	}
	return false
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
	name := ctx.qualifiedSymbolName(sym)
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
		binder.trace.RecordCalculationEnter(shape.Kind, shape.Name)
	}
	if err := ctx.bindCalcParameters(shape, binder, args, ec.scope, env, nil); err != nil {
		if binder.trace != nil {
			binder.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
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
			binder.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
		} else {
			binder.trace.RecordCalculationExit(shape.Kind, shape.Name, boolValue(holds))
		}
	}
	if err != nil {
		return Value{}, calcFrame(shape.Kind, shape.Name, err)
	}
	return boolValue(holds), nil
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
