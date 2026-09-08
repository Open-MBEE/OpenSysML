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
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

const setTensorSource = `package W {
	private import ScalarValues::*;
	private import Collections::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	private import TensorCalculations::*;

	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
	attribute e : Set { :>> elements = (); }
	attribute mixed : Set { :>> elements = ("b", 2, true, "a"); }

	attribute cubeRef : TensorMeasurementReference {
		:>> dimensions = (2, 2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);

	calc def SizeOf { in c : Integer[0..*]; return : Natural = SequenceFunctions::size(c); }
	calc sizeOf : SizeOf;
	calc def Corner { in t : TensorQuantityValue; return : ScalarQuantityValue = t#(2, 2, 2); }
	calc corner : Corner;
}`

// A set arrives as a Set holding each element once in canonical order, a
// tensor as a TensorQuantity of its rank, over every transport; each sent back
// is read as itself.
func TestSetsAndTensorsCrossEveryTransport(t *testing.T) {
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
			for _, capability := range []string{opensysml.CapabilitySetValues, opensysml.CapabilityTensorValues} {
				if !info.Has(capability) {
					t.Errorf("capabilities %v do not name %s", info.Capabilities, capability)
				}
			}
			model := parse(t, client, setTensorSource)

			for expr, want := range map[string]opensysml.Value{
				"W::s.elements":     opensysml.Set{opensysml.Int(1), opensysml.Int(2), opensysml.Int(3)},
				"W::e.elements":     opensysml.Set{},
				"W::mixed.elements": opensysml.Set{opensysml.Bool(true), opensysml.Int(2), opensysml.String("a"), opensysml.String("b")},
			} {
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %#v, want %#v", expr, got, want)
				}
			}

			cube, err := client.Evaluate(ctx, model, "W::cube")
			if err != nil {
				t.Fatalf("Evaluate(W::cube): %v", err)
			}
			tq, ok := cube.(opensysml.TensorQuantity)
			if !ok {
				t.Fatalf("W::cube = %#v, want a TensorQuantity", cube)
			}
			if !reflect.DeepEqual(tq.Dimensions, []int64{2, 2, 2}) || len(tq.Components) != 8 {
				t.Fatalf("W::cube = %v, want dimensions (2, 2, 2) and 8 components", tq)
			}
			for i, q := range tq.Components {
				if q.Magnitude != opensysml.Real(float64(i+1)) || q.Unit != "Pa" || q.Term == nil {
					t.Errorf("component %d = %#v, want %d.0 Pa with its reduction", i+1, q, i+1)
				}
			}
			if got := fmt.Sprint(tq); got != "Tensor(2, 2, 2)[1, 2, 3, 4, 5, 6, 7, 8] Pa" {
				t.Errorf("String() = %q", got)
			}

			// The tensor sent back is indexed by the model as itself.
			calc, err := client.EvaluateCalc(ctx, model, "W::corner", tq)
			if err != nil {
				t.Fatalf("EvaluateCalc(corner): %v", err)
			}
			if q, ok := calc.Result.(opensysml.Quantity); !ok || q.Magnitude != opensysml.Real(8) || q.Unit != "Pa" {
				t.Errorf("corner(cube) = %#v, want 8.0 Pa", calc.Result)
			}

			// A set sent in any order is read as its elements.
			calc, err = client.EvaluateCalc(ctx, model, "W::sizeOf", opensysml.Set{opensysml.Int(3), opensysml.Int(1), opensysml.Int(2)})
			if err != nil {
				t.Fatalf("EvaluateCalc(sizeOf): %v", err)
			}
			if calc.Result != opensysml.Int(3) {
				t.Errorf("sizeOf({3, 1, 2}) = %#v, want 3", calc.Result)
			}

			// A set listing an element twice is refused before it is sent, not sent as a set of one.
			_, err = client.EvaluateCalc(ctx, model, "W::sizeOf", opensysml.Set{opensysml.Int(1), opensysml.Real(1)})
			var status *opensysml.StatusError
			if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument || !strings.Contains(status.Message, "set lists a member twice") {
				t.Errorf("sizeOf({1, 1.0}) err = %v, want an invalid-argument status naming the repeated element", err)
			}
			// A tensor its components do not fill is refused before it is sent.
			short := opensysml.TensorQuantity{Dimensions: []int64{2, 2, 2}, Components: tq.Components[:7]}
			_, err = client.EvaluateCalc(ctx, model, "W::corner", short)
			if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument || !strings.Contains(status.Message, "tensor components do not fill its dimensions") {
				t.Errorf("corner(short) err = %v, want an invalid-argument status naming the shape", err)
			}
		})
	}
}

