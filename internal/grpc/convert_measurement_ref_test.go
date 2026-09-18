package grpc

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
)

// measurementRefWireModel yields a measurement reference as a feature value —
// named, composed and read off a quantity — and takes one back as a calc
// argument and an action input that convert by it.
const measurementRefWireModel = `
package M {
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
  calc def Measured { in n : Real; in target : MeasurementUnit; return : ScalarQuantityValue = '['(n, target); }
  calc measured : Measured;

  action convert {
    in x : ScalarQuantityValue;
    in target : MeasurementUnit;
    out y : ScalarQuantityValue;
    first start;
    action inner { assign y := ConvertQuantity(x, target); }
    then done;
    succession first start then inner;
  }
}
`

// mustMeasurementRefModel parses measurementRefWireModel into srv.
func mustMeasurementRefModel(t *testing.T, srv *Service) string {
	t.Helper()
	return mustParse(t, srv, measurementRefWireModel)
}

func measurementRefValue(unit, unitID string, term *pb.UnitTerm) *pb.Value {
	return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: &pb.MeasurementRef{Unit: unit, UnitTerm: term, UnitId: unitID}}}
}

func metreTerm() *pb.UnitTerm {
	return &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: 1}}}
}

func kilometreTerm() *pb.UnitTerm {
	return &pb.UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: 1}}}
}

