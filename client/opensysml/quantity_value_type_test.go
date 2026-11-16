package opensysml_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const quantityValueTypeSource = `package M {
	private import SI::*;
	private import Quantities::*;

	attribute e : ISQ::EnergyValue = 5.0 [J];
	attribute bare = 5.0 [J];

	calc def AsTorque { in x : ISQ::TorqueValue[1]; return : ScalarQuantityValue[1] = x; }
	calc asTorque : AsTorque;
	calc def AsEnergy { in x : ISQ::EnergyValue[1]; return : ScalarQuantityValue[1] = x; }
	calc asEnergy : AsEnergy;
}`

// Over every transport a quantity names the quantity value type it is held as, and sent
// back the sibling type refuses it, where the same magnitude held as no type is taken.
func TestQuantityValueTypeCrossesEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, quantityValueTypeSource)
			read := func(expr, valueType string) opensysml.Quantity {
				t.Helper()
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				q, ok := got.(opensysml.Quantity)
				if !ok || q.ValueType != valueType {
					t.Fatalf("%s = %#v, want a quantity held as %q", expr, got, valueType)
				}
				return q
			}
			energy := read("M::e", "ISQThermodynamics::EnergyValue")
			bare := read("M::bare", "")

			if _, err := client.EvaluateCalc(ctx, model, "M::asTorque", energy); err == nil || !strings.Contains(err.Error(), "type mismatch") {
				t.Errorf("asTorque(M::e sent back): err = %v, want a type mismatch", err)
			}
			for calc, arg := range map[string]opensysml.Quantity{"M::asEnergy": energy, "M::asTorque": bare} {
				res, err := client.EvaluateCalc(ctx, model, calc, arg)
				if err != nil {
					t.Fatalf("EvaluateCalc(%s): %v", calc, err)
				}
				if !opensysml.Equal(res.Result, bare) {
					t.Errorf("%s(%#v) = %#v, want 5.0 J", calc, arg, res.Result)
				}
			}
		})
	}
}
