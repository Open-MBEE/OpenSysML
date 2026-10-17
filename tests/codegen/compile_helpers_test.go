package codegen_test

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"

	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all"
)

// compileCalc compiles the calc named in s to the codegen IR for target, as
// `sysml -compile` does.
func compileCalc(s *repl.Session, name string, target codegen.Target) (*codegen.Program, error) {
	return repl.CompileCalc(s, name, func(model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*codegen.Program, error) {
		return codegen.New(model, resolver).Compile(entry, target)
	})
}

// errorDiagnostics keeps the diagnostics of error severity.
func errorDiagnostics(diags []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			out = append(out, d)
		}
	}
	return out
}
