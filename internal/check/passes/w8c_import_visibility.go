package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLValidator's validateImportTopLevelVisibility message.
const msgTopLevelImportPrivate = "Top level import must be private"

// TopLevelImportPass checks that an import owned by the root namespace is
// private (KerML 8.2.3.4.2, validateImportTopLevelVisibility): a root namespace
// has no owner, so a non-private import there exports into nothing. It runs at
// the constraint tier so a recovered parse is not reported on twice and this
// error does not gate the type and constraint tiers.
type TopLevelImportPass struct{}

func (TopLevelImportPass) Level() PassLevel { return LevelConstraint }

func (TopLevelImportPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if root == nil {
		return nil
	}
	var diags []diag.Diagnostic
	for _, m := range root.Members {
		imp, ok := unwrapType(m).(*ast.Import)
		if !ok || imp.IsExpose {
			continue
		}
		if imp.Visibility != ast.VisibilityPublic && imp.Visibility != ast.VisibilityProtected {
			continue
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     imp.Span(),
			Message:  msgTopLevelImportPrivate,
			Code:     "import-top-level-visibility",
			Source:   "constraint",
		})
	}
	return diags
}