// A service without set_values or tensor_values would read the input as null,
// so the client refuses to send one, however deeply nested; and what such a
// service reports for the value is an unsupported null naming it.
func TestSetAndTensorInputsNeedTheirCapabilities(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilitySetValues, opensysml.CapabilityTensorValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	pascal := opensysml.Quantity{Magnitude: opensysml.Real(1), Unit: "Pa", Term: &opensysml.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []opensysml.UnitFactor{
		{UnitID: "SI::kilogram", Exponent: 1}, {UnitID: "SI::metre", Exponent: -1}, {UnitID: "SI::second", Exponent: -2},
	}}}
	line := opensysml.TensorQuantity{Dimensions: []int64{1}, Components: []opensysml.Quantity{pascal}}
	set := opensysml.Set{opensysml.Int(1)}
	for name, client := range map[string]opensysml.Client{
		"connect-proto": dialClient(t, server.URL),
		"connect-json":  dialClient(t, server.URL, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, setTensorSource)
			for label, input := range map[string]opensysml.Value{
				"set":         set,
				"nested":      opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{set}},
				"in an array": opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{set}},
			} {
				_, err := client.EvaluateCalc(ctx, model, "W::sizeOf", input)
				wantCapabilityRefusal(t, "EvaluateCalc "+label, err, opensysml.CapabilitySetValues)
			}
			for label, input := range map[string]opensysml.Value{
				"tensor":   line,
				"nested":   opensysml.Sequence{line},
				"in a set": opensysml.Set{line},
			} {
				_, err := client.EvaluateCalc(ctx, model, "W::corner", input)
				var status *opensysml.StatusError
				if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented {
					t.Errorf("EvaluateCalc %s: err = %v, want CodeUnimplemented", label, err)
				}
			}

			for expr, want := range map[string]string{
				"W::s.elements": "unsupported: set Set{1, 2, 3}",
				"W::cube":       "unsupported: tensor quantity Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]",
			} {
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				if got != opensysml.Null(want) {
					t.Errorf("%s without the capability = %#v, want Null(%q)", expr, got, want)
				}
			}
		})
	}
}

func wantCapabilityRefusal(t *testing.T, op string, err error, capability string) {
	t.Helper()
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented || !strings.Contains(status.Message, capability) {
		t.Errorf("%s: err = %v, want CodeUnimplemented naming %s", op, err, capability)
	}
}

func TestSetAndTensorRenderAsSysMLWrites(t *testing.T) {
	metre := opensysml.Quantity{Magnitude: opensysml.Int(3), Unit: "m"}
	second := opensysml.Quantity{Magnitude: opensysml.Real(2), Unit: "s"}
	for _, testcase := range []struct {
		value opensysml.Value
		want  string
	}{
		{opensysml.Set{}, "{}"},
		{opensysml.Set{opensysml.Int(1), opensysml.String("a")}, "{1, a}"},
		{opensysml.Set{opensysml.Set{opensysml.Int(1)}, opensysml.Set{}}, "{{1}, {}}"},
		{opensysml.TensorQuantity{Dimensions: []int64{2}, Components: []opensysml.Quantity{metre, metre}}, "Tensor(2)[3, 3] m"},
		{opensysml.TensorQuantity{Dimensions: []int64{1, 2}, Components: []opensysml.Quantity{metre, second}}, "Tensor(1, 2)[3 m, 2 s]"},
		{opensysml.TensorQuantity{Dimensions: nil, Components: []opensysml.Quantity{metre}}, "Tensor()[3] m"},
	} {
		if got := fmt.Sprint(testcase.value); got != testcase.want {
			t.Errorf("%#v renders as %q, want %q", testcase.value, got, testcase.want)
		}
	}
}
