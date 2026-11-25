package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeRedefinitionResolvesToNothing marks a `:>>` clause naming no member of the declaring type.
const CodeRedefinitionResolvesToNothing = "redefinition-resolves-to-nothing"

// StrictRedefinitionPass reports targetless redefinitions, in strict conformance only.
// It is an export-integrity check over members, not an extension to the grammar.
type StrictRedefinitionPass struct{}

func (StrictRedefinitionPass) Level() PassLevel { return LevelConstraint }

// Run walks the document's features only when the caller asked for strict conformance.
func (StrictRedefinitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || !ctx.Options.Conformance.IsStrict() || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &strictRedefinitionChecker{model: ctx.Model(), seen: make(map[*symbols.Symbol]bool)}
	c.walk(rootScope)
	return c.diags
}

type strictRedefinitionChecker struct {
	model *semantics.Model
	seen  map[*symbols.Symbol]bool
	diags []diag.Diagnostic
}

// walk visits each symbol once, deduping the two keys of a short/primary-name declaration.
func (c *strictRedefinitionChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym == nil || c.seen[sym] {
			return true
		}
		c.seen[sym] = true
		c.check(sym)
		c.walk(sym.Scope)
		return true
	})
}

func (c *strictRedefinitionChecker) check(sym *symbols.Symbol) {
	if !sym.IsFeature() {
		return
	}
	types, named := c.contextTypes(sym)
	if len(types) == 0 {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		// Chains and unresolved names are judged by their own rules.
		target := c.model.RelationshipTarget(sym, rel)
		if chainTarget(rel.Target) || target == nil {
			continue
		}
		member, _ := ast.TargetName(rel.Target)
		if member == "" || c.inaccessible(sym, rel, target) || c.resolvesIn(types, sym, member) {
			continue
		}
		typ := types[0]
		if named != nil {
			typ = named
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  fmt.Sprintf("member %s redefines nothing resolvable in %s", member, typ.Name),
			Code:     CodeRedefinitionResolvesToNothing,
			Source:   "constraint",
		})
	}
}

// inaccessible leaves a failed featuring-type check to the always-on redefinition rule.
func (c *strictRedefinitionChecker) inaccessible(sym *symbols.Symbol, rel *ast.Relationship, target *symbols.Symbol) bool {
	if target == nil || target == sym || !target.IsFeature() {
		return false
	}
	accessed := target
	if path := c.model.ChainTargetPath(sym, rel.Kind, rel.Target); len(path) > 0 {
		accessed = path[0]
	}
	return accessed != nil && accessed.IsFeature() && !c.model.CanAccess(sym, accessed)
}

// contextTypes lists the declaring type's members and, for a portion, its occurrence's.
// Only snapshot and timeslice owners contribute an enclosing occurrence's declared type.
func (c *strictRedefinitionChecker) contextTypes(sym *symbols.Symbol) ([]*symbols.Symbol, *symbols.Symbol) {
	var types []*symbols.Symbol
	var named *symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	add := func(owner *symbols.Symbol) {
		if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
			return
		}
		declared := c.model.DeclaredTypes(owner)
		fromTyping := len(declared) > 0
		if !fromTyping {
			declared = []*symbols.Symbol{owner}
		}
		for _, typ := range declared {
			if typ == nil || seen[typ] {
				continue
			}
			seen[typ] = true
			types = append(types, typ)
			if fromTyping && named == nil {
				named = typ
			}
		}
		// A typed usage that redefines or subsets a usage inherits that usage's
		// members, so its own declaring view joins the declared type's.
		if redefinesOrSubsets(owner) && !seen[owner] {
			seen[owner] = true
			types = append(types, owner)
		}
	}
	owner := c.model.OwningType(sym)
	if owner == nil && sym.OwnerScope != nil {
		owner = sym.OwnerScope.Owner()
	}
	add(owner)
	for portionUsage(owner) {
		parent := owner.OwnerScope
		if parent == nil {
			break
		}
		owner = parent.Owner()
		add(owner)
	}
	return types, named
}

// resolvesIn checks the declaring member view, which suspends the mask the clause itself causes.
func (c *strictRedefinitionChecker) resolvesIn(types []*symbols.Symbol, sym *symbols.Symbol, member string) bool {
	for _, typ := range types {
		if typ == nil {
			continue
		}
		for _, m := range c.model.MembersOfDeclaring(typ, sym) {
			if m.Name == member || m.ShortName == member {
				return true
			}
		}
	}
	return false
}

// portionUsage reports a snapshot or timeslice, whose names read from its occurrence.
func portionUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		return sym.Facts != nil && sym.Facts.Modifiers.Has(symbols.ModPortion)
	}
	u, ok := sym.Decl.(*ast.Usage)
	return ok && u.Portion != ast.PortionNone
}

// redefinesOrSubsets reports whether a usage redefines or subsets: both make
// the target's features members of the usage.
func redefinesOrSubsets(sym *symbols.Symbol) bool {
	if sym == nil || !sym.IsFeature() {
		return false
	}
	if sym.Recorded() {
		if len(sym.Facts.Redefines) > 0 || !sym.Facts.References.IsZero() {
			return true
		}
		for _, rel := range sym.Facts.Relationships {
			if rel.Kind == ast.RelRedefines || rel.Kind == ast.RelSubsets || rel.Kind.ReferenceSubsets() {
				return true
			}
		}
		return false
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		if rel.Kind == ast.RelRedefines || rel.Kind == ast.RelSubsets || rel.Kind.ReferenceSubsets() {
			return true
		}
	}
	return false
}

// chainTarget reports a chain target, unwrapping a feature reference to its name.
func chainTarget(node ast.Node) bool {
	if ref, ok := node.(*ast.FeatureReference); ok {
		node = ref.Name
	}
	_, ok := node.(*ast.FeatureChainExpr)
	return ok
}
