package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const extraArgumentsModel = `
package SM {
    attribute def Go;
    state def Lamp { entry; then off; state off; state on; transition t1 first off accept Go then on; }
    part def Dev { exhibit state lamp : Lamp; }
    part dev : Dev;
    view def V;
    view v : V { expose SM::dev; }
    constraint c { 1 < 2 }
    action act { action step; }
    occurrence def Talk;
}
`

// A command taking one argument refuses a line carrying more, as %validate and
// %metadata do, rather than running on the first word.
func TestSingleArgumentCommandsRefuseExtraWords(t *testing.T) {
	saved := filepath.Join(t.TempDir(), "out.sysml")
	for _, tc := range []struct{ line, want string }{
		{"%print SM::dev extra", "usage: %print [name]"},
		{"%save " + saved + " extra", "usage: %save <file.sysml|file.ttl|file.json>"},
		{"%verbosity quiet loud", "usage: %verbosity [quiet|normal|debug]"},
		{"%search dev extra", "usage: %search <substring>"},
		{"%view SM::v extra", "usage: %view <name>"},
		{"%graphs SM::act extra", "usage: %graphs <name>"},
		{"%instantiate SM::dev extra words", "usage: %instantiate <name>"},
		{"%check SM::c extra", "usage: %check <name>"},
		{"%explain SM::c extra", "usage: %explain <name>"},
		{"%solve SM::c extra", "usage: %solve <name>"},
		{"%break step extra", "usage: %break <node>"},
		{"%occurrence SM::Talk extra", "usage: %occurrence <name>"},
		{"%advance 2 min", `error: invalid time "2 min" (expected a number of time units, e.g. 30 or 30s)`},
		{"%advance 1 bogus junk", `error: invalid time "1 bogus junk" (expected a number of time units, e.g. 30 or 30s)`},
	} {
		t.Run(tc.line, func(t *testing.T) {
			s := NewSession()
			submitModel(t, s, extraArgumentsModel)
			wantLines(t, mustMeta(t, s, tc.line), []string{tc.want})
		})
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Errorf("%%save with extra words wrote %s (stat error %v)", saved, err)
	}
}

// A refused line changes nothing: the clock stays put, no instance is created
// and the output level is kept.
func TestRefusedExtraWordsLeaveTheSessionUntouched(t *testing.T) {
	s := NewSession()
	submitModel(t, s, extraArgumentsModel)

	run(t, s, "%instantiate SM::dev extra words")
	if got := run(t, s, "%instances"); strings.Contains(got, "SM::dev") {
		t.Errorf("refused %%instantiate created an instance:\n%s", got)
	}
	run(t, s, "%verbosity quiet loud")
	if got := s.Verbosity(); got != VerbosityNormal {
		t.Errorf("refused %%verbosity set the level to %s", got)
	}

	run(t, s, "%instantiate SM::dev")
	run(t, s, "%state SM::dev")
	run(t, s, "%advance 2 min")
	run(t, s, "%advance 1 bogus junk")
	wants(t, run(t, s, "%advance 1"), "simulation time is now 1.0")
}

// The clock's unit may follow the number, attached or apart, bare or bracketed.
func TestAdvanceReadsATrailingSecondsUnit(t *testing.T) {
	s := NewSession()
	submitModel(t, s, extraArgumentsModel)
	run(t, s, "%instantiate SM::dev")
	run(t, s, "%state SM::dev")
	for _, step := range []struct{ line, now string }{
		{"%advance 1s", "1.0"}, {"%advance 1 s", "2.0"}, {"%advance 1[s]", "3.0"}, {"%advance 1 [s]", "4.0"},
		{"%advance 1 [SI::s]", "5.0"}, {"%advance 1[SI::s]", "6.0"},
	} {
		wants(t, run(t, s, step.line), "simulation time is now "+step.now)
	}
}
