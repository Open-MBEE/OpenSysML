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
		case strings.Contains(block, `sysml:isLibraryElement "true"`):
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

// TestTypedSuccessionEndsAreNamedAsItsConnectionDefinitionsEnds: the minted ends of a succession
// typed by a connection definition redefine that definition's ends and take their names.
func TestTypedSuccessionEndsAreNamedAsItsConnectionDefinitionsEnds(t *testing.T) {
	src := `package P {
	part def A;
	part def B;
	connection def AB {
		end head : A;
		end tail : B;
	}
	part def Q {
		part a : A;
		part b : B;
		succession s : AB first a then b;
	}
}
`
	turtle := string(roundTripsExactly(t, src))
	for _, name := range []string{"head", "tail"} {
		if !strings.Contains(turtle, `sysml:memberName "`+name+`"`) {
			t.Errorf("no end of the typed succession is named %q as AB's end is", name)
		}
	}
	for _, name := range []string{"earlierOccurrence", "laterOccurrence"} {
		if strings.Contains(turtle, `sysml:memberName "`+name+`"`) {
			t.Errorf("a typed succession's end is named %q, not as the end of AB it redefines", name)
		}
	}
}

// TestSuccessionTypedByAnAssociationWithUnnamedEndsTakesTheirEffectiveNames: an end of the typed
// succession redefines an unnamed end of its association, which is named by the end it redefines.
func TestSuccessionTypedByAnAssociationWithUnnamedEndsTakesTheirEffectiveNames(t *testing.T) {
	src := `package P {
	private import Occurrences::*;
	occurrence def A;
	occurrence def B;
	assoc Link specializes HappensBefore {
		end feature redefines earlierOccurrence : A;
		end feature redefines laterOccurrence : B;
	}
	occurrence def Q {
		occurrence a : A;
		occurrence b : B;
		succession s : Link first a then b;
	}
}
`
	turtle := string(roundTripsExactly(t, src))
	for _, name := range []string{"earlierOccurrence", "laterOccurrence"} {
		if !strings.Contains(turtle, `sysml:memberName "`+name+`"`) {
			t.Errorf("no end of the association-typed succession is named %q", name)
		}
	}
}
