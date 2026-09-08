package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// Every accepted spelling reads back to itself; the empty spelling is the default.
func TestParseSchedulePolicy(t *testing.T) {
	cases := []struct{ spelling, want string }{
		{"", "reverse"},
		{"reverse", "reverse"},
		{"declared", "declared"},
		{"seed:0", "seed:0"},
		{"seed:42", "seed:42"},
		{"seed:18446744073709551615", "seed:18446744073709551615"},
	}
	for _, c := range cases {
		policy, err := ParseSchedulePolicy(c.spelling)
		if err != nil {
			t.Errorf("%q: %v", c.spelling, err)
			continue
		}
		if got := policy.String(); got != c.want {
			t.Errorf("%q: String() = %q, want %q", c.spelling, got, c.want)
		}
		if policy.IsDefault() != (c.want == "reverse") {
			t.Errorf("%q: IsDefault() = %v", c.spelling, policy.IsDefault())
		}
	}
}

// A spelling that names no policy is a typed error saying which spelling and why.
func TestParseSchedulePolicyRejectsUnknownSpellings(t *testing.T) {
	for _, spelling := range []string{"seed", "seed:", "seed:-1", "seed:abc", "seed:1.5", "seed: 1", "explore", "random", "Reverse", "declared "} {
		_, err := ParseSchedulePolicy(spelling)
		if err == nil {
			t.Errorf("%q: accepted", spelling)
			continue
		}
		var typed *SchedulePolicyError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidSchedulePolicy) {
			t.Errorf("%q: error %T %v is not a SchedulePolicyError", spelling, err, err)
			continue
		}
		if typed.Spelling != spelling || !strings.Contains(err.Error(), fmt.Sprintf("%q", spelling)) {
			t.Errorf("%q: error %q does not name the spelling", spelling, err)
		}
	}
}

// choiceModel has one of every kind of choice point an action run reports: three
// tokens steppable in one step, each writing x and order, and two holding guards.
const choiceModel = `package test {
	private import ScalarValues::*;
	action route {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;
		attribute x : Integer = 0;
		attribute order : String = "";
		first start;
		fork split;
		action a { assign x := 1; assign order := order + "a"; }
		action b { assign x := 2; assign order := order + "b"; }
		action c { assign x := 3; assign order := order + "c"; }
		join sync;
		then decide select;
			if level > 50 then warn;
			if level > 70 then alarm;
		action warn { assign handler := 1; }
		then done;
		action alarm { assign handler := 2; }
		then done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
	}
}`

// runChoiceModel runs choiceModel under policy and returns its trace, outputs
// and choice points.
func runChoiceModel(t *testing.T, policy SchedulePolicy) (string, map[string]Value, []ChoicePoint) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, choiceModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	ctx.SetSchedule(policy)
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	trace := NewTraceRecorder()
	exec.SetTrace(trace)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("%s: %v", policy, err)
	}
	return trace.String(), exec.Results(), ctx.Choices()
}

