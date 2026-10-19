package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// heldOne is the element a pin of unstated or multi-valued multiplicity holds one value
// as: a one-element sequence (KerML 1.0 §7.3.4, §7.4.8), whose element the pin was bound to.
func heldOne(t *testing.T, name string, v Value) Value {
	t.Helper()
	if v.Kind != ValSequence || v.Sequence() == nil || v.Sequence().Size() != 1 {
		t.Fatalf("%s = %s, want a one-element sequence", name, FormatValue(v))
	}
	return v.Sequence().Elements()[0]
}

// heldInt is the Integer a pin holds as its one element.
func heldInt(t *testing.T, name string, v Value) int64 {
	t.Helper()
	elem := heldOne(t, name, v)
	if elem.Kind != ValConst || elem.Const.Kind != semantics.ValInt {
		t.Fatalf("%s = %s, want an Integer held as the one element", name, FormatValue(v))
	}
	return elem.Const.Int
}

// soleIntOutput is intOutput for an output pin, which holds its Integer as its one element.
func soleIntOutput(t *testing.T, outputs map[string]Value, name string) int64 {
	t.Helper()
	val, ok := outputs[name]
	if !ok {
		t.Fatalf("output %q missing from %v", name, outputs)
	}
	return heldInt(t, name, val)
}

// assertSoleIntOutput is assertIntOutput for an output pin.
func assertSoleIntOutput(t *testing.T, outputs map[string]Value, name string, want int64) {
	t.Helper()
	if got := soleIntOutput(t, outputs, name); got != want {
		t.Errorf("%s = %v, want %d", name, got, want)
	}
}

// soleRealOutputs reads the Reals the named output pins hold, each as its one element.
func soleRealOutputs(t *testing.T, out map[string]Value, names ...string) []float64 {
	t.Helper()
	got := make([]float64, len(names))
	for i, name := range names {
		v := heldOne(t, name, out[name])
		if v.Kind != ValConst || v.Const.Kind != semantics.ValReal {
			t.Fatalf("%s = %v, want a Real", name, v)
		}
		got[i] = v.Const.AsReal()
	}
	return got
}
