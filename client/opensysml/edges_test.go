package opensysml_test

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// TestIntegerArithmeticBeyondInt64IsExact: KerML Integers are unbounded, so
// a sum of positives past int64 is the BigInt it is, never a wrapped Int.
func TestIntegerArithmeticBeyondInt64IsExact(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	for expression, want := range map[string]string{
		"9223372036854775807 + 1":  "9223372036854775808",
		"-9223372036854775807 - 2": "-9223372036854775809",
		"9223372036854775807 * 2":  "18446744073709551614",
		"9223372036854775808":      "9223372036854775808",
		"2 ** 70":                  "1180591620717411303424",
	} {
		value, err := client.Evaluate(context.Background(), model, expression)
		got, ok := value.(opensysml.BigInt)
		if err != nil || !ok || got.String() != want {
			t.Errorf("Evaluate(%q) = %#v, %v; want BigInt %s", expression, value, err, want)
		}
	}
	value, err := client.Evaluate(context.Background(), model, "(2 ** 70) / (2 ** 69) == 2")
	if err != nil || value != opensysml.Bool(true) {
		t.Errorf("Evaluate((2 ** 70) / (2 ** 69) == 2) = %#v, %v; want true", value, err)
	}
}

// TestRationalArithmeticIsExact: a KerML Rational is a rational number, so a
// quotient or decimal no binary64 holds arrives as the exact Rational it is,
// and one a binary64 holds as that Real.
func TestRationalArithmeticIsExact(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	for expression, want := range map[string]string{
		"1 / 3":        "1/3",
		"0.1 + 0.2":    "0.3",
		"(2 / 3) ** 3": "8/27",
		"1.0e400 / 4":  "2.5e+399",
	} {
		value, err := client.Evaluate(context.Background(), model, expression)
		got, ok := value.(opensysml.Rational)
		if err != nil || !ok || got.String() != want {
			t.Errorf("Evaluate(%q) = %#v, %v; want Rational %s", expression, value, err, want)
		}
	}
	if value, err := client.Evaluate(context.Background(), model, "0.25 + 0.25"); err != nil || value != opensysml.Real(0.5) {
		t.Errorf("Evaluate(0.25 + 0.25) = %#v, %v; want Real 0.5", value, err)
	}
	if value, err := client.Evaluate(context.Background(), model, "0.1 + 0.2 == 0.3"); err != nil || value != opensysml.Bool(true) {
		t.Errorf("Evaluate(0.1 + 0.2 == 0.3) = %#v, %v; want true", value, err)
	}
	third := opensysml.NewRational(big.NewRat(1, 3))
	if third.Float64() != 1.0/3.0 || third.Rat().Cmp(big.NewRat(2, 6)) != 0 {
		t.Errorf("Rational 1/3 = %v (%v)", third, third.Float64())
	}
}

// TestRealOutsideItsRangeIsAFailure: a Real no float64 holds is reported, not
// read as an infinity. The literal 1e400 is an exact Rational; ToReal is Real.
func TestRealOutsideItsRangeIsAFailure(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	value, err := client.Evaluate(context.Background(), model, `RealFunctions::ToReal("1e400")`)
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Evaluate(ToReal(1e400)) = %#v, %v; want a failure", value, err)
	}
}

// TestRealsInRangeStillEvaluate: the range check reports only what no Real
// holds.
func TestRealsInRangeStillEvaluate(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	value, err := client.Evaluate(context.Background(), model, `RealFunctions::ToReal("1.5e308") / 2.0`)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	realVal, ok := value.(opensysml.Real)
	if !ok || math.IsInf(float64(realVal), 0) {
		t.Fatalf("value = %#v, want a finite Real", value)
	}
}

