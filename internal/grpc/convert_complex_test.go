package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
)

const complexWireModel = `
package C {
  private import ScalarValues::*;
  private import ComplexFunctions::*;
  part def Signal {
    attribute z : Complex = rect(1.5, -2.0);
    attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
  }
  action conj {
    in z : Complex;
    out w : Complex;
    first start;
    action inner { assign w := rect(re(z), -im(z)); }
    done;
    succession first start then inner;
    succession first inner then done;
  }
  calc def Echo { in z : Complex; return : Complex = z; }
  calc echo : Echo;
}
`

// A Complex crosses as the one Complex arm, both parts intact, never as a pair
// of Reals a client would read as two values.
func TestComplexToProto(t *testing.T) {
	idx := symbols.NewIndex()
	pv := protoconv.ValueToProto(runtime.NewComplex(complex(1.5, -2)), idx)
	c := pv.GetComplex()
	if c == nil {
		t.Fatalf("got kind %T, want complex", pv.GetKind())
	}
	if c.Real != 1.5 || c.Imaginary != -2 {
		t.Errorf("complex = %v, want 1.5 - 2.0i", c)
	}

	seq := runtime.NewSequence()
	seq.Append(runtime.NewComplex(complex(1, 2)))
	seq.Append(runtime.NewComplex(complex(3, 4)))
	pv = protoconv.ValueToProto(runtime.NewSequenceValue(seq), idx)
	elements := pv.GetSequence().GetElements()
	if len(elements) != 2 {
		t.Fatalf("a sequence of two Complex values crossed as %d elements", len(elements))
	}
	for i, want := range []complex128{complex(1, 2), complex(3, 4)} {
		if got := protoconv.ProtoToComplex(elements[i].GetComplex()); got != want {
			t.Errorf("element %d = %v, want %v", i, got, want)
		}
	}
}

// The wire form decodes to the same runtime value, so a Complex a client sends
// back is the Complex it received.
func TestComplexRoundTrip(t *testing.T) {
	idx := symbols.NewIndex()
	for _, z := range []complex128{complex(0, 0), complex(1.5, -2), complex(-0.25, 1e300), complex(3, 0)} {
		back, err := protoconv.ProtoToValueIn(protoconv.ValueToProto(runtime.NewComplex(z), idx), idx, nil)
		if err != nil {
			t.Fatalf("protoconv.ProtoToValueIn(%v): %v", z, err)
		}
		if back.Kind != runtime.ValComplex || back.Complex() != z {
			t.Errorf("round trip of %v = %s (%v)", z, back.Kind, back)
		}
	}

	empty, err := protoconv.ProtoToValueIn(&pb.Value{Kind: &pb.Value_Complex{Complex: &pb.Complex{}}}, idx, nil)
	if err != nil {
		t.Fatalf("protoconv.ProtoToValueIn(empty Complex): %v", err)
	}
	if empty.Kind != runtime.ValComplex || empty.Complex() != 0 {
		t.Errorf("empty Complex message = %v, want 0 + 0i", empty)
	}
}

// The Complex arm is advertised, so a client can require it, and a service
// withholding it names the value as unsupported rather than splitting it.
func TestComplexValuesCapability(t *testing.T) {
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityComplexValues
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityComplexValues)
	}

	srv := mustNewServiceWithout(t, CapabilityComplexValues)
	pv := &pb.Value{Kind: &pb.Value_Complex{Complex: protoconv.ComplexToProto(complex(0, 1))}}
	srv.filterValueCapabilities(pv)
	if want := "unsupported: complex number 0.0 + 1.0i"; pv.GetNull() != want {
		t.Errorf("withheld complex = %v, want null %q", pv, want)
	}
}

