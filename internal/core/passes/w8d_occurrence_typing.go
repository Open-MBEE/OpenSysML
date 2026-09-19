package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Pilot SysMLValidator (2026-05) checkOccurrenceUsage, constraint
// validateOccurrenceUsageType: every type of an occurrence usage must be a Class.
const msgOccurrenceUsageType = "An occurrence, item or part must be typed by occurrence definitions."

// Pilot SysMLValidator (2026-05) checkOccurrenceUsage, constraint
// validateEventOccurrenceUsageReference.
const msgEventReferenceOccurrence = "Must reference an occurrence."

// w8dOccurrenceUsageKinds are the occurrence usages the rule covers. The
// reference exempts ports, connections, metadata usages and steps, which our
// taxonomy keeps as kinds of their own.
var w8dOccurrenceUsageKinds = map[ast.UsageKind]bool{
	ast.UsagePart:       true,
	ast.UsageItem:       true,
	ast.UsageOccurrence: true,
	ast.UsageIndividual: true,
}

// w8dNonOccurrenceUsageKinds are the usage kinds that are never occurrences, so
// an event occurrence may not reference them.
var w8dNonOccurrenceUsageKinds = map[ast.UsageKind]bool{
	ast.UsageAttribute:   true,
	ast.UsageEnumeration: true,
}

// w8dTypecheckedTypingKinds are the usage kinds whose own data-type typing the
// type checker already rejects, so this pass must not report it twice.
var w8dTypecheckedTypingKinds = map[ast.UsageKind]bool{
	ast.UsagePart:       true,
	ast.UsageItem:       true,
	ast.UsageIndividual: true,
}

// w8dDataTypeKinds are the definition kinds that are DataTypes rather than
// Classes, so no occurrence may be typed by them.
var w8dDataTypeKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolAttributeDef:   true,
	symbols.SymbolEnumerationDef: true,
}

// W8DOccurrenceTypingPass checks that an occurrence, item or part is typed by
// occurrence definitions only (SysML v2 §8.3.9.11, OccurrenceUsage).
type W8DOccurrenceTypingPass struct{}

func (W8DOccurrenceTypingPass) Level() PassLevel { return LevelType }

func (W8DOccurrenceTypingPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	oc := &w8dOccurrenceChecker{resolver: ctx.Resolver()}
	kit.WalkSymbols(ctx, rootScope, oc.check)
	return oc.diags
}

type w8dOccurrenceChecker struct {
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

func (oc *w8dOccurrenceChecker) check(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || !w8dOccurrenceUsageKinds[u.Kind] {
		return
	}
	if u.IsEvent {
		oc.checkEventReference(sym, u)
	}
	for _, typ := range usageTypesOf(oc.resolver, sym, true, make(map[*symbols.Symbol]bool)) {
		if !w8dDataTypeKinds[typ.sym.Kind] {
			continue
		}
		if typ.declared && (w8dTypecheckedTypingKinds[u.Kind] || u.IsIndividual || u.Portion != ast.PortionNone) {
			continue
		}
		oc.diags = append(oc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  msgOccurrenceUsageType,
			Code:     "occurrence-usage-type",
			Source:   "type",
		})
		return
	}
}

// checkEventReference checks that an event occurrence references an occurrence.
func (oc *w8dOccurrenceChecker) checkEventReference(sym *symbols.Symbol, u *ast.Usage) {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		target, ok := oc.resolver.ResolveTarget(kit.ReferenceScope(sym), rel.Target)
		if !ok || target == nil {
			continue
		}
		ref, ok := target.Decl.(*ast.Usage)
		if !ok || !w8dNonOccurrenceUsageKinds[ref.Kind] {
			continue
		}
		oc.diags = append(oc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  msgEventReferenceOccurrence,
			Code:     "event-reference-occurrence",
			Source:   "type",
		})
	}
}

// usageTypesOf returns the types of a usage: the definitions it declares with
// `:`, plus the types of the features it subsets, redefines or references, which
// it inherits (FeatureUtil.getAllTypesOf).
func usageTypesOf(resolver *resolve.Resolver, sym *symbols.Symbol, own bool, visited map[*symbols.Symbol]bool) []w8dUsageType {
	if sym == nil || visited[sym] {
		return nil
	}
	visited[sym] = true
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	scope := kit.ReferenceScope(sym)
	var types []w8dUsageType
	for _, rel := range decl.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSubsets, ast.RelRedefines, ast.RelReferences:
		default:
			continue
		}
		target, ok := resolver.ResolveTarget(scope, rel.Target)
		if !ok || target == nil {
			continue
		}
		if target.Kind == symbols.SymbolAlias {
			if resolved, ok := resolver.ResolveAliasTarget(target); ok && resolved != nil {
				target = resolved
			}
		}
		if rel.Kind == ast.RelTyping {
			types = append(types, w8dUsageType{sym: target, declared: own})
			continue
		}
		types = append(types, usageTypesOf(resolver, target, false, visited)...)
	}
	return types
}

// w8dUsageType is a type of a usage, declared on the usage itself or inherited
// from a feature it subsets, redefines or references.
type w8dUsageType struct {
	sym      *symbols.Symbol
	declared bool
}
