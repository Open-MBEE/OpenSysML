package repl

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// synthModel states values, an enumeration, two variation points and a division,
// which is what %solve and %configure are asked about.
const synthModel = `
package Synth {
	private import ScalarValues::*;
	public import SI::*;
	public import ISQ::*;

	enum def Finish {
		enum polished;
		enum brushed;
	}

	attribute def Nesting;
	attribute def Trim;

	part def Panel {
		attribute width : Integer = 4;
		attribute height : Integer;
		attribute finish : Finish = Finish::polished;
		attribute maxSpeed : ISQSpaceTime::SpeedValue = 5.4 [km/h];

		assert constraint fits {
			width + height <= 10
		}

		assert constraint polishedIsWide {
			finish == Finish::polished implies width >= 6
		}

		assert constraint speedIsBounded {
			maxSpeed <= 2.0 [m/s]
		}
	}

	part def Ring {
		attribute nesting : Nesting;
		attribute trim : Trim;
	}

	part ringFamily : Ring {
		variation attribute :>> nesting {
			variant attribute nestingTrue;
			variant attribute nestingFalse;
		}

		variation attribute :>> trim {
			variant attribute plain;
			variant attribute gilded;
		}

		assert constraint variantsAgree {
			nesting == nesting::nestingTrue implies trim == trim::gilded
		}
	}

	constraint def Sharing {
		in total : Natural;
		in shares : Natural;
		assert constraint { total / shares >= 2 }
	}

	constraint def Contradictory {
		in i : Integer;
		assert constraint { i > 8 and i < 3 }
	}

	constraint def Exponential {
		in i : Integer;
		assert constraint { i ** 2 == 4 }
	}
}
`

// TestSolveSynthesisesWhatIsFree: the declared width is fixed and the height,
// which is declared nowhere, is the value the solver answers with.
func TestSolveSynthesisesWhatIsFree(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve fits")
	wants(t, got, "✓ Constraint fits has values satisfying it",
		"Already fixed:", "Synth::Panel::width = 4  (declared)",
		"Synthesised:", "Synth::Panel::height = ", "One witness:")
	rejects(t, got, "error:")

	reports := s.SolveValues("fits")
	if len(reports) != 1 || reports[0].Status != SolveSat || reports[0].Solver == "" {
		t.Fatalf("SolveValues answered %+v, want one sat report naming the solver", reports)
	}
}

// TestSolveKeepsWhatAnObjectHolds: an object's values are the ones fixed, named
// as held by it, and %solve creates no object of its own.
func TestSolveKeepsWhatAnObjectHolds(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%instantiate Panel"), "Created instance")
	objects := len(s.instances)

	got := run(t, s, "%solve fits")
	wants(t, got, "Already fixed:", "held by object 1", "Synthesised:", "Synth::Panel::height = ")
	if len(s.instances) != objects {
		t.Errorf("%%solve left %d objects, want the %d there were: it materializes nothing",
			len(s.instances), objects)
	}
}

// TestSolveKeepsWhatAnObjectHoldsOverASubmission: a declaration that changes
// nothing the object is of leaves its values fixed, as a verdict about them
// still reads them.
func TestSolveKeepsWhatAnObjectHoldsOverASubmission(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%instantiate Panel"), "Created instance")
	if res := s.Submit("package Unrelated { }"); len(res.Diagnostics) != 0 {
		t.Fatalf("submitting an unrelated package reported %v", res.Diagnostics)
	}

	got := run(t, s, "%solve fits")
	wants(t, got, "Already fixed:", "Synth::Panel::width = 4  (held by object 1)",
		"Synthesised:", "Synth::Panel::height = ")
	rejects(t, got, "Synthesised:\n    Synth::Panel::width")
}

// TestSolveReportsNoValuesConsistentWithWhatIsFixed: an unsat verdict about a
// query fixing values says so, and names the fixed values in the conflict.
func TestSolveReportsNoValuesConsistentWithWhatIsFixed(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve polishedIsWide")
	wants(t, got, "✗ Constraint polishedIsWide has no values consistent with the values already fixed",
		"In the conflict:", "Synth::Panel::finish = Finish::polished", "Synth::Panel::width = 4")
	if reports := s.SolveValues("polishedIsWide"); reports[0].Status != SolveUnsat {
		t.Errorf("status is %s, want unsat", reports[0].Status)
	}
}

