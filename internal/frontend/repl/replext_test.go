package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"

	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all"
)

// CompileCalc compiles to the codegen IR for target, as `sysml -compile` does.
func (s *Session) CompileCalc(name string, target codegen.Target) (*codegen.Program, error) {
	return CompileCalc(s, name, func(model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*codegen.Program, error) {
		return codegen.New(model, resolver).Compile(entry, target)
	})
}
