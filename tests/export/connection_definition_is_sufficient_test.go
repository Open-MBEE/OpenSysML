package export_test

import (
	"regexp"
	"testing"
)

// A connection, interface and allocation definition state isSufficient = true in the graph, which no
// keyword notates (SysML v2.1 §8.3.13.3); no other definition or usage does, and the notation round-trips.
func TestConnectionDefinitionSufficiencyIsStatedByTheGraph(t *testing.T) {
	const src = `package Links {
    part def D;
    port def P;
    connection def CD {
        end part src : D;
        end part dst : D;
    }
    interface def ID {
        end port a : P;
        end port b : P;
    }
    allocation def AD {
        end part task : D;
        end part onto : D;
    }
    connection def Empty;
    part def H {
        part a : D;
        part b : D;
        connection c : CD connect a to b;
    }
}
`
	turtle := idTurtle(t, src)
	for name, want := range map[string]bool{
		"Links::CD":    true,
		"Links::ID":    true,
		"Links::AD":    true,
		"Links::Empty": true,
		"Links::D":     false,
		"Links::P":     false,
		"Links::H":     false,
		"Links::H::c":  false,
	} {
		block := regexp.MustCompile(`sysml:qualifiedName "` + regexp.QuoteMeta(name) + `"[^\n]*\n(?:[^\n]+\n)*`).Find(turtle)
		if block == nil {
			t.Fatalf("%s is not in the graph", name)
		}
		if got := regexp.MustCompile(`sysml:isSufficient "true"`).Match(block); got != want {
			t.Errorf("%s states isSufficient true: %t, want %t:\n%s", name, got, want, block)
		}
	}
	if back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != src {
		t.Errorf("definitions were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
}
