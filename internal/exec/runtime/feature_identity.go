package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A frame binds values by simple name, which is the name a body writes and the
// one a user reads back; name resolution (KerML 8.2.3.5) binds a written name to
// one feature, and two features in a performance's reach may share a simple
// name: a nested node's attribute and the body's, a state's attribute and the
// definition's, an accept payload and a body attribute. Each frame therefore
// also indexes its bindings by the feature declared — held — and a read resolves
// its name first and then reads the frame holding that feature, falling back to
// the name walk only for a name resolution does not bind.

// holdFeature records that held binds sym, and every feature sym redefines — by
// clause or implicitly, as a step's parameter redefines its behavior's at the same
// position — under name: a body written against the redefined feature reads the
// redefining one.
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
	for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(sym) {
		if _, bound := (*held)[redefined]; !bound {
			(*held)[redefined] = name
		}
	}
}

// heldName returns the name f binds sym under, if it holds that feature.
func (f *frame) heldName(sym *symbols.Symbol) (string, bool) {
	if sym == nil || f.held == nil {
		return "", false
	}
	name, ok := f.held[sym]
	return name, ok
}

// bindsUnheld reports whether f binds name without recording the feature it is: a
// binding resolution has no sight of, such as a pin an invocation adopts.
func (f *frame) bindsUnheld(name string) bool {
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

// lookupHeld reads the name qn spells from the innermost frame holding the feature it
// resolves to; a frame holding it with no value decides the read too, returning it unset.
func (ec *EvalContext) lookupHeld(qn *ast.QualifiedName) (val Value, ok bool, unset *symbols.Symbol, err error) {
	if ec.ctx.model.resolver == nil || ec.scope == nil {
		return Value{}, false, nil, nil
	}
	name := qn.Parts[0].Text
	sym, ok := ec.lookupName(name)
	if !ok || sym == nil {
		return Value{}, false, nil, nil
	}
	for i := len(ec.frames) - 1; i >= 0; i-- {
		f := &ec.frames[i]
		bound, holds := f.heldName(sym)
		if !holds {
			if f.bindsUnheld(name) {
				return Value{}, false, nil, nil
			}
			continue
		}
		if val, ok, err := f.read(ec.ctx, bound); err != nil {
			return Value{}, false, nil, err
		} else if ok {
			return val, true, nil, nil
		}
		// A frame masking or hiding its binding leaves the read to the frames outside it.
		if f.hides(bound) {
			continue
		}
		return Value{}, false, sym, nil
	}
	return Value{}, false, nil, nil
}

// lookupNamed reads name from the frames as Lookup does, passing over a frame that
// binds the name as a feature other than unset, the one the name resolves to.
func (ec *EvalContext) lookupNamed(name string, unset *symbols.Symbol) (Value, bool, error) {
	if unset == nil {
		return ec.Lookup(name)
	}
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if ec.frames[i].holdsAnother(name, unset) {
			continue
		}
		if val, ok, err := ec.frames[i].read(ec.ctx, name); err != nil {
			return Value{}, false, err
		} else if ok {
			return val, true, nil
		}
	}
	return Value{}, false, nil
}

// holdsAnother reports whether f binds name as a feature other than sym.
func (f *frame) holdsAnother(name string, sym *symbols.Symbol) bool {
	name = canonical(f.aliases, name)
	for held, bound := range f.held {
		if bound == name && held != sym {
			return true
		}
	}
	return false
}

// shadowedInherited yields the features action inherits that a same-named member
// hides without redefining them (KerML 1.0 §8.2.3.5.3): taken reports the names the
// performance binds, bound the features it holds, the visible and the redefined.
// A hidden feature is yielded with its ShadowedKey; library features stay behind
// the seam as everywhere else.
func (ctx *Context) shadowedInherited(
	action *symbols.Symbol, taken func(string) bool, bound func(*symbols.Symbol) bool,
	yield func(member *symbols.Symbol, usage *ast.Usage, key string),
) {
	if action == nil {
		return
	}
	var seen map[*symbols.Symbol]bool
	for _, general := range ctx.model.semantics.AllSupertypes(action) {
		if ctx.libraryDeclared(general) {
			continue
		}
		for _, member := range ctx.model.semantics.MembersOf(general) {
			usage, ok := member.Decl.(*ast.Usage)
			if !ok || !lower.DeclaresNodeFeature(usage) {
				continue
			}
			name, _ := ast.EffectiveName(usage)
			if name == "" {
				name = member.Name
			}
			if name == "" || !taken(name) || seen[member] || bound(member) || ctx.libraryDeclared(member) {
				continue
			}
			key := lower.ShadowedKey(member)
			if key == "" {
				continue
			}
			if seen == nil {
				seen = make(map[*symbols.Symbol]bool)
			}
			seen[member] = true
			yield(member, usage, key)
		}
	}
}

// heldKey is the name a frame binds the feature that name, written in scope,
// resolves to — the ShadowedKey of a hidden inherited feature — or name itself when
// the frame holds no other binding for it. A statement written in the frame's own
// scope spells its features by name, so only one written elsewhere resolves.
func heldKey(ctx *Context, held map[*symbols.Symbol]string, own, scope *symbols.Scope, name string) string {
	if scope == nil || scope == own || len(held) == 0 {
		return name
	}
	sym, ok := ctx.lookupName(scope, name)
	if !ok || sym == nil {
		return name
	}
	if key, holds := held[sym]; holds && key != name {
		return key
	}
	return name
}

// heldDeclaration is the scope and name a write to the feature f binds under key
// is checked against: a hidden inherited feature's own declaration, found through
// the held index, else scope and key as written.
func heldDeclaration(f *actionFrame, scope *symbols.Scope, key string) (*symbols.Scope, string) {
	if f == nil || !f.shadowed {
		return scope, key
	}
	return declarationHeldUnder(f.declared, scope, key)
}

// declarationHeldUnder finds the hidden feature held binds under key — the one
// whose ShadowedKey it is, not a feature it redefines, which the index binds under
// the same key — and answers its declaring scope and name.
func declarationHeldUnder(held map[*symbols.Symbol]string, scope *symbols.Scope, key string) (*symbols.Scope, string) {
	for sym, bound := range held {
		if bound == key && sym.OwnerScope != nil && lower.ShadowedKey(sym) == key {
			return sym.OwnerScope, sym.Name
		}
	}
	return scope, key
}
