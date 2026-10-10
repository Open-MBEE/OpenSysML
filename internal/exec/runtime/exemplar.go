package runtime

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// exemplars are the objects `this` denotes in a check about no object. Such a
// check reads a type's declared defaults, so `this` there is one exemplar of the
// type, made when first read and abandoned with the check along with what it
// holds; any other object the check reads on the way stays what its usage denotes.
type exemplars struct {
	mark int
	made map[*symbols.Symbol]*Instance
}

// beginExemplars opens a check about no object and returns what closes it,
// abandoning the exemplars the check made and the objects they hold.
func (ctx *Context) beginExemplars() func() {
	prior := ctx.exemplars
	set := &exemplars{mark: len(ctx.created)}
	ctx.exemplars = set
	return func() {
		ctx.exemplars = prior
		if len(set.made) > 0 {
			roots := make(map[*Instance]bool, len(set.made))
			for _, inst := range set.made {
				roots[inst] = true
			}
			ctx.abandonInstancesHeldBy(set.mark, roots)
		}
	}
}

// exemplarOf is the exemplar of sym that `this` denotes in the check about no
// object under way; checking is false outside one.
func (ctx *Context) exemplarOf(sym *symbols.Symbol) (inst *Instance, checking bool, err error) {
	set := ctx.exemplars
	if set == nil || sym == nil {
		return nil, false, nil
	}
	if inst, ok := set.made[sym]; ok {
		return inst, true, nil
	}
	inst, err = ctx.materialize(sym, 0, nil, "")
	if err != nil {
		return nil, true, err
	}
	if set.made == nil {
		set.made = make(map[*symbols.Symbol]*Instance)
	}
	set.made[sym] = inst
	return inst, true, nil
}
