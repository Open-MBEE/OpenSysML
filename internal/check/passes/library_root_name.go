package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// LibraryRootNamePass reports a top-level member named like a standard
// library package: a qualified name starting with it can resolve to the
// library package instead, as the OMG pilot's index does where the library
// is indexed first, so the name is a portability hazard. The warning is not
// an error: the specification allows same-named roots.
type LibraryRootNamePass struct{}

func (LibraryRootNamePass) Level() PassLevel { return LevelNameResolution }

func (LibraryRootNamePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	if w9cIsLibraryDocument(ctx, name) {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	seen := map[source.Span]bool{}
	rootScope.ForEachMember(func(sym *symbols.Symbol) bool {
		for _, key := range w9cKeysOf(sym) {
			if seen[key.span] {
				continue
			}
			for _, lib := range ctx.Index.LookupQualified(key.name) {
				if ctx.Index.Library(lib) {
					seen[key.span] = true
					diags = append(diags, diag.Diagnostic{
						Severity: diag.SeverityWarning,
						Span:     key.span,
						Message:  "Root namespace `" + key.name + "` has the name of a standard library package, so qualified names starting with it may resolve to the library",
						Code:     "library-root-name",
						Source:   "name-resolution",
					})
					break
				}
			}
		}
		return true
	})
	sort.SliceStable(diags, func(i, j int) bool { return diags[i].Span.Offset < diags[j].Span.Offset })
	return diags
}
