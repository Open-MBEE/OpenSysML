package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// functionWireModel yields calcs as values — a definition, a usage with an
// unsupplied input, one read off a part, and one closing over a body — and
// takes one back as a calc argument and an action input that apply it.
const functionWireModel = `
package F {
  private import ScalarValues::*;

  calc def Unary { in v : Real; return : Real; }
  calc def Sq :> Unary { in :>> v; return : Real = v * v; }
  calc def Cube :> Unary { in :>> v; return : Real = v * v * v; }
  calc def Apply { in calc f : Unary; in a : Real; return : Real = f(a); }
  calc apply : Apply;
  calc sq : Sq;
  calc def PickSq { return : Unary = sq; }
  calc pickSq : PickSq;
  attribute fns = (Sq, Cube);

  part def Holder {
    attribute k : Real = 2.0;
    calc scale { in x : Real; return : Real = x * k; }
  }
  part holder : Holder;

  calc def Outer {
    in k : Real;
    calc inner :> Unary { in :>> v; return : Real = v * k; }
    return : Unary = inner;
  }
  calc outer : Outer;

  action run {
    in calc f { in v : Real; return : Real; }
    in a : Real;
    out y : Real;
    first start;
    action inner { assign y := f(a); }
    then done;
    succession first start then inner;
  }
}
`

// mustEvaluateIn evaluates expr with the subject named instantiated as self.
func mustEvaluateIn(t *testing.T, srv *Service, modelHash, subject, expr string) *pb.Value {
	t.Helper()
	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: modelHash, SubjectSymbolId: subject, Expression: expr})
	if err != nil {
		t.Fatalf("Evaluate(%s in %s): %v", expr, subject, err)
	}
	if resp.Error != "" {
		t.Fatalf("Evaluate(%s in %s): %s", expr, subject, resp.Error)
	}
	return resp.Result
}

func functionValue(calcID string, selfID int64) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Function{Function: &pb.Function{CalcId: calcID, SelfId: selfID}}}
}

