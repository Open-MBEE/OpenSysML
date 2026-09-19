package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Pilot SysMLValidator (2026-05) checkOccurrenceUsage, constraints
// validateOccurrenceUsageIndividualDefinition and
// validateOccurrenceUsagePortionOwning.
const (
	msgIndividualManyTypes = "At most one individual definition is allowed."
	msgPortionOwner        = "Must be owned by an occurrence definition or usage."
)

// W10BIndividualTypingPass checks that a usage carrying the `individual`
// modifier is typed by exactly one individual definition (SysML v2 §8.3.9.11).
// Other types are allowed beside it, as they are in the reference.
type W10BIndividualTypingPass struct{}

func (W10BIndividualTypingPass) Level() PassLevel { return LevelType }

func (W10BIndividualTypingPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	resolver := ctx.Resolver()
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || (!u.IsIndividual && u.Kind != ast.UsageIndividual) {
			return
		}
		msg := msgIndividualOneType
		switch n := w10bIndividualDefTypings(resolver, sym); {
		case n == 1:
			return
		case n > 1:
			msg = msgIndividualManyTypes
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  msg,
			Code:     "individual-typing",
			Source:   "type",
		})
	})
	return diags
}

// w10bIndividualDefTypings counts the distinct individual definitions typing
// sym, declared on it or inherited from a feature it subsets or redefines.
func w10bIndividualDefTypings(resolver *resolve.Resolver, sym *symbols.Symbol) int {
	seen := make(map[*symbols.Symbol]bool)
	for _, typ := range usageTypesOf(resolver, sym, true, make(map[*symbols.Symbol]bool)) {
		if w10bIsIndividualDef(typ.sym) {
			seen[typ.sym] = true
		}
	}
	return len(seen)
}

// w10bIsIndividualDef reports whether sym is an individual definition, written
// either as `individual def` or as `individual <kind> def`.
func w10bIsIndividualDef(sym *symbols.Symbol) bool {
	if sym.Kind == symbols.SymbolIndividualDef {
		return true
	}
	def, ok := sym.Decl.(*ast.Definition)
	return ok && def.IsIndividual
}

// W10BPortionOwnerPass checks that a `snapshot` or `timeslice` usage is owned by
// an occurrence definition or usage: a portion of an occurrence needs the
// occurrence it is a portion of (SysML v2 §8.3.9.11).
type W10BPortionOwnerPass struct{}

func (W10BPortionOwnerPass) Level() PassLevel { return LevelConstraint }

func (W10BPortionOwnerPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || u.Portion == ast.PortionNone || w10bOwnedByOccurrence(sym) {
			return
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  msgPortionOwner,
			Code:     "portion-owner",
			Source:   "constraint",
		})
	})
	return diags
}

// w10bOwnedByOccurrence reports whether the declaration owning sym is an
// occurrence definition or an occurrence usage.
func w10bOwnedByOccurrence(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	switch d := owner.Decl.(type) {
	case *ast.Definition:
		return isOccurrenceDefKind(defSymbolKind(d.Kind))
	case *ast.Usage:
		if d.IsIndividual || d.Portion != ast.PortionNone {
			return true
		}
		return isOccurrenceDefKind(usageWantsDefKind(d.Kind))
	}
	return false
}
