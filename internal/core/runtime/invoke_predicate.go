package runtime

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A constraint or requirement is a predicate (KerML §7.4.9; SysML v2 §7.19, §7.21):
// invoked as an expression, it answers whether its conditions hold for the
// arguments bound to its subject and input parameters. Its other features keep
// the values it binds them, read in the environment the invocation is written in.

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

// predicateShape is the invocation interface of a predicate: its parameters, the
// subject first, flattened along the declarations it takes members from.
type predicateShape struct {
	Sym     *symbols.Symbol
	Label   string
	Params  []calcParameter
	Aliases map[string]string
}

// predicateShapeOf builds the invocation interface of the predicate sym, memoized.
func (ctx *Context) predicateShapeOf(sym *symbols.Symbol) *predicateShape {
	if cached, ok := ctx.predicateShapes[sym]; ok {
		return cached
	}
	supers := ctx.model.MemberSources(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && isPredicateDecl(supers[i].Decl) && !ctx.frameDeclared(supers[i]) {
			chain = append(chain, supers[i])
		}
	}
	chain = append(chain, sym)
	shape := &predicateShape{Sym: sym, Label: predicateKind(sym) + " " + ctx.qualifiedSymbolName(sym)}
	shape.Params = ctx.calcParameters(chain, &shape.Aliases)
	ctx.predicateShapes[sym] = shape
	return shape
}

// invokePredicate applies the predicate sym to args and answers whether its
// conditions hold. The environment enclosing the invocation stays readable, so a
// feature the predicate binds to a name of its owner reads the owner's binding.
func (ec *EvalContext) invokePredicate(sym *symbols.Symbol, args calcArgs) (Value, error) {
	ctx := ec.ctx
	shape := ctx.predicateShapeOf(sym)
	if len(args.positional) > len(shape.Params) {
		return Value{}, fmt.Errorf("%w: %s takes %d argument(s), got %d",
			ErrCalcArity, shape.Label, len(shape.Params), len(args.positional))
	}
	names := make([]string, 0, len(args.named))
	for name := range args.named {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !hasParameter(shape.Params, name) {
			return Value{}, fmt.Errorf("%w: %s has no input parameter %q", ErrUnknownParameter, shape.Label, name)
		}
	}

	env := ec.flattenedFrame()
	env.aliases = mergeAliases(env.aliases, shape.Aliases)
	for i := range shape.Params {
		param := &shape.Params[i]
		value, bound := Value{}, false
		switch {
		case i < len(args.positional):
			value, bound = args.positional[i], true
		default:
			value, bound = args.named[param.Name]
		}
		if !bound {
			// A parameter left to its own binding is read as the predicate's feature;
			// one binding nothing is reported when a condition reads it.
			continue
		}
		what := func() string { return fmt.Sprintf("%s: argument for parameter %q", shape.Label, param.Name) }
		if err := param.Decl.check(ctx, &value, what); err != nil {
			return Value{}, err
		}
		if err := param.checkFunction(&value, what); err != nil {
			return Value{}, err
		}
		env.vars[param.Name] = value
	}

	members := ctx.chainMembers(sym, sym.OwnerScope)
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:      sym,
		kind:     predicateKind(sym),
		what:     "require condition",
		self:     ec.self,
		bindings: env,
		negated:  NegatedDecl(sym),
	}, ctx.conditionsOf(sym, members))
	if errors.Is(err, ErrViolated) {
		return boolValue(false), nil
	}
	if err != nil {
		return Value{}, err
	}
	return boolValue(holds), nil
}

// hasParameter reports whether params has one named name.
func hasParameter(params []calcParameter, name string) bool {
	for i := range params {
		if params[i].Name == name {
			return true
		}
	}
	return false
}

// flattenedFrame is the environment's bindings as one frame of its own storage,
// an inner binding shadowing an outer one of the same name.
func (ec *EvalContext) flattenedFrame() frame {
	out := frame{vars: make(map[string]Value)}
	for _, f := range ec.frames {
		f.each(func(name string, value Value) { out.vars[name] = value })
		out.aliases = mergeAliases(out.aliases, f.aliases)
		if f.owner != nil {
			out.owner, out.run = f.owner, f.run
		}
		if f.perf != nil {
			out.perf = f.perf
		}
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