// A calc held as a value crosses as the declaration it is a value of; a client
// echoing what the service sent reads back the same function, which applies.
func TestFunctionRoundTrip(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, functionWireModel)
	cached, ok := srv.cache.Get(modelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	for expr, want := range map[string]string{
		"F::Sq":     "F::Sq",
		"F::sq":     "F::sq",
		"F::pickSq": "F::sq",
		"F::apply":  "F::apply",
	} {
		pv := mustEvaluate(t, srv, modelHash, expr)
		fn := pv.GetFunction()
		if fn == nil {
			t.Fatalf("%s crossed as %T: %v", expr, pv.GetKind(), pv)
		}
		if fn.GetCalcId() != want || fn.GetSelfId() != 0 {
			t.Errorf("%s = %v, want calc_id %q closing over no object", expr, fn, want)
		}

		rt, _, release := srv.newRuntime(cached)
		back, err := ProtoToRuntimeValue(rt, pv, idx, sem)
		if err != nil {
			release()
			t.Fatalf("ProtoToRuntimeValue(%s): %v", expr, err)
		}
		if back.Kind != runtime.ValFunction || runtime.FormatValue(back) != want {
			t.Errorf("%s read back as %s %s, want the function %s", expr, back.Kind, runtime.FormatValue(back), want)
		}
		release()
	}

	fns := mustEvaluate(t, srv, modelHash, "F::fns")
	elems := fns.GetSequence().GetElements()
	if len(elems) != 2 || elems[0].GetFunction().GetCalcId() != "F::Sq" || elems[1].GetFunction().GetCalcId() != "F::Cube" {
		t.Fatalf("F::fns = %v, want a sequence of the functions Sq and Cube", fns)
	}

	// A calc usage read off a part closes over that part, which crosses by ID and
	// resolves the calc's feature names when the value comes back.
	scale := mustEvaluateIn(t, srv, modelHash, "F::holder", "scale")
	if scale.GetFunction().GetCalcId() != "F::Holder::scale" || scale.GetFunction().GetSelfId() == 0 {
		t.Fatalf("holder.scale = %v, want the calc F::Holder::scale closing over holder", scale)
	}
	func() {
		rt, _, release := srv.newRuntime(cached)
		defer release()
		holder, err := rt.Instantiate(lookupNamed(idx, "F::holder")[0])
		if err != nil {
			t.Fatalf("Instantiate(holder): %v", err)
		}
		fn, isFunction, err := rt.FunctionValueOn(lookupNamed(idx, "F::Holder::scale")[0], holder)
		if err != nil || !isFunction {
			t.Fatalf("FunctionValueOn(scale, holder) = %v, %v, %v", fn, isFunction, err)
		}
		pv := ValueToProtoIn(rt, fn, idx)
		if pv.GetFunction().GetCalcId() != "F::Holder::scale" || pv.GetFunction().GetSelfId() != holder.ID {
			t.Fatalf("scale over holder crossed as %v, want calc_id F::Holder::scale, self_id %d", pv, holder.ID)
		}
		back, err := ProtoToRuntimeValue(rt, pv, idx, sem)
		if err != nil || back.Kind != runtime.ValFunction || back.FunctionSelf() != holder {
			t.Errorf("scale over holder read back as %v, %v; want the function over holder", back, err)
		}
		if back.Function() != fn.Function() {
			t.Errorf("scale over holder read back as the calc %v, want %v", back.Function(), fn.Function())
		}
	}()

	// A function read in applies as the argument of a calc and an action.
	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "F::apply", Arguments: []*pb.Value{functionValue("F::Sq", 0), realValue(3)}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(apply): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetRealValue(); got != 9 {
		t.Errorf("apply(Sq, 3.0) = %v, want 9.0", calc.Result)
	}
	act, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: "F::run",
		Inputs:         map[string]*pb.Value{"f": functionValue("F::Cube", 0), "a": realValue(2)},
	})
	if err != nil || act.Error != "" {
		t.Fatalf("ExecuteAction: err = %v, error = %q", err, act.GetError())
	}
	if y := act.Outputs["y"].GetRealValue(); y != 8 {
		t.Errorf("output y = %v, want 8.0", act.Outputs["y"])
	}

	// A function closing over a body's bindings has no wire form: it is named
	// as unsupported rather than sent as a calc it could not be rebuilt from.
	closed := mustEvaluate(t, srv, modelHash, "F::outer(3.0)")
	if closed.GetFunction() != nil || !strings.Contains(closed.GetNull(), "unsupported: function F::Outer::inner") {
		t.Errorf("F::outer(3.0) = %v, want an unsupported null naming F::Outer::inner", closed)
	}
}

