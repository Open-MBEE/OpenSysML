package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// CheckArgs are the values a check of a constraint or requirement supplies for
// its `in` parameters: by position in declaration order, the subject never
// among them, or by name. The zero value supplies nothing.
type CheckArgs struct {
	Positional []Value
	Named      map[string]Value
}

// supplies reports whether args bind anything.
func (args CheckArgs) supplies() bool {
	return len(args.Positional) > 0 || len(args.Named) > 0
}

// checkParameter is one `in` parameter a constraint or requirement declares,
// as the member closest to the checked element declares it.
type checkParameter struct {
	name   string
	names  []string
	member scopedMember
	def    ast.Node
}

// checkParameters are the `in` parameters sym declares or inherits, each in the
// position of the declaration that introduced it, under the declaration closest
// to sym: a redefinition, renaming or not, refines the inherited slot in place and
// keeps its default when it states none.
func (ctx *Context) checkParameters(sym *symbols.Symbol, members []scopedMember) []checkParameter {
	var params []checkParameter
	index := make(map[string]int)
	for _, member := range members {
		usage, ok := member.node.(*ast.Usage)
		if !ok || usage.Kind == ast.UsageSubject || usage.Kind == ast.UsageActor {
			continue
		}
		if usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
			continue
		}
		name := effectiveName(usage)
		if name == "" {
			continue
		}
		param := checkParameter{
			name:   name,
			names:  ctx.memberNames(sym, member, name, usage.Ident.ShortName),
			member: member,
			def:    usage.Value,
		}
		if at, seen := ctx.checkParameterSlot(sym, index, member, name); seen {
			if param.def == nil {
				param.def = params[at].def
			}
			param.names = unionNames(param.names, params[at].names)
			params[at] = param
			index[name] = at
			continue
		}
		index[name] = len(params)
		params = append(params, param)
	}
	return params
}

// checkParameterSlot is the slot among the parameters indexed so far that member,
// an `in` parameter of owner named name, redeclares: the one of the same name, or
// of a parameter it redefines under another name.
func (ctx *Context) checkParameterSlot(owner *symbols.Symbol, index map[string]int, member scopedMember, name string) (int, bool) {
	if at, seen := index[name]; seen {
		return at, true
	}
	memberSym := memberSymbol(member.scope, member.node)
	if memberSym == nil {
		return 0, false
	}
	for _, redefined := range ctx.redefinedFeatures(memberSym, owner) {
		if at, seen := index[redefined.Name]; seen {
			return at, true
		}
	}
	return 0, false
}

func unionNames(names, more []string) []string {
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}
	for _, n := range more {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

// bindCheckArgs binds args to the `in` parameters of the checked element into
// bindings, under every name each parameter is known by. An argument naming no
// parameter, more positional arguments than open parameters, or a parameter left
// with no argument, no default and no same-named value on the checked object
// self to read instead is refused before any value is held.
func (ctx *Context) bindCheckArgs(sym *symbols.Symbol, kind, element string, members []scopedMember, self *Instance, args CheckArgs, bindings map[string]Value) error {
	params := ctx.checkParameters(sym, members)
	label := kind + " " + element
	bound := make(map[string]Value, len(args.Named)+len(args.Positional))
	for name, value := range args.Named {
		if !hasCheckParameter(params, name) {
			return fmt.Errorf("%w: %s has no input parameter %q", ErrUnknownParameter, label, name)
		}
		bound[name] = value
	}
	open := make([]checkParameter, 0, len(params))
	for _, param := range params {
		if _, ok := bound[param.name]; !ok {
			open = append(open, param)
		}
	}
	if len(args.Positional) > len(open) {
		return fmt.Errorf("%w: %s takes %d argument(s), got %d", ErrCalcArity, label, len(open), len(args.Positional))
	}
	for i, value := range args.Positional {
		bound[open[i].name] = value
	}
	for _, param := range params {
		if _, ok := bound[param.name]; !ok && param.def == nil && !ctx.checkParameterOptional(param) && !ctx.carriesFeature(self, param.name) {
			return fmt.Errorf("%w: %s parameter %q has no argument and no default", ErrUnboundParameter, label, param.name)
		}
	}

	commit, rollback := ctx.beginJournal()
	for _, param := range params {
		value, ok := bound[param.name]
		if !ok {
			continue
		}
		if err := ctx.holdBound(sym, param.member, fmt.Sprintf("%s: argument %s", label, param.name), value); err != nil {
			rollback()
			return err
		}
		for _, name := range param.names {
			bindings[name] = value
		}
	}
	commit()
	return nil
}

// carriesFeature reports whether inst holds a value under name, which an unbound
// parameter of a check on inst reads in place of an argument.
func (ctx *Context) carriesFeature(inst *Instance, name string) bool {
	if inst == nil {
		return false
	}
	fv, err := inst.GetFeatureValue(ctx, name)
	return err == nil && fv != nil
}

func hasCheckParameter(params []checkParameter, name string) bool {
	for _, param := range params {
		if param.name == name {
			return true
		}
	}
	return false
}

// checkParameterOptional reports whether a parameter written with a multiplicity
// admitting no value (`[0..1]`) may go unbound. A bare `in` parameter may not:
// a condition cannot be decided over a missing value.
func (ctx *Context) checkParameterOptional(param checkParameter) bool {
	usage, ok := param.member.node.(*ast.Usage)
	if !ok || usage.Multiplicity == nil {
		return false
	}
	sym := memberSymbol(param.member.scope, param.member.node)
	return sym != nil && ctx.model.semantics.EffectiveParameterRange(sym).AllowsNone()
}