// A measurement reference crosses as the unit written, its reduction and, for a
// unit the model declares, the declaration; it reads back as the same reference,
// identity included, whether the declaration is sent or only the text.
func TestMeasurementRefRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustMeasurementRefModel(t, srv)
	cached, ok := srv.cache.Get(modelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		expr, unit, unitID, term string
	}{
		{"SI::m", "m", "SI::metre", "SI::metre"},
		{"SI::km", "km", "SI::kilometre", "1000/1·SI::metre"},
		{"M::q.mRef", "km", "SI::kilometre", "1000/1·SI::metre"},
		{"M::u", "m", "SI::metre", "SI::metre"},
		{"M::speed", "m/s", "", "SI::metre·SI::second^-1"},
		{"SI::m / SI::m", "", "", "1"},
		{"MeasurementReferences::one", "one", "MeasurementReferences::one", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			pv := mustEvaluate(t, srv, modelHash, tc.expr)
			pm := pv.GetMeasurementRef()
			if pm == nil {
				t.Fatalf("%s crossed as %T: %v", tc.expr, pv.GetKind(), pv)
			}
			if pm.GetUnit() != tc.unit || pm.GetUnitId() != tc.unitID || describeUnitTerm(pm.GetUnitTerm()) != tc.term {
				t.Fatalf("%s = %v, want unit %q, unit_id %q reducing to %s", tc.expr, pm, tc.unit, tc.unitID, tc.term)
			}

			// A client echoing what the service sent gets the same reference back.
			back, err := protoconv.ProtoToValueIn(pv, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			assertSameRef(t, back, tc.unit, tc.unitID, tc.term)

			// A client naming only the unit text is read as a Quantity's unit is.
			if tc.unit == "" {
				return
			}
			textOnly := measurementRefValue(tc.unit, "", pm.GetUnitTerm())
			back, err = protoconv.ProtoToValueIn(textOnly, idx, sem)
			if err != nil {
				t.Fatalf("protoconv.ProtoToValueIn(text only): %v", err)
			}
			assertSameRef(t, back, tc.unit, tc.unitID, tc.term)

			// A client naming only the declaration is read as spelt by its symbol.
			if tc.unitID == "" {
				return
			}
			idOnly := measurementRefValue("", tc.unitID, pm.GetUnitTerm())
			back, err = protoconv.ProtoToValueIn(idOnly, idx, sem)
			if err != nil {
				t.Fatalf("protoconv.ProtoToValueIn(unit_id only): %v", err)
			}
			assertSameRef(t, back, tc.unit, tc.unitID, tc.term)
		})
	}

	// A qualified spelling names the same declaration as the short one.
	for _, text := range []string{"SI::km", "SI::kilometre", "kilometre"} {
		back, err := protoconv.ProtoToValueIn(measurementRefValue(text, "SI::kilometre", kilometreTerm()), idx, sem)
		if err != nil {
			t.Fatalf("protoconv.ProtoToValueIn(%s): %v", text, err)
		}
		assertSameRef(t, back, text, "SI::kilometre", "1000/1·SI::metre")
	}

	// A reference read back converts by, and equals, the one the model holds.
	km, err := protoconv.ProtoToValueIn(mustEvaluate(t, srv, modelHash, "SI::km"), idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	m, err := protoconv.ProtoToValueIn(mustEvaluate(t, srv, modelHash, "SI::m"), idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	if km.MeasurementRef().Declaration() == m.MeasurementRef().Declaration() {
		t.Error("km and m read back as the same declaration")
	}
	if !km.MeasurementRef().Unit.Term.Commensurable(m.MeasurementRef().Unit.Term) {
		t.Error("km and m read back as incommensurable")
	}
}

// assertSameRef checks a reference read back against what was sent.
func assertSameRef(t *testing.T, back runtime.Value, unit, unitID, term string) {
	t.Helper()
	if back.Kind != runtime.ValMeasurementRef {
		t.Fatalf("read back as %s: %v", back.Kind, back)
	}
	ref := back.MeasurementRef()
	if unit != "" && ref.Unit.Text != unit {
		t.Errorf("read back as %q, want %q", ref.Unit.Text, unit)
	}
	if got := describeUnitTerm(protoconv.UnitTermToProto(ref.Unit.Term)); got != term {
		t.Errorf("read back reducing to %s, want %s", got, term)
	}
	gotID := ""
	if decl := ref.Declaration(); decl != nil {
		gotID = symbols.FQNOf(decl)
	}
	if gotID != unitID {
		t.Errorf("read back naming declaration %q, want %q", gotID, unitID)
	}
}

// A malformed reference is refused with a typed error naming what is wrong,
// never read as another unit.
func TestMalformedMeasurementRefsAreRejected(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustMeasurementRefModel(t, srv)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"empty", measurementRefValue("", "", nil), protoconv.ErrMeasurementRefEmpty},
		{"named unit without its reduction", measurementRefValue("km", "", nil), protoconv.ErrUnitNotReduced},
		{"declaration without its reduction", measurementRefValue("", "SI::kilometre", nil), protoconv.ErrUnitNotReduced},
		{"unusable scale", measurementRefValue("m", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 0, Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"zero scale", measurementRefValue("m", "", &pb.UnitTerm{ScaleNum: 0, ScaleDen: 1, Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"NaN scale", measurementRefValue("m", "", &pb.UnitTerm{ScaleNum: math.NaN(), ScaleDen: 1, Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"infinite scale", measurementRefValue("m", "", &pb.UnitTerm{ScaleNum: math.Inf(1), ScaleDen: 1, Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"infinite denominator", measurementRefValue("m", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: math.Inf(-1), Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"term-only NaN scale", measurementRefValue("", "", &pb.UnitTerm{ScaleNum: math.NaN(), ScaleDen: 1, Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"term-only infinite scale", measurementRefValue("", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: math.Inf(1), Factors: metreTerm().Factors}), protoconv.ErrUnitScaleUnusable},
		{"NaN exponent", measurementRefValue("", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: math.NaN()}}}), protoconv.ErrUnitExponentUnusable},
		{"infinite exponent", measurementRefValue("", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: math.Inf(-1)}}}), protoconv.ErrUnitExponentUnusable},
		{"repeated exponents overflowing", measurementRefValue("", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::metre", Exponent: math.MaxFloat64},
			{UnitId: "SI::metre", Exponent: math.MaxFloat64},
		}}), protoconv.ErrUnitExponentUnusable},
		{"unknown base unit", measurementRefValue("furlong", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::furlong", Exponent: 1}}}), protoconv.ErrUnknownBaseUnit},
		{"factor over a part", measurementRefValue("M", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "M::convert", Exponent: 1}}}), protoconv.ErrNotAMeasurementUnit},
		{"unnamed factor", measurementRefValue("x", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{Exponent: 1}}}), protoconv.ErrUnknownBaseUnit},
		{"qualified text disagreeing with its reduction", measurementRefValue("SI::km", "", metreTerm()), protoconv.ErrUnitTextMismatch},
		{"composed text disagreeing with its reduction", measurementRefValue("SI::m * SI::s", "", metreTerm()), protoconv.ErrUnitTextMismatch},
		{"unknown declaration", measurementRefValue("km", "SI::furlong", kilometreTerm()), protoconv.ErrUnknownMeasurementUnit},
		{"declaration that is not a unit", measurementRefValue("", "M::convert", kilometreTerm()), protoconv.ErrNotAMeasurementUnit},
		{"declaration disagreeing with the reduction", measurementRefValue("km", "SI::kilometre", metreTerm()), protoconv.ErrUnitTextMismatch},
		{"text naming another declaration", measurementRefValue("m", "SI::kilometre", kilometreTerm()), protoconv.ErrUnitIDMismatch},
		{"text composing the declaration", measurementRefValue("km * km", "SI::kilometre", kilometreTerm()), protoconv.ErrUnitIDMismatch},
		{"text that is no name", measurementRefValue("km +", "SI::kilometre", kilometreTerm()), protoconv.ErrUnitIDMismatch},
		{"nested in a sequence", &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{
			intValue(1), measurementRefValue("km", "", nil),
		}}}}, protoconv.ErrUnitNotReduced},
		{"nested in an array", arrayValue([]int64{1}, measurementRefValue("", "", nil)), protoconv.ErrMeasurementRefEmpty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := protoconv.ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("protoconv.ProtoToValueIn = %v, %v; want %v", val, err, tc.want)
			}
			if val.Kind != runtime.ValInvalid {
				t.Errorf("a rejected value was still returned: %v", val)
			}
		})
	}

	// A reference's units resolve against the model, so one needs its symbols.
	for name, val := range map[string]*pb.Value{
		"factors":  measurementRefValue("m", "", metreTerm()),
		"unit_id":  measurementRefValue("", "SI::metre", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1}),
		"in array": arrayValue([]int64{1}, measurementRefValue("m", "", metreTerm())),
	} {
		if _, err := protoconv.ProtoToValueIn(val, nil, nil); !errors.Is(err, protoconv.ErrMeasurementRefNeedsIndex) {
			t.Errorf("%s without an index: err = %v, want %v", name, err, protoconv.ErrMeasurementRefNeedsIndex)
		}
	}
	// A short name no declaration reducing so bears is a client's own label for
	// the reduction, as a Quantity's is: it names no declaration.
	label, err := protoconv.ProtoToValueIn(measurementRefValue("km", "", metreTerm()), idx, sem)
	if err != nil || label.Kind != runtime.ValMeasurementRef {
		t.Fatalf("short label = %v, %v; want a reference", label, err)
	}
	if ref := label.MeasurementRef(); ref.Declaration() != nil || ref.Unit.Text != "km" || describeUnitTerm(protoconv.UnitTermToProto(ref.Unit.Term)) != "SI::metre" {
		t.Errorf("short label read as %v naming %v, want an opaque km reducing to SI::metre", ref, ref.Declaration())
	}

	// A dimension-one reference names no base unit, so it needs none.
	one, err := protoconv.ProtoToValueIn(measurementRefValue("", "", &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1}), nil, nil)
	if err != nil || one.Kind != runtime.ValMeasurementRef || !one.MeasurementRef().Unit.Term.Dimensionless() {
		t.Errorf("dimension one without an index = %v, %v; want a dimension-one reference", one, err)
	}
}

