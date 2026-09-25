package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// NameResolutionPass resolves every reference in a document via the Plan-3
// resolver and reports unresolved / ambiguous references.
type NameResolutionPass struct{}

// Level reports the name-resolution dependency level.
func (NameResolutionPass) Level() PassLevel { return LevelNameResolution }

// Run resolves the document and adapts resolver diagnostics.
func (NameResolutionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	// Initialize model to enable inheritance-aware resolution
	_ = ctx.Model()
	r := ctx.Resolver()
	r.ResolveDocument(name, root)
	rd := r.Diagnostics
	if len(rd) == 0 {
		return nil
	}
	out := make([]diag.Diagnostic, 0, len(rd))
	for _, d := range rd {
		code := d.Code
		if code == "" {
			code = "unresolved"
			if strings.HasPrefix(d.Message, "ambiguous") {
				code = "ambiguous"
			}
		}
		severity := diag.SeverityError
		if d.Warning {
			severity = diag.SeverityWarning
		}
		out = append(out, diag.Diagnostic{
			Severity: severity,
			Span:     d.Span,
			Message:  d.Message,
			Code:     code,
			Source:   "name-resolution",
			Fixes:    d.Fixes,
		})
	}
	return out
}
