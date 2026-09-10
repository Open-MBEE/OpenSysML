package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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
	for _, spelling := range []string{"seed", "seed:", "seed:-1", "seed:abc", "seed:1.5", "seed: 1", "explore:", "explore:width=3", "random", "Reverse", "declared "} {
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
	mustSchedule(t, ctx, policy)
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
		mustSchedule(t, ctx, policy)
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

// The guard of the branch a decision takes is read by the run itself: a branch
// picked past the first was only previewed, so the run reads it again for real,
// while a branch it did not take leaves nothing but its choice in the trace.
func TestPickedGuardIsReadByTheRun(t *testing.T) {
	const alarmGuard = "eval literal 70 -> 70"
	took := make(map[string]bool)
	for seed := 0; seed < 16; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		trace, outputs, _ := runChoiceModel(t, policy)
		handler := FormatTraceValue(outputs["handler"])
		took[handler] = true
		if read := strings.Contains(trace, alarmGuard); read != (handler == "2") {
			t.Errorf("%s: handler = %s but the run's reading of the alarm guard in the trace is %v\n%s", policy, handler, read, trace)
		}
	}
	if len(took) != 2 {
		t.Fatalf("sixteen seeds took only handler %v; both guards hold", took)
	}
}

// stepChoiceModel drives exec step by step to completion, calling between after
// the first step, and returns the run's trace.
func stepChoiceModel(t *testing.T, exec *ActionExecutor, between func()) string {
	t.Helper()
	trace := NewTraceRecorder()
	exec.SetTrace(trace)
	for i := 0; exec.State() != StateCompleted; i++ {
		if i > 50 {
			t.Fatal("the run did not complete in fifty steps")
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if i == 0 {
			between()
			exec.SetTrace(trace)
		}
	}
	return trace.String()
}

// A run a debugger drives step by step keeps drawing from its own seeded sequence
// when another run, under another policy, is driven to completion in between.
func TestDrivenRunKeepsItsSchedulerAcrossOtherRuns(t *testing.T) {
	for seed := 0; seed < 16; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, choiceModel))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
		if sym == nil {
			t.Fatal("action not found")
		}
		newExecutor := func(policy SchedulePolicy) *ActionExecutor {
			mustSchedule(t, ctx, policy)
			exec, err := ctx.CreateActionExecutor(sym)
			if err != nil {
				t.Fatalf("create executor: %v", err)
			}
			return exec
		}
		alone := stepChoiceModel(t, newExecutor(policy), func() {})
		interrupted := stepChoiceModel(t, newExecutor(policy), func() {
			ctx.SetTrace(NewTraceRecorder())
			other := newExecutor(mustPolicy(t, "declared"))
			if err := other.RunToCompletion(); err != nil {
				t.Fatalf("run in between: %v", err)
			}
		})
		if alone != interrupted {
			t.Errorf("%s: the run driven across another run differs from the same run driven alone\n=== ALONE ===\n%s\n=== INTERRUPTED ===\n%s", policy, alone, interrupted)
		}
	}
}

// A decision previewed for a machine a debugger drives names the transition that
// machine's own run fires next, after other runs have used the context, and the
// preview consumes none of the run's draws.
func TestDecidePredictsTheDrivenRunsTransition(t *testing.T) {
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
	for seed := 0; seed < 16; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
		if sym == nil {
			t.Fatal("state machine not found")
		}
		mustSchedule(t, ctx, policy)
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatalf("%s: create executor: %v", policy, err)
		}
		// Another run in between draws from a scheduler of its own and leaves it behind.
		if _, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"}); err != nil {
			t.Fatalf("%s: run in between: %v", policy, err)
		}
		msg := Message{SignalType: "Go"}
		first, err := exec.Decide(msg)
		if err != nil || len(first.Fires) != 1 {
			t.Fatalf("%s: Decide(Go) = %+v, %v; want one transition firing", policy, first, err)
		}
		if again, err := exec.Decide(msg); err != nil || again.Fires[0] != first.Fires[0] {
			t.Errorf("%s: a second Decide(Go) = %+v, %v; want %q again, the preview drawing nothing", policy, again, err, first.Fires[0])
		}
		exec.SendSignal("Go", nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("%s: ProcessNextEvent: %v", policy, err)
		}
		final := activeLeaf(exec)
		finals[final] = true
		if !strings.HasSuffix(first.Fires[0], "-> "+final) {
			t.Errorf("%s: Decide(Go) named %q but the run fired into %s", policy, first.Fires[0], final)
		}
	}
	if len(finals) != 2 {
		t.Errorf("sixteen seeds reached only %v; both transitions are admissible", finals)
	}
}