// Evaluate, Instantiate, ExecuteAction and EvaluateCalc carry a measurement
// reference as itself, in both directions, and one read in converts a quantity.
func TestMeasurementRefCrossesEveryValueSurface(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash := mustMeasurementRefModel(t, srv)

	units := mustEvaluate(t, srv, modelHash, "M::units")
	elems := units.GetSequence().GetElements()
	if len(elems) != 2 || elems[0].GetMeasurementRef().GetUnitId() != "SI::metre" || elems[1].GetMeasurementRef().GetUnitId() != "SI::second" {
		t.Fatalf("M::units = %v, want a sequence of the references m and s", units)
	}

	inst, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: "M"})
	if err != nil || inst.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, inst.GetError())
	}
	for name, want := range map[string]string{"u": "SI::metre", "speed": ""} {
		fv := inst.Instance.FeatureValues[name]
		if fv == nil || fv.Error != "" || fv.Value.GetMeasurementRef() == nil || fv.Value.GetMeasurementRef().GetUnitId() != want {
			t.Errorf("feature value %s = %v, want a measurement reference naming %q", name, fv, want)
		}
	}

	metre := measurementRefValue("m", "SI::metre", metreTerm())
	threeKm := &pb.Value{Kind: &pb.Value_Quantity{Quantity: mustEvaluateQuantity(t, srv, modelHash, "M::q")}}
	act, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: "M::convert",
		Inputs:         map[string]*pb.Value{"x": threeKm, "target": metre},
	})
	if err != nil || act.Error != "" {
		t.Fatalf("ExecuteAction: err = %v, error = %q", err, act.GetError())
	}
	if y := act.Outputs["y"].GetQuantity(); y == nil || y.GetRealMagnitude() != 3000 || y.GetUnit() != "m" {
		t.Errorf("output y = %v, want 3000.0 [m]", act.Outputs["y"])
	}

	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::toUnit", Arguments: []*pb.Value{threeKm, metre}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(toUnit): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetQuantity(); got == nil || got.GetRealMagnitude() != 3000 || got.GetUnit() != "m" {
		t.Errorf("toUnit(3 [km], m) = %v, want 3000.0 [m]", calc.Result)
	}

	// The text alone, spelt as a model would write it, is read as a Quantity's unit is.
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::measured", Arguments: []*pb.Value{
		realValue(2.5), measurementRefValue("SI::km", "", kilometreTerm()),
	}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(measured): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetQuantity(); got == nil || got.GetRealMagnitude() != 2.5 || got.GetUnit() != "SI::km" {
		t.Errorf("measured(2.5, SI::km) = %v, want 2.5 [SI::km]", calc.Result)
	}

	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::unitOf", Arguments: []*pb.Value{threeKm}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(unitOf): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetMeasurementRef(); got == nil || got.GetUnit() != "km" || got.GetUnitId() != "SI::kilometre" {
		t.Errorf("unitOf(3 [km]) = %v, want the reference km", calc.Result)
	}

	// A malformed argument is an in-band error, as a malformed quantity is.
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::toUnit", Arguments: []*pb.Value{
		threeKm, measurementRefValue("m", "SI::kilometre", kilometreTerm()),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(malformed): %v", err)
	}
	if !strings.Contains(calc.Error, protoconv.ErrUnitIDMismatch.Error()) {
		t.Errorf("EvaluateCalc(malformed) error = %q, want one naming %v", calc.Error, protoconv.ErrUnitIDMismatch)
	}
}

