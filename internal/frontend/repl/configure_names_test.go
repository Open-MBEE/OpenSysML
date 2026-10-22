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
    private import ScalarValues::*;
    part def 'Space Craft' { attribute 'vertical speed' : Real; }
    requirement def 'Touchdown Rule' { subject craft : 'Space Craft'; require constraint { craft.'vertical speed' <= 1.5 } }
    requirement 'the touchdown' : 'Touchdown Rule';
    part 'slow craft' {
        part lander : 'Space Craft' { attribute :>> 'vertical speed' = 1.0; }
        assert satisfy requirement 'soft landing' : 'Touchdown Rule' by lander;
    }
    part 'fast craft' {
        part 'the lander' : 'Space Craft' { attribute :>> 'vertical speed' = 2.0; }
        assert satisfy 'the touchdown' by 'the lander';
    }
}
`

// rejectsPlainNames fails on a line writing a name that needs quoting without them.
func rejectsPlainNames(t *testing.T, out string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		for _, plain := range []string{"Cfg Space::", "::Engine Choices", "::the family", "::4cyl", "::6cyl", "::auto box", "Part bad one", "Part the family", "satisfy soft landing", "Part fast craft", "of Touchdown Rule"} {
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

// A satisfaction's subject is the assertion as written, its names quoted as the
// notation quotes them; a verdict about a part names the part the same way.
func TestSolveWritesASatisfactionsSubjectAsTheNotationDoes(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quotedModel)
	soft := run(t, s, "%solve 'Cfg Space'::'slow craft'::'soft landing'")
	wants(t, soft, "✓ Satisfaction satisfy 'soft landing' by lander has values satisfying it")
	rejectsPlainNames(t, soft)

	fast := run(t, s, "%solve 'Cfg Space'::'fast craft'")
	wants(t, fast, "✗ Part 'fast craft' has no values consistent with the value already fixed",
		"'Cfg Space'::'fast craft'::'the lander.vertical speed' = 2.0  (declared)",
		"Against: required condition { craft.'vertical speed' <= 1.5 } of 'Touchdown Rule' at ")
	rejectsPlainNames(t, fast)
}
