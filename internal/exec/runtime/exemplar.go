package runtime

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// exemplarOf is the exemplar of sym that `this` denotes in the check about no
// object under way; checking is false outside one.
func (ctx *Context) exemplarOf(sym *symbols.Symbol) (inst *Instance, checking bool, err error) {
	set := ctx.checkObjects
	for set != nil && !set.noObject {
		set = set.outer
	}
	if set == nil || sym == nil {
		return nil, false, nil
	}
	if inst, ok := set.exemplars[sym]; ok {
		return inst, true, nil
	}
	inst, err = ctx.materialize(sym, 0, nil, "")
	if err != nil {
		return nil, true, err
	}
	if set.exemplars == nil {
		set.exemplars = make(map[*symbols.Symbol]*Instance)
	}
	set.exemplars[sym] = inst
	return inst, true, nil
}
