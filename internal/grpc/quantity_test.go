package grpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// quantityModel exercises every shape of quantity a feature value can hold: one written
// with a simple unit, one computed into a compound unit, one written in a scaled
// compound unit, and one inside a nested part.
const quantityModel = `
package P {
	private import ScalarValues::*;

	part def Engine {
		attribute power : ISQ::PowerValue = 300.0 [SI::W];
	}

	part def Car {
		attribute m : ISQ::MassValue = 5.0 [SI::kg];
		attribute n : ScalarValues::Real = 2.0;
		attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];
		attribute writtenSpeed = 5.4 [SI::km/SI::h];
		attribute count = 3 [SI::m];
		part engine : Engine;
	}
}
`

// mustQuantityModel parses quantityModel and returns the service, the model hash,
// the index the model's names resolve against and the semantics over it.
func mustQuantityModel(t *testing.T) (*Service, string, *symbols.Index, *semantics.Model) {
	t.Helper()

	srv := mustNewService(t, 4)
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: quantityModel},
		ContentHash: "quantity-model",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range resp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}
	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	return srv, resp.ModelHash, cached.Index, NewSymbolContext(cached.Index).Semantics
}

// mustEvaluateQuantity evaluates expr and returns the quantity it produced.
func mustEvaluateQuantity(t *testing.T, srv *Service, modelHash, expr string) *pb.Quantity {
	t.Helper()

	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
		ModelHash:  modelHash,
		Expression: expr,
	})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", expr, err)
	}
	if resp.Error != "" {
		t.Fatalf("Evaluate(%s): %s", expr, resp.Error)
	}
	quantity := resp.Result.GetQuantity()
	if quantity == nil {
		t.Fatalf("Evaluate(%s) = %v, want a quantity", expr, resp.Result)
	}
	return quantity
}

// TestQuantityCrossesTheWire pins what a quantity carries: the magnitude in the
// unit written, never reduced, plus the reduction that makes it comparable.
func TestQuantityCrossesTheWire(t *testing.T) {
	srv, modelHash, _, _ := mustQuantityModel(t)

	tests := []struct {
		expr       string
		unit       string
		real       float64
		intVal     int64
		isInt      bool
		reduction  string
		wantScaled bool
	}{
		// A prefixed unit reduces to its base unit and a scale: kg is 1000 grams.
		{expr: "5.0 [SI::kg]", unit: "SI::kg", real: 5.0, reduction: "1000/1·SI::gram", wantScaled: true},
		{expr: "3 [SI::m]", unit: "SI::m", intVal: 3, isInt: true, reduction: "SI::metre"},
		{expr: "10.0 [SI::m] / 2.0 [SI::s]", unit: "SI::'m/s'", real: 5.0, reduction: "SI::metre·SI::second^-1"},
		{expr: "5.4 [SI::km/SI::h]", unit: "SI::km/SI::h", real: 5.4, reduction: "5/18·SI::metre·SI::second^-1", wantScaled: true},
		// Grouping the notation needs survives, so the text reads back as the unit written.
		{expr: "3.0 [SI::m/(SI::s*SI::kg)]", unit: "SI::m/(SI::s*SI::kg)", real: 3.0, reduction: "1/1000·SI::gram^-1·SI::metre·SI::second^-1", wantScaled: true},
		{expr: "4.0 [(SI::m*SI::s)**2]", unit: "(SI::m*SI::s)**2", real: 4.0, reduction: "SI::metre^2·SI::second^2"},
		{expr: "8.0 [(SI::m**2)**3]", unit: "(SI::m**2)**3", real: 8.0, reduction: "SI::metre^6"},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			got := mustEvaluateQuantity(t, srv, modelHash, tc.expr)
			if got.GetUnit() != tc.unit {
				t.Errorf("unit = %q, want %q", got.GetUnit(), tc.unit)
			}
			if tc.isInt {
				if got.GetIntMagnitude() != tc.intVal {
					t.Errorf("int_magnitude = %d, want %d", got.GetIntMagnitude(), tc.intVal)
				}
			} else if got.GetRealMagnitude() != tc.real {
				t.Errorf("real_magnitude = %v, want %v", got.GetRealMagnitude(), tc.real)
			}
			if reduction := describeUnitTerm(got.GetUnitTerm()); reduction != tc.reduction {
				t.Errorf("reduction = %q, want %q", reduction, tc.reduction)
			}
			scaled := got.GetUnitTerm().GetScaleNum() != got.GetUnitTerm().GetScaleDen()
			if scaled != tc.wantScaled {
				t.Errorf("scale = %v/%v, want scaled = %v",
					got.GetUnitTerm().GetScaleNum(), got.GetUnitTerm().GetScaleDen(), tc.wantScaled)
			}
		})
	}
}

