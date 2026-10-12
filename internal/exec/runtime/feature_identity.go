package runtime

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// A frame binds values by simple name, which is the name a body writes and the
// one a user reads back; name resolution (KerML 8.2.3.5) binds a written name to
// one feature, and two features in a performance's reach may share a simple
// name: a nested node's attribute and the body's, a state's attribute and the
// definition's, an accept payload and a body attribute. Each frame therefore
// also indexes its bindings by the feature declared — held — and a read resolves
// its name first and then reads the frame holding that feature, falling back to
// the name walk only for a name resolution does not bind.

// holdFeature records that held binds sym, and every feature sym redefines,
// under name: a body written against the redefined feature reads the redefining one.
func (ctx *Context) holdFeature(held *map[*symbols.Symbol]string, sym *symbols.Symbol, name string) {
	if sym == nil || name == "" {
		return
	}
	if *held == nil {
		*held = make(map[*symbols.Symbol]string)
	}
	if _, bound := (*held)[sym]; bound {
		return
	}
	(*held)[sym] = name
	for _, redefined := range ctx.model.semantics.RedefinedFeatures(sym) {
		ctx.holdFeature(held, redefined, name)
	}
}

// heldName returns the name f binds sym under, if it holds that feature.
func (f frame) heldName(sym *symbols.Symbol) (string, bool) {
	if sym == nil || f.held == nil {
		return "", false
	}
	name, ok := f.held[sym]
	return name, ok
}

// bindsUnheld reports whether f binds name without recording the feature it is: a
// binding resolution has no sight of, such as a pin an invocation adopts.
func (f frame) bindsUnheld(name string) bool {
	if !f.has(name) {
		return false
	}
	for _, bound := range f.held {
		if bound == name {
			return false
		}
	}
	return true
}

// holdResolved copies into held the adopted features scope resolves their name to.
// A pin the node inherits from the action typing it is in reach of resolution; one an
// invocation adopts is not, and stays unheld so the name walk answers for it.
func (ctx *Context) holdResolved(held *map[*symbols.Symbol]string, adopted map[*symbols.Symbol]string, scope *symbols.Scope) {
	if ctx.model.resolver == nil || scope == nil {
		return
	}
	for sym, name := range adopted {
		if resolved, ok := ctx.model.resolver.LookupName(scope, name); !ok || adopted[resolved] != name {
			continue
		}
		if *held == nil {
			*held = make(map[*symbols.Symbol]string)
		}
		if _, bound := (*held)[sym]; !bound {
			(*held)[sym] = name
		}
	}
}

// lookupHeld reads name as the feature it resolves to in the evaluation scope: the
// innermost frame holding that feature answers, whatever else binds the name. It
// reports false for a name resolving to nothing a frame holds, or one a nearer
// frame binds without a feature recorded, which the name walk then answers.
func (ec *EvalContext) lookupHeld(name string) (Value, bool, error) {
	if ec.ctx.model.resolver == nil || ec.scope == nil {
		return Value{}, false, nil
	}
	sym, ok := ec.lookupName(name)
	if !ok || sym == nil {
		return Value{}, false, nil
	}
	for i := len(ec.frames) - 1; i >= 0; i-- {
		bound, holds := ec.frames[i].heldName(sym)
		if !holds {
			if ec.frames[i].bindsUnheld(name) {
				return Value{}, false, nil
			}
			continue
		}
		if val, ok, err := ec.frames[i].read(ec.ctx, bound); err != nil {
			return Value{}, false, err
		} else if ok {
			return val, true, nil
		}
	}
	return Value{}, false, nil
}
