package runtime

import (
	"errors"
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
	ctx := NewContext(NewModel(model, resolver), 1000)
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
	ctx := NewContext(NewModel(model, resolver), 1000)
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

// TestDeclaredActionInputsBind: a parameter and a plain attribute both name a
// feature the caller may seed.
func TestDeclaredActionInputsBind(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, inputNamesModel)
	ctx := NewContext(NewModel(model, resolver), 1000)
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