// TestQuantityRoundTrip is the fidelity requirement: a quantity that goes out
// and comes back is the same quantity, unit included — same magnitude, same unit
// as written, and a reduction over the very base-unit symbols it left with.
func TestQuantityRoundTrip(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)

	for _, expr := range []string{
		"5.0 [SI::kg]",
		"3 [SI::m]",
		"10.0 [SI::m] / 2.0 [SI::s]",
		"5.4 [SI::km/SI::h]",
		"(2.0 [SI::m])**2",
		"3.0 [SI::m/(SI::s*SI::kg)]",
		"4.0 [(SI::m*SI::s)**2]",
		"8.0 [(SI::m**2)**3]",
		"6.0 [SI::m/SI::s/SI::kg]",
		"2.0 [SI::'m/s²'] * 3.0 [SI::s]",
	} {
		t.Run(expr, func(t *testing.T) {
			sent := mustEvaluateQuantity(t, srv, modelHash, expr)

			val, err := ProtoToValueIn(&pb.Value{Kind: &pb.Value_Quantity{Quantity: sent}}, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if val.Kind != runtime.ValQuantity || val.Quantity() == nil {
				t.Fatalf("kind = %v, want a quantity", val.Kind)
			}

			back := QuantityToProto(val.Quantity())
			if back.GetUnit() != sent.GetUnit() {
				t.Errorf("unit = %q, want %q", back.GetUnit(), sent.GetUnit())
			}
			if back.GetIntMagnitude() != sent.GetIntMagnitude() || back.GetRealMagnitude() != sent.GetRealMagnitude() {
				t.Errorf("magnitude = %v, want %v", back.GetMagnitude(), sent.GetMagnitude())
			}
			if describeUnitTerm(back.GetUnitTerm()) != describeUnitTerm(sent.GetUnitTerm()) {
				t.Errorf("reduction = %q, want %q",
					describeUnitTerm(back.GetUnitTerm()), describeUnitTerm(sent.GetUnitTerm()))
			}
			if !val.Quantity().Unit.Term.Commensurable(mustUnitTerm(t, sent, idx, sem)) {
				t.Error("round-tripped quantity is not commensurable with the one sent")
			}
		})
	}
}

// mustUnitTerm rebuilds the unit term of sent, for comparing a round-trip
// against a second, independent reconstruction.
func mustUnitTerm(t *testing.T, sent *pb.Quantity, idx *symbols.Index, sem *semantics.Model) semantics.UnitTerm {
	t.Helper()

	val, err := ProtoToQuantity(sent, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	return val.Quantity().Unit.Term
}

// TestQuantityFromWireIsNormalized pins that a hand-built reduction — factors in
// any order, a base unit repeated, an exponent that cancels — is commensurable
// with the same unit the model derives, which compares factors element-wise.
func TestQuantityFromWireIsNormalized(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	derived := mustEvaluateQuantity(t, srv, modelHash, "10.0 [SI::m] / 2.0 [SI::s]")

	byHand := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "SI::m/SI::s",
		UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::second", Exponent: -1},
			{UnitId: "SI::gram", Exponent: 0},
			{UnitId: "SI::metre", Exponent: 2},
			{UnitId: "SI::metre", Exponent: -1},
		}},
	}

	val, err := ProtoToQuantity(byHand, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	if !val.Quantity().Unit.Term.Commensurable(mustUnitTerm(t, derived, idx, sem)) {
		t.Errorf("reduction = %s, want it commensurable with %s",
			val.Quantity().Unit.Term, describeUnitTerm(derived.GetUnitTerm()))
	}
}

// TestQuantityFromWireNeedsTheModel pins the two ways a quantity cannot be read
// back: without the model's symbols, and over a base unit it does not declare.
func TestQuantityFromWireNeedsTheModel(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	sent := mustEvaluateQuantity(t, srv, modelHash, "5.0 [SI::kg]")

	if _, err := ProtoToQuantity(sent, nil, nil); !errors.Is(err, ErrQuantityNeedsIndex) {
		t.Errorf("without an index: err = %v, want ErrQuantityNeedsIndex", err)
	}

	unknown := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "Made::up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "Made::up", Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(unknown, idx, sem); !errors.Is(err, ErrUnknownBaseUnit) {
		t.Errorf("over an undeclared base unit: err = %v, want ErrUnknownBaseUnit", err)
	}

	for _, scale := range []*pb.UnitTerm{
		{ScaleNum: 1, ScaleDen: 0}, {ScaleNum: 0, ScaleDen: 1},
		{ScaleNum: math.NaN(), ScaleDen: 1}, {ScaleNum: 1, ScaleDen: math.NaN()},
		{ScaleNum: math.Inf(1), ScaleDen: 1}, {ScaleNum: 1, ScaleDen: math.Inf(-1)},
	} {
		unusable := &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
			Unit:      "SI::m",
			UnitTerm:  scale,
		}
		if _, err := ProtoToQuantity(unusable, idx, sem); !errors.Is(err, ErrUnitScaleUnusable) {
			t.Errorf("over scale %g/%g: err = %v, want ErrUnitScaleUnusable",
				scale.ScaleNum, scale.ScaleDen, err)
		}
	}

	for _, exponent := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		unusable := &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
			Unit:      "SI::m",
			UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: exponent}}},
		}
		if _, err := ProtoToQuantity(unusable, idx, sem); !errors.Is(err, ErrUnitExponentUnusable) {
			t.Errorf("over exponent %g: err = %v, want ErrUnitExponentUnusable", exponent, err)
		}
	}

	overflowing := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "SI::m",
		UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::metre", Exponent: math.MaxFloat64},
			{UnitId: "SI::metre", Exponent: math.MaxFloat64},
		}},
	}
	if _, err := ProtoToQuantity(overflowing, idx, sem); !errors.Is(err, ErrUnitExponentUnusable) {
		t.Errorf("over repeated exponents summing past the largest double: err = %v, want ErrUnitExponentUnusable", err)
	}

	noMagnitude := &pb.Quantity{Unit: "SI::kg"}
	if _, err := ProtoToQuantity(noMagnitude, idx, sem); err == nil {
		t.Error("a quantity with no magnitude must be reported, not read as zero")
	}
}

