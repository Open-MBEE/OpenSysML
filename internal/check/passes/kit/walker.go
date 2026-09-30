package kit

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w8cWalker visits every symbol of a document's scope tree once, deduping by
// pointer: one declaration may be registered under several keys.
type Walker struct {
	Ctx *Context
	// walked is what the first walk visited, which w8cSymbols already dedupes;
	// seen is built only when a later walk must dedupe against it.
	walked []*symbols.Symbol
	seen   map[*symbols.Symbol]bool
}

func (w *Walker) Walk(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if w == nil || scope == nil {
		return
	}
	syms := MemberSymbols(w.Ctx, scope)
	if w.walked == nil {
		w.walked = syms
		for _, sym := range syms {
			visit(sym)
		}
		return
	}
	if w.seen == nil {
		w.seen = make(map[*symbols.Symbol]bool, len(w.walked))
		for _, sym := range w.walked {
			w.seen[sym] = true
		}
	}
	for _, sym := range syms {
		if w.seen[sym] {
			continue
		}
		w.seen[sym] = true
		visit(sym)
	}
}

// w8cSymbols lists the scope tree's symbols once each, in visiting order, and
// caches the list on ctx for the passes that share it.
func MemberSymbols(ctx *Context, root *symbols.Scope) []*symbols.Symbol {
	if ctx != nil {
		if cached, ok := ctx.memberCache[root]; ok {
			return cached
		}
	}
	out := collectMemberSymbols(root)
	if ctx != nil {
		if ctx.memberCache == nil {
			ctx.memberCache = make(map[*symbols.Scope][]*symbols.Symbol)
		}
		ctx.memberCache[root] = out
	}
	return out
}

func collectMemberSymbols(root *symbols.Scope) []*symbols.Symbol {
	seen := make(map[*symbols.Symbol]bool)
	var out []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym == nil || seen[sym] {
				return true
			}
			seen[sym] = true
			out = append(out, sym)
			walk(sym.Scope)
			return true
		})
	}
	walk(root)
	return out
}

// w8cScopeOf returns the scope a declaration's own references resolve in.
func DeclarationScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return sym.Scope
}

// w8cMultiplicityOf returns the multiplicity a declaration declares, or nil.
func MultiplicityOf(sym *symbols.Symbol) *ast.Multiplicity {
	if sym == nil {
		return nil
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Multiplicity
	case *ast.ConnectorEnd:
		return d.Multiplicity
	case *ast.SubjectMember:
		return d.Multiplicity
	case *ast.AssumeMember:
		return d.Multiplicity
	case *ast.RequireMember:
		return d.Multiplicity
	default:
		return semantics.UsageMultiplicityOf(sym)
	}
}

// w8cIsReference reports whether n names a feature rather than computing a value.
func IsReference(n ast.Node) bool {
	switch n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	default:
		return false
	}
}

// w8cChainStep is one chaining feature: the node resolving to it and the span
// of the segment naming it.
type ChainStep struct {
	Node ast.Node
	Span source.Span
}

// w8cChainSteps splits a chain target into chaining features: only a `.` starts
// a new one, a `::`-qualified name is a single chaining feature.
func ChainSteps(target ast.Node) []ChainStep {
	switch t := target.(type) {
	case nil:
		return nil
	case *ast.FeatureReference:
		if t.Name == nil {
			return nil
		}
		return []ChainStep{{Node: t, Span: t.Name.Span()}}
	case *ast.FeatureChainExpr:
		steps := ChainSteps(t.Operand)
		span := t.Span()
		if t.Member != nil {
			span = t.Member.Span()
		}
		return append(steps, ChainStep{Node: t, Span: span})
	default:
		return []ChainStep{{Node: target, Span: target.Span()}}
	}
}

// UnwrapMembership strips the membership a declaration reaches a body wrapped in.
func UnwrapMembership(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// BodyScope returns the scope decl declares into, or scope itself when the
// scope builder gave it none.
func BodyScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	if scope == nil || decl == nil {
		return scope
	}
	if child := scope.ChildFor(decl); child != nil {
		return child
	}
	return scope
}

// ScopedNode is a node of a document with the scope it is written in.
type ScopedNode struct {
	Scope *symbols.Scope
	Node  ast.Node
}

// ScopedNodes lists what WalkScoped visits under root, in visiting order. The
// list of the document ctx analyzes is kept on ctx for the passes that walk it
// again; any other root is walked each time.
func ScopedNodes(ctx *Context, root *symbols.Scope) []ScopedNode {
	if root == nil {
		return nil
	}
	own := ctx != nil && ctx.ownRoot() == root
	if own {
		if cached, ok := ctx.scopedNodes[root]; ok {
			return cached
		}
	}
	var out []ScopedNode
	WalkScoped(root, func(scope *symbols.Scope, node ast.Node) {
		out = append(out, ScopedNode{Scope: scope, Node: node})
	})
	if own {
		if ctx.scopedNodes == nil {
			ctx.scopedNodes = make(map[*symbols.Scope][]ScopedNode)
		}
		ctx.scopedNodes[root] = out
	}
	return out
}

// WalkScoped visits every node of the document whose scope tree root heads,
// each with the scope it is written in: a node that declares a scope of its own
// is visited in the enclosing one, and its descendants in its own.
func WalkScoped(root *symbols.Scope, visit func(scope *symbols.Scope, node ast.Node)) {
	if root == nil {
		return
	}
	owned := map[ast.Node]*symbols.Scope{}
	var index func(*symbols.Scope)
	index = func(s *symbols.Scope) {
		if n := s.Node(); n != nil {
			if _, seen := owned[n]; !seen {
				owned[n] = s
			}
		}
		for _, child := range s.Children() {
			index(child)
		}
	}
	index(root)
	var walk func(*symbols.Scope)
	walk = func(s *symbols.Scope) {
		if n := s.Node(); n != nil && owned[n] == s {
			ast.Inspect(n, func(node ast.Node) bool {
				if node == n {
					return true
				}
				visit(s, node)
				_, nested := owned[node]
				return !nested
			})
		}
		for _, child := range s.Children() {
			walk(child)
		}
	}
	walk(root)
}
