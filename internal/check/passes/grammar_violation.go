package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Parser warning codes the analysis reports as errors: the reference rejects both
// forms, while the parser reads them into the tree the author intended.
const (
	CodeImportVisibility      = "import-visibility"
	CodeExposeVisibility      = "expose-visibility"
	CodeEnumerationBodyMember = "enumeration-body-member"
)

// GrammarViolationPass escalates the parser's warnings on a bare `import` (ImportPrefix), a
// visibility-prefixed `expose` and an illegal EnumerationBody member to errors; all read, so none gates a tier.
type GrammarViolationPass struct{}

func (GrammarViolationPass) Level() PassLevel { return LevelSyntax }

func (GrammarViolationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil {
		return nil
	}
	var diags []diag.Diagnostic
	for _, d := range ctx.ParseDiagnostics {
		if d.Severity != diag.SeverityWarning {
			continue
		}
		switch d.Code {
		case CodeImportVisibility, CodeExposeVisibility, CodeEnumerationBodyMember:
			d.Severity = diag.SeverityError
			d.Notation = true
			diags = append(diags, d)
		}
	}
	return diags
}
