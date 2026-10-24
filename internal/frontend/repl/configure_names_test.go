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
    requirement def 'Base Rule' { subject s; attribute x : Integer; require constraint { x > 10 } }
    requirement def 'Derived Rule' :> 'Base Rule' { require constraint { x < 1 } }
    constraint def 'Nat Rule' { in 'the speed' : Natural; assert constraint { 'the speed' < 0 } }
}
`

// rejectsPlainNames fails on a line writing a name that needs quoting without them.
func rejectsPlainNames(t *testing.T, out string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		for _, plain := range []string{"Cfg Space::", "::Engine Choices", "::the family", "::4cyl", "::6cyl", "::auto box", "Part bad one", "Part the family", "satisfy soft landing", "Part fast craft", "of Touchdown Rule", "requirement Derived Rule", "declared by Base Rule", "::bad one", "::Nat Rule", "::the speed", "declared by the speed"} {
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
	wants(t, all, "✓ Abstract part 'the family' permits 2 selections of variants, which are all of them",
		"'Cfg Space'::'the family'::engine = 'Cfg Space'::'Engine Choices'::'4cyl'",
		"'Cfg Space'::'the family'::trans = 'Cfg Space'::'the family'::trans::'auto box'")
	rejectsPlainNames(t, all)

	refused := run(t, s, "%configure 'Cfg Space'::'the family' engine='6cyl' trans=man")
	wants(t, refused, "✗ the chosen variants are not consistent with Abstract part 'the family'",
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

// An explanation names the element each condition came from as the notation does,
// and a condition the explained element declares itself names no other declarer.
func TestExplainWritesTheDeclaringElementAsTheNotationDoes(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quotedModel)
	got := run(t, s, "%explain 'Cfg Space'::'Derived Rule'")
	wants(t, got, "✗ Requirement 'Derived Rule' is unsatisfiable",
		"`x > 10` — requirement 'Derived Rule', declared by 'Base Rule'",
		"`x < 1` — requirement 'Derived Rule',")
	rejects(t, got, "`x < 1` — requirement 'Derived Rule', declared by")
	rejectsPlainNames(t, got)
}

// Every row of an explanation names its element, its declarer and the values it
// states as the notation does: a feature's own domain names no other declarer,
// and a fixed value writes the variant it was fixed at the way the model did.
func TestExplainWritesEveryRowAsTheNotationDoes(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quotedModel)
	nat := run(t, s, "%explain 'Cfg Space'::'Nat Rule'")
	wants(t, nat, "✗ Constraint 'Nat Rule' is unsatisfiable",
		"declared domain: `a Natural is not negative` — declaration 'Cfg Space'::'Nat Rule'::'the speed', at ",
		"required condition: `'the speed' < 0` — constraint 'Nat Rule', at ")
	rejectsPlainNames(t, nat)

	bad := run(t, s, "%explain 'Cfg Space'::'bad one'")
	wants(t, bad, "✗ Part 'bad one' is unsatisfiable",
		"fixed value: `'Cfg Space'::'bad one'::engine == 'Cfg Space'::'Engine Choices'::'6cyl', declared by the model` — feature 'Cfg Space'::'bad one'::engine, at ",
		"fixed value: `'Cfg Space'::'bad one'::trans == 'Cfg Space'::'the family'::trans::man, declared by the model` — feature 'Cfg Space'::'bad one'::trans, at ",
		"— part 'bad one', declared by 'the family', at ")
	rejectsPlainNames(t, bad)
}