// TestQuantityOverSomethingThatIsNotAUnit pins that a reduction is only accepted
// over measurement units: a name resolving to a part, or to nothing at all, is
// rejected rather than measured in.
func TestQuantityOverSomethingThatIsNotAUnit(t *testing.T) {
	_, _, idx, sem := mustQuantityModel(t)

	overAPart := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "P::Car",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "P::Car", Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(overAPart, idx, sem); !errors.Is(err, ErrNotAMeasurementUnit) {
		t.Errorf("over a part: err = %v, want ErrNotAMeasurementUnit", err)
	}

	// An empty name is a lookup of the document root, which would otherwise
	// resolve to exactly one symbol and pass as a base unit.
	unnamed := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "made up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(unnamed, idx, sem); !errors.Is(err, ErrUnknownBaseUnit) {
		t.Errorf("over an unnamed factor: err = %v, want ErrUnknownBaseUnit", err)
	}
}

// TestQuantityFeatureValuesAndNestedQuantities drives Instantiate: every quantity feature value
// of a part, and the quantity inside the part it holds, cross as quantities.
func TestQuantityFeatureValuesAndNestedQuantities(t *testing.T) {
	srv, modelHash, _, _ := mustQuantityModel(t)

	resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{
		ModelHash: modelHash,
		SymbolId:  "P::Car",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Instantiate: %s", resp.Error)
	}

	for name, want := range map[string]string{
		"m":            "5 [SI::kg] = 1000/1·SI::gram",
		"derivedSpeed": "5 [SI::'m/s'] = SI::metre·SI::second^-1",
		"writtenSpeed": "5.4 [SI::km/SI::h] = 5/18·SI::metre·SI::second^-1",
		"count":        "3 [SI::m] = SI::metre",
	} {
		fv, ok := resp.Instance.FeatureValues[name]
		if !ok {
			t.Errorf("missing feature value %q", name)
			continue
		}
		if fv.Error != "" {
			t.Errorf("feature value %q: %s", name, fv.Error)
			continue
		}
		if got := describeQuantity(fv.Value.GetQuantity()); got != want {
			t.Errorf("feature value %q = %q, want %q", name, got, want)
		}
	}

	// The ordinary real feature value is untouched by the quantity arm.
	if got := resp.Instance.FeatureValues["n"].GetValue().GetRealValue(); got != 2.0 {
		t.Errorf("feature value n = %v, want 2", got)
	}

	engineID := resp.Instance.FeatureValues["engine"].GetValue().GetInstanceId()
	if engineID == 0 {
		t.Fatal("feature value engine holds no instance")
	}
	var engine *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == engineID {
			engine = inst
		}
	}
	if engine == nil {
		t.Fatalf("instance %d is not in the response graph", engineID)
	}
	wantPower := "300 [SI::W] = 1000/1·SI::gram·SI::metre^2·SI::second^-3"
	if got := describeQuantity(engine.FeatureValues["power"].GetValue().GetQuantity()); got != wantPower {
		t.Errorf("nested feature value power = %q, want %q", got, wantPower)
	}
}

// TestQuantityWithoutItsReduction pins that a named unit sent with no reduction
// is rejected: dimension one would make it commensurable with a bare number.
func TestQuantityWithoutItsReduction(t *testing.T) {
	_, _, idx, sem := mustQuantityModel(t)

	unreduced := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "Furlongs::furlong",
	}
	if _, err := ProtoToQuantity(unreduced, idx, sem); !errors.Is(err, ErrUnitNotReduced) {
		t.Errorf("error = %v, want %v", err, ErrUnitNotReduced)
	}

	// A magnitude under no unit at all is dimension one, which is what it says.
	dimensionless := &pb.Quantity{Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5}}
	val, err := ProtoToQuantity(dimensionless, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	if len(val.Quantity().Unit.Term.Factors) != 0 {
		t.Errorf("factors = %v, want none", val.Quantity().Unit.Term.Factors)
	}
}

