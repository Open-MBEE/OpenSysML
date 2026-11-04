package runtime

import "testing"

const actionUsageParameterValuesSource = `package P {
	action def Step {
		in x;
		out y;
		first start;
		then assign y := x + 1;
	}
	action s : Step {
		in x = 9;
	}
	action r : Step {
		in x redefines x = 9;
	}
	action def Count {
		attribute x = 1;
		out y;
		first start;
		then assign y := x;
	}
	action hides : Count {
		attribute x = 9;
	}
}
`

func executeActionUsage(t *testing.T, name string, inputs map[string]Value) map[string]Value {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, actionUsageParameterValuesSource))
	outputs, err := ctx.ExecuteActionWithInputs(oneSymbol(t, idx, "P::"+name), inputs)
	if err != nil {
		t.Fatalf("ExecuteAction(%s): %v", name, err)
	}
	return outputs
}

func soleInteger(t *testing.T, name string, v Value) int64 {
	t.Helper()
	elements := elementsOf(v)
	if len(elements) != 1 || elements[0].Kind != ValConst {
		t.Fatalf("%s = %v, want one Integer", name, v)
	}
	return elements[0].Const.Int
}

// An action usage performed with no argument computes from its own `in` value,
// which redefines the definition's parameter implicitly by name or explicitly.
func TestActionUsageOwnInputValueIsPerformed(t *testing.T) {
	for _, name := range []string{"s", "r"} {
		outputs := executeActionUsage(t, name, nil)
		if got := soleInteger(t, "y", outputs["y"]); got != 10 {
			t.Errorf("%s.y = %d, want 10", name, got)
		}
	}
}

// An explicit input overrides the usage's own value as it overrides a default.
func TestActionUsageInputOverridesOwnValue(t *testing.T) {
	outputs := executeActionUsage(t, "s", map[string]Value{"x": integerValue(4)})
	if got := soleInteger(t, "y", outputs["y"]); got != 5 {
		t.Errorf("s.y = %d, want 5", got)
	}
}

// A usage member that only hides a same-named feature of the definition, redefining
// nothing, leaves the body reading the definition's own feature (KerML 1.0 §8.2.3.5.3).
func TestActionUsageNamesakeDoesNotReplaceBodyFeature(t *testing.T) {
	outputs := executeActionUsage(t, "hides", nil)
	if got := soleInteger(t, "y", outputs["y"]); got != 1 {
		t.Errorf("hides.y = %d, want 1", got)
	}
}