// A function naming no calc of the model, an object the runtime does not
// hold, or read with no runtime to bind it in, is refused with a typed error.
func TestMalformedFunctionsAreRejected(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, functionWireModel)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics
	rt, _, release := srv.newRuntime(cached)

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"empty", functionValue("", 0), ErrFunctionUnbound},
		{"unknown declaration", functionValue("F::Nope", 0), ErrFunctionUnbound},
		{"declaration that is not a calc", functionValue("F::holder", 0), ErrFunctionUnbound},
		{"calc usage computing a result", functionValue("F::pickSq", 0), ErrFunctionUnbound},
		{"object the runtime does not hold", functionValue("F::Sq", 12345), ErrFunctionUnbound},
		{"nested in a sequence", &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{
			intValue(1), functionValue("F::Nope", 0),
		}}}}, ErrFunctionUnbound},
		{"nested in an array", arrayValue([]int64{1}, functionValue("", 0)), ErrFunctionUnbound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ProtoToRuntimeValue(rt, tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToRuntimeValue = %v, %v; want %v", val, err, tc.want)
			}
			if val.Kind != runtime.ValInvalid {
				t.Errorf("a rejected value was still returned: %v", val)
			}
		})
	}

	for name, val := range map[string]*pb.Value{
		"bare":     functionValue("F::Sq", 0),
		"in array": arrayValue([]int64{1}, functionValue("F::Sq", 0)),
	} {
		if _, err := ProtoToValueIn(val, idx, sem); !errors.Is(err, ErrFunctionNeedsRuntime) {
			t.Errorf("%s without a runtime: err = %v, want %v", name, err, ErrFunctionNeedsRuntime)
		}
	}
	release()

	// Over the service, a malformed argument is an in-band error, as a
	// malformed quantity is; a function where a number is due is a type error.
	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "F::apply", Arguments: []*pb.Value{functionValue("F::Nope", 0), realValue(3)}})
	if err != nil {
		t.Fatalf("EvaluateCalc(unknown): %v", err)
	}
	if !strings.Contains(calc.Error, ErrFunctionUnbound.Error()) {
		t.Errorf("EvaluateCalc(unknown) error = %q, want one naming %v", calc.Error, ErrFunctionUnbound)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "F::apply", Arguments: []*pb.Value{realValue(3), realValue(3)}})
	if err != nil {
		t.Fatalf("EvaluateCalc(scalar as function): %v", err)
	}
	if !strings.Contains(calc.Error, runtime.ErrTypeMismatch.Error()) {
		t.Errorf("EvaluateCalc(scalar as function) error = %q, want one naming %v", calc.Error, runtime.ErrTypeMismatch)
	}
}

// The arm is advertised under its own capability; a service withholding it
// names the function as unsupported and refuses one sent to it.
func TestFunctionCapability(t *testing.T) {
	ctx := context.Background()
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityFunctionValues
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityFunctionValues)
	}

	withheld := mustNewServiceWithout(t, CapabilityFunctionValues)
	modelHash := mustParse(t, withheld, functionWireModel)
	for expr, want := range map[string]string{
		"F::Sq":     "unsupported: function F::Sq",
		"F::pickSq": "unsupported: function F::sq",
	} {
		got := mustEvaluate(t, withheld, modelHash, expr)
		if got.GetFunction() != nil {
			t.Errorf("%s crossed as a function without %s: %v", expr, CapabilityFunctionValues, got)
		}
		if got.GetNull() != want {
			t.Errorf("%s without %s = %v, want null %q", expr, CapabilityFunctionValues, got, want)
		}
	}
	fns := mustEvaluate(t, withheld, modelHash, "F::fns")
	for i, want := range []string{"F::Sq", "F::Cube"} {
		if got := fns.GetSequence().GetElements()[i].GetNull(); got != "unsupported: function "+want {
			t.Errorf("F::fns#%d without %s = %q", i+1, CapabilityFunctionValues, got)
		}
	}

	sq := functionValue("F::Sq", 0)
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{sq}}}}
	for name, input := range map[string]*pb.Value{"function": sq, "nested": nested} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "F::run",
			Inputs:         map[string]*pb.Value{"f": input, "a": realValue(2)},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityFunctionValues) {
			t.Errorf("ExecuteAction with %s input without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityFunctionValues, err)
		}
		_, err = withheld.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "F::apply", Arguments: []*pb.Value{input, realValue(3)}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityFunctionValues) {
			t.Errorf("EvaluateCalc with %s argument without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityFunctionValues, err)
		}
	}
}

func TestValueCarriesFunction(t *testing.T) {
	one := intValue(1)
	sq := functionValue("F::Sq", 0)
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
		{"function", sq, true},
		{"sequence of ints", sequence(one, one), false},
		{"sequence with a function", sequence(one, sequence(sq)), true},
		{"array of functions", arrayValue([]int64{1}, sq), true},
	} {
		if got := ValueCarriesFunction(tc.value); got != tc.want {
			t.Errorf("ValueCarriesFunction(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
	if ValueCarriesStructured(sq) {
		t.Error("a bare function is not a structured value")
	}
}
