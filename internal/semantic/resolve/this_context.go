package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Every library declaration of the context occurrence feature ends in `::this`,
// which each occurrence kind redefines ([KerML] Occurrences::Occurrence::this).
const (
	thisFeatureName   = "this"
	thisFeatureSuffix = "::this"
)

// IsOccurrenceThis reports whether sym is the context occurrence feature `this`,
// whose members a chain reads from the object it denotes (see ThisContext).
func (r *Resolver) IsOccurrenceThis(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Name == thisFeatureName || strings.HasSuffix(sym.Name, thisFeatureSuffix) {
		return true
	}
	// Only a name that is itself a tail of `::this` can qualify to that suffix,
	// so the qualified name is built for those alone.
	return strings.HasSuffix(thisFeatureSuffix, sym.Name) &&
		strings.HasSuffix(r.registeredFQN(sym), thisFeatureSuffix)
}

// ThisContext returns the object `this` denotes where scope was written: the
// innermost enclosing object, since an owned performance and its subperformances
// take their owner's `this` ([KerML] Objects::ownedPerformances, [SysML]
// Parts::Part::this, Actions::Action::subactions). A behavior usage is not its
// own context — it keeps walking — but a behavior definition is the occurrence
// it defines, so `this` inside one denotes that definition itself and the walk
// stops there. Nil in a standalone behavior, where `this` is the performance
// itself.
func (r *Resolver) ThisContext(scope *symbols.Scope) *symbols.Symbol {
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil {
			continue
		}
		switch {
		case isObjectKind(owner.Kind):
			return owner
		case isBehaviorDefKind(owner.Kind):
			return owner
		case isBehaviorUsageKind(owner.Kind):
			continue
		default:
			return nil
		}
	}
	return nil
}

// isObjectKind reports whether a symbol declares an object: the `this` of the
// performances it owns.
func isObjectKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage:
		return true
	}
	return false
}

// isBehaviorDefKind reports whether a symbol is a behavior definition: the
// occurrence its `this` denotes.
func isBehaviorDefKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolActionDef, symbols.SymbolStateDef, symbols.SymbolCalcDef,
		symbols.SymbolCaseDef, symbols.SymbolAnalysisCaseDef,
		symbols.SymbolVerificationCaseDef, symbols.SymbolUseCaseDef:
		return true
	}
	return false
}

// isBehaviorUsageKind reports whether a symbol is a behavior usage: a performance
// that takes its `this` from what owns it.
func isBehaviorUsageKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage:
		return true
	}
	return false
}
