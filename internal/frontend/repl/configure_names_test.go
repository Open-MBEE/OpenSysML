package repl

import (
	"strings"
	"testing"
)

// quotedModel is a product line whose names the notation must quote, so every
// line of a report has to write them as the notation does.
const quotedModel = `
package 'Cfg Space' {
    part def E; part def T;
    variation part def 'Engine Choices' :> E { variant part '4cyl' : E; variant part '6cyl' : E; }
    abstract part 'the family' {
        part engine : 'Engine Choices'[1];
        variation part trans : T { variant man : T; variant 'auto box' : T; }
        assert constraint { (engine == 'Engine Choices'::'4cyl' and trans == trans::man) xor (engine == 'Engine Choices'::'6cyl' and trans == trans::'auto box') }
    }
    part 'bad one' :> 'the family' {
        part redefines engine = 'Engine Choices'::'6cyl';
        part redefines trans = trans::man;
    }
}
`

// rejectsPlainNames fails on a line writing a name that needs quoting without them.
func rejectsPlainNames(t *testing.T, out string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		for _, plain := range []string{"Cfg Space::", "::Engine Choices", "::the family", "::4cyl", "::6cyl", "::auto box", "Part bad one", "Part the family"} {
			if strings.Contains(line, plain) {
				t.Errorf("a name written without its quotes in %q", line)
			}
		}
	}
}

func TestConfigureWritesEveryNameAsTheNotationDoes(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quotedModel)
	all := run(t, s, "%configure 'Cfg Space'::'the family' all")
	wants(t, all, "✓ Part 'the family' permits 2 selections of variants, which are all of them",
		"'Cfg Space'::'the family'::engine = 'Cfg Space'::'Engine Choices'::'4cyl'",
		"'Cfg Space'::'the family'::trans = 'Cfg Space'::'the family'::trans::'auto box'")
	rejectsPlainNames(t, all)

	refused := run(t, s, "%configure 'Cfg Space'::'the family' engine='6cyl' trans=man")
	wants(t, refused, "✗ the chosen variants are not consistent with Part 'the family'",
		"In the conflict: 'Cfg Space'::'the family'::engine = 'Cfg Space'::'Engine Choices'::'6cyl', 'Cfg Space'::'the family'::trans = 'Cfg Space'::'the family'::trans::man",
		"Against: required condition {", "of 'the family' at ")
	rejectsPlainNames(t, refused)

	bad := run(t, s, "%configure 'Cfg Space'::'bad one'")
	wants(t, bad, "✗ Part 'bad one' permits no selection of variants consistent with the values it fixes",
		"Already fixed:",
		"'Cfg Space'::'bad one'::engine = 'Cfg Space'::'Engine Choices'::'6cyl'  (declared)",
		"'Cfg Space'::'bad one'::trans = 'Cfg Space'::'the family'::trans::man  (declared)",
		"In the conflict: 'Cfg Space'::'bad one'::engine = 'Cfg Space'::'Engine Choices'::'6cyl', 'Cfg Space'::'bad one'::trans = 'Cfg Space'::'the family'::trans::man")
	rejectsPlainNames(t, bad)
}
