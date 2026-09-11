package opensysml_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// TestArithmeticOutsideItsRangeIsAFailureNotAWrappedValue: a caller reading
// Int(-9223372036854775808) from a sum of positives has no way to tell it apart
// from a computed value, so the range is reported instead.
func TestArithmeticOutsideItsRangeIsAFailureNotAWrappedValue(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	for _, expression := range []string{
		"9223372036854775807 + 1",
		"-9223372036854775807 - 2",
		"9223372036854775807 * 2",
		"9223372036854775808",
		"1e400",
	} {
		value, err := client.Evaluate(context.Background(), model, expression)
		if !errors.Is(err, opensysml.ErrFailure) {
			t.Errorf("Evaluate(%q) = %#v, %v; want a failure", expression, value, err)
		}
	}
}

// TestRealsInRangeStillEvaluate: the range check reports only what no Real
// holds.
func TestRealsInRangeStillEvaluate(t *testing.T) {
	client := newClient(t)
	model := parseVehicle(t, client)

	value, err := client.Evaluate(context.Background(), model, "1.5e308 / 2.0")
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
