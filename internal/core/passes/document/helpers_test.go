package document_test

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
)

func parserDiagnostics(p *parser.Parser) []diag.Diagnostic {
	out := make([]diag.Diagnostic, 0, len(p.Diagnostics))
	for _, diagnostic := range p.Diagnostics {
		out = append(out, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     diagnostic.Span,
			Message:  diagnostic.Message,
			Source:   "parser",
		})
	}
	return out
}
