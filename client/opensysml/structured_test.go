package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

const structuredSource = `package S {
	private import ScalarValues::*;
	private import Collections::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;

	attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
	attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
	attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];

	calc def Length { in x : CartesianVectorValue; return : Real = norm(x); }
	calc length : Length;
	calc def Doubled { in x : CartesianVectorValue; return : CartesianVectorValue = cartesianVectorScalarMult(x, 2.0); }
	calc doubled : Doubled;

	action scale {
		in x : CartesianVectorValue;
		out y : CartesianVectorValue;
		first start;
		action inner { assign y := cartesianVectorScalarMult(x, 2.0); }
		then done;
		succession first start then inner;
	}
}`

// An Array, a Vector and a VectorQuantity arrive as themselves over every
// transport — shape, numeric kinds, units and reductions intact — and a Vector
// sent back is the vector the calc or action computes with.
func TestStructuredValuesCrossEveryTransport(t *testing.T) {
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
			if !info.Has(opensysml.CapabilityStructuredValues) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityStructuredValues)
			}
			model := parse(t, client, structuredSource)

			grid, err := client.Evaluate(ctx, model, "S::grid")
			if err != nil {
				t.Fatalf("Evaluate(S::grid): %v", err)
			}
			wantGrid := opensysml.Array{
				Dimensions: []int64{2, 3},
				Elements: []opensysml.Value{
					opensysml.Int(1), opensysml.Int(2), opensysml.Int(3),
					opensysml.Int(4), opensysml.Int(5), opensysml.Int(6),
				},
			}
			if !reflect.DeepEqual(grid, wantGrid) {
				t.Errorf("S::grid = %#v, want %#v", grid, wantGrid)
			}

			v, err := client.Evaluate(ctx, model, "S::v")
			if err != nil {
				t.Fatalf("Evaluate(S::v): %v", err)
			}
			if want := (opensysml.Vector{opensysml.Real(3), opensysml.Real(4)}); !reflect.DeepEqual(v, want) {
				t.Errorf("S::v = %#v, want %#v", v, want)
			}

			d, err := client.Evaluate(ctx, model, "S::d")
			if err != nil {
				t.Fatalf("Evaluate(S::d): %v", err)
			}
			vq, ok := d.(opensysml.VectorQuantity)
			if !ok || len(vq) != 2 {
				t.Fatalf("S::d = %#v, want a VectorQuantity of two", d)
			}
			for i, want := range []opensysml.Real{3, 4} {
				q := vq[i]
				if q.Magnitude != want || q.Unit != "m" || q.Term == nil || len(q.Term.Factors) != 1 || q.Term.Factors[0].UnitID != "SI::metre" {
					t.Errorf("S::d[%d] = %#v, want %v m reducing to SI::metre", i, q, want)
				}
			}

			sent := opensysml.Vector{opensysml.Real(3), opensysml.Real(4)}
			calc, err := client.EvaluateCalc(ctx, model, "S::length", sent)
			if err != nil {
				t.Fatalf("EvaluateCalc(length): %v", err)
			}
			if calc.Result != opensysml.Real(5) {
				t.Errorf("length(⟨3.0, 4.0⟩) = %#v, want Real(5)", calc.Result)
			}
			calc, err = client.EvaluateCalc(ctx, model, "S::doubled", sent)
			if err != nil {
				t.Fatalf("EvaluateCalc(doubled): %v", err)
			}
			if want := (opensysml.Vector{opensysml.Real(6), opensysml.Real(8)}); !reflect.DeepEqual(calc.Result, want) {
				t.Errorf("doubled(⟨3.0, 4.0⟩) = %#v, want %#v", calc.Result, want)
			}

			run, err := client.ExecuteAction(ctx, model, "S::scale", map[string]opensysml.Value{"x": sent})
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if want := (opensysml.Vector{opensysml.Real(6), opensysml.Real(8)}); !reflect.DeepEqual(run.Outputs["y"], want) {
				t.Errorf("y = %#v, want %#v", run.Outputs["y"], want)
			}

			// A malformed vector is refused by the service, in band, as a
			// malformed quantity is.
			bad := opensysml.Vector{opensysml.Real(3), opensysml.Int(4)}
			_, err = client.EvaluateCalc(ctx, model, "S::length", opensysml.Sequence{bad, opensysml.String("x")})
			if err == nil {
				t.Error("EvaluateCalc with a sequence for a vector parameter succeeded")
			}
		})
	}
}

