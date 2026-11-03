package export_test

import (
	"regexp"
	"testing"
)

// A declared end that varies in time is constant by derivation (SysML v2.1 §8.2.2.6.1): the graph
// states isConstant for it, and the notation rebuilt from the graph alone carries no `constant`
// keyword — neither for an end that never wrote one nor for one that wrote the redundant word.
func TestDerivedEndConstancyComesBackFromTheGraphAlone(t *testing.T) {
	const src = `package Ends {
    part def D;
    connection def CD {
        end part src : D;
        constant end part dst : D;
    }
    part def H {
        part a : D;
        part b : D;
        connection c connect a to b;
    }
}
`
	const want = `package Ends {
    part def D;
    connection def CD {
        end part src : D;
        end part dst : D;
    }
    part def H {
        part a : D;
        part b : D;
        connection c connect a to b;
    }
}
`
	turtle := idTurtle(t, src)
	for _, end := range []string{"Ends::CD::src", "Ends::CD::dst"} {
		block := regexp.MustCompile(`sysml:qualifiedName "` + regexp.QuoteMeta(end) + `"[^\n]*\n(?:[^\n]+\n)*`).Find(turtle)
		if !regexp.MustCompile(`sysml:isConstant "true"`).Match(block) {
			t.Errorf("%s does not state isConstant true:\n%s", end, block)
		}
	}
	if back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != want {
		t.Errorf("ends were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}
