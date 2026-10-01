package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// CompileCalc compiles the named calc def and every calc it invokes with
// compile, the codegen compiler a caller links. The error names the construct
// that kept a calc from compiling.
func CompileCalc[P any](s *Session, name string, compile func(*semantics.Model, *resolve.Resolver, *symbols.Symbol) (P, error)) (P, error) {
	defer s.enter()()
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolCalcDef, symbols.SymbolCalcUsage)
	if err != nil {
		var zero P
		return zero, err
	}
	idx := s.browseIndex()
	if idx == nil {
		var zero P
		return zero, fmt.Errorf("no declarations loaded")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	return compile(model, resolver, sym)
}