// A service without structured_values would read a structured input as null,
// so the client refuses to send one, however deeply nested, over either body
// encoding; and what such a service reports for a structured value is an
// unsupported null naming it.
func TestStructuredInputNeedsStructuredValues(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityStructuredValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	for name, client := range map[string]opensysml.Client{
		"connect-proto": dialClient(t, server.URL),
		"connect-json":  dialClient(t, server.URL, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, structuredSource)
			vector := opensysml.Vector{opensysml.Real(3), opensysml.Real(4)}
			for label, input := range map[string]opensysml.Value{
				"vector":          vector,
				"array":           opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{opensysml.Int(1)}},
				"vector quantity": opensysml.VectorQuantity{{Magnitude: opensysml.Real(1), Unit: "m"}},
				"nested":          opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{vector}},
				"in an array":     opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{vector}},
			} {
				_, err := client.ExecuteAction(ctx, model, "S::scale", map[string]opensysml.Value{"x": input})
				wantStructuredValuesRefusal(t, "ExecuteAction "+label, err)
				_, err = client.EvaluateCalc(ctx, model, "S::length", input)
				wantStructuredValuesRefusal(t, "EvaluateCalc "+label, err)
			}

			for expr, want := range map[string]string{
				"S::grid": "unsupported: array Array(2, 3)[1, 2, 3, 4, 5, 6]",
				"S::v":    "unsupported: vector ⟨3.0, 4.0⟩",
				"S::d":    "unsupported: vector quantity ⟨3.0, 4.0⟩ [m]",
			} {
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				if got != opensysml.Null(want) {
					t.Errorf("%s without structured_values = %#v, want Null(%q)", expr, got, want)
				}
			}
		})
	}
}

func wantStructuredValuesRefusal(t *testing.T, op string, err error) {
	t.Helper()
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented || !strings.Contains(status.Message, opensysml.CapabilityStructuredValues) {
		t.Errorf("%s: err = %v, want CodeUnimplemented naming %s", op, err, opensysml.CapabilityStructuredValues)
	}
}

func TestStructuredValuesRenderAsSysMLWrites(t *testing.T) {
	metre := func(m opensysml.Number) opensysml.Quantity { return opensysml.Quantity{Magnitude: m, Unit: "m"} }
	for _, testcase := range []struct {
		value opensysml.Value
		want  string
	}{
		{opensysml.Array{Dimensions: []int64{2, 2}, Elements: []opensysml.Value{opensysml.Int(1), opensysml.Int(2), opensysml.Int(3), opensysml.Int(4)}}, "Array(2, 2)[1, 2, 3, 4]"},
		{opensysml.Array{Elements: []opensysml.Value{opensysml.Real(7)}}, "Array()[7]"},
		{opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{metre(opensysml.Int(3))}}, "Array(1)[3 m]"},
		{opensysml.Vector{opensysml.Real(3), opensysml.Real(4)}, "⟨3, 4⟩"},
		{opensysml.Vector{opensysml.Int(1), opensysml.Int(2)}, "⟨1, 2⟩"},
		{opensysml.VectorQuantity{metre(opensysml.Real(3)), metre(opensysml.Real(4))}, "⟨3, 4⟩ m"},
		{opensysml.VectorQuantity{metre(opensysml.Real(1)), {Magnitude: opensysml.Real(2), Unit: "kg"}}, "⟨1 m, 2 kg⟩"},
	} {
		if got := fmt.Sprintf("%v", testcase.value); got != testcase.want {
			t.Errorf("%#v renders as %q, want %q", testcase.value, got, testcase.want)
		}
	}
}
