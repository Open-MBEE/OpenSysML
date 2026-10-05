package parser

import "github.com/Open-MBEE/OpenSysML/internal/syntax/diag"

// CodeSyntax is the diagnostic code every reporter gives a parse error.
const CodeSyntax = "syntax"

// AsDiagnostics presents a parse's errors and warnings as the diagnostics the
// analysis passes take and every frontend reports: an error is an error coded
// syntax, a warning keeps the code the parser gave it, and both name syntax
// as their source and carry the parser's fixes. The errors come first.
func AsDiagnostics(errors, warnings []Diagnostic) []diag.Diagnostic {
	out := make([]diag.Diagnostic, 0, len(errors)+len(warnings))
	for _, d := range errors {
		out = append(out, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     CodeSyntax,
			Source:   CodeSyntax,
			Fixes:    d.Fixes,
		})
	}
	for _, w := range warnings {
		out = append(out, diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Span:     w.Span,
			Message:  w.Message,
			Code:     w.Code,
			Source:   CodeSyntax,
			Fixes:    w.Fixes,
		})
	}
	return out
}
