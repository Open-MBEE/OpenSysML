package export_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// bigIntegerModel writes Integer literals beyond int64 and an expression whose
// value is one.
const bigIntegerModel = `package P {
	private import ScalarValues::*;
	attribute wide : Integer = 123456789012345678901234567890;
	attribute edge : Integer = 9223372036854775808;
	attribute power : Integer = 2 ** 70;
}
`

// xsd:integer is unbounded, so an Integer literal beyond int64 crosses RDF as
// its full decimal and comes back as the same Integer, with and without the
// source text.
func TestBigIntegerLiteralsRoundTripThroughRDF(t *testing.T) {
	turtle := roundTripsExactly(t, bigIntegerModel)
	for _, want := range []string{
		`"123456789012345678901234567890"^^xsd:integer`,
		`"9223372036854775808"^^xsd:integer`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("graph lacks %s:\n%s", want, turtle)
		}
	}
	back := toNotation(t, withoutSourceText(t, turtle))
	for _, want := range []string{
		"= 123456789012345678901234567890;",
		"= 9223372036854775808;",
		"= 2 ** 70;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone should spell %q\n--- notation ---\n%s", want, back)
		}
	}
	ctx, scope := runtimeModel(t, back)
	for expr, digits := range map[string]string{
		"wide":  "123456789012345678901234567890",
		"edge":  "9223372036854775808",
		"power": "1180591620717411303424",
	} {
		want, ok := new(big.Int).SetString(digits, 10)
		if !ok {
			t.Fatalf("bad digits %q", digits)
		}
		got := evalExportExpr(t, ctx, scope, expr)
		if got.Kind != runtime.ValConst || !got.Const.Equal(semantics.BigIntValue(want)) {
			t.Errorf("%s = %v after the round trip, want %s", expr, got, digits)
		}
	}
}
