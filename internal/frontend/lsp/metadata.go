package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// isAnnotationBodyScope reports whether scope is the anonymous scope built for
// the members of a `@A { ... }` annotation body.
func isAnnotationBodyScope(scope *symbols.Scope) bool {
	if scope == nil || !scope.BodyLocal() {
		return false
	}
	_, ok := scope.Node().(*ast.PrefixMetadata)
	return ok
}

// usageNameAt returns the usage declaration whose own name contains offset, or
// nil when the cursor is elsewhere.
func usageNameAt(scope *symbols.Scope, offset int) *symbols.Symbol {
	sym := symbolAtOffset(scope, offset)
	if sym == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	for _, sp := range []source.Span{usage.Ident.NameSpan, usage.Ident.ShortNameSpan} {
		if sp.Len > 0 && offset >= sp.Offset && offset < sp.End() {
			return sym
		}
	}
	return nil
}
