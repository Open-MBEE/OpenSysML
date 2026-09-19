package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// CompileCalc compiles the named calc def and every calc it invokes to the
// codegen IR. The error names the construct that kept a calc from compiling.
func (s *Session) CompileCalc(name string) (*codegen.Program, error) {
	defer s.enter()()
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolCalcDef, symbols.SymbolCalcUsage)
	if err != nil {
		return nil, err
	}
	idx := s.browseIndex()
	if idx == nil {
		return nil, fmt.Errorf("no declarations loaded")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	return codegen.New(model, resolver).Compile(sym)
}