// TestQuantityFromWireComposesAsWritten pins that a quantity read back from the
// wire keeps the named units its unit text composes, so an operation on it in a
// calc cancels and merges them exactly as it does for a quantity written in the model.
func TestQuantityFromWireComposesAsWritten(t *testing.T) {
	srv := mustNewService(t, 4)
	source := `package Q {
	private import ScalarValues::*;
	private import SI::*;
	calc def Dist { in v; in dt; v * dt }
	calc def Area { in a; in b; a * b }
	calc def Per { in a; in b; a / b }
	calc def Metre { 2.0 [m] }
	calc def Kilometre { 3.0 [km] }
}
package Nautical {
	private import MeasurementReferences::*;
	private import ISQ::*;
	private import SI::*;
	attribute <fathom> 'fathom' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1.8288; } }
	attribute <cable> 'cable' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 182.88; } }
	calc def Fathom { 2.0 [fathom] }
	calc def Cable { 1.0 [cable] }
}
package Imperial {
	private import MeasurementReferences::*;
	private import ISQ::*;
	private import SI::*;
	attribute <fathom> 'fathom' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1.8288; } }
	attribute <cable> 'cable' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 185.3184; } }
	calc def Cable { 1.0 [cable] }
}
`
	hash := mustVerifyModel(t, srv, source, "quantity-composes-as-written")
	evaluate := func(calc string, args ...*pb.Quantity) *pb.Quantity {
		t.Helper()
		req := &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: calc}
		for _, arg := range args {
			req.Arguments = append(req.Arguments, &pb.Value{Kind: &pb.Value_Quantity{Quantity: arg}})
		}
		resp, err := srv.EvaluateCalc(context.Background(), req)
		if err != nil {
			t.Fatalf("EvaluateCalc %s: %v", calc, err)
		}
		if resp.Error != "" {
			t.Fatalf("EvaluateCalc %s reported %q", calc, resp.Error)
		}
		if resp.Result.GetQuantity() == nil {
			t.Fatalf("EvaluateCalc %s = %v, want a quantity", calc, resp.Result)
		}
		return resp.Result.GetQuantity()
	}

	speed := mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m] / 1.0 [SI::s]")
	if speed.GetUnit() != "SI::'m/s'" {
		t.Fatalf("speed crosses the wire in %q, want SI::'m/s'", speed.GetUnit())
	}
	dist := evaluate("Q::Dist", speed, mustEvaluateQuantity(t, srv, hash, "2.0 [SI::s]"))
	if got := describeQuantity(dist); got != "6 [SI::m] = SI::metre" {
		t.Errorf("m/s * s over the wire = %s, want 6 [SI::m] = SI::metre", got)
	}

	// A scaled named unit composed folds its scale into the magnitude, as it does
	// locally: two kilometres squared are four million square metres.
	byHand := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
		Unit:      "SI::km",
		UnitTerm:  mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]").GetUnitTerm(),
	}
	area := evaluate("Q::Area", byHand, byHand)
	if got := describeQuantity(area); got != "4e+06 [SI::'m²'] = SI::metre^2" {
		t.Errorf("km * km over the wire = %s, want 4e+06 [SI::'m²'] = SI::metre^2", got)
	}

	// A unit whose name the notation quotes is one unit when composed: `'A/m'`
	// times `m` reduces to the ampere, and squared to a base-unit product.
	density := mustEvaluateQuantity(t, srv, hash, "2.0 [SI::'A/m']")
	if density.GetUnit() != "SI::'A/m'" {
		t.Fatalf("a quoted unit crosses the wire in %q, want SI::'A/m'", density.GetUnit())
	}
	if got := describeQuantity(evaluate("Q::Area", density, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m]"))); got != "6 [SI::A] = SI::ampere" {
		t.Errorf("'A/m' * m over the wire = %s, want 6 [SI::A] = SI::ampere", got)
	}
	if got := describeQuantity(evaluate("Q::Area", density, density)); got != "4 [A**2/m**2] = SI::ampere^2·SI::metre^-2" {
		t.Errorf("'A/m' * 'A/m' over the wire = %s, want 4 [A**2/m**2] = SI::ampere^2·SI::metre^-2", got)
	}

	// A unit named through an alias is the unit the alias stands for: SI::'m/s²'
	// reduces as SI::'m⋅s⁻²' does and composes with SI::s to a coherent speed.
	accel := mustEvaluateQuantity(t, srv, hash, "2.0 [SI::'m/s²']")
	if accel.GetUnit() != "SI::'m/s²'" {
		t.Fatalf("an aliased unit crosses the wire in %q, want SI::'m/s²'", accel.GetUnit())
	}
	if got := describeQuantity(evaluate("Q::Area", accel, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::'m⋅s⁻²']"))); got != "6 [m**2/s**4] = SI::metre^2·SI::second^-4" {
		t.Errorf("'m/s²' * 'm⋅s⁻²' over the wire = %s, want 6 [m**2/s**4] = SI::metre^2·SI::second^-4", got)
	}
	if got := describeQuantity(evaluate("Q::Dist", accel, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::s]"))); got != "6 [SI::'m/s'] = SI::metre·SI::second^-1" {
		t.Errorf("'m/s²' * s over the wire = %s, want 6 [SI::'m/s'] = SI::metre·SI::second^-1", got)
	}
	shortAlias := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
		Unit:      "'m/s²'*s",
		UnitTerm:  speed.GetUnitTerm(),
	}
	if got := describeQuantity(evaluate("Q::Dist", shortAlias, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::s]"))); got != "6 [SI::m] = SI::metre" {
		t.Errorf("short 'm/s²'*s over the wire * s = %s, want 6 [SI::m] = SI::metre", got)
	}

	// Unit text that is no unit expression is one opaque unit: still a quantity
	// over the reduction sent, and still what the sender wrote, quoted as one name
	// once composed so that it reads back as one unit.
	opaque := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "metres per second",
		UnitTerm:  speed.GetUnitTerm(),
	}
	got := describeQuantity(evaluate("Q::Dist", opaque, mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")))
	if got != "5 ['metres per second'*SI::s] = SI::metre" {
		t.Errorf("opaque unit over the wire = %s, want 5 ['metres per second'*SI::s] = SI::metre", got)
	}

	// A unit written short, as an import let the sender write it, is the unit of
	// that name reducing as sent: `m` is SI::m, and cancels or merges with SI::m
	// written in full.
	unqualified := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "m/s",
		UnitTerm:  speed.GetUnitTerm(),
	}
	got = describeQuantity(evaluate("Q::Dist", unqualified, mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")))
	if got != "5 [m] = SI::metre" {
		t.Errorf("unqualified unit over the wire = %s, want 5 [m] = SI::metre", got)
	}
	metre := evaluate("Q::Metre")
	if metre.GetUnit() != "m" {
		t.Fatalf("a quantity written under an import crosses the wire in %q, want m", metre.GetUnit())
	}
	got = describeQuantity(evaluate("Q::Area", metre, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m]")))
	if got != "6 [SI::'m²'] = SI::metre^2" {
		t.Errorf("m * SI::m over the wire = %s, want 6 [SI::'m²'] = SI::metre^2", got)
	}

	// A short name the model does not declare, or declares as a unit the
	// reduction contradicts, is opaque: it was not certainly that unit.
	for _, tc := range []struct{ unit, want string }{
		{"ft", "6 [SI::m*ft] = SI::metre^2"},
		{"km", "6 [SI::m*km] = SI::metre^2"},
	} {
		short := &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
			Unit:      tc.unit,
			UnitTerm:  metre.GetUnitTerm(),
		}
		got = describeQuantity(evaluate("Q::Area", short, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m]")))
		if got != tc.want {
			t.Errorf("%s over a metre reduction * SI::m = %s, want %s", tc.unit, got, tc.want)
		}
	}
	// Nor does the opaque unit merge with the resolved unit spelt the same way:
	// `km**2` would read as a million square metres where the reduction has a thousand.
	opaqueKm := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
		Unit:      "km",
		UnitTerm:  metre.GetUnitTerm(),
	}
	got = describeQuantity(evaluate("Q::Area", opaqueKm, evaluate("Q::Kilometre")))
	if got != "6 [km*km] = 1000/1·SI::metre^2" {
		t.Errorf("opaque km over a metre reduction * km = %s, want 6 [km*km] = 1000/1·SI::metre^2", got)
	}

	// A derived unit the model declares outside its base unit's namespace keeps
	// its identity when written short: `cable` is Nautical::cable, the one unit of
	// that name whose reduction is the one sent, so squared it folds to an area.
	fathom := evaluate("Nautical::Fathom")
	if fathom.GetUnit() != "fathom" {
		t.Fatalf("a custom unit written under its package crosses the wire in %q, want fathom", fathom.GetUnit())
	}
	inFull := func(unit string, term *pb.UnitTerm) *pb.Quantity {
		return &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
			Unit:      unit,
			UnitTerm:  term,
		}
	}
	cable := evaluate("Nautical::Cable")
	if cable.GetUnit() != "cable" {
		t.Fatalf("a custom unit written under its package crosses the wire in %q, want cable", cable.GetUnit())
	}
	got = describeQuantity(evaluate("Q::Area", cable, inFull("Nautical::cable", cable.GetUnitTerm())))
	if got != "66890.1888 [SI::'m²'] = SI::metre^2" {
		t.Errorf("cable * Nautical::cable over the wire = %s, want 66890.1888 [SI::'m²'] = SI::metre^2", got)
	}
	// Two packages declaring one short name for the same unit is an ambiguity the
	// reduction cannot settle: the text stays opaque rather than picked at random.
	got = describeQuantity(evaluate("Q::Area", fathom, inFull("Nautical::fathom", fathom.GetUnitTerm())))
	if got != "4 [Nautical::fathom*fathom] = 3.34450944/1·SI::metre^2" {
		t.Errorf("ambiguous fathom * Nautical::fathom over the wire = %s, want 4 [Nautical::fathom*fathom] = 3.34450944/1·SI::metre^2", got)
	}
	// One short name written twice may name two units, each read where the
	// reduction puts it: `cable*cable` over both cables is Nautical::cable times
	// Imperial::cable, folding to the area their two lengths span; dividing by
	// either cable written in full leaves the other's length.
	imperialCable := evaluate("Imperial::Cable")
	cables := evaluate("Q::Area", cable, imperialCable)
	if got := describeQuantity(cables); got != "33891.028992 [SI::'m²'] = SI::metre^2" {
		t.Fatalf("two cables cross the wire as %s, want 33891.028992 [SI::'m²'] = SI::metre^2", got)
	}
	for _, tc := range []struct {
		by   string
		term *pb.UnitTerm
		want string
	}{
		{"Nautical::cable", cable.GetUnitTerm(), "92.6592 [SI::m] = SI::metre"},
		{"Imperial::cable", imperialCable.GetUnitTerm(), "91.44 [SI::m] = SI::metre"},
	} {
		got = describeQuantity(evaluate("Q::Per", cables, inFull(tc.by, tc.term)))
		if got != tc.want {
			t.Errorf("cable*cable over the wire / %s = %s, want %s", tc.by, got, tc.want)
		}
	}

	// A quantity sent under no unit text is its base units, composing with the named
	// units they are; under a scale it is the reduction itself, opaque.
	unnamed := func(term *pb.UnitTerm) *pb.Quantity {
		return &pb.Quantity{Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2}, UnitTerm: term}
	}
	second := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")
	for _, tc := range []struct {
		name string
		calc string
		args []*pb.Quantity
		want string
	}{
		{"speed times seconds", "Q::Dist", []*pb.Quantity{unnamed(speed.GetUnitTerm()), second}, "2 [SI::metre] = SI::metre"},
		{"speed times a metre", "Q::Area", []*pb.Quantity{unnamed(speed.GetUnitTerm()), metre}, "4 [SI::'m²⋅s⁻¹'] = SI::metre^2·SI::second^-1"},
		{"kilometres times a metre", "Q::Area", []*pb.Quantity{unnamed(byHand.GetUnitTerm()), metre}, "4 ['1000·metre'*m] = 1000/1·SI::metre^2"},
		{"kilometres alone", "Q::Area", []*pb.Quantity{unnamed(byHand.GetUnitTerm()), unnamed(byHand.GetUnitTerm())}, "4 ['1000·metre'**2] = 1e+06/1·SI::metre^2"},
	} {
		if got := describeQuantity(evaluate(tc.calc, tc.args...)); got != tc.want {
			t.Errorf("%s, sent without unit text, = %s, want %s", tc.name, got, tc.want)
		}
	}
	// A scaled ratio sent nameless names no dimension-one unit: it scales what it
	// multiplies and cancels to a number, as an unnamed ratio does locally.
	hundredth := unnamed(&pb.UnitTerm{ScaleNum: 1, ScaleDen: 100})
	if got := describeQuantity(evaluate("Q::Area", hundredth, metre)); got != "4 ['1/100'*m] = 1/100·SI::metre" {
		t.Errorf("a nameless hundredth * m = %s, want 4 ['1/100'*m] = 1/100·SI::metre", got)
	}
	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash, SymbolId: "Q::Area",
		Arguments: []*pb.Value{{Kind: &pb.Value_Quantity{Quantity: hundredth}}, {Kind: &pb.Value_Quantity{Quantity: hundredth}}},
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("EvaluateCalc Q::Area over nameless hundredths: %v %q", err, resp.GetError())
	}
	if real, ok := resp.Result.GetKind().(*pb.Value_RealValue); !ok || real.RealValue != 0.0004 {
		t.Errorf("a nameless hundredth squared = %v, want the number 0.0004", resp.Result)
	}
}

