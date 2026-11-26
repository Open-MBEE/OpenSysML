package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A parameter an action's notation states is an input and masks the inout parameter of its kind it redefines:
// the action has no output, however the document was read. An accept's payload is inout as the one it
// redefines is (SysML 8.3.17.2 validateAcceptActionUsageParameters), so it is the accept's one output.
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
			var got []string
			for _, output := range outputs {
				got = append(got, m.QualifiedNameOf(output))
			}
			var want []string
			if name == "acc" {
				want = []string{"P::T::a::acc::m"}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s: %s lists the outputs %v, want %v", build, name, got, want)
			}
		}
	}
}
