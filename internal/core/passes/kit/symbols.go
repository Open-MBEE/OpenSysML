package kit

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// WalkSymbols visits every symbol of the scope subtree exactly once.
func WalkSymbols(ctx *Context, root *symbols.Scope, visit func(*symbols.Symbol)) {
	for _, sym := range Symbols(ctx, root) {
		visit(sym)
	}
}

func Symbols(ctx *Context, root *symbols.Scope) []*symbols.Symbol {
	if ctx != nil {
		if cached, ok := ctx.symbolCache[root]; ok {
			return cached
		}
	}
	out := collectSymbols(root)
	if ctx != nil {
		if ctx.symbolCache == nil {
			ctx.symbolCache = make(map[*symbols.Scope][]*symbols.Symbol)
		}
		ctx.symbolCache[root] = out
	}
	return out
}

func collectSymbols(root *symbols.Scope) []*symbols.Symbol {
	seenSyms := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var out []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym == nil || seenSyms[sym] {
				return true
			}
			seenSyms[sym] = true
			out = append(out, sym)
			walk(sym.Scope)
			return true
		})
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
	return out
}

// ReferenceScope returns the scope a symbol's own references resolve in.
func ReferenceScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}