// Tokens parked at accepts nothing in flight answers are not a choice, so steps
// taken while they wait draw nothing: the choices once the messages arrive are the
// same however many idle steps a debugger took first.
func TestParkedTokensDrawNothing(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action listen {
			attribute a : Integer = 0;
			attribute b : Integer = 0;
			first start;
			fork split;
			action left accept x : Integer;
			action right accept y : Integer;
			action la { assign a := 1; }
			action ra { assign b := 1; }
			join sync;
			done;
			succession first start then split;
			succession first split then left;
			succession first split then right;
			succession first left then la;
			succession first right then ra;
			succession first la then sync;
			succession first ra then sync;
			succession first sync then done;
		}
	}`
	one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	choicesAfterIdling := func(policy SchedulePolicy, idle int) []ChoicePoint {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "listen", ast.DefAction)
		if sym == nil {
			t.Fatal("action not found")
		}
		mustSchedule(t, ctx, policy)
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("%s: create executor: %v", policy, err)
		}
		for i := 0; exec.State() != StateWaiting; i++ {
			if i > 10 {
				t.Fatalf("%s: the accepts did not park in ten steps", policy)
			}
			if err := exec.Step(); err != nil {
				t.Fatalf("%s: step %d: %v", policy, i, err)
			}
		}
		for i := 0; i < idle; i++ {
			if err := exec.Step(); err != nil {
				t.Fatalf("%s: idle step %d: %v", policy, i, err)
			}
		}
		if got := ctx.Choices(); len(got) != 0 {
			t.Fatalf("%s: choices while parked = %v, want none", policy, got)
		}
		ctx.PostMessage(Message{SignalType: "Integer", Value: &one})
		ctx.PostMessage(Message{SignalType: "Integer", Value: &one})
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("%s: run on: %v", policy, err)
		}
		choices := ctx.Choices()
		if len(choices) != 2 {
			t.Fatalf("%s: choices = %v, want the accepts' order and the assignments' order", policy, choices)
		}
		return choices
	}
	for seed := 0; seed < 16; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		prompt := choicesAfterIdling(policy, 0)
		idled := choicesAfterIdling(policy, 5)
		for i := range prompt {
			if prompt[i].Taken != idled[i].Taken {
				t.Errorf("%s: choice %d took %d after five idle steps, %d without them (%v)",
					policy, i, idled[i].Taken, prompt[i].Taken, idled[i])
			}
		}
	}
}

// A composite state reached from every leaf of its orthogonal regions offers one
// transition per dispatch or change poll: the choice among its enabled
// transitions draws once, so the run takes the seed's first draw and its next
// choice takes the second.
func TestSharedAncestorChoiceDrawsOnce(t *testing.T) {
	const regions = `state busy parallel {
				state left { entry; then l; state l; }
				state right { entry; then r; state r; }
			}
			state low;
			state high;
			state calm;
			state loud;
			transition first low accept Next if level > 5 then calm;
			transition first low accept Next if level > 7 then loud;
			transition first high accept Next if level > 5 then calm;
			transition first high accept Next if level > 7 then loud;`
	cases := []struct {
		name   string
		src    string
		events []string
	}{
		{"dispatch", `package test {
			state Dispatcher {
				attribute level : Integer = 8;
				entry; then busy;
				` + regions + `
				transition first busy accept Go if level > 5 then low;
				transition first busy accept Go if level > 7 then high;
			}
		}`, []string{"Go", "Next"}},
		{"change poll", `package test {
			state Dispatcher {
				attribute level : Integer = 0;
				entry; then start;
				state start;
				` + regions + `
				transition first start do assign level := 8 then busy;
				transition first busy accept when level > 5 then low;
				transition first busy accept when level > 7 then high;
			}
		}`, []string{"Next"}},
	}
	for _, tc := range cases {
		for seed := 0; seed < 16; seed++ {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, tc.src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
			if sym == nil {
				t.Fatal("state machine not found")
			}
			policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
			mustSchedule(t, ctx, policy)
			_, visited, err := ctx.ExecuteStateWithEvents(sym, tc.events)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.name, policy, err)
			}
			choices := ctx.Choices()
			if len(choices) != 2 {
				t.Fatalf("%s %s: choices = %v, want one out of busy and one after it (visited %v)",
					tc.name, policy, choices, visited)
			}
			draws := policy.start()
			for i, choice := range choices {
				if choice.Kind != ChoiceTransition {
					t.Fatalf("%s %s: choice %d is %v, want a transition choice", tc.name, policy, i, choice)
				}
				if want := draws.pick(2); choice.Taken != want {
					t.Errorf("%s %s: choice %d took %d, the seed's draw is %d (%v)",
						tc.name, policy, i, choice.Taken, want, choice)
				}
			}
		}
	}
}

// A composite state with several enabled transitions loses to a nested state that
// also reacts: the choice it never got to make draws nothing, so the run's first
// reported choice takes the seed's first draw.
func TestOutrankedChoiceDrawsNothing(t *testing.T) {
	const after = `state low;
			state high;
			state calm;
			state loud;
			transition first busy accept Next if level > 5 then low;
			transition first busy accept Next if level > 7 then high;
			transition first low accept Then if level > 5 then calm;
			transition first low accept Then if level > 7 then loud;
			transition first high accept Then if level > 5 then calm;
			transition first high accept Then if level > 7 then loud;`
	cases := []struct {
		name   string
		src    string
		events []string
	}{
		{"dispatch", `package test {
			state Dispatcher {
				attribute level : Integer = 8;
				entry; then busy;
				state busy parallel {
					state left { entry; then l; state l; state l2; transition first l accept Go then l2; }
					state right { entry; then r; state r; }
				}
				transition first busy accept Go if level > 5 then low;
				transition first busy accept Go if level > 7 then high;
				` + after + `
			}
		}`, []string{"Go", "Next", "Then"}},
		{"change poll", `package test {
			state Dispatcher {
				attribute level : Integer = 0;
				entry; then start;
				state start;
				state busy parallel {
					state left { entry; then l; state l; state l2; transition first l accept when level > 5 then l2; }
					state right { entry; then r; state r; }
				}
				transition first start do assign level := 8 then busy;
				transition first busy accept when level > 5 then low;
				transition first busy accept when level > 7 then high;
				` + after + `
			}
		}`, []string{"Next", "Then"}},
	}
	for _, tc := range cases {
		for seed := 0; seed < 16; seed++ {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, tc.src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
			if sym == nil {
				t.Fatal("state machine not found")
			}
			policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
			mustSchedule(t, ctx, policy)
			_, visited, err := ctx.ExecuteStateWithEvents(sym, tc.events)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.name, policy, err)
			}
			if !slices.Contains(visited, "l2") {
				t.Fatalf("%s %s: the nested transition did not fire (visited %v)", tc.name, policy, visited)
			}
			choices := ctx.Choices()
			if len(choices) != 2 {
				t.Fatalf("%s %s: choices = %v, want one out of busy and one after it (visited %v)",
					tc.name, policy, choices, visited)
			}
			draws := policy.start()
			for i, choice := range choices {
				if choice.Kind != ChoiceTransition {
					t.Fatalf("%s %s: choice %d is %v, want a transition choice", tc.name, policy, i, choice)
				}
				if want := draws.pick(2); choice.Taken != want {
					t.Errorf("%s %s: choice %d took %d, the seed's draw is %d (%v)",
						tc.name, policy, i, choice.Taken, want, choice)
				}
			}
		}
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

// dueOrderChoices runs machines whose timers fall due at one instant of the
// shared clock under policy and returns the run's choices.
func dueOrderChoices(t *testing.T, policy SchedulePolicy, machines int) []ChoicePoint {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			state metronome {
				attribute beat : Integer = 0;
				entry; then ticking;
				state ticking;
				state ticked;
				transition ticking then ticked accept after 5 [s] do assign beat := beat + 1;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "metronome", ast.DefState)
	if sym == nil {
		t.Fatal("state metronome not found")
	}
	ctx.SetSchedule(policy)
	execs := make([]*StateExecutor, machines)
	for i := range execs {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatalf("create machine %d: %v", i, err)
		}
		execs[i] = exec
	}
	report, err := ctx.Advance(5)
	if err != nil {
		t.Fatalf("%s: Advance: %v", policy, err)
	}
	if report.Events != int64(machines) {
		t.Errorf("%s: %d events dispatched, want one per machine (%d)", policy, report.Events, machines)
	}
	for i, exec := range execs {
		if got := exec.CurrentState(); got == nil || getNodeName(got) != "ticked" {
			t.Errorf("%s: machine %d in %v after the advance, want ticked", policy, i, got)
		}
		if got := exec.StateData()["beat"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("%s: machine %d beat = %v, want 1", policy, i, got)
		}
	}
	return ctx.Choices()
}

// Executors due at one instant are a choice point (last created first by default,
// first under `declared`, a draw under a seed); one executor due is no choice.
func TestDueOrderChoice(t *testing.T) {
	if choices := dueOrderChoices(t, DefaultSchedulePolicy, 1); len(choices) != 0 {
		t.Errorf("one machine due: choices = %v, want none", choices)
	}

	for _, tc := range []struct {
		policy string
		taken  int
	}{{"reverse", 1}, {"declared", 0}} {
		choices := dueOrderChoices(t, mustPolicy(t, tc.policy), 2)
		if len(choices) != 1 {
			t.Fatalf("%s: choices = %v, want the one due-order choice", tc.policy, choices)
		}
		c := choices[0]
		if c.Kind != ChoiceDueOrder || c.Where != "t=5.0" || len(c.Alternatives) != 2 || c.Taken != tc.taken {
			t.Errorf("%s: choice = %+v, want a due order at t=5.0 over two machines taking %d", tc.policy, c, tc.taken)
		}
		want := fmt.Sprintf("at t=5.0: due %s, %s (unordered; ran %s first)",
			c.Alternatives[0], c.Alternatives[1], c.Alternatives[tc.taken])
		if got := c.Describe(); got != want {
			t.Errorf("%s: Describe() = %q, want %q", tc.policy, got, want)
		}
	}

	taken := map[int]bool{}
	for seed := 0; seed < 16; seed++ {
		choices := dueOrderChoices(t, mustPolicy(t, fmt.Sprintf("seed:%d", seed)), 2)
		if len(choices) != 1 || choices[0].Kind != ChoiceDueOrder {
			t.Fatalf("seed:%d: choices = %v, want one due-order choice", seed, choices)
		}
		taken[choices[0].Taken] = true
	}
	if !taken[0] || !taken[1] {
		t.Errorf("sixteen seeds took only %v; want both orders drawn", taken)
	}
}

// changeWatchOrder advances a clock past the instant a raiser sets a value two
// machines watch for, each taking the value it sees and bumping it, under policy;
// it returns what each machine saw and the run's choices.
func changeWatchOrder(t *testing.T, policy SchedulePolicy) ([]int64, []ChoicePoint) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			part def Cell { attribute mark : Integer = 0; }
			part cell : Cell;
			state raiser {
				entry; then holding;
				state holding;
				accept after 2 [s] then raised;
				state raised { entry assign cell.mark := 1; }
			}
			state taker {
				attribute seen : Integer = -1;
				entry; then waiting;
				state waiting;
				accept when cell.mark > 0 then took;
				state took {
					entry action take { assign seen := cell.mark; assign cell.mark := cell.mark + 1; }
				}
			}
		}
	`))
	root := idx.DocumentRoot("<test>")
	ctx.SetSchedule(policy)
	if _, err := ctx.CreateStateExecutor(findSymbolByName(root, "raiser", ast.DefState)); err != nil {
		t.Fatalf("create raiser: %v", err)
	}
	taker := findSymbolByName(root, "taker", ast.DefState)
	takers := make([]*StateExecutor, 2)
	for i := range takers {
		exec, err := ctx.CreateStateExecutor(taker)
		if err != nil {
			t.Fatalf("create taker %d: %v", i, err)
		}
		takers[i] = exec
	}
	if _, err := ctx.Advance(2); err != nil {
		t.Fatalf("%s: Advance: %v", policy, err)
	}
	seen := make([]int64, len(takers))
	for i, exec := range takers {
		if got := exec.CurrentState(); got == nil || getNodeName(got) != "took" {
			t.Errorf("%s: taker %d in %v after the advance, want took", policy, i, got)
		}
		v := exec.StateData()["seen"]
		if v.Kind != ValConst {
			t.Fatalf("%s: taker %d seen = %v, want an integer", policy, i, v)
		}
		seen[i] = v.Const.Int
	}
	return seen, ctx.Choices()
}