// TestOpaqueUnitFactorsSurviveTheWire pins that a unit composed of an opaque
// factor and a unit the model declares crosses the wire with that boundary
// intact: read back, the declared unit still cancels and the opaque factor
// remains, whatever made it opaque — text that is no unit expression, a name no
// unit or two units bear, or a scaled reduction sent nameless.
func TestOpaqueUnitFactorsSurviveTheWire(t *testing.T) {
	srv := mustNewService(t, 4)
	source := `package Q {
	private import ScalarValues::*;
	private import SI::*;
	calc def Times { in a; in b; a * b }
	calc def Per { in a; in b; a / b }
}
package Nautical {
	private import MeasurementReferences::*;
	private import ISQ::*;
	private import SI::*;
	attribute <fathom> 'fathom' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1.8288; } }
	calc def Fathom { 1.0 [fathom] }
}
package Imperial {
	private import MeasurementReferences::*;
	private import ISQ::*;
	private import SI::*;
	attribute <fathom> 'fathom' : LengthUnit { :>> unitConversion: ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1.8288; } }
}
`
	hash := mustVerifyModel(t, srv, source, "opaque-factors-survive-the-wire")
	evaluate := func(calc string, args ...*pb.Quantity) *pb.Quantity {
		t.Helper()
		req := &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: calc}
		for _, arg := range args {
			req.Arguments = append(req.Arguments, &pb.Value{Kind: &pb.Value_Quantity{Quantity: arg}})
		}
		resp, err := srv.EvaluateCalc(context.Background(), req)
		if err != nil {
			t.Fatalf("EvaluateCalc %s: %v", calc, err)
		}
		if resp.Error != "" {
			t.Fatalf("EvaluateCalc %s reported %q", calc, resp.Error)
		}
		if resp.Result.GetQuantity() == nil {
			t.Fatalf("EvaluateCalc %s = %v, want a quantity", calc, resp.Result)
		}
		return resp.Result.GetQuantity()
	}
	quantity := func(magnitude float64, unit string, term *pb.UnitTerm) *pb.Quantity {
		return &pb.Quantity{Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: magnitude}, Unit: unit, UnitTerm: term}
	}
	sent := func(unit string, term *pb.UnitTerm) *pb.Quantity { return quantity(6, unit, term) }
	second := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")
	metre := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::m]")
	speedTerm := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::m] / 1.0 [SI::s]").GetUnitTerm()
	kilometreTerm := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]").GetUnitTerm()
	fathomTerm := evaluate("Nautical::Fathom").GetUnitTerm()
	nauticalFathom := quantity(1, "Nautical::fathom", fathomTerm)

	for _, tc := range []struct {
		name                        string
		opaque, known               *pb.Quantity
		composed, divided, restored string
	}{
		{
			"text that is no unit expression",
			sent("metres per second", speedTerm), second,
			"6 ['metres per second'*SI::s] = SI::metre",
			"6 ['metres per second'] = SI::metre·SI::second^-1",
			"1 [SI::s] = SI::second",
		},
		{
			"a short name no unit bears",
			sent("smoot", metre.GetUnitTerm()), metre,
			"6 [SI::m*smoot] = SI::metre^2",
			"6 [smoot] = SI::metre",
			"1 [SI::m] = SI::metre",
		},
		{
			"a qualified name no unit bears",
			sent("Imperial::smoot", metre.GetUnitTerm()), metre,
			"6 [Imperial::smoot*SI::m] = SI::metre^2",
			"6 [Imperial::smoot] = SI::metre",
			"1 [SI::m] = SI::metre",
		},
		{
			"a short name two units bear",
			sent("fathom", fathomTerm), nauticalFathom,
			"6 [Nautical::fathom*fathom] = 3.34450944/1·SI::metre^2",
			"6 [fathom] = 3.34450944/1.8288·SI::metre",
			"1 [Nautical::fathom] = 3.34450944/1.8288·SI::metre",
		},
		{
			"a short name spelling a keyword",
			sent("then", metre.GetUnitTerm()), metre,
			"6 ['then'*SI::m] = SI::metre^2",
			"6 ['then'] = SI::metre",
			"1 [SI::m] = SI::metre",
		},
		{
			"a qualified name whose segment spells a keyword",
			sent("SI::then", metre.GetUnitTerm()), metre,
			"6 [SI::'then'*SI::m] = SI::metre^2",
			"6 [SI::'then'] = SI::metre",
			"1 [SI::m] = SI::metre",
		},
		{
			"a qualified name led by a keyword, which is no expression",
			sent("in::m", metre.GetUnitTerm()), metre,
			"6 ['in::m'*SI::m] = SI::metre^2",
			"6 ['in::m'] = SI::metre",
			"1 [SI::m] = SI::metre",
		},
		{
			"a scaled reduction sent nameless",
			sent("", kilometreTerm), metre,
			"6 ['1000·metre'*SI::m] = 1000/1·SI::metre^2",
			"6 ['1000·metre'] = 1000/1·SI::metre",
			"1 [SI::m] = SI::metre",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			composed := evaluate("Q::Times", tc.known, tc.opaque)
			if got := describeQuantity(composed); got != tc.composed {
				t.Fatalf("known * opaque = %s, want %s", got, tc.composed)
			}
			if got := describeQuantity(evaluate("Q::Per", composed, tc.known)); got != tc.divided {
				t.Errorf("(known * opaque) over the wire / known = %s, want %s", got, tc.divided)
			}
			// The opaque factor is one unit under one spelling: sent again, it cancels itself.
			if got := describeQuantity(evaluate("Q::Per", composed, tc.opaque)); got != tc.restored {
				t.Errorf("(known * opaque) over the wire / opaque = %s, want %s", got, tc.restored)
			}
		})
	}

	// Opaque text holding what a quoted name must escape is spelt as one name the
	// notation reads back, so the declared factor beside it still cancels alone.
	for _, text := range []string{"it's", `back\slash`, "metres\nper second", "metres\r\nper second", `'A/m'*m`} {
		t.Run(fmt.Sprintf("opaque %q", text), func(t *testing.T) {
			opaque := sent(text, speedTerm)
			spelt := lexer.UnrestrictedNameText(text)
			composed := evaluate("Q::Times", second, opaque)
			if got := describeQuantity(composed); got != fmt.Sprintf("6 [%s*SI::s] = SI::metre", spelt) {
				t.Fatalf("SI::s * %q = %s, want 6 [%s*SI::s] = SI::metre", text, got, spelt)
			}
			if got := describeQuantity(evaluate("Q::Per", composed, second)); got != fmt.Sprintf("6 [%s] = SI::metre·SI::second^-1", spelt) {
				t.Errorf("(SI::s * %q) over the wire / SI::s = %s, want 6 [%s] = SI::metre·SI::second^-1", text, got, spelt)
			}
			if got := describeQuantity(evaluate("Q::Per", composed, opaque)); got != "1 [SI::s] = SI::second" {
				t.Errorf("(SI::s * %q) over the wire / %q = %s, want 1 [SI::s] = SI::second", text, text, got)
			}
		})
	}

	// One text sent as two units, reducing to a metre and to a second, is two units:
	// they neither merge nor cancel, and what they compose reads back as it was sent.
	t.Run("one text, two reductions", func(t *testing.T) {
		metres, seconds := sent("smoot", metre.GetUnitTerm()), sent("smoot", second.GetUnitTerm())
		product := evaluate("Q::Times", metres, seconds)
		if got := describeQuantity(product); got != "36 [smoot*smoot] = SI::metre·SI::second" {
			t.Fatalf("smoot (m) * smoot (s) = %s, want 36 [smoot*smoot] = SI::metre·SI::second", got)
		}
		if got := describeQuantity(evaluate("Q::Per", product, second)); got != "36 ['smoot*smoot'/SI::s] = SI::metre" {
			t.Errorf("(smoot*smoot) over the wire / SI::s = %s, want 36 ['smoot*smoot'/SI::s] = SI::metre", got)
		}
		quotient := evaluate("Q::Per", metres, seconds)
		if got := describeQuantity(quotient); got != "1 [smoot/smoot] = SI::metre·SI::second^-1" {
			t.Fatalf("smoot (m) / smoot (s) = %s, want 1 [smoot/smoot] = SI::metre·SI::second^-1", got)
		}
		if got := describeQuantity(evaluate("Q::Times", quotient, second)); got != "1 ['smoot/smoot'*SI::s] = SI::metre" {
			t.Errorf("(smoot/smoot) over the wire * SI::s = %s, want 1 ['smoot/smoot'*SI::s] = SI::metre", got)
		}
	})

	// A text every name of which reads as a unit, yet contradicting the reduction
	// sent, has no factor to blame: it stays one opaque unit through the wire.
	contradiction := evaluate("Q::Times", sent("km", metre.GetUnitTerm()), mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]"))
	if got := describeQuantity(contradiction); got != "6 [SI::km*km] = 1000/1·SI::metre^2" {
		t.Fatalf("SI::km * a km reducing to a metre = %s, want 6 [SI::km*km] = 1000/1·SI::metre^2", got)
	}
	if got := describeQuantity(evaluate("Q::Per", contradiction, mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]"))); got != "6 ['SI::km*km'/SI::km] = SI::metre" {
		t.Errorf("(SI::km * opaque km) over the wire / SI::km = %s, want 6 ['SI::km*km'/SI::km] = SI::metre", got)
	}
}