// Evaluate, Instantiate and ExecuteAction all carry a Complex as itself, in
// both directions, including inside a sequence.
func TestComplexCrossesEveryValueSurface(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: complexWireModel},
		ContentHash: "complex-wire",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parsed.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	eval, err := srv.Evaluate(ctx, &pb.EvaluateRequest{ModelHash: parsed.ModelHash, Expression: "ComplexFunctions::rect(1.5, -2.0)"})
	if err != nil || eval.Error != "" {
		t.Fatalf("Evaluate: err = %v, error = %q", err, eval.GetError())
	}
	if got := protoconv.ProtoToComplex(eval.Result.GetComplex()); eval.Result.GetComplex() == nil || got != complex(1.5, -2) {
		t.Errorf("Evaluate result = %v, want complex 1.5 - 2.0i", eval.Result)
	}

	inst, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: parsed.ModelHash, SymbolId: "C::Signal"})
	if err != nil || inst.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, inst.GetError())
	}
	z := inst.Instance.FeatureValues["z"]
	if z == nil || z.Error != "" || z.Value.GetComplex() == nil || protoconv.ProtoToComplex(z.Value.GetComplex()) != complex(1.5, -2) {
		t.Errorf("feature value z = %v, want complex 1.5 - 2.0i", z)
	}
	zs := inst.Instance.FeatureValues["zs"]
	if zs == nil || zs.Error != "" || len(zs.Values) != 2 {
		t.Fatalf("feature value zs = %v, want two values", zs)
	}
	for i, want := range []complex128{complex(1, 2), complex(3, 4)} {
		if zs.Values[i].GetComplex() == nil || protoconv.ProtoToComplex(zs.Values[i].GetComplex()) != want {
			t.Errorf("zs[%d] = %v, want complex %v", i, zs.Values[i], want)
		}
	}

	act, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      parsed.ModelHash,
		ActionSymbolId: "C::conj",
		Inputs:         map[string]*pb.Value{"z": {Kind: &pb.Value_Complex{Complex: protoconv.ComplexToProto(complex(2, 5))}}},
	})
	if err != nil || act.Error != "" {
		t.Fatalf("ExecuteAction: err = %v, error = %q", err, act.GetError())
	}
	if w := act.Outputs["w"]; w.GetComplex() == nil || protoconv.ProtoToComplex(w.GetComplex()) != complex(2, -5) {
		t.Errorf("output w = %v, want complex 2.0 - 5.0i", w)
	}

	withheld := mustNewServiceWithout(t, CapabilityComplexValues)
	parsedW, err := withheld.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: complexWireModel}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	evalW, err := withheld.Evaluate(ctx, &pb.EvaluateRequest{ModelHash: parsedW.ModelHash, Expression: "ComplexFunctions::rect(1.5, -2.0)"})
	if err != nil || evalW.Error != "" {
		t.Fatalf("Evaluate without complex_values: err = %v, error = %q", err, evalW.GetError())
	}
	if !strings.Contains(evalW.Result.GetNull(), "complex number 1.5 - 2.0i") {
		t.Errorf("Evaluate without complex_values = %v, want an unsupported null naming the value", evalW.Result)
	}
}

func TestValueCarriesComplex(t *testing.T) {
	z := &pb.Value{Kind: &pb.Value_Complex{Complex: protoconv.ComplexToProto(complex(1, 2))}}
	one := &pb.Value{Kind: &pb.Value_IntValue{IntValue: 1}}
	sequence := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	for _, testcase := range []struct {
		name  string
		value *pb.Value
		want  bool
	}{
		{"nil", nil, false},
		{"int", one, false},
		{"complex", z, true},
		{"sequence of ints", sequence(one, one), false},
		{"sequence with a complex", sequence(one, z), true},
		{"nested sequence with a complex", sequence(one, sequence(sequence(z))), true},
		{"empty sequence", sequence(), false},
	} {
		if got := protoconv.ValueCarriesComplex(testcase.value); got != testcase.want {
			t.Errorf("protoconv.ValueCarriesComplex(%s) = %v, want %v", testcase.name, got, testcase.want)
		}
	}
}

// A service without complex_values refuses a Complex input outright, however
// deeply a sequence nests it, rather than reading it as another value.
func TestComplexInputNeedsComplexValues(t *testing.T) {
	ctx := context.Background()
	withheld := mustNewServiceWithout(t, CapabilityComplexValues)
	parsed, err := withheld.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: complexWireModel}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	z := &pb.Value{Kind: &pb.Value_Complex{Complex: protoconv.ComplexToProto(complex(2, 5))}}
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{
		{Kind: &pb.Value_IntValue{IntValue: 1}},
		{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{z}}}},
	}}}}
	for name, input := range map[string]*pb.Value{"complex": z, "nested": nested} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      parsed.ModelHash,
			ActionSymbolId: "C::conj",
			Inputs:         map[string]*pb.Value{"z": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityComplexValues) {
			t.Errorf("ExecuteAction with %s input without complex_values: err = %v, want UNIMPLEMENTED naming the capability", name, err)
		}
		_, err = withheld.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{
			ModelHash: parsed.ModelHash,
			SymbolId:  "C::echo",
			Arguments: []*pb.Value{input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityComplexValues) {
			t.Errorf("EvaluateCalc with %s argument without complex_values: err = %v, want UNIMPLEMENTED naming the capability", name, err)
		}
	}

	served := mustNewService(t, 4)
	parsedS, err := served.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: complexWireModel}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	calc, err := served.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: parsedS.ModelHash, SymbolId: "C::echo", Arguments: []*pb.Value{z}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc: err = %v, error = %q", err, calc.GetError())
	}
	if calc.Result.GetComplex() == nil || protoconv.ProtoToComplex(calc.Result.GetComplex()) != complex(2, 5) {
		t.Errorf("echo(2 + 5i) = %v, want the complex back", calc.Result)
	}
}
