package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestISQMassValueVisibility(t *testing.T) {
	idx := symbols.NewIndex()
	src := DefaultSource()
	cache, _ := NewCache()
	loader := NewLoader(src, cache)

	// Load ISQBase
	loader.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx)
	loader.load("Domain Libraries/Quantities and Units/Quantities.sysml", idx)
	loader.load("Domain Libraries/Quantities and Units/MeasurementReferences.sysml", idx)
	loader.load("Domain Libraries/Quantities and Units/ISQBase.sysml", idx)

	// Check ISQBase::MassValue visibility
	syms := idx.LookupQualified("ISQBase::MassValue")
	if len(syms) == 0 {
		t.Fatal("ISQBase::MassValue not found")
	}

	sym := syms[0]
	t.Logf("ISQBase::MassValue visibility: %v", sym.Visibility)
	t.Logf("  VisibilityDefault=%v, VisibilityPublic=%v, VisibilityPrivate=%v",
		ast.VisibilityDefault, ast.VisibilityPublic, ast.VisibilityPrivate)

	if sym.Visibility == ast.VisibilityPrivate {
		t.Error("MassValue is marked private!")
	}
}