// TestQuantityFromWireRejectsUnitTextItsReductionContradicts pins that a unit
// written as one thing but reduced to another is rejected rather than read as
// the text for display and the reduction for arithmetic.
func TestQuantityFromWireRejectsUnitTextItsReductionContradicts(t *testing.T) {
	srv, hash, idx, sem := mustQuantityModel(t)
	seconds := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]").GetUnitTerm()
	kilograms := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::kg]").GetUnitTerm()
	metres := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::m]").GetUnitTerm()

	for _, tc := range []struct {
		name string
		unit string
		term *pb.UnitTerm
	}{
		{"another dimension", "SI::m", seconds},
		{"a composed unit over another dimension", "SI::m/SI::s", kilograms},
		{"another scale of the same dimension", "SI::km", metres},
		{"a scale off by more than rounding", "SI::m", &pb.UnitTerm{ScaleNum: 1 + 1e-10, ScaleDen: 1, Factors: metres.GetFactors()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pq := &pb.Quantity{
				Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
				Unit:      tc.unit,
				UnitTerm:  tc.term,
			}
			if _, err := ProtoToQuantity(pq, idx, sem); !errors.Is(err, ErrUnitTextMismatch) {
				t.Errorf("ProtoToQuantity(%s over %s) err = %v, want ErrUnitTextMismatch",
					tc.unit, describeUnitTerm(tc.term), err)
			}
		})
	}

	// The same text over the reduction it does have is read, in either factor order.
	agreeing := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "SI::m/SI::s",
		UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::second", Exponent: -1},
			{UnitId: "SI::metre", Exponent: 1},
		}},
	}
	if _, err := ProtoToQuantity(agreeing, idx, sem); err != nil {
		t.Errorf("ProtoToQuantity(SI::m/SI::s over metre·second^-1): %v", err)
	}

	// A scale off by the rounding of composing it another way is the same scale,
	// and the quantity read carries the model's own reduction, not the noisy one.
	kilometres := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]").GetUnitTerm()
	noisy := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "SI::km",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1000 * (1 + 0x1p-52), ScaleDen: 1, Factors: kilometres.GetFactors()},
	}
	val, err := ProtoToQuantity(noisy, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity(SI::km over a scale one ulp off): %v", err)
	}
	if got := val.Quantity().Unit.Term.Scale; got != semantics.UnitScale(1000) {
		t.Errorf("SI::km read over a scale one ulp off keeps scale %v, want the model's 1000", got)
	}
}

