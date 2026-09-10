package runtime

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// libraryTier reports the tier of the library that declares sym, TierNone for a
// declaration of the model under evaluation.
func (ctx *Context) libraryTier(sym *symbols.Symbol) symbols.LibraryTier {
	if ctx == nil || ctx.model.resolver == nil {
		return symbols.TierNone
	}
	idx := ctx.model.resolver.Index()
	if idx == nil {
		return symbols.TierNone
	}
	return idx.LibraryTier(sym)
}
