package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func ratConst(t *testing.T, text string) runtime.Value {
	t.Helper()
	v, err := semantics.ParseRationalText(text, semantics.DefaultMaxIntegerBits)
	if err != nil {
		t.Fatalf("bad rational %q: %v", text, err)
	}
	return runtime.Value{Kind: runtime.ValConst, Const: v}
}

// A Rational a double holds exactly keeps the real_value arm; any other crosses
// as its lowest terms in rational_value, and both read back as the Rational sent.
func TestRationalValueRoundTripsOverTheWire(t *testing.T) {
	idx := symbols.NewIndex()
	for _, tc := range []struct {
		text     string
		num, den string
	}{
		{text: "0.5"}, {text: "1800"}, {text: "-0.375"},
		{text: "1/3", num: "1", den: "3"},
		{text: "0.1", num: "1", den: "10"},
		{text: "-25/12", num: "-25", den: "12"},
		{text: "1e400", num: "1" + strings.Repeat("0", 400), den: "1"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			value := ratConst(t, tc.text)
			sent := protoconv.ValueToProto(value, idx)
			if tc.num == "" {
				if _, ok := sent.GetKind().(*pb.Value_RealValue); !ok {
					t.Fatalf("crossed as %T, want real_value", sent.GetKind())
				}
			} else if r := sent.GetRationalValue(); r.GetNumerator() != tc.num || r.GetDenominator() != tc.den {
				t.Fatalf("rational_value = %v (kind %T), want %s/%s", r, sent.GetKind(), tc.num, tc.den)
			}
			wire, err := proto.Marshal(sent)
			if err != nil {
				t.Fatal(err)
			}
			var received pb.Value
			if err := proto.Unmarshal(wire, &received); err != nil {
				t.Fatal(err)
			}
			back, err := protoconv.ProtoToValueIn(&received, idx, nil)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if tc.num == "" {
				if f, _ := value.Const.BinaryExact(); back.Const.Kind != semantics.ValReal || back.Const.Real != f {
					t.Errorf("read back %s, want the Real %v", runtime.FormatValue(back), f)
				}
			} else if back.Const.Kind != semantics.ValRational || semantics.CompareRat(back.Const, value.Const) != 0 {
				t.Errorf("read back %s, want %s", runtime.FormatValue(back), runtime.FormatValue(value))
			}
		})
	}
}

// A quantity's Rational magnitude crosses in rational_magnitude unless a double
// holds it exactly.
func TestRationalQuantityMagnitudeCrossesExactly(t *testing.T) {
	sent := protoconv.QuantityToProto(&runtime.Quantity{Num: ratConst(t, "1/3").Const})
	if r := sent.GetRationalMagnitude(); r.GetNumerator() != "1" || r.GetDenominator() != "3" {
		t.Fatalf("rational_magnitude = %v (magnitude %T)", r, sent.GetMagnitude())
	}
	exact := protoconv.QuantityToProto(&runtime.Quantity{Num: ratConst(t, "0.25").Const})
	if m, ok := exact.GetMagnitude().(*pb.Quantity_RealMagnitude); !ok || m.RealMagnitude != 0.25 {
		t.Errorf("a binary64 magnitude crossed as %v, want real_magnitude 0.25", exact.GetMagnitude())
	}
}

// Each Rational has one encoding: a fraction not in lowest terms, over a
// non-positive denominator, or one a double holds exactly is refused.
func TestRationalValueReadsOnlyCanonicalTerms(t *testing.T) {
	idx := symbols.NewIndex()
	for _, terms := range [][2]string{{"2", "6"}, {"1", "-3"}, {"-1", "-3"}, {"1", "0"}, {"1", "2"}, {"3", "1"}, {"0", "1"}, {"1.5", "7"}, {"", "3"}, {"1", ""}} {
		pv := &pb.Value{Kind: &pb.Value_RationalValue{RationalValue: &pb.Rational{Numerator: terms[0], Denominator: terms[1]}}}
		if _, err := protoconv.ProtoToValueIn(pv, idx, nil); !errors.Is(err, protoconv.ErrRationalNotCanonical) {
			t.Errorf("rational_value %s/%s: err = %v, want %v", terms[0], terms[1], err, protoconv.ErrRationalNotCanonical)
		}
	}
}

