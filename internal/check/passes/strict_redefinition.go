package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeRedefinitionResolvesToNothing marks a `:>>` clause that names no member of
// the type declaring the redefining feature.
const CodeRedefinitionResolvesToNothing = "redefinition-resolves-to-nothing"

// StrictRedefinitionPass reports redefinitions that name nothing in the type that
// declares them. A Redefinition replaces a feature the declaring type inherits
// (KerML 1.0 §8.3.3.3.6): a name the resolver reaches only through an import or
// the enclosing namespace is not a member of that type, and the redefinition
// would be written into the export as a handle with no target. Under strict
// conformance that is an error; in the default mode the pass reports nothing,
// so models using OpenSysML's looser name lookup are unaffected.
type StrictRedefinitionPass struct{}

func (StrictRedefinitionPass) Level() PassLevel { return LevelConstraint }

// Run walks the document's symbols in the strict conformance mode only.
func (StrictRedefinitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || !ctx.Options.Conformance.IsStrict() || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &strictRedefinitionChecker{
		model: ctx.Model(),
		seen:  make(map[*symbols.Symbol]bool),
	}
	c.walk(rootScope)
	return c.diags
}

type strictRedefinitionChecker struct {
	model *semantics.Model
	seen  map[*symbols.Symbol]bool
	diags []diag.Diagnostic
}

// walk visits every symbol of the subtree once: a declaration with a short and
// a primary name registers the same *Symbol under two keys.
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
	types := c.contextTypes(sym)
	if len(types) == 0 {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		// A chain redefines below the feature its prefix denotes; whether that
		// feature owns the leaf is a separate rule (redefinition-through-reference).
		if chainTarget(rel.Target) {
			continue
		}
		member, _ := ast.TargetName(rel.Target)
		if member == "" {
			continue
		}
		// A name that names no element is the name-resolution tier's finding;
		// reporting it here too would duplicate it.
		if c.model.RelationshipTarget(sym, rel) == nil {
			continue
		}
		if c.resolvesIn(types, sym, member) {
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  fmt.Sprintf("member %s redefines nothing resolvable in %s", member, types[0].Name),
			Code:     CodeRedefinitionResolvesToNothing,
			Source:   "constraint",
		})
	}
}

// contextTypes is the chain of types whose members a redefinition written in
// sym can name. The innermost is the declared type of the owning usage, or the
// owning definition; the enclosing usages contribute theirs too, since a body
// nested in one (a snapshot or timeslice of an occurrence) redefines features
// of the type that occurrence instantiates. Packages and namespaces own no
// feature members and end the chain.
func (c *strictRedefinitionChecker) contextTypes(sym *symbols.Symbol) []*symbols.Symbol {
	var types []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	add := func(owner *symbols.Symbol) {
		if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
			return
		}
		declared := c.model.DeclaredTypes(owner)
		if len(declared) == 0 {
			declared = []*symbols.Symbol{owner}
		}
		for _, typ := range declared {
			if typ != nil && !seen[typ] {
				seen[typ] = true
				types = append(types, typ)
			}
		}
	}
	if owner := c.model.OwningType(sym); owner != nil {
		add(owner)
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		add(scope.Owner())
	}
	return types
}

// resolvesIn reports whether the type offers a member of that name to the
// declaration being written. The declaring view suspends the mask the feature
// being checked would itself cause, so its own redefinition stays visible.
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

// chainTarget reports whether a relationship target is written as a feature
// chain (`a.b`) rather than a name, unwrapping a feature reference to its name.
func chainTarget(node ast.Node) bool {
	if ref, ok := node.(*ast.FeatureReference); ok {
		node = ref.Name
	}
	_, ok := node.(*ast.FeatureChainExpr)
	return ok
}
