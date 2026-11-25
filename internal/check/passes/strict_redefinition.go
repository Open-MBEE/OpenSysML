package passes

import (
	"fmt"

	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
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
	c := &strictRedefinitionChecker{model: ctx.Model(), resolver: ctx.Resolver(), seen: make(map[*symbols.Symbol]bool)}
	c.walk(rootScope)
	return c.diags
}

type strictRedefinitionChecker struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	seen     map[*symbols.Symbol]bool
	diags    []diag.Diagnostic
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
	types, named, owner := c.contextTypes(sym)
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
		if member == "" || c.inaccessible(sym, rel, target) || c.resolvesIn(types, sym, member) ||
			c.declaringScopeProvides(sym, owner, types, target, member) || c.chainedBaseFeature(types, target) {
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
func (c *strictRedefinitionChecker) contextTypes(sym *symbols.Symbol) ([]*symbols.Symbol, *symbols.Symbol, *symbols.Symbol) {
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
		for _, ft := range c.model.FeaturingTypes(owner) {
			if ft != nil && !seen[ft] {
				seen[ft] = true
				types = append(types, ft)
			}
		}
		// A `featured by` clause can name a nested member whose symbol is still
		// being resolved; match it by name so its own view joins the context.
		for _, rel := range semantics.RelationshipsOf(owner) {
			if rel == nil || rel.Kind != ast.RelFeaturedBy || rel.Target == nil {
				continue
			}
			written, _ := ast.TargetName(rel.Target)
			if written != "" && (written == sym.Name || written == sym.ShortName) && !seen[sym] {
				seen[sym] = true
				types = append(types, sym)
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
	declaring := owner
	add(owner)
	for portionUsage(owner) {
		parent := owner.OwnerScope
		if parent == nil {
			break
		}
		owner = parent.Owner()
		add(owner)
	}
	return types, named, declaring
}

// declaringScopeProvides reports whether the clause's target is a name the
// declaration's scope legitimately supplies, beyond the declaring type's members.
func (c *strictRedefinitionChecker) declaringScopeProvides(sym, owner *symbols.Symbol, types []*symbols.Symbol, target *symbols.Symbol, member string) bool {
	if owner == nil || target == nil || target.OwnerScope == nil {
		return false
	}
	if slices.Contains(types, sym) && slices.Contains(c.model.AllRedefinedFeatures(sym), target) {
		return true
	}
	// A type that contributes members to the context also supplies the target's.
	targetOwner := target.OwnerScope.Owner()
	for _, typ := range types {
		if slices.Contains(c.model.MemberSources(typ), targetOwner) {
			return true
		}
	}
	if slices.Contains(c.model.MemberSources(owner), targetOwner) {
		return true
	}
	if !owner.IsFeature() {
		// A redefinition in a definition may name its enclosing namespace's
		// features, including ones reached by an import (KerML 8.2.3.5.2).
		if c.namespaceProvides(owner.OwnerScope, target, member) {
			return true
		}
		return target.OwnerScope.Owner() != nil && target.OwnerScope.Owner().Kind == symbols.SymbolPackage
	}
	if variantUsage(owner) {
		if c.namespaceProvides(owner.OwnerScope, target, member) {
			return true
		}
	}
	if !portionUsage(owner) && (len(c.model.DeclaredTypes(owner)) == 0 || redefinesOrSubsets(owner)) {
		// A usage with no declared typing reads names from its enclosing
		// namespace; one that redefines or subsets stands in for the original.
		if c.namespaceProvides(owner.OwnerScope, target, member) {
			return true
		}
	}
	if !redefinesOrSubsets(owner) {
		// A plain typed usage may name a feature of its enclosing namespace.
		// A portion reads names from the occurrence it portions, not that namespace.
		if !portionUsage(owner) {
			for s := owner.OwnerScope; s != nil; s = s.Parent() {
				if s == target.OwnerScope && isNamespace(s.Owner()) {
					return true
				}
			}
		}
		return false
	}
	// A usage that redefines or subsets stands in for the original feature,
	// so the names of the types that enclose it are in its declaration's scope.
	for enc := owner.OwnerScope.Owner(); enc != nil; {
		if c.resolvesIn([]*symbols.Symbol{enc}, sym, member) {
			return true
		}
		for _, typ := range c.model.DeclaredTypes(enc) {
			if c.resolvesIn([]*symbols.Symbol{typ}, sym, member) {
				return true
			}
		}
		if enc.OwnerScope == nil {
			break
		}
		enc = enc.OwnerScope.Owner()
	}
	return false
}

// isNamespace reports whether sym is a package or namespace rather than a type.
func isNamespace(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace)
}

// variantUsage reports whether a usage is a variant of an enclosing variation;
// a variant inherits the variation's naming context.
func variantUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() && sym.Facts != nil {
		return sym.Facts.Modifiers.Has(symbols.ModVariant)
	}
	u, ok := sym.Decl.(*ast.Usage)
	return ok && u.IsVariant
}

// namespaceProvides reports whether target is the name member in a scope or
// one of its ancestors, including a feature reached by an import.
func (c *strictRedefinitionChecker) namespaceProvides(scope *symbols.Scope, target *symbols.Symbol, member string) bool {
	for scope != nil {
		if target.OwnerScope == scope {
			return true
		}
		if c.resolver != nil {
			if found, ok := c.resolver.LookupName(scope, member); ok && found == target {
				return true
			}
		}
		scope = scope.Parent()
	}
	return false
}

// chainedBaseFeature reports whether target is a chain leaf declared by the
// hierarchy: `Anything::self` chains `things.that`.
func (c *strictRedefinitionChecker) chainedBaseFeature(types []*symbols.Symbol, target *symbols.Symbol) bool {
	var found bool
	visit := func(src *symbols.Symbol) {
		if found || src == nil || src.Scope == nil {
			return
		}
		src.Scope.ForEachMember(func(feature *symbols.Symbol) bool {
			if feature == nil {
				return true
			}
			for _, rel := range semantics.RelationshipsOf(feature) {
				if rel == nil || rel.Kind != ast.RelChains || rel.Target == nil {
					continue
				}
				if _, ok := rel.Target.(*ast.FeatureChainExpr); !ok {
					continue
				}
				path := c.model.ChainTargetPath(feature, rel.Kind, rel.Target)
				if len(path) > 1 && path[len(path)-1] == target {
					found = true
					return false
				}
			}
			return true
		})
	}
	for _, typ := range types {
		visit(typ)
		for _, src := range c.model.MemberSources(typ) {
			visit(src)
		}
	}
	return found
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