const rationalWireModel = `
package R {
  private import ScalarValues::*;
  calc def Third { in x : Rational; return : Rational = x / 3; }
  calc third : Third;

  attribute oneThird = 1 / 3;
  attribute half = 1 / 2;
  attribute both [*] = (1, 1 / 3);

  action pass {
    in x;
    out y;
    first start;
    action inner { assign y := x; }
    then done;
    succession first start then inner;
  }
}
`

// A calc invoked over gRPC takes and returns exact Rationals.
func TestEvaluateCalcTakesAndReturnsRationals(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, rationalWireModel)
	for _, tc := range []struct {
		arg      *pb.Value
		num, den string
	}{
		{arg: &pb.Value{Kind: &pb.Value_RationalValue{RationalValue: &pb.Rational{Numerator: "1", Denominator: "10"}}}, num: "1", den: "30"},
		{arg: intValue(1), num: "1", den: "3"},
	} {
		resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
			ModelHash: hash,
			SymbolId:  "R::third",
			Arguments: []*pb.Value{tc.arg},
		})
		if err != nil {
			t.Fatalf("EvaluateCalc: %v", err)
		}
		if resp.Error != "" {
			t.Fatalf("EvaluateCalc reported %q", resp.Error)
		}
		if r := resp.Result.GetRationalValue(); r.GetNumerator() != tc.num || r.GetDenominator() != tc.den {
			t.Errorf("third(%v) = %v, want rational_value %s/%s", tc.arg, resp.Result, tc.num, tc.den)
		}
	}
}

// A service withholding rational_values reports an exact Rational as
// unsupported — nested in a sequence included — and refuses one sent to it, as
// big_int_values does; a Rational a double holds is unaffected.
func TestRationalCapability(t *testing.T) {
	ctx := context.Background()
	withheld := mustNewServiceWithout(t, CapabilityRationalValues)
	modelHash := mustParse(t, withheld, rationalWireModel)

	const want = "unsupported: constant 1/3"
	if got := mustEvaluate(t, withheld, modelHash, "R::oneThird"); got.GetNull() != want {
		t.Errorf("R::oneThird without %s = %v, want null %q", CapabilityRationalValues, got, want)
	}
	if got := mustEvaluate(t, withheld, modelHash, "R::half"); got.GetRealValue() != 0.5 {
		t.Errorf("R::half without %s = %v, want real_value 0.5", CapabilityRationalValues, got)
	}
	both := mustEvaluate(t, withheld, modelHash, "R::both").GetSequence().GetElements()
	if len(both) != 2 || both[0].GetIntValue() != 1 || both[1].GetNull() != want {
		t.Errorf("R::both without %s = %v, want (1, withheld)", CapabilityRationalValues, both)
	}

	third := &pb.Value{Kind: &pb.Value_RationalValue{RationalValue: &pb.Rational{Numerator: "1", Denominator: "3"}}}
	quantity := &pb.Value{Kind: &pb.Value_Quantity{Quantity: &pb.Quantity{
		Magnitude: &pb.Quantity_RationalMagnitude{RationalMagnitude: &pb.Rational{Numerator: "1", Denominator: "3"}},
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1},
	}}}
	filtered := proto.Clone(quantity).(*pb.Value)
	withheld.filterValueCapabilities(filtered)
	if _, ok := protoconv.UnsupportedReason(filtered); !ok {
		t.Errorf("quantity without %s = %v, want it reported unsupported", CapabilityRationalValues, filtered)
	}
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{third}}}}
	vector := &pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{third}}}}
	for name, input := range map[string]*pb.Value{"rational": third, "nested": nested, "vector": vector, "quantity": quantity} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "R::pass",
			Inputs:         map[string]*pb.Value{"x": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityRationalValues) {
			t.Errorf("ExecuteAction with %s input without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityRationalValues, err)
		}
	}

	available := mustNewService(t, 10)
	t.Cleanup(available.Close)
	availableHash := mustParse(t, available, rationalWireModel)
	if r := mustEvaluate(t, available, availableHash, "R::oneThird").GetRationalValue(); r.GetNumerator() != "1" || r.GetDenominator() != "3" {
		t.Errorf("R::oneThird = %v, want rational_value 1/3", r)
	}
}
