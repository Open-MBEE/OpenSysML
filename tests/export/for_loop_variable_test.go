package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const forLoopVariableModel = `package Loops {
    part def Component {
        action run;
    }
    part def Special :> Component;
    part def Assembly {
        part components : Special [1..*];
        action plain {
            for c in components {
            }
        }
        action typed {
            for c : Component, Special in components {
                perform c.run;
            }
        }
        action quoted {
            for <s> 'my var' :> components in components {
            }
        }
    }
}
`

// A `for` variable is an element of the graph, so its name, short name and head
// survive a conversion that keeps no source text.
func TestForLoopVariableRoundTripsFromTheGraphAlone(t *testing.T) {
	first, err := convert.Convert("loops.sysml", []byte(forLoopVariableModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(first), "sysx:loopVariable") {
		t.Errorf("the graph states the loop variable as a name rather than as its element:\n%s", first)
	}
	notation := string(structuralRoundTrip(t, "loops", first))
	for _, want := range []string{
		"for c in components {",
		"for c : Component, Special in components {",
		"for <s> 'my var' subsets components in components {",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation written from the graph alone lacks %q:\n%s", want, notation)
		}
	}
}

// A graph written before the variable was an element states its name alone, and still reads.
func TestForLoopVariableReadsTheNameAnOlderGraphStates(t *testing.T) {
	first, err := convert.Convert("loops.sysml", []byte(forLoopVariableModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	const stated = "sysml:loopVariable expr:Loops__Assembly__plain___400_ploopVariable ;"
	if !strings.Contains(string(first), stated) {
		t.Fatalf("the graph does not state %s:\n%s", stated, first)
	}
	older := strings.Replace(string(withoutSourceText(t, first)), stated, `sysx:loopVariable "c" ;`, 1)
	notation, err := convert.Convert("loops.ttl", []byte(older), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("an older graph's loop variable: %v", err)
	}
	if !strings.Contains(string(notation), "for c in components {") {
		t.Errorf("the notation lacks the variable the older graph names:\n%s", notation)
	}
}
