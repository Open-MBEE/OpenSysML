package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A parameter an action's notation states is an input, an accept's payload included, and masks the
// inout parameter of its kind it redefines: the action has no output, however the document was read.
func TestStatedParametersMaskTheOutputsTheyRedefine(t *testing.T) {
	builds := map[string]func(*testing.T, string) (*Model, *symbols.Scope){
		"walked": buildModelWithStdlib, "unwalked": buildModelWithStdlibUnwalked,
	}
	for build, model := range builds {
		m, root := model(t, controlParameterSource)
		for _, name := range []string{"asg", "lp", "w", "lu", "cond", "snd", "acc", "ter"} {
			action := nestedSym(t, root, "P::T::a::"+name)
			outputs, ok := m.ReflectiveElements(action, "output")
			if !ok {
				t.Fatalf("%s: %s answers no output", build, name)
			}
			for _, output := range outputs {
				t.Errorf("%s: %s lists the output %s, want none", build, name, m.QualifiedNameOf(output))
			}
		}
	}
}
