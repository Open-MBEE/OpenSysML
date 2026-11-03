package export_test

import (
	"strings"
	"testing"
)

// TestMintedSuccessionEndsAreNamedAsHappensBeforeNames: a `then`, `first` or transition succession the
// notation mints names its ends earlierOccurrence/laterOccurrence, the ends of Occurrences::happensBeforeLinks.
func TestMintedSuccessionEndsAreNamedAsHappensBeforeNames(t *testing.T) {
	src := `package P {
	action def Drive {
		first start;
		action a;
		then action b;
		then done;
	}
	action def Walk {
		action a; action b;
		succession s first a then b;
	}
	state def Machine {
		state off;
		transition off_on first off then on;
		state on;
	}
}
`
	turtle := string(roundTripsExactly(t, src))
	successions, earlier, later := 0, 0, 0
	for _, block := range strings.Split(turtle, "\n\n") {
		switch {
		case strings.Contains(block, "a sysml:SuccessionAsUsage") || strings.Contains(block, "a sysml:Succession ;"):
			successions++
		case !strings.Contains(block, "sysml:EndFeatureMembership"):
		case strings.Contains(block, `sysml:memberName "earlierOccurrence"`):
			earlier++
		case strings.Contains(block, `sysml:memberName "laterOccurrence"`):
			later++
		default:
			t.Errorf("a minted succession end's EndFeatureMembership states no HappensBefore end name\n%s", block)
		}
	}
	if successions < 3 || earlier != successions || later != successions {
		t.Fatalf("%d successions, %d earlierOccurrence and %d laterOccurrence ends", successions, earlier, later)
	}
	if strings.Contains(turtle, `sysml:memberName "source"`) || strings.Contains(turtle, `sysml:memberName "target"`) {
		t.Errorf("a succession end is not a binary connector's source/target")
	}
}