// TestSolveReportsConditionsThatConflictOnTheirOwn: with nothing fixed, unsat is
// the conditions conflicting, which is a different report.
func TestSolveReportsConditionsThatConflictOnTheirOwn(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve Contradictory")
	wants(t, got, "✗ Constraint Contradictory has no satisfying values: its conditions conflict on their own")
	rejects(t, got, "already fixed")
}

// TestSolveReportsAQuantityAsDeclared: a value declared in km/h is reported in
// the unit it was written in, not in the base unit it was scaled to.
func TestSolveReportsAQuantityAsDeclared(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%solve speedIsBounded"), "Synth::Panel::maxSpeed = 5.4 [km/h]  (declared)")
}

// TestSolveUnderANaturalDomainAndADivisorGuard: the shares synthesised are a
// number the guarded division permits, so the witness is one the evaluator could
// take.
func TestSolveUnderANaturalDomainAndADivisorGuard(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve Sharing")
	wants(t, got, "✓ Constraint Sharing has values satisfying it", "Synth::Sharing::shares = ")
	if strings.Contains(got, "Synth::Sharing::shares = 0") {
		t.Errorf("the shares synthesised are 0, which the division is guarded against:\n%s", got)
	}
}

func TestSolveReportsAnUntranslatableElement(t *testing.T) {
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve Exponential")
	wants(t, got, "error:", "not translatable for solving")
	if reports := s.SolveValues("Exponential"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

func TestSolveReportsAnUnknownName(t *testing.T) {
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%solve Nope"), "error:")
}

func TestSolveWithoutAName(t *testing.T) {
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%solve"), "usage: %solve <name>")
}

// An absent solver is a typed error naming what to install, never values made up.
func TestSolveReportsAnAbsentSolver(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve fits")
	wants(t, got, "error:", "z3")
	rejects(t, got, "Synthesised:")
	if reports := s.SolveValues("fits"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

// A solver answering nothing usable is a process failure, distinct from unknown.
func TestSolveReportsASolverProcessFailure(t *testing.T) {
	script := t.TempDir() + "/mute"
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve fits")
	wants(t, got, "error:", "mute")
	rejects(t, got, "Synthesised:", "unknown")
}

// An undecided answer stays undecided, with the reason the solver gave.
func TestSolveReportsAnUndecidedAnswer(t *testing.T) {
	script := t.TempDir() + "/undecided"
	fake := "#!/bin/sh\nprintf 'unknown\\n(:reason-unknown \"incomplete\")\\n'\nwhile read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, synthModel)
	got := run(t, s, "%solve fits")
	wants(t, got, "? Constraint fits has no values decided either way", "incomplete")
	rejects(t, got, "Synthesised:", "error:")
	if reports := s.SolveValues("fits"); reports[0].Status != SolveUnknown {
		t.Errorf("status is %s, want unknown", reports[0].Status)
	}
}

// TestConfigureSynthesisesASelection: with no selection given, one consistent
// selection is reported, assigning every variation point the conditions read.
func TestConfigureSynthesisesASelection(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%configure variantsAgree")
	wants(t, got, "✓ Constraint variantsAgree permits a selection of variants",
		"Synth::ringFamily::nesting = Synth::ringFamily::nesting::",
		"Synth::ringFamily::trim = Synth::ringFamily::trim::",
		"%configure variantsAgree all")
	if strings.Contains(got, "nesting::nestingTrue") && !strings.Contains(got, "trim::gilded") {
		t.Errorf("the selection reported does not satisfy the constraint:\n%s", got)
	}
}

// TestConfigureChecksAChosenSelection: a selection the conditions permit is
// consistent, and one they forbid is not, with the choices in the conflict named.
func TestConfigureChecksAChosenSelection(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	ok := run(t, s, "%configure variantsAgree nesting=nestingTrue trim=gilded")
	wants(t, ok, "✓ the chosen variants are consistent with Constraint variantsAgree",
		"Synth::ringFamily::nesting = Synth::ringFamily::nesting::nestingTrue  (chosen)")
	rejects(t, ok, "error:")

	bad := run(t, s, "%configure variantsAgree nesting=nestingTrue trim=plain")
	wants(t, bad, "✗ the chosen variants are not consistent with Constraint variantsAgree",
		"In the conflict:")
}

// TestConfigureEnumeratesEverySelection: `all` reports each consistent selection
// once and says they are all there are.
func TestConfigureEnumeratesEverySelection(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%configure variantsAgree all")
	wants(t, got, "permits 3 selections of variants, which are all of them", "  1.", "  2.", "  3.")
	rejects(t, got, "  4.", "up to the bound")
}

// TestConfigureStopsAtItsBound: a count reports that many and says the
// enumeration was cut short rather than implying it showed all of them.
func TestConfigureStopsAtItsBound(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%configure variantsAgree all 2")
	wants(t, got, "permits 2 selections of variants, reported up to the bound", "  1.", "  2.")
	rejects(t, got, "  3.", "which are all of them")
}

// TestConfigureRejectsWhatItCannotAnswer: every malformed request is a message
// saying what to write instead, never a guess.
func TestConfigureRejectsWhatItCannotAnswer(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	cases := map[string]string{
		"%configure variantsAgree nesting=bogus":               "is not a variant of",
		"%configure variantsAgree bogus=plain":                 "is not a variation point these conditions read",
		"%configure variantsAgree nesting=plain nesting=plain": "is chosen twice",
		"%configure variantsAgree all 0":                       "is not a count of selections to report",
		"%configure variantsAgree x":                           "is not a selection",
		"%configure variantsAgree nesting=nestingTrue all":     "stands alone",
		// Part of a segment names nothing: `amily::nesting` is not ringFamily's.
		"%configure variantsAgree amily::nesting=nestingTrue": "is not a variation point these conditions read",
		"%configure variantsAgree nesting=ingTrue":            "is not a variant of",
		"%configure variantsAgree all 2 3":                    "one word too many",
		"%configure variantsAgree all 2 nesting=nestingTrue":  "one word too many",
	}
	for command, want := range cases {
		got := run(t, s, command)
		wants(t, got, "error:", want)
	}
}

// TestConfigureOnAnElementReadingNoVariation: an element with no variation point
// has no configuration to report, and the message says what to ask instead.
func TestConfigureOnAnElementReadingNoVariation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel)
	got := run(t, s, "%configure fits")
	wants(t, got, "error:", "no variation point is read", "%check fits")
}

func TestConfigureWithoutAName(t *testing.T) {
	s := checkSession(t, synthModel)
	wants(t, run(t, s, "%configure"), "usage: %configure <name>")
}

func TestConfigureReportsAnAbsentSolver(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	s := checkSession(t, synthModel)
	got := run(t, s, "%configure variantsAgree")
	wants(t, got, "error:", "z3")
	rejects(t, got, "permits")
	if reports := s.ConfigureVariants("variantsAgree", nil); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

func TestSolveAndConfigureAreListedInHelpAndCompletion(t *testing.T) {
	help := strings.Join(helpText(), "\n")
	wants(t, help, "%solve <name>", "%configure <name>")
	listed := map[string]bool{}
	for _, name := range metaCommands() {
		listed[name] = true
	}
	for _, name := range []string{"%solve", "%configure"} {
		if !listed[name] {
			t.Errorf("%s is not dispatched", name)
		}
	}
}

// %solve and %configure declare nothing, so an action debugging session survives
// them exactly as it survives %check.
func TestSolveAndConfigureKeepADebuggingSession(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, synthModel+`
package Debug {
	action def Walk {
		first start;
		then action step1;
		then done;
	}
}`)
	wants(t, run(t, s, "%action Debug::Walk"), "Walk")
	run(t, s, "%solve fits")
	run(t, s, "%configure variantsAgree all")
	rejects(t, run(t, s, "%tokens"), "no active")
}
