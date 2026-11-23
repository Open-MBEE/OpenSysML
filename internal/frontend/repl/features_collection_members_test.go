package repl

import (
	"fmt"
	"strings"
	"testing"
)

const collectionMembersModel = `package FeaturesCollection {
	part def Slot { attribute number : ScalarValues::Integer; }
	part system {
		part slots : Slot [2] = (slot1, slot2);
		part slot1 : Slot { :>> number = 1; }
		part slot2 : Slot { :>> number = 2; }
		part spare : Slot [1..*] { :>> number = 3; }
	}
}`

// Each object of a multi-valued feature is headed by its place and identity, with
// its own features beneath; a single-valued feature's row is heading enough.
func TestFeaturesHeadsCollectionMembers(t *testing.T) {
	s := loadSource(t, collectionMembersModel)
	run(t, s, "%instantiate FeaturesCollection::system")
	got := run(t, s, "%features FeaturesCollection::system all")
	wantsInOrder(t, got,
		"\n  slots = [Instance(ID: 2), Instance(ID: 3)]",
		"\n    [1] Instance(ID: 2)\n      number = 1\n",
		"\n    [2] Instance(ID: 3)\n      number = 2\n",
		"\n  slot1 = Instance(ID: 2)\n    number = 1\n",
		"\n  slot2 = Instance(ID: 3)\n    number = 2\n",
		"\n  spare = [Instance(ID: 4)]\n    [1] Instance(ID: 4)\n      number = 3\n",
	)
}

// A member's heading is a line of the listing, so the default bound counts it.
func TestFeaturesMemberHeadingsSpendTheLineBudget(t *testing.T) {
	s := loadSource(t, `package Rack {
		part def Slot { attribute number : ScalarValues::Integer = 1; }
		part def Shelf { part slots : Slot [40]; }
	}`)
	run(t, s, "%instantiate Rack::Shelf")
	got := run(t, s, "%features Rack::Shelf")
	wants(t, got, "\n    [1] Instance(ID: 2)\n      number = 1\n", "… (listing truncated; %features Rack::Shelf all shows it whole")
	// Two header lines and the truncation line stand outside the bound.
	if n := strings.Count(got, "\n") + 1; n > maxFeatureValueLines+3 {
		t.Errorf("bounded listing has %d lines, over the %d-line bound:\n%s", n, maxFeatureValueLines, got)
	}
	headings := strings.Count(run(t, s, "%features Rack::Shelf all"), "] Instance(ID: ")
	if headings != 40 {
		t.Errorf("whole listing heads %d members, want 40", headings)
	}
}

// A behavior performed several times runs once per occurrence; each occurrence's
// values are headed as a collection member's are, and a single run's are not.
func TestFeaturesHeadsRepeatedBehaviorOccurrences(t *testing.T) {
	s := loadSource(t, `package Repeated {
		action def Run { attribute n : ScalarValues::Integer = 1; }
		part def Machine { perform action run : Run [2]; perform action once : Run; }
	}`)
	run(t, s, "%instantiate Repeated::Machine")
	got := run(t, s, "%features Repeated::Machine all")
	wantsInOrder(t, got,
		"\n  run: performed action, completed\n    [1] Instance(ID: ",
		"\n      n = 1\n",
		"\n    [2] Instance(ID: ",
		"\n      n = 1\n",
		"\n  once: performed action, completed\n    n = 1\n",
	)
}

// A heading is emitted only where a line beneath it is left, whichever row of
// a member the bound falls on; the leading attributes move where it falls.
func TestFeaturesMemberHeadingLeavesALineBeneath(t *testing.T) {
	for lead := range 6 {
		s := loadSource(t, fmt.Sprintf(`package Rack {
			part def Slot { attribute number : ScalarValues::Integer = 1; }
			part def Shelf { %s part slots : Slot [150]; attribute label : ScalarValues::Integer = 9; }
		}`, leadingAttributes(lead)))
		run(t, s, "%instantiate Rack::Shelf")
		lines := strings.Split(run(t, s, "%features Rack::Shelf"), "\n")
		for i, line := range lines {
			if strings.Contains(line, "] Instance(ID: ") && (i+1 == len(lines) || !strings.Contains(lines[i+1], "number = 1")) {
				t.Errorf("%d leading: heading %q is not followed by its member's features", lead, line)
			}
		}
		wants(t, strings.Join(lines, "\n"), "\n  label = 9")
	}
}

func leadingAttributes(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "attribute a%d : ScalarValues::Integer = 8; ", i)
	}
	return b.String()
}
