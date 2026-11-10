package repl

import (
	"strings"
	"testing"
)

// %features lists a requirement's actors, stakeholders and concerns once the members filling them
// are held, making up nothing; a collection no held member fills stays unexpanded.
func TestFeaturesListsTheRoleCollectionsOfARequirement(t *testing.T) {
	s := loadSource(t, `package P {
		part def Person;
		concern def Safety;
		requirement def R { subject s : Person; actor driver : Person; stakeholder owner : Person; frame concern sf : Safety; }
		requirement r : R;
	}`)
	run(t, s, "%instantiate P::r")
	out := run(t, s, "%features P::r")
	for _, want := range []string{"actors = [Instance(ID: ", "stakeholders = [Instance(ID: ", "concerns = [Instance(ID: ", "constraints = [Instance(ID: ", "assumptions = []"} {
		if !strings.Contains(out, want) {
			t.Errorf("%%features lacks %q:\n%s", want, out)
		}
	}
}