func mustPolicy(t *testing.T, spelling string) SchedulePolicy {
	t.Helper()
	policy, err := ParseSchedulePolicy(spelling)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// The default policy steps the branch declared last first and takes the first
// holding guard; `declared` steps in declaration order and takes the same guard.
func TestReverseAndDeclaredOrders(t *testing.T) {
	_, outputs, choices := runChoiceModel(t, DefaultSchedulePolicy)
	if got := FormatTraceValue(outputs["order"]); got != `"cba"` {
		t.Errorf("reverse: order = %s, want \"cba\"", got)
	}
	if got := FormatTraceValue(outputs["x"]); got != "1" {
		t.Errorf("reverse: x = %s, want 1 (the write of the branch stepped last)", got)
	}
	if got := FormatTraceValue(outputs["handler"]); got != "1" {
		t.Errorf("reverse: handler = %s, want 1 (the first holding guard)", got)
	}
	assertChoicesMatchRun(t, choices, outputs)

	_, outputs, choices = runChoiceModel(t, mustPolicy(t, "declared"))
	if got := FormatTraceValue(outputs["order"]); got != `"abc"` {
		t.Errorf("declared: order = %s, want \"abc\"", got)
	}
	if got := FormatTraceValue(outputs["x"]); got != "3" {
		t.Errorf("declared: x = %s, want 3", got)
	}
	if got := FormatTraceValue(outputs["handler"]); got != "1" {
		t.Errorf("declared: handler = %s, want 1", got)
	}
	assertChoicesMatchRun(t, choices, outputs)
}

// The same seed gives the same trace on every run; seeds differ from one another
// and from the default, and whatever a seed took is what its choice points say.
func TestSeededSchedulingIsReproducible(t *testing.T) {
	traces := make(map[string]bool)
	defaultTrace, _, _ := runChoiceModel(t, DefaultSchedulePolicy)
	traces[defaultTrace] = true
	for seed := 0; seed < 16; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		first, outputs, choices := runChoiceModel(t, policy)
		second, _, _ := runChoiceModel(t, policy)
		if first != second {
			t.Fatalf("%s: two runs differ\n=== FIRST ===\n%s\n=== SECOND ===\n%s", policy, first, second)
		}
		if len(choices) != 4 {
			t.Fatalf("%s: choices = %v, want token order, two write orders and a decision branch", policy, choices)
		}
		assertChoicesMatchRun(t, choices, outputs)
		traces[first] = true
	}
	if len(traces) < 3 {
		t.Errorf("sixteen seeds and the default gave %d distinct traces; seeds are meant to differ", len(traces))
	}
}

// assertChoicesMatchRun checks that every choice point reports the alternative
// the run observably took: the branch stepped first, the write that stood, the
// guard followed.
func assertChoicesMatchRun(t *testing.T, choices []ChoicePoint, outputs map[string]Value) {
	t.Helper()
	order := strings.Trim(FormatTraceValue(outputs["order"]), `"`)
	for _, c := range choices {
		took := c.Alternatives[c.Taken]
		switch c.Kind {
		case ChoiceTokenOrder:
			if !strings.HasSuffix(took, "@"+order[:1]) {
				t.Errorf("%s: took %q, but %q ran first", c, took, order[:1])
			}
		case ChoiceWriteOrder:
			name := strings.SplitN(took, " := ", 2)[0]
			if !strings.HasPrefix(took, name+" := "+FormatTraceValue(outputs[name])+" ") {
				t.Errorf("%s: %q stood, but %s = %s", c, took, name, FormatTraceValue(outputs[name]))
			}
		case ChoiceDecisionBranch:
			want := map[string]string{"1": "1->warn", "2": "2->alarm"}[FormatTraceValue(outputs["handler"])]
			if took != want {
				t.Errorf("%s: took %q, but handler = %s", c, took, FormatTraceValue(outputs["handler"]))
			}
		}
	}
}

// Under a seeded policy a state with two transitions enabled by one event fires
// the one the seed picks, and the choice point names that one.
func TestSeededTransitionChoiceMatchesTheRun(t *testing.T) {
	src := `package test {
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if level > 5 then low;
			transition first idle accept Go if level > 7 then high;
		}
	}`
	finals := make(map[string]bool)
	for seed := 0; seed < 8; seed++ {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
		if sym == nil {
			t.Fatal("state machine not found")
		}
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		ctx.SetSchedule(policy)
		_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
		if err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		final := visited[len(visited)-1]
		finals[final] = true
		choices := ctx.Choices()
		if len(choices) != 1 || choices[0].Kind != ChoiceTransition {
			t.Fatalf("%s: choices = %v, want one transition choice", policy, choices)
		}
		if took := choices[0].Alternatives[choices[0].Taken]; !strings.HasSuffix(took, "->"+final) {
			t.Errorf("%s: took %q but ended in %s", policy, took, final)
		}
	}
	if len(finals) != 2 {
		t.Errorf("eight seeds reached only %v; both transitions are admissible", finals)
	}
}

// A guard probe previews the run under the seed's generator and hands it back
// untouched, so probing does not shift the choices the run goes on to make.
func TestProbeLeavesTheSeededSequenceInPlace(t *testing.T) {
	s := mustPolicy(t, "seed:7").start()
	restore := s.mark()
	drawn := s.pick(1000)
	restore()
	if again := s.pick(1000); again != drawn {
		t.Errorf("after a probe drew %d the run drew %d; the probe consumed the sequence", drawn, again)
	}
}
