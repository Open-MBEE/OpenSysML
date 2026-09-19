package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot SysMLValidator (2026-05) constraints validateCalculationDefinitionOnlyOneResult,
// validateStateDefinition{Entry,Do,Exit}Action, validatePortDefinitionOwnedUsagesNotComposite,
// validatePortUsageNestedUsagesNotComposite and validatePortUsageIsReference. The
// one-objective rule stays unimplemented by decision (see docs/project/spec-compliance.md).
const (
	msgOnlyOneReturn        = "Only one return parameter is allowed"
	msgOnlyOneEntryAction   = "A state may have at most one entry action."
	msgOnlyOneDoAction      = "A state may have at most one do action."
	msgOnlyOneExitAction    = "A state may have at most one exit action."
	msgPortDefComposite     = "Owned usages of a port definition (other than ports) must be referential."
	msgPortUsageComposite   = "Nested usages in a port usage (other than ports) must be referential."
	msgVariantPortComposite = "A port usage must be referential."
)

// W10BStructuralPass checks how many members of a kind a declaration owns and
// whether they may be composite. The rules read the declaration only, so they
// sit at the type tier rather than behind it.
type W10BStructuralPass struct{}

func (W10BStructuralPass) Level() PassLevel { return LevelType }

func (W10BStructuralPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w10bStructuralChecker{}
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		c.check(sym.Decl)
	})
	return c.diags
}

type w10bStructuralChecker struct {
	diags []diag.Diagnostic
}

func (c *w10bStructuralChecker) check(decl ast.Node) {
	members := ast.DeclMembers(decl)
	if len(members) == 0 {
		return
	}
	if stateLikeDecl(decl) {
		var entry, do, exit []ast.Node
		for _, m := range members {
			switch m.(type) {
			case *ast.EntryMember:
				entry = append(entry, m)
			case *ast.DoMember:
				do = append(do, m)
			case *ast.ExitMember:
				exit = append(exit, m)
			}
		}
		c.reportExtra(entry, msgOnlyOneEntryAction, "state-entry-action")
		c.reportExtra(do, msgOnlyOneDoAction, "state-do-action")
		c.reportExtra(exit, msgOnlyOneExitAction, "state-exit-action")
	}
	if returnOwnerDecl(decl) {
		var returns []ast.Node
		for _, m := range members {
			if u, ok := unwrapUsageMember(m); ok && u.IsResult {
				returns = append(returns, m)
			}
		}
		c.reportExtra(returns, msgOnlyOneReturn, "only-one-return-parameter")
	}
	if msg, ok := portOwnerMessage(decl); ok {
		for _, m := range members {
			u, ok := unwrapUsageMember(m)
			if !ok || u.IsVariant {
				continue
			}
			c.checkVariantPorts(u)
			if u.Kind != ast.UsagePort && w10bIsComposite(u) {
				c.report(m, msg, "port-owned-usage-composite")
			}
		}
	}
}

// checkVariantPorts reports composite port variants under a port owner (validatePortUsageIsReference):
// a variant is no nested usage, so the owner's check reaches it only through here.
func (c *w10bStructuralChecker) checkVariantPorts(variation *ast.Usage) {
	if !variation.IsVariation {
		return
	}
	for _, m := range variation.Members {
		u, ok := unwrapUsageMember(m)
		if !ok || !u.IsVariant {
			continue
		}
		if u.Kind == ast.UsagePort && !w10bReferential(u) {
			c.report(m, msgVariantPortComposite, "variant-port-composite")
		}
		c.checkVariantPorts(u)
	}
}

func (c *w10bStructuralChecker) reportExtra(members []ast.Node, msg, code string) {
	if len(members) < 2 {
		return
	}
	for _, m := range members[1:] {
		c.report(m, msg, code)
	}
}

func (c *w10bStructuralChecker) report(node ast.Node, msg, code string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     node.Span(),
		Message:  msg,
		Code:     code,
		Source:   "type",
	})
}

// w10bReferential reports the declarations that are referential whatever their kind:
// `ref`, a direction, an end, an event, reference subsetting, a bare `variant x;`.
func w10bReferential(u *ast.Usage) bool {
	return u.IsReference || u.Direction != ast.DirNone || u.IsEnd || u.IsEvent ||
		w10bReferences(u) || u.IsVariantReference()
}

// w10bIsComposite reports whether a usage owns its occurrences: referential
// declarations do not; of the rest, parts, items and occurrences do by default.
func w10bIsComposite(u *ast.Usage) bool {
	if w10bReferential(u) {
		return false
	}
	if u.IsComposite {
		return true
	}
	switch u.Kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence:
		return true
	}
	return false
}

func w10bReferences(u *ast.Usage) bool {
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			return true
		}
	}
	return false
}

func portOwnerMessage(decl ast.Node) (string, bool) {
	switch d := decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefPort {
			return msgPortDefComposite, true
		}
	case *ast.Usage:
		if d.Kind == ast.UsagePort {
			return msgPortUsageComposite, true
		}
	}
	return "", false
}

// returnOwnerDecl reports whether decl may own a `return` parameter: a
// calculation, an expression or a case.
func returnOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc || isCaseDefKind(d.Kind)
	case *ast.Usage:
		return d.Kind == ast.UsageCalc || d.Kind == ast.UsageExpr || isCaseUsageKind(d.Kind)
	}
	return false
}