// TestEvaluateReadsTheWholeExpression: `1 = 1` is a declaration's notation, and
// answering the 1 the parser read would silently drop the rest.
func TestEvaluateReadsTheWholeExpression(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	for _, expression := range []string{"1 = 1", "1 + 1 rubbish"} {
		value, err := client.Evaluate(context.Background(), model, expression)
		if !errors.Is(err, opensysml.ErrFailure) {
			t.Errorf("Evaluate(%q) = %#v, %v; want a failure", expression, value, err)
		}
	}

	value, err := client.Evaluate(context.Background(), model, "  1 == 1  ")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if value != opensysml.Bool(true) {
		t.Errorf("1 == 1 = %#v, want true", value)
	}
}

const inputActionSource = `package Demo {
	action bump {
		in attribute step = 1;
		attribute result = 0;
		first start;
		action inner { assign result := result + step; }
		done;
		succession first start then inner;
		succession first inner then done;
	}
}`

// TestExecuteActionReportsAnInputTheActionDoesNotDeclare: a misspelled input
// would otherwise be accepted silently and the action run with its default.
func TestExecuteActionReportsAnInputTheActionDoesNotDeclare(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseSource(context.Background(), inputActionSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}

	run, err := client.ExecuteAction(context.Background(), model, "Demo::bump",
		map[string]opensysml.Value{"stepp": opensysml.Int(4)})
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Fatalf("ExecuteAction = %#v, %v; want a failure", run, err)
	}
	if !strings.Contains(err.Error(), "stepp") {
		t.Errorf("error = %v, want it to name the input", err)
	}

	run, err = client.ExecuteAction(context.Background(), model, "Demo::bump",
		map[string]opensysml.Value{"step": opensysml.Int(4)})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := run.Outputs["result"]; got != opensysml.Int(4) {
		t.Errorf("result = %#v, want 4", got)
	}
}

const outputActionSource = `package Demo {
	action measure {
		in attribute step = 1;
		out attribute total;
		first start;
		action inner { assign total := step + 1; }
		done;
		succession first start then inner;
		succession first inner then done;
	}
}`

// TestExecuteActionReportsSeedingAnOutputParameter: an `out` parameter is what
// the run answers with, so a caller writing it would read back its own value.
func TestExecuteActionReportsSeedingAnOutputParameter(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseSource(context.Background(), outputActionSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}

	run, err := client.ExecuteAction(context.Background(), model, "Demo::measure",
		map[string]opensysml.Value{"total": opensysml.Int(99)})
	if !errors.Is(err, opensysml.ErrFailure) {
		t.Fatalf("ExecuteAction = %#v, %v; want a failure", run, err)
	}
	if !strings.Contains(err.Error(), "total") {
		t.Errorf("error = %v, want it to name the parameter", err)
	}

	run, err = client.ExecuteAction(context.Background(), model, "Demo::measure",
		map[string]opensysml.Value{"step": opensysml.Int(4)})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := run.Outputs["total"]; got != opensysml.Int(5) {
		t.Errorf("total = %#v, want 5", got)
	}
}

const anonymousSatisfySource = `package Demo {
	part def Truck {
		attribute payload = 900.0;
	}
	requirement def PayloadLimit {
		subject truck : Truck;
		require constraint { truck.payload <= 1000.0 }
	}
	requirement payloadHolds : PayloadLimit;
	part loadedTruck : Truck;
	part {
		assert satisfy payloadHolds by loadedTruck;
	}
}`

// TestAnAnonymousAssertionCarriesNoElementID: an ElementID is a symbol the
// caller can look up, so an unnamed assertion reports none.
func TestAnAnonymousAssertionCarriesNoElementID(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseSource(context.Background(), anonymousSatisfySource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}

	satisfaction, err := client.VerifySatisfaction(context.Background(), model, "")
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(satisfaction.Verdicts) == 0 {
		t.Fatal("no verdicts, so the anonymous assertion was not evaluated")
	}
	for _, verdict := range satisfaction.Verdicts {
		if verdict.ElementID == "" {
			continue
		}
		if _, err := client.LookupSymbol(context.Background(), model, verdict.ElementID); err != nil {
			t.Errorf("ElementID %q is not a symbol: %v", verdict.ElementID, err)
		}
	}
}
