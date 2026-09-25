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

const measurementRefSource = `package M {
	private import ScalarValues::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	private import QuantityCalculations::*;

	attribute q : ISQ::LengthValue = 3 [km];
	attribute u : MeasurementUnit = m;
	attribute speed = m / s;
	attribute units : MeasurementUnit[*] = (m, s);

	calc def ToUnit { in x : ScalarQuantityValue; in target : MeasurementUnit; return : ScalarQuantityValue = ConvertQuantity(x, target); }
	calc toUnit : ToUnit;
	calc def UnitOf { in x : ScalarQuantityValue; return : MeasurementUnit = x.mRef; }
	calc unitOf : UnitOf;

	action convert {
		in x : ScalarQuantityValue;
		in target : MeasurementUnit;
		out y : ScalarQuantityValue;
		first start;
		action inner { assign y := ConvertQuantity(x, target); }
		then done;
		succession first start then inner;
	}
}`

// A bare measurement reference arrives as itself over every transport — unit
// as written, reduction, and the declaration it names — and one sent back is
// the unit ConvertQuantity converts to.
func TestMeasurementRefsCrossEveryTransport(t *testing.T) {
	address := startService(t)
	metre := opensysml.MeasurementRef{
		Unit:   "m",
		Term:   &opensysml.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []opensysml.UnitFactor{{UnitID: "SI::metre", Exponent: 1}}},
		UnitID: "SI::metre",
	}
	kilometre := opensysml.MeasurementRef{
		Unit:   "km",
		Term:   &opensysml.UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []opensysml.UnitFactor{{UnitID: "SI::metre", Exponent: 1}}},
		UnitID: "SI::kilometre",
	}
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
			if !info.Has(opensysml.CapabilityMeasurementRefs) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityMeasurementRefs)
			}
			model := parse(t, client, measurementRefSource)

			for expr, want := range map[string]opensysml.MeasurementRef{
				"M::u":      metre,
				"M::q.mRef": kilometre,
			} {
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %#v, want %#v", expr, got, want)
				}
			}

			speed, err := client.Evaluate(ctx, model, "M::speed")
			if err != nil {
				t.Fatalf("Evaluate(M::speed): %v", err)
			}
			ref, ok := speed.(opensysml.MeasurementRef)
			if !ok || ref.Unit != "m/s" || ref.UnitID != "" || ref.Term == nil || len(ref.Term.Factors) != 2 {
				t.Errorf("M::speed = %#v, want a composed m/s over two base units naming no declaration", speed)
			}

			units, err := client.Evaluate(ctx, model, "M::units")
			if err != nil {
				t.Fatalf("Evaluate(M::units): %v", err)
			}
			seq, ok := units.(opensysml.Sequence)
			if !ok || len(seq) != 2 || !reflect.DeepEqual(seq[0], metre) {
				t.Errorf("M::units = %#v, want a sequence opening with %#v", units, metre)
			}

			three := opensysml.Quantity{Magnitude: opensysml.Int(3), Unit: "km", Term: kilometre.Term}
			calc, err := client.EvaluateCalc(ctx, model, "M::toUnit", three, metre)
			if err != nil {
				t.Fatalf("EvaluateCalc(toUnit): %v", err)
			}
			converted, ok := calc.Result.(opensysml.Quantity)
			if !ok || converted.Magnitude != opensysml.Real(3000) || converted.Unit != "m" {
				t.Errorf("toUnit(3 km, m) = %#v, want 3000.0 m", calc.Result)
			}
			calc, err = client.EvaluateCalc(ctx, model, "M::unitOf", three)
			if err != nil {
				t.Fatalf("EvaluateCalc(unitOf): %v", err)
			}
			if !reflect.DeepEqual(calc.Result, kilometre) {
				t.Errorf("unitOf(3 km) = %#v, want %#v", calc.Result, kilometre)
			}

			run, err := client.ExecuteAction(ctx, model, "M::convert", map[string]opensysml.Value{"x": three, "target": metre})
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			converted, ok = run.Outputs["y"].(opensysml.Quantity)
			if !ok || converted.Magnitude != opensysml.Real(3000) || converted.Unit != "m" {
				t.Errorf("y = %#v, want 3000.0 m", run.Outputs["y"])
			}

			// A reference whose reduction disagrees with the unit it names is
			// refused by the service, in band, as a malformed quantity is.
			bad := opensysml.MeasurementRef{Unit: "m", Term: kilometre.Term, UnitID: "SI::metre"}
			if _, err = client.EvaluateCalc(ctx, model, "M::toUnit", three, bad); err == nil {
				t.Error("EvaluateCalc with a reference reducing to the wrong term succeeded")
			}
			if _, err = client.EvaluateCalc(ctx, model, "M::toUnit", three, opensysml.MeasurementRef{Unit: "m"}); err == nil {
				t.Error("EvaluateCalc with a reference lacking its reduction succeeded")
			}
		})
	}
}

