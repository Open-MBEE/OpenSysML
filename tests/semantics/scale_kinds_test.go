package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const scaleKindsModel = `private import ScalarValues::*;
private import SI::*;
private import ISQ::*;
private import MeasurementReferences::*;

package S {
	attribute mohs : OrdinalScale { :>> unit = one; }
	attribute bearing : CyclicRatioScale { :>> unit = '°'; :>> modulus = 360; }
	attribute dBm : LogarithmicScale { :>> unit = one; :>> logarithmBase = 10; :>> factor = 10; :>> exponent = 1; :>> referenceQuantity = 0.001 [W]; }
	attribute octave : LogarithmicScale { :>> unit = one; :>> logarithmBase = 2; :>> factor = 1; :>> exponent = 1; }
	attribute selfish : LogarithmicScale { :>> unit = one; :>> logarithmBase = 2; :>> factor = 1; :>> exponent = 1; :>> referenceQuantity = 1 [selfish]; }
	attribute ping : LogarithmicScale { :>> unit = one; :>> logarithmBase = 2; :>> factor = 1; :>> exponent = 1; :>> referenceQuantity = 1 [pong]; }
	attribute pong : LogarithmicScale { :>> unit = one; :>> logarithmBase = 2; :>> factor = 1; :>> exponent = 1; :>> referenceQuantity = 1 [ping]; }
}`

func scaleKindsFixture(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("q.sysml", []byte(scaleKindsModel)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument("q.sysml", root)
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// TestScaleKindDimensions: a cyclic value is a magnitude, an ordinal one a point, and a
// logarithmic one a point of its referenceQuantity's dimension, unknown when it binds none.
func TestScaleKindDimensions(t *testing.T) {
	m, idx := scaleKindsFixture(t)
	scope := idx.DocumentRoot("q.sysml")
	mohs, dBm := lookupOne(t, idx, "S::mohs"), lookupOne(t, idx, "S::dBm")
	for _, tc := range []struct {
		expr  string
		dim   string
		scale *symbols.Symbol
	}{
		{"450 [S::bearing]", "1", nil},
		{"-(450 [S::bearing])", "1", nil},
		{"2 * 450 [S::bearing]", "1", nil},
		{"450 [S::bearing] / 9 [s]", "T^-1", nil},
		{"450 [S::bearing] - 90 [S::bearing]", "1", nil},
		{"7 [S::mohs]", "1", mohs},
		{"30.0 [S::dBm]", "L^2·M·T^-3", dBm},
	} {
		dim, ok := m.DimensionOfExpr(scope, parseExpr(t, tc.expr))
		if !ok || dim.String() != tc.dim || dim.Scale != tc.scale {
			t.Errorf("DimensionOfExpr(%s) = %s on %v (known %v), want %s on %v", tc.expr, dim, dim.Scale, ok, tc.dim, tc.scale)
		}
	}
	// A referenceQuantity on its own scale, directly or through another, measures nothing.
	for _, expr := range []string{"3.0 [S::octave]", "2 * 7 [S::mohs]", "2 * 30.0 [S::dBm]", "1 [S::selfish]", "1 [S::ping] > 2 [S::pong]"} {
		if dim, ok := m.DimensionOfExpr(scope, parseExpr(t, expr)); ok {
			t.Errorf("DimensionOfExpr(%s) = %s, want unknown", expr, dim)
		}
	}
	if _, ok := m.EvalQuantity(scope, parseExpr(t, "450 [S::bearing] + 30 ['°']")); ok {
		t.Error("a sum on a cyclic ratio scale folded; its mapping and modulus are the runtime's to read")
	}
}
