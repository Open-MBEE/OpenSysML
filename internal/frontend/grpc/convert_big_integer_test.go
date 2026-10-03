package grpc

import (
	"context"
	"errors"
	"math/big"
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

const bigIntegerWireModel = `
package B {
  private import ScalarValues::*;
  calc def Square { in x : Integer; return : Integer = x * x; }
  calc square : Square;
}
`

func bigConst(t *testing.T, digits string) runtime.Value {
	t.Helper()
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		t.Fatalf("bad digits %q", digits)
	}
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.BigIntValue(n)}
}

// An Integer that fits int64 keeps the int_value arm; one beyond it crosses as
// its decimal in big_int_value, and both read back as the Integer sent.
func TestBigIntegerValueRoundTripsOverTheWire(t *testing.T) {
	idx := symbols.NewIndex()
	for _, tc := range []struct {
		name  string
		value runtime.Value
		big   string
	}{
		{name: "int64 max stays int_value", value: intConst(9223372036854775807)},
		{name: "int64 min stays int_value", value: intConst(-9223372036854775808)},
		{name: "one past int64 max", value: bigConst(t, "9223372036854775808"), big: "9223372036854775808"},
		{name: "one past int64 min", value: bigConst(t, "-9223372036854775809"), big: "-9223372036854775809"},
		{name: "2**70", value: bigConst(t, "1180591620717411303424"), big: "1180591620717411303424"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sent := protoconv.ValueToProto(tc.value, idx)
			if tc.big == "" {
				if _, ok := sent.GetKind().(*pb.Value_IntValue); !ok {
					t.Fatalf("crossed as %T, want int_value", sent.GetKind())
				}
			} else if got := sent.GetBigIntValue(); got != tc.big {
				t.Fatalf("big_int_value = %q (kind %T), want %q", got, sent.GetKind(), tc.big)
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
			if back.Kind != runtime.ValConst || !back.Const.Equal(tc.value.Const) {
				t.Errorf("read back %s, want %s", runtime.FormatValue(back), runtime.FormatValue(tc.value))
			}
		})
	}
}

// A quantity whose Integer magnitude is beyond int64 crosses in
// big_int_magnitude and keeps its unit.
func TestBigIntegerQuantityMagnitudeCrossesAsDecimal(t *testing.T) {
	sent := protoconv.QuantityToProto(&runtime.Quantity{Num: bigConst(t, "-1180591620717411303424").Const})
	if got := sent.GetBigIntMagnitude(); got != "-1180591620717411303424" {
		t.Fatalf("big_int_magnitude = %q (magnitude %T)", got, sent.GetMagnitude())
	}
	fits := protoconv.QuantityToProto(&runtime.Quantity{Num: semantics.IntValue(7)})
	if _, ok := fits.GetMagnitude().(*pb.Quantity_IntMagnitude); !ok {
		t.Errorf("a fitting magnitude crossed as %T, want int_magnitude", fits.GetMagnitude())
	}
}

// big_int_value carries decimal digits with an optional sign and nothing else;
// a spelling that fits int64 reads as the int64 Integer it equals.
func TestBigIntegerValueReadsOnlyDecimal(t *testing.T) {
	idx := symbols.NewIndex()
	for _, text := range []string{"", "-", "0x10", "1e30", "12 34", "+-1", "١٢"} {
		_, err := protoconv.ProtoToValueIn(&pb.Value{Kind: &pb.Value_BigIntValue{BigIntValue: text}}, idx, nil)
		if !errors.Is(err, protoconv.ErrBigIntegerNotDecimal) {
			t.Errorf("big_int_value %q: err = %v, want %v", text, err, protoconv.ErrBigIntegerNotDecimal)
		}
	}
	back, err := protoconv.ProtoToValueIn(&pb.Value{Kind: &pb.Value_BigIntValue{BigIntValue: "42"}}, idx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, fits := back.Const.Int64(); !fits || n != 42 || back.Const.IsBigInt() {
		t.Errorf("big_int_value \"42\" read as %s, want the int64 Integer 42", runtime.FormatValue(back))
	}
}

// A calc invoked over gRPC takes and returns Integers beyond int64 exactly.
func TestEvaluateCalcTakesAndReturnsBigIntegers(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, bigIntegerWireModel)
	for _, tc := range []struct {
		arg  *pb.Value
		want string
	}{
		{arg: intValue(4294967296), want: "18446744073709551616"},
		{arg: &pb.Value{Kind: &pb.Value_BigIntValue{BigIntValue: "-9223372036854775809"}}, want: "85070591730234615884290395931651604481"},
	} {
		resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
			ModelHash: hash,
			SymbolId:  "B::square",
			Arguments: []*pb.Value{tc.arg},
		})
		if err != nil {
			t.Fatalf("EvaluateCalc: %v", err)
		}
		if resp.Error != "" {
			t.Fatalf("EvaluateCalc reported %q", resp.Error)
		}
		if resp.Result.GetBigIntValue() != tc.want {
			t.Errorf("square(%v) = %v, want big_int_value %s", tc.arg, resp.Result, tc.want)
		}
	}
}

