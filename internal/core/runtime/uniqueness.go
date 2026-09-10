package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// uniquenessRefusal says which value repeats in a sequence written to a unique
// feature, by set equality, or is empty; a set-held feature drops repeats instead.
func (ctx *Context) uniquenessRefusal(unique, holdsSet bool, value *Value) string {
	if !unique || holdsSet || value.Kind != ValSequence {
		return ""
	}
	elements := elementsOf(*value)
	seen := NewSet()
	for i, element := range elements {
		if !seen.Contains(element) {
			seen.Add(element)
			continue
		}
		first := 0
		for first < i && !valueEqual(elements[first], element) {
			first++
		}
		return semantics.UniquenessViolation(ctx.elementText(element), first+1, i+1)
	}
	return ""
}

// declaredUniquenessRefusal is uniquenessRefusal for the value a standalone
// multi-valued feature declares, read outside any instance.
func (ctx *Context) declaredUniquenessRefusal(sym *symbols.Symbol, value *Value) string {
	mult := ctx.featureMultiplicity(sym, nil)
	if !mult.Upper.Infinite && mult.Upper.Value <= 1 {
		return ""
	}
	return ctx.uniquenessRefusal(ctx.model.IsUnique(sym), ctx.holdsSet(sym, ctx.findOwnerType(sym), mult), value)
}
