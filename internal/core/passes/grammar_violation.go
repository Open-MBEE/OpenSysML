package passes

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// Parser warning codes the analysis reports as errors: the reference rejects both
// forms, while the parser reads them into the tree the author intended.
const (
	CodeImportVisibility      = "import-visibility"
	CodeEnumerationBodyMember = "enumeration-body-member"
)

// GrammarViolationPass escalates the parser's warnings on a bare `import` (ImportPrefix)
// and an illegal EnumerationBody member to errors; both read, so neither gates a tier.
type GrammarViolationPass struct{}

func (GrammarViolationPass) Level() PassLevel { return LevelSyntax }

func (GrammarViolationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil {
		return nil
	}
	var diags []Diagnostic
	for _, d := range ctx.ParseDiagnostics {
		if d.Severity != SeverityWarning {
			continue
		}
		switch d.Code {
		case CodeImportVisibility, CodeEnumerationBodyMember:
			d.Severity = SeverityError
			d.Notation = true
			diags = append(diags, d)
		}
	}
	return diags
}