const bigIntegerCapabilityModel = `
package W {
  private import ScalarValues::*;

  attribute wide = 2 ** 70;
  attribute narrow = 2 ** 62;
  attribute both [*] = (1, 2 ** 70);

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

// The arms are advertised under their own capability, so a client can require
// it; a service withholding it reports a wide Integer as unsupported — nested in
// a sequence and as a quantity magnitude included — and refuses one sent to it
// rather than reading it as null. An Integer within int64 is unaffected.
func TestBigIntCapability(t *testing.T) {
	ctx := context.Background()
	if all := Capabilities(); all[len(all)-2] != CapabilityBigIntValues || all[len(all)-1] != CapabilityRationalValues {
		t.Errorf("capabilities %v do not end with %q then %q, the newest", all, CapabilityBigIntValues, CapabilityRationalValues)
	}

	withheld := mustNewServiceWithout(t, CapabilityBigIntValues)
	modelHash := mustParse(t, withheld, bigIntegerCapabilityModel)

	const want = "unsupported: constant 1180591620717411303424"
	if got := mustEvaluate(t, withheld, modelHash, "W::wide"); got.GetNull() != want {
		t.Errorf("W::wide without %s = %v, want null %q", CapabilityBigIntValues, got, want)
	}
	if got := mustEvaluate(t, withheld, modelHash, "W::narrow"); got.GetIntValue() != 1<<62 {
		t.Errorf("W::narrow without %s = %v, want int_value 2**62", CapabilityBigIntValues, got)
	}
	both := mustEvaluate(t, withheld, modelHash, "W::both").GetSequence().GetElements()
	if len(both) != 2 || both[0].GetIntValue() != 1 || both[1].GetNull() != want {
		t.Errorf("W::both without %s = %v, want (1, withheld)", CapabilityBigIntValues, both)
	}

	quantity := &pb.Value{Kind: &pb.Value_Quantity{Quantity: &pb.Quantity{
		Magnitude: &pb.Quantity_BigIntMagnitude{BigIntMagnitude: "1180591620717411303424"},
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1},
	}}}
	if !protoconv.ValueHoldsBigInt(quantity) {
		t.Errorf("a quantity with a big_int_magnitude does not hold a wide Integer")
	}
	filtered := proto.Clone(quantity).(*pb.Value)
	withheld.filterValueCapabilities(filtered)
	if _, ok := protoconv.UnsupportedReason(filtered); !ok {
		t.Errorf("quantity without %s = %v, want it reported unsupported", CapabilityBigIntValues, filtered)
	}

	wide := &pb.Value{Kind: &pb.Value_BigIntValue{BigIntValue: "1180591620717411303424"}}
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{wide}}}}
	vector := &pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{wide}}}}
	for name, input := range map[string]*pb.Value{"wide": wide, "nested": nested, "vector": vector, "quantity": quantity} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "W::pass",
			Inputs:         map[string]*pb.Value{"x": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityBigIntValues) {
			t.Errorf("ExecuteAction with %s input without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityBigIntValues, err)
		}
	}

	available := mustNewService(t, 10)
	t.Cleanup(available.Close)
	availableHash := mustParse(t, available, bigIntegerCapabilityModel)
	if got := mustEvaluate(t, available, availableHash, "W::wide"); got.GetBigIntValue() != "1180591620717411303424" {
		t.Errorf("W::wide = %v, want big_int_value 1180591620717411303424", got)
	}
}
