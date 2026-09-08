package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

const functionSource = `package F {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Cube { in v : Real; return : Real = v * v * v; }
	calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc apply : Apply;
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	attribute pick = Identity(Sq);
	attribute nine = Apply(Sq, 3.0);
	part def Scaler {
		attribute k : Real = 2.0;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part holder : Scaler;
	attribute scaler = holder.scale;

	action applyTwice {
		in calc f { in v : Real; return : Real; }
		in a : Real;
		out y : Real;
		first start;
		action inner { assign y := f(f(a)); }
		then done;
		succession first start then inner;
	}
}`

// A calc read as a value arrives as a Function naming it over every transport,
// one read off an object naming that object too, and a Function sent back binds
// the calc-typed parameter the calc invokes.
func TestFunctionsCrossEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			info, err := client.ServerInfo(ctx)
			if err != nil {
				t.Fatalf("ServerInfo: %v", err)
			}
			if !info.Has(opensysml.CapabilityFunctionValues) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityFunctionValues)
			}
			model := parse(t, client, functionSource)

			pick, err := client.Evaluate(ctx, model, "F::pick")
			if err != nil {
				t.Fatalf("Evaluate(F::pick): %v", err)
			}
			if pick != (opensysml.Function{CalcID: "F::Sq"}) {
				t.Errorf("F::pick = %#v, want the function F::Sq bound to no object", pick)
			}

			scaler, err := client.Evaluate(ctx, model, "F::scaler")
			if err != nil {
				t.Fatalf("Evaluate(F::scaler): %v", err)
			}
			fn, ok := scaler.(opensysml.Function)
			if !ok || fn.CalcID != "F::Scaler::scale" || fn.Self == 0 {
				t.Errorf("F::scaler = %#v, want the function F::Scaler::scale bound to holder", scaler)
			}

			nine, err := client.Evaluate(ctx, model, "F::nine")
			if err != nil {
				t.Fatalf("Evaluate(F::nine): %v", err)
			}
			if nine != opensysml.Real(9) {
				t.Errorf("F::nine = %#v, want 9.0", nine)
			}

			calc, err := client.EvaluateCalc(ctx, model, "F::apply", opensysml.Function{CalcID: "F::Cube"}, opensysml.Real(2))
			if err != nil {
				t.Fatalf("EvaluateCalc(apply): %v", err)
			}
			if calc.Result != opensysml.Real(8) {
				t.Errorf("apply(Cube, 2.0) = %#v, want 8.0", calc.Result)
			}

			run, err := client.ExecuteAction(ctx, model, "F::applyTwice", map[string]opensysml.Value{
				"f": opensysml.Function{CalcID: "F::Sq"}, "a": opensysml.Real(3),
			})
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if run.Outputs["y"] != opensysml.Real(81) {
				t.Errorf("y = %#v, want 81.0", run.Outputs["y"])
			}

			// A function naming no calc, or an object this call did not
			// create, is refused in band rather than invoked.
			for label, bad := range map[string]opensysml.Function{
				"an unknown calc":   {CalcID: "F::Nothing"},
				"a non-calc":        {CalcID: "F::holder"},
				"an unknown object": {CalcID: "F::Scaler::scale", Self: 99},
			} {
				if _, err := client.EvaluateCalc(ctx, model, "F::apply", bad, opensysml.Real(2)); err == nil {
					t.Errorf("EvaluateCalc with a function naming %s succeeded", label)
				}
			}
			var status *opensysml.StatusError
			_, err = client.EvaluateCalc(ctx, model, "F::apply", opensysml.Function{}, opensysml.Real(2))
			if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
				t.Errorf("EvaluateCalc with an empty function: err = %v, want CodeInvalidArgument", err)
			}
		})
	}
}

// A service without function_values would read a function input as null, so
// the client refuses to send one, however deeply nested; and what such a
// service reports for a calc read as a value is an unsupported null naming it.
func TestFunctionInputNeedsFunctionValues(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityFunctionValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	sq := opensysml.Function{CalcID: "F::Sq"}
	for name, client := range map[string]opensysml.Client{
		"connect-proto": dialClient(t, server.URL),
		"connect-json":  dialClient(t, server.URL, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, functionSource)
			for label, input := range map[string]opensysml.Value{
				"function":    sq,
				"nested":      opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{sq}},
				"in an array": opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{sq}},
			} {
				_, err := client.ExecuteAction(ctx, model, "F::applyTwice", map[string]opensysml.Value{"f": input, "a": opensysml.Real(3)})
				wantFunctionValuesRefusal(t, "ExecuteAction "+label, err)
				_, err = client.EvaluateCalc(ctx, model, "F::apply", input, opensysml.Real(2))
				wantFunctionValuesRefusal(t, "EvaluateCalc "+label, err)
			}

			// Invocation inside the model needs no capability: only the
			// value crossing the boundary does.
			nine, err := client.Evaluate(ctx, model, "F::nine")
			if err != nil {
				t.Fatalf("Evaluate(F::nine): %v", err)
			}
			if nine != opensysml.Real(9) {
				t.Errorf("F::nine without function_values = %#v, want 9.0", nine)
			}
			pick, err := client.Evaluate(ctx, model, "F::pick")
			if err != nil {
				t.Fatalf("Evaluate(F::pick): %v", err)
			}
			if pick != opensysml.Null("unsupported: function F::Sq") {
				t.Errorf("F::pick without function_values = %#v, want an unsupported null naming F::Sq", pick)
			}
		})
	}
}

func wantFunctionValuesRefusal(t *testing.T, op string, err error) {
	t.Helper()
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented || !strings.Contains(status.Message, opensysml.CapabilityFunctionValues) {
		t.Errorf("%s: err = %v, want CodeUnimplemented naming %s", op, err, opensysml.CapabilityFunctionValues)
	}
}

func TestFunctionRendersAsItsCalc(t *testing.T) {
	for _, testcase := range []struct {
		value opensysml.Value
		want  string
	}{
		{opensysml.Function{CalcID: "F::Sq"}, "F::Sq"},
		{opensysml.Function{CalcID: "F::Scaler::scale", Self: 1}, "F::Scaler::scale"},
		{opensysml.Sequence{opensysml.Function{CalcID: "F::Sq"}, opensysml.Function{CalcID: "F::Cube"}}, "[F::Sq F::Cube]"},
	} {
		if got := fmt.Sprintf("%v", testcase.value); got != testcase.want {
			t.Errorf("%#v renders as %q, want %q", testcase.value, got, testcase.want)
		}
	}
}
