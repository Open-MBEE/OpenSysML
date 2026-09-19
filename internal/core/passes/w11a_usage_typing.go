package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Messages of the reference-kind rules: a performed action names an action and
// an exhibited state a state (SysML v2 §8.3.16.7 validatePerformActionUsage,
// §8.3.17.6 validateExhibitStateUsage).
const (
	msgReferenceAction  = "Must reference an action."
	msgReferenceState   = "Must reference a state."
	msgReferenceUseCase = "Must reference a use case."
)

// w11aReferenceKinds is the definition family a reference member must name,
// keyed by the keyword that introduces it. A use case and a flow connection are
// actions too, so the family is matched through the definition taxonomy.
var w11aReferenceKinds = map[string]struct {
	kind symbols.SymbolKind
	msg  string
}{
	"perform": {symbols.SymbolActionDef, msgReferenceAction},
	"exhibit": {symbols.SymbolStateDef, msgReferenceState},
	"include": {symbols.SymbolUseCaseDef, msgReferenceUseCase},
}

// w11aInheritedTypingKinds are the usage kinds whose type is constrained by
// their kind and whose typing may be inherited rather than declared: a port is
// typed by port definitions, an action by behaviors and a state by state
// definitions (SysML v2 §8.3.11.4, §8.3.16.6, §8.3.17.5). The occurrence family
// has its own pass (W8D); a declared typing is the type checker's.
var w11aInheritedTypingKinds = map[ast.UsageKind]bool{
	ast.UsagePort:   true,
	ast.UsageAction: true,
	ast.UsageState:  true,
}

// W11AUsageTypingPass checks the kind of a usage's type where the typing is
// inherited through a subsetting, redefinition or reference subsetting, and the
// kind of the feature a `perform`/`exhibit` member references.
type W11AUsageTypingPass struct{}

func (W11AUsageTypingPass) Level() PassLevel { return LevelType }

func (W11AUsageTypingPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w11aUsageChecker{resolver: ctx.Resolver()}
	kit.WalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w11aUsageChecker struct {
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

func (c *w11aUsageChecker) check(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return
	}
	c.checkReference(sym, u)
	if !w11aInheritedTypingKinds[u.Kind] {
		return
	}
	msg, code, ok := pilotTypingMessage(declKind{useKind: u.Kind, span: u.Span()})
	if !ok {
		return
	}
	for _, typ := range usageTypesOf(c.resolver, sym, true, make(map[*symbols.Symbol]bool)) {
		// A declared typing is reported by the type checker, and an
		// unclassified or KerML type constrains nothing.
		if typ.declared || typ.sym.Kind == symbols.SymbolUnknown ||
			typ.sym.Kind == symbols.SymbolKerMLType || !isDefKind(typ.sym.Kind) {
			continue
		}
		if isCompatibleTyping(u.Kind, u.Direction, typ.sym.Kind, false) {
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  msg,
			Code:     code,
			Source:   "type",
		})
		return
	}
}

// checkReference checks that the feature a reference member names is of the kind
// the keyword performs or exhibits.
func (c *w11aUsageChecker) checkReference(sym *symbols.Symbol, u *ast.Usage) {
	keyword := u.Keyword
	relKind := ast.RelReferences
	if u.Kind == ast.UsageUseCase && keyword == "" {
		keyword = "include"
		relKind = ast.RelIncludes
	}
	want, ok := w11aReferenceKinds[keyword]
	if !ok {
		return
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != relKind || rel.Target == nil {
			continue
		}
		target, ok := c.resolver.ResolveTarget(kit.ReferenceScope(sym), rel.Target)
		if !ok || target == nil {
			continue
		}
		kind := w11aFamilyOf(target)
		if kind == symbols.SymbolUnknown || defKindSpecializes(kind, want.kind) {
			if keyword == "include" && target.Kind == symbols.SymbolUseCaseUsage {
				c.checkIncludedUseCase(sym, target)
			}
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  want.msg,
			Code:     "usage-reference-kind",
			Source:   "type",
		})
	}
}

func (c *w11aUsageChecker) checkIncludedUseCase(usage, target *symbols.Symbol) {
	for _, typ := range usageTypesOf(c.resolver, target, true, make(map[*symbols.Symbol]bool)) {
		if typ.sym.Kind == symbols.SymbolUnknown ||
			typ.sym.Kind == symbols.SymbolKerMLType || !isDefKind(typ.sym.Kind) {
			continue
		}
		if compatibleTyping(ast.UsageUseCase, ast.DirNone, typ.sym.Kind) {
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     usage.Decl.Span(),
			Message:  oneTypeUsageMessages[ast.UsageUseCase],
			Code:     "one-type",
			Source:   "type",
		})
		return
	}
}

// w11aFamilyOf is the definition kind a declaration belongs to: a usage is
// classified by the definition kind that types it.
func w11aFamilyOf(sym *symbols.Symbol) symbols.SymbolKind {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return usageWantsDefKind(d.Kind)
	case *ast.Definition:
		return defSymbolKind(d.Kind)
	}
	return symbols.SymbolUnknown
}