// The arm is advertised under its own capability, so a client can require it;
// a service withholding it names the reference as unsupported — what a client
// built before the arm existed was always sent — nested in a sequence included,
// and refuses one sent to it rather than reading it as another value.
func TestMeasurementRefCapability(t *testing.T) {
	ctx := context.Background()
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityMeasurementRefs
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityMeasurementRefs)
	}

	withheld := mustNewServiceWithout(t, CapabilityMeasurementRefs)
	modelHash := mustMeasurementRefModel(t, withheld)
	for expr, want := range map[string]string{
		"SI::m":       "unsupported: measurement reference m",
		"M::speed":    "unsupported: measurement reference m/s",
		"M::q.mRef":   "unsupported: measurement reference km",
		"SI::m/SI::m": "unsupported: measurement reference 1",
	} {
		got := mustEvaluate(t, withheld, modelHash, expr)
		if got.GetMeasurementRef() != nil || got.GetQuantity() != nil {
			t.Errorf("%s crossed as %T without %s: %v", expr, got.GetKind(), CapabilityMeasurementRefs, got)
		}
		if got.GetNull() != want {
			t.Errorf("%s without %s = %v, want null %q", expr, CapabilityMeasurementRefs, got, want)
		}
	}
	units := mustEvaluate(t, withheld, modelHash, "M::units")
	for i, want := range []string{"m", "s"} {
		if got := units.GetSequence().GetElements()[i].GetNull(); got != "unsupported: measurement reference "+want {
			t.Errorf("M::units#%d without %s = %q", i+1, CapabilityMeasurementRefs, got)
		}
	}

	// A structured client without the arm still gets its structured values, the
	// reference inside them withheld.
	pv := arrayValue([]int64{1}, measurementRefValue("m", "SI::metre", metreTerm()))
	withheld.filterValueCapabilities(pv)
	if pv.GetArray() == nil || pv.GetArray().GetElements()[0].GetNull() != "unsupported: measurement reference m" {
		t.Errorf("array of a reference without %s = %v, want the element withheld", CapabilityMeasurementRefs, pv)
	}

	metre := measurementRefValue("m", "SI::metre", metreTerm())
	threeKm := &pb.Value{Kind: &pb.Value_Quantity{Quantity: mustEvaluateQuantity(t, withheld, modelHash, "M::q")}}
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{metre}}}}
	for name, input := range map[string]*pb.Value{"reference": metre, "nested": nested} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "M::convert",
			Inputs:         map[string]*pb.Value{"x": threeKm, "target": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityMeasurementRefs) {
			t.Errorf("ExecuteAction with %s input without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityMeasurementRefs, err)
		}
		_, err = withheld.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::toUnit", Arguments: []*pb.Value{threeKm, input}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityMeasurementRefs) {
			t.Errorf("EvaluateCalc with %s argument without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityMeasurementRefs, err)
		}
	}

	// A quantity's unit is not the arm: it crosses with or without the capability.
	if q := mustEvaluate(t, withheld, modelHash, "M::q"); q.GetQuantity() == nil {
		t.Errorf("M::q without %s = %v, want a quantity", CapabilityMeasurementRefs, q)
	}
}

func TestValueCarriesMeasurementRef(t *testing.T) {
	one := intValue(1)
	metre := measurementRefValue("m", "SI::metre", metreTerm())
	sequence := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	for _, tc := range []struct {
		name  string
		value *pb.Value
		want  bool
	}{
		{"nil", nil, false},
		{"int", one, false},
		{"quantity", &pb.Value{Kind: &pb.Value_Quantity{Quantity: &pb.Quantity{Unit: "m"}}}, false},
		{"reference", metre, true},
		{"sequence of ints", sequence(one, one), false},
		{"sequence with a reference", sequence(one, sequence(metre)), true},
		{"array of references", arrayValue([]int64{1}, metre), true},
	} {
		if got := protoconv.ValueCarriesMeasurementRef(tc.value); got != tc.want {
			t.Errorf("protoconv.ValueCarriesMeasurementRef(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
	if protoconv.ValueCarriesStructured(metre) {
		t.Error("a bare reference is not a structured value")
	}
}