// TestQuantityAsAnActionInput drives ExecuteAction with a quantity input: it is
// decoded against the model, and one that cannot be read is reported by name.
func TestQuantityAsAnActionInput(t *testing.T) {
	srv := mustNewService(t, 4)
	content := `
package A {
	private import ScalarValues::*;

	action heavier {
		attribute mass : ISQ::MassValue = 1.0 [SI::kg];
		first start;
		action inner {
			assign mass := mass + 1.0 [SI::kg];
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "quantity-action-input",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	sent := mustEvaluateQuantity(t, srv, parseResp.ModelHash, "5.0 [SI::kg]")
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "A::heavier",
		Inputs:         map[string]*pb.Value{"mass": {Kind: &pb.Value_Quantity{Quantity: sent}}},
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteAction: %s", resp.Error)
	}
	want := "6 [SI::kg] = 1000/1·SI::gram"
	if got := describeQuantity(resp.Outputs["mass"].GetQuantity()); got != want {
		t.Errorf("output mass = %q, want %q", got, want)
	}

	unreadable := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "Made::up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "Made::up", Exponent: 1}}},
	}
	bad, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "A::heavier",
		Inputs:         map[string]*pb.Value{"mass": {Kind: &pb.Value_Quantity{Quantity: unreadable}}},
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if !strings.Contains(bad.Error, "mass") || !strings.Contains(bad.Error, "unknown base unit") {
		t.Errorf("error = %q, want it to name the input and the unknown base unit", bad.Error)
	}
}

// TestQuantityInVerdict drives a constraint over quantities, whose verdict path
// reports the values it compared.
func TestQuantityInVerdict(t *testing.T) {
	srv := mustNewService(t, 4)
	content := `
package V {
	private import ScalarValues::*;

	part def Car {
		attribute mass : ISQ::MassValue = 2500.0 [SI::kg];

		constraint withinLimit {
			mass <= 3000.0 [SI::kg]
		}
	}
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "quantity-verdict",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       parseResp.ModelHash,
		SymbolId:        "V::Car::withinLimit",
		SubjectSymbolId: "V::Car",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint: %s", resp.Error)
	}
	if resp.Verdict == nil || !resp.Verdict.Holds {
		t.Fatalf("verdict = %v, want one that holds", resp.Verdict)
	}

	// The subject's quantity feature value reads back from the verdict, which is what makes
	// a verdict over quantities diagnosable from a client.
	var subject *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == resp.Verdict.InstanceId {
			subject = inst
		}
	}
	if subject == nil {
		t.Fatalf("verdict instance %d is not in the response", resp.Verdict.InstanceId)
	}
	wantMass := "2500 [SI::kg] = 1000/1·SI::gram"
	if got := describeQuantity(subject.FeatureValues["mass"].GetValue().GetQuantity()); got != wantMass {
		t.Errorf("subject mass = %q, want %q", got, wantMass)
	}
}
