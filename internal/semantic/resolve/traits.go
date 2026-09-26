package resolve

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// DeclarationTraits answers, for a loaded symbol, what this package asks of a
// declaration, as the bits its interface record carries: a recorded symbol
// answers the same predicates from these bits.
func (r *Resolver) DeclarationTraits(sym *symbols.Symbol) symbols.Modifiers {
	var mods symbols.Modifiers
	set := func(on bool, mod symbols.Modifiers) {
		if on {
			mods |= mod
		}
	}
	set(isParameter(sym), symbols.ModParameter)
	set(ImplicitlyRedefined(sym), symbols.ModImplicitlyRedefined)
	set(contributesName(sym), symbols.ModContributesName)
	set(r.hasUnresolvedRedefinition(sym), symbols.ModUnresolvedRedefinition)
	set(ParameterizedByName(sym), symbols.ModParameterizedByName)
	set(sym.Naming != symbols.NamedByDeclaration && !r.BindsName(sym), symbols.ModNamesNothing)
	return mods
}

// recordedElements restores the elements a record fact names by fully-qualified
// name, aliases resolved through, dropping what no longer declares one.
func (r *Resolver) recordedElements(refs []symbols.ElementRef) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, ref := range refs {
		if target := r.recordedElement(ref); target != nil {
			out = append(out, target)
		}
	}
	return out
}

// RecordedElement restores the element a record fact names, nil for none.
func (r *Resolver) RecordedElement(ref symbols.ElementRef) *symbols.Symbol {
	return r.recordedElement(ref)
}

// recordedElement restores the element a record fact names, nil for none.
func (r *Resolver) recordedElement(ref symbols.ElementRef) *symbols.Symbol {
	if r.idx == nil {
		return nil
	}
	target := r.idx.Element(ref)
	if target == nil {
		return nil
	}
	if resolved, ok := r.ResolveAliasTarget(target); ok {
		return resolved
	}
	return nil
}
