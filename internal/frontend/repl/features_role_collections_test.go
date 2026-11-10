package repl

import (
	"strings"
	"testing"
)

// A requirement's referential actors and stakeholders read as the objects its actor and stakeholder
// members hold; %features lists an unread implied collection as [] and the read one by its members.
func TestRoleCollectionsOfARequirementReadTheirMembers(t *testing.T) {
	s := loadSource(t, `package P {
		part def Person;
		concern def Safety;
		requirement def R { subject s : Person; actor driver : Person; stakeholder owner : Person; frame concern sf : Safety; }
		requirement r : R;
	}`)
	run(t, s, "%instantiate P::r")
	before := run(t, s, "%features P::r")
	for _, want := range []string{"actors = []", "stakeholders = []", "concerns = []"} {
		if !strings.Contains(before, want) {
			t.Errorf("%%features before any read lacks %q:\n%s", want, before)
		}
	}
	for _, name := range []string{"actors", "stakeholders", "concerns"} {
		if got := run(t, s, "%eval in #1 : "+name); !strings.Contains(got, "[Instance(ID: ") {
			t.Errorf("%%eval %s = %s, want one held object", name, got)
		}
	}
	after := run(t, s, "%features P::r")
	for _, want := range []string{"actors = [Instance(ID: ", "stakeholders = [Instance(ID: ", "concerns = [Instance(ID: ", "assumptions = []"} {
		if !strings.Contains(after, want) {
			t.Errorf("%%features after the reads lacks %q:\n%s", want, after)
		}
	}
}