// Machines polling a change condition at one instant are a due-order choice like
// any other work due: the one drawn polls first and what it does is what the next sees.
func TestChangeWatchOrderChoice(t *testing.T) {
	// The taker drawn first sees the raised 1 and leaves 2 for the other.
	wantSeen := func(t *testing.T, policy string, seen []int64, first int) {
		t.Helper()
		if seen[first] != 1 || seen[1-first] != 2 {
			t.Errorf("%s: takers saw %v, want taker %d to see 1 and the other 2", policy, seen, first)
		}
	}
	for _, tc := range []struct {
		policy string
		taken  int
	}{{"reverse", 1}, {"declared", 0}} {
		seen, choices := changeWatchOrder(t, mustPolicy(t, tc.policy))
		if len(choices) != 2 {
			t.Fatalf("%s: choices = %v, want the two polls of both takers, at t=0.0 and t=2.0", tc.policy, choices)
		}
		if c := choices[0]; c.Kind != ChoiceDueOrder || c.Where != "t=0.0" || len(c.Alternatives) != 2 {
			t.Errorf("%s: choice = %+v, want a due order at t=0.0 over the two takers", tc.policy, c)
		}
		c := choices[1]
		if c.Kind != ChoiceDueOrder || c.Where != "t=2.0" || len(c.Alternatives) != 2 || c.Taken != tc.taken {
			t.Errorf("%s: choice = %+v, want a due order at t=2.0 over the two takers taking %d", tc.policy, c, tc.taken)
		}
		wantSeen(t, tc.policy, seen, tc.taken)
	}

	taken := map[int]bool{}
	for seed := 0; seed < 16; seed++ {
		policy := fmt.Sprintf("seed:%d", seed)
		seen, choices := changeWatchOrder(t, mustPolicy(t, policy))
		if len(choices) != 2 || choices[1].Kind != ChoiceDueOrder || choices[1].Where != "t=2.0" {
			t.Fatalf("%s: choices = %v, want the due-order choice at t=2.0 after the one at t=0.0", policy, choices)
		}
		wantSeen(t, policy, seen, choices[1].Taken)
		taken[choices[1].Taken] = true
	}
	if !taken[0] || !taken[1] {
		t.Errorf("sixteen seeds took only %v; want both orders drawn", taken)
	}
}
