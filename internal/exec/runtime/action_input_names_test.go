package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// inputNamesModel declares one input parameter and one attribute, the two names
// an input may bind.
const inputNamesModel = `action bump {
	in attribute step = 1;
	attribute result = 0;
	first start;
	action inner { assign result := result + step; }
	done;
	succession first start then inner;
	succession first inner then done;
}`

// TestUnknownActionInputIsReported: an input naming no feature of the action is
// reported, not bound into the feature space and answered as an output.
func TestUnknownActionInputIsReported(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, inputNamesModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	bump := resolveSymbol(t, root, "bump")

	outputs, err := ctx.ExecuteActionWithInputs(bump, map[string]Value{"nope": constInt(7)})
	if !errors.Is(err, ErrUnknownActionInput) {
		t.Fatalf("outputs = %v, err = %v; want ErrUnknownActionInput", outputs, err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %v, want it to name the input", err)
	}
}

// outputParameterModel declares a parameter the action writes back, which a
// caller reads rather than seeds.
const outputParameterModel = `action measure {
	in attribute step = 1;
	out attribute total;
	first start;
	action inner { assign total := step + 1; }
	done;
	succession first start then inner;
	succession first inner then done;
}`

// TestSeedingAnOutputParameterIsReported: an `out` parameter is an answer, so
// binding it as an input is reported rather than silently overwritten.
func TestSeedingAnOutputParameterIsReported(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, outputParameterModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	measure := resolveSymbol(t, root, "measure")

	outputs, err := ctx.ExecuteActionWithInputs(measure, map[string]Value{"total": constInt(99)})
	if !errors.Is(err, ErrOutputActionInput) {
		t.Fatalf("outputs = %v, err = %v; want ErrOutputActionInput", outputs, err)
	}
	if !strings.Contains(err.Error(), "total") {
		t.Errorf("error = %v, want it to name the parameter", err)
	}

	outputs, err = ctx.ExecuteActionWithInputs(measure, map[string]Value{"step": constInt(4)})
	if err != nil {
		t.Fatalf("ExecuteActionWithInputs: %v", err)
	}
	if got := outputs["total"]; got.Const.Int != 5 {
		t.Fatalf("total = %+v, want 5", got)
	}
}

// mixModel declares two inputs and an output for ActionInputs to bind over.
const mixModel = `action mix {
	in attribute a;
	in attribute b;
	out attribute c;
}`

// TestActionInputsBindsAsAnInvocationDoes: positional arguments bind the first
// input parameters in declaration order and named ones the rest; a parameter
// two arguments would bind is ErrDuplicateArgument, a name no parameter carries
// is ErrUnknownParameter, and more positional arguments than parameters is
// ErrActionArity.
func TestActionInputsBindsAsAnInvocationDoes(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, mixModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	mix := resolveSymbol(t, root, "mix")

	if got := ctx.ActionInputNames(mix); fmt.Sprint(got) != "[a b]" {
		t.Fatalf("ActionInputNames = %v, want [a b]", got)
	}
	inputs, err := ctx.ActionInputs(mix, []Value{constInt(7)}, map[string]Value{"b": constInt(3)})
	if err != nil {
		t.Fatalf("ActionInputs: %v", err)
	}
	if inputs["a"].Const.Int != 7 || inputs["b"].Const.Int != 3 {
		t.Fatalf("inputs = %+v, want a = 7 bound positionally, b = 3 by name", inputs)
	}
	_, err = ctx.ActionInputs(mix, []Value{constInt(7)}, map[string]Value{"a": constInt(3)})
	if !errors.Is(err, ErrDuplicateArgument) {
		t.Fatalf("positional and named a = %v, want ErrDuplicateArgument", err)
	}
	_, err = ctx.ActionInputs(mix, nil, map[string]Value{"c": constInt(1)})
	if !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("named c = %v, want ErrUnknownParameter", err)
	}
	_, err = ctx.ActionInputs(mix, []Value{constInt(1), constInt(2), constInt(3)}, nil)
	if !errors.Is(err, ErrActionArity) {
		t.Fatalf("three positional = %v, want ErrActionArity", err)
	}
	if !strings.Contains(err.Error(), "takes 2 input parameter(s), got 3 argument(s)") {
		t.Errorf("error = %v, want the arity spelled", err)
	}
}

// TestDeclaredActionInputsBind: a parameter and a plain attribute both name a
// feature the caller may seed.
func TestDeclaredActionInputsBind(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, inputNamesModel)
	ctx := NewContext(typedModel(model, resolver), 1000)
	bump := resolveSymbol(t, root, "bump")

	outputs, err := ctx.ExecuteActionWithInputs(bump, map[string]Value{
		"step":   constInt(4),
		"result": constInt(10),
	})
	if err != nil {
		t.Fatalf("ExecuteActionWithInputs: %v", err)
	}
	if got := outputs["result"]; got.Const.Int != 14 {
		t.Fatalf("result = %+v, want 14", got)
	}
}