// A service without measurement_refs would read a reference input as null, so
// the client refuses to send one, however deeply nested; and what such a
// service reports for a reference is an unsupported null naming it.
func TestMeasurementRefInputNeedsMeasurementRefs(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityMeasurementRefs})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	metre := opensysml.MeasurementRef{Unit: "m", Term: &opensysml.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []opensysml.UnitFactor{{UnitID: "SI::metre", Exponent: 1}}}}
	for name, client := range map[string]opensysml.Client{
		"connect-proto": dialClient(t, server.URL),
		"connect-json":  dialClient(t, server.URL, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model := parse(t, client, measurementRefSource)
			three := opensysml.Quantity{Magnitude: opensysml.Int(3), Unit: "km", Term: &opensysml.UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: metre.Term.Factors}}
			for label, input := range map[string]opensysml.Value{
				"reference":   metre,
				"nested":      opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{metre}},
				"in an array": opensysml.Array{Dimensions: []int64{1}, Elements: []opensysml.Value{metre}},
			} {
				_, err := client.ExecuteAction(ctx, model, "M::convert", map[string]opensysml.Value{"x": three, "target": input})
				wantMeasurementRefsRefusal(t, "ExecuteAction "+label, err)
				_, err = client.EvaluateCalc(ctx, model, "M::toUnit", three, input)
				wantMeasurementRefsRefusal(t, "EvaluateCalc "+label, err)
			}

			// Quantities stay theirs: only the bare reference needs the capability.
			calc, err := client.EvaluateCalc(ctx, model, "M::unitOf", three)
			if err != nil {
				t.Fatalf("EvaluateCalc(unitOf): %v", err)
			}
			if calc.Result != opensysml.Null("unsupported: measurement reference km") {
				t.Errorf("unitOf(3 km) without measurement_refs = %#v", calc.Result)
			}
			for expr, want := range map[string]string{
				"M::u":     "unsupported: measurement reference m",
				"M::speed": "unsupported: measurement reference m/s",
			} {
				got, err := client.Evaluate(ctx, model, expr)
				if err != nil {
					t.Fatalf("Evaluate(%s): %v", expr, err)
				}
				if got != opensysml.Null(want) {
					t.Errorf("%s without measurement_refs = %#v, want Null(%q)", expr, got, want)
				}
			}
		})
	}
}

func wantMeasurementRefsRefusal(t *testing.T, op string, err error) {
	t.Helper()
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeUnimplemented || !strings.Contains(status.Message, opensysml.CapabilityMeasurementRefs) {
		t.Errorf("%s: err = %v, want CodeUnimplemented naming %s", op, err, opensysml.CapabilityMeasurementRefs)
	}
}

func TestMeasurementRefRendersAsSysMLWrites(t *testing.T) {
	for _, testcase := range []struct {
		value opensysml.Value
		want  string
	}{
		{opensysml.MeasurementRef{Unit: "km", UnitID: "SI::kilometre"}, "km"},
		{opensysml.MeasurementRef{Unit: "m/s"}, "m/s"},
		{opensysml.Sequence{opensysml.MeasurementRef{Unit: "m"}, opensysml.MeasurementRef{Unit: "s"}}, "[m s]"},
		{opensysml.MeasurementRef{Term: &opensysml.UnitTerm{ScaleNum: 1, ScaleDen: 1}}, "1"},
		{opensysml.MeasurementRef{Term: &opensysml.UnitTerm{ScaleNum: 1000, ScaleDen: 3600, Factors: []opensysml.UnitFactor{
			{UnitID: "SI::metre", Exponent: 1}, {UnitID: "SI::second", Exponent: -1},
		}}}, "1000/3600·SI::metre·SI::second^-1"},
		{opensysml.MeasurementRef{Unit: "km/h", Term: &opensysml.UnitTerm{ScaleNum: 1000, ScaleDen: 3600}}, "km/h"},
	} {
		if got := fmt.Sprintf("%v", testcase.value); got != testcase.want {
			t.Errorf("%#v renders as %q, want %q", testcase.value, got, testcase.want)
		}
	}
}
