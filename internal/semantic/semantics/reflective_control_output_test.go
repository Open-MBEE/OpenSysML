package semantics

import "testing"

// An assignment's replacementValues is an input that redefines the inout parameter of its kind,
// so the action has no output: the redefined parameter is not inherited beside it.
func TestStatedParametersMaskTheOutputsTheyRedefine(t *testing.T) {
	m, root := buildModelWithStdlibUnwalked(t, controlParameterSource)
	for _, name := range []string{"asg", "lp", "w", "lu", "cond", "snd", "ter"} {
		action := nestedSym(t, root, "P::T::a::"+name)
		outputs, ok := m.ReflectiveElements(action, "output")
		if !ok {
			t.Fatalf("%s answers no output", name)
		}
		for _, output := range outputs {
			t.Errorf("%s lists the output %s, want none", name, m.QualifiedNameOf(output))
		}
	}
}
