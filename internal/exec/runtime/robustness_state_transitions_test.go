package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessStateTransitions covers transitions whose endpoints do not resolve,
// pseudostates with nothing enabled, completion events and history.
func TestRuntimeRobustnessStateTransitions(t *testing.T) {
	t.Run("state_dangling_transition", testStateDanglingTransition)
	t.Run("state_transition_endpoint_misspelled", testStateTransitionEndpointMisspelled)
	t.Run("state_transition_endpoint_in_another_machine", testStateTransitionEndpointInAnotherMachine)
	t.Run("state_transition_endpoint_never_resolved", testStateTransitionEndpointNeverResolved)
	t.Run("state_transition_endpoint_naming_a_first_marker", testStateTransitionEndpointNamingAFirstMarker)
	t.Run("state_junction_without_an_outgoing_transition", testStateJunctionWithoutAnOutgoingTransition)
	t.Run("state_choice_without_an_enabled_branch", testStateChoiceWithoutAnEnabledBranch)
	t.Run("state_event_after_completion", testStateEventAfterCompletion)
	t.Run("state_completion_rests_in_done", testStateCompletionRestsInDone)
	t.Run("state_nested_region_completion_keeps_siblings_running", testStateNestedRegionCompletionKeepsSiblingsRunning)
	t.Run("state_transition_without_a_target", testStateTransitionWithoutATarget)
	t.Run("state_transition_effect_reads_an_unknown_feature", testStateTransitionEffectReadsAnUnknownFeature)
	t.Run("state_cross_region_transitions_ping_pong", testStateCrossRegionTransitionsPingPong)
	t.Run("sourceless_transition_with_nothing_before", testSourcelessTransitionWithNothingBefore)
	t.Run("sourceless_transition_after_a_non_state", testSourcelessTransitionAfterANonState)
	t.Run("region_pseudostate_without_satisfied_guard", testRegionPseudostateWithoutSatisfiedGuard)
	t.Run("region_pseudostate_cycle", testRegionPseudostateCycle)
	t.Run("history_outside_composite_state", testHistoryOutsideCompositeState)
	t.Run("history_without_record_default_or_entry", testHistoryWithoutRecordDefaultOrEntry)
	t.Run("signal_no_level_of_a_composite_state_accepts", testSignalNoLevelOfACompositeStateAccepts)
	t.Run("stale_composite_timer_in_a_region", testStaleCompositeTimerInARegion)
	t.Run("composite_self_transition_with_no_substate_to_re_enter", testCompositeSelfTransitionWithNoSubstateToReEnter)
	t.Run("exit_of_nested_regions_with_a_history_pseudostate", testExitOfNestedRegionsWithAHistoryPseudostate)
}

// testStateTransitionEndpointMisspelled: a misspelled endpoint is a
// name-resolution diagnostic, so lowering leaves the edge out and the machine
// runs to a halt in the state it reached rather than panicking or hanging.
func testStateTransitionEndpointMisspelled(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)

	var endpoint *resolve.Diagnostic
	for i, diag := range ctx.model.resolver.Diagnostics {
		if strings.Contains(diag.Message, "donee") {
			endpoint = &ctx.model.resolver.Diagnostics[i]
		}
	}
	if endpoint == nil {
		t.Fatalf("expected a name-resolution diagnostic for 'donee', got: %v", ctx.model.resolver.Diagnostics)
	}
	if endpoint.Code != "unresolved" {
		t.Errorf("expected code %q, got %q", "unresolved", endpoint.Code)
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on a machine whose transition names nothing")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointNeverResolved: executed without a name-resolution
// pass, as the REPL and the service handlers do, an endpoint naming nothing
// leaves its edge out; the machine still runs, and the misspelling is reported
// by whoever resolves the document rather than by lowering.
func testStateTransitionEndpointNeverResolved(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on an endpoint no resolution pass reported")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointInAnotherMachine: an endpoint naming a state of a
// different machine resolves, so no name diagnostic reports it; the state
// transition check reports it, and lowering backstops the check with a typed
// error rather than dropping the edge.
func testStateTransitionEndpointInAnotherMachine(t *testing.T) {
	src := `package test {
		state Other {
			entry; then start;
			state start;
			state running;
			succession first start then running;
		}
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then Other::running;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)

	for _, diag := range ctx.model.resolver.Diagnostics {
		if strings.Contains(diag.Message, "running") {
			t.Fatalf("the endpoint resolves, so name resolution reports nothing: %v", diag)
		}
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint that is not a vertex of this machine")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex of this machine, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateTransitionEndpointNamingAFirstMarker: a member that is no vertex —
// here an entry action — still makes an endpoint name one, which the state
// transition check reports and is backstopped here with a typed error rather
// than a panic.
func testStateTransitionEndpointNamingAFirstMarker(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			state other;
			entry marker { }
			succession first init then busy;
			transition first busy then marker;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint naming a marker rather than a vertex")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateJunctionWithoutAnOutgoingTransition: a junction no transition leaves
// routes a transition reaching it nowhere, which the state transition check
// reports; reaching it at run time errors rather than panicking or hanging.
// testStateEventAfterCompletion: a signal sent to a machine that already reached
// `done` is discarded, leaving it completed rather than restarting or panicking.
func testStateEventAfterCompletion(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			transition first init accept stop then done;
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after completion: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("the machine left completion on a late signal, state %s", exec.State())
	}
}

// testStateCompletionRestsInDone: a completed machine names `done` as the state
// it came to rest in, so a caller reading the configuration sees a state, not nil.
func testStateCompletionRestsInDone(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy accept stop then done;
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	assertCurrentState(t, exec, ast.DoneFeature)
}

// testStateNestedRegionCompletionKeepsSiblingsRunning: a region of a composite
// state reaching `done` leaves its sibling region running, and events it keeps
// answering are still delivered rather than dropped by an early completion.
func testStateNestedRegionCompletionKeepsSiblingsRunning(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then busy;
			state busy parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept stop then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					state rbusy;
					transition first rstart accept go then rbusy;
				}
			}
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with the sibling region still running")
	}
	exec.SendSignal("go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the first region completed: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Errorf("the machine completed while the sibling region rests outside `done`")
	}
	if got := exec.finalStateName(); got != "done+rbusy" {
		t.Errorf("expected the regions in done+rbusy, got %q", got)
	}
}

func testStateJunctionWithoutAnOutgoingTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			junction stuck;
			succession first init then busy;
			transition first busy then stuck;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error for a junction no transition leaves")
		}
		if !strings.Contains(err.Error(), "junction stuck has no outgoing transitions") {
			t.Errorf("expected the error to name the junction, got %v", err)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on a junction no transition leaves")
	}
}

// testStateChoiceWithoutAnEnabledBranch: a choice's guards are read once the
// transition into it has run its effect; when none holds and no unguarded branch
// remains, the run fails at that instant with a typed error naming the choice.
func testStateChoiceWithoutAnEnabledBranch(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute x : Integer = 0;
			entry; then init;
			state init;
			state busy;
			choice pick;
			state seen;
			succession first init then busy;
			transition first busy do assign x := 2 then pick;
			transition first pick if x == 1 then seen;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if !errors.Is(err, errChoiceWithoutBranch) {
			t.Fatalf("expected errChoiceWithoutBranch: the incoming effect wrote x := 2 and the only branch wants 1; got %v", err)
		}
		if !strings.Contains(err.Error(), "pick") {
			t.Errorf("expected the error to name the choice, got %v", err)
		}
		if x := exec.StateData()["x"]; !valueEqual(x, integerValue(2)) {
			t.Errorf("x = %v, want 2: the incoming effect had run when the choice was read", x)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on a choice no branch leaves")
	}
}

// testStateTransitionWithoutATarget: a transition with no target names no edge,
// so lowering reports it rather than dereferencing the absent target.
func testStateTransitionWithoutATarget(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 1000)

	dangling := transitionMember("init", "busy")
	dangling.Target = nil
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			&ast.StateNode{Name: "busy"},
			dangling,
		},
	}

	_, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err == nil {
		t.Fatal("expected an error for a transition without a target")
	}
	if !strings.Contains(err.Error(), "names no target") {
		t.Errorf("expected a missing-target error, got: %v", err)
	}
}

// testStateCrossRegionTransitionsPingPong: guardless successions crossing back
// and forth between two regions never settle, so the event budget bounds the run
// with a typed error instead of hanging.
func testStateCrossRegionTransitionsPingPong(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state running parallel {
				state left {
					entry; then ls;
					state ls;
					state lidle;
					succession first ls then lidle;
					transition first lidle then rtarget;
				}
				state right {
					entry; then rs;
					state rs;
					state ridle;
					state rtarget;
					succession first rs then ridle;
					transition first rtarget then lidle;
				}
			}
			succession first init then running;
		}
	}`)
	exec.ctx.maxStateEvents = 50

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	var err error
	select {
	case err = <-done:
	case <-watchdog(30 * time.Second):
		t.Fatal("run to completion hangs on successions crossing between regions")
	}
	if err == nil {
		t.Fatal("expected a budget error for cross-region successions that never settle")
	}
	if !strings.Contains(err.Error(), MaxStateEventsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxStateEventsEnvVar)
	}
}

// testStateTransitionEffectReadsAnUnknownFeature: a statement written as a
// transition's effect executes lowered like any other, so one reading a feature
// the machine does not declare reports rather than firing on a missing value.
func testStateTransitionEffectReadsAnUnknownFeature(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute counter : Integer = 0;
			entry; then init;
			state init;
			state active;
			succession first init then active;
			transition first active do assign counter := missingName + 1 then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
}

// testSignalNoLevelOfACompositeStateAccepts: a signal neither the active substate
// nor any composite state enclosing it accepts is dropped by run-to-completion,
// so walking outward for a trigger ends in the machine standing still rather than
// erroring or hanging.
func testSignalNoLevelOfACompositeStateAccepts(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state outer {
				state middle {
					state inner;
					state other;
					transition first inner accept step then other;
				}
				state recovered;
				transition first middle accept abort then recovered;
			}
			state stopped;
			succession first init then inner;
			transition first outer accept shutdown then stopped;
		}
	}`)
	exec.SendSignal("unknown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "inner" {
		t.Errorf("expected the unaccepted signal to leave the machine in inner, got %v", exec.CurrentState())
	}
}

// testCompositeSelfTransitionWithNoSubstateToReEnter: a composite state that
// declares no starting substate is re-entered by its own self-transition without
// erroring or hanging, and stays active with no substate of its own.
func testCompositeSelfTransitionWithNoSubstateToReEnter(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state Working {
				state Step1;
			}
			succession first init then Working::Step1;
			transition first Working accept restart then Working;
		}
	}`)
	exec.SendSignal("restart", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "Working" {
		t.Errorf("expected the re-entered composite state to be active, got %v", exec.CurrentState())
	}
}

// testStaleCompositeTimerInARegion: a time trigger on a composite state inside an
// orthogonal region whose composite is left before the timer expires is dropped,
// leaving the sibling region where it was rather than erroring or hanging.
func testStaleCompositeTimerInARegion(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state working parallel {
				state left {
					entry; then lstart;
					state lstart;
					state grouping {
						state step1;
						accept after 5 then late;
					}
					state moved;
					state late;
					transition first lstart then step1;
					transition first grouping accept skip then moved;
				}
				state right {
					entry; then rstart;
					state rstart;
					state watching;
					succession first rstart then watching;
				}
			}
			succession first init then working;
		}
	}`)
	exec.SendSignal("skip", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["moved"] || !active["watching"] || active["late"] {
		t.Errorf("expected the stale composite timer to leave moved and watching active, got %v", active)
	}
}

// testExitOfNestedRegionsWithAHistoryPseudostate: leaving a composite state whose
// region holds another composite with a region of its own, then returning through a
// deep history, restores the recorded configuration rather than erroring or hanging.
func testExitOfNestedRegionsWithAHistoryPseudostate(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state outer parallel {
				state left {
					entry; then lstart;
					state lstart;
					state grouping parallel {
						state inner {
							entry; then gstart;
							state gstart;
							state g1;
							state g2;
							transition first gstart then g1;
							transition first g1 accept advance then g2;
						}
					}
					transition first lstart then grouping;
				}
				state right {
					entry; then rstart;
					state rstart;
					state watching;
					transition first rstart then watching;
				}
				deep history resume;
			}
			state away;
			succession first init then outer;
			transition first outer accept leave then away;
			transition first away accept back then resume;
		}
	}`)
	for _, signal := range []string{"advance", "leave", "back"} {
		exec.SendSignal(signal, nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run to completion after %s: %v", signal, err)
		}
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["g2"] || !active["watching"] {
		t.Errorf("expected the deep history to restore g2 and watching, got %v", active)
	}
}

// testHistoryOutsideCompositeState: a history in a parallel machine's own body
// belongs to no region and no composite state, so it must report, not guess.
func testHistoryOutsideCompositeState(t *testing.T) {
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:       ast.UsageState,
		Ident:      ast.Identification{Name: "Machine"},
		IsParallel: true,
		Members: []ast.Node{
			&ast.StateNode{Name: "left", Substates: []ast.Node{
				entryStart("init"),
				&ast.StateNode{Name: "init"},
				&ast.StateNode{Name: "away"},
				transitionMember("init", "away"),
				transitionMember("away", "H"),
			}},
			&ast.StateNode{Name: "right", Substates: []ast.Node{
				entryStart("idle"),
				&ast.StateNode{Name: "idle"},
			}},
			&ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"},
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	_, err := exec.resolveAndFire(nil, transitionBetween(t, exec, "away", "H"), nil, nil)
	if err == nil {
		t.Fatal("expected an error for a history outside any composite state")
	}
	if !strings.Contains(err.Error(), "must be declared inside the composite state") {
		t.Errorf("expected an ownership error, got: %v", err)
	}
}

// testHistoryWithoutRecordDefaultOrEntry: before its composite state has ever
// been exited a history has nothing to restore; with no default transition it
// falls back on the owner's entry transition, and when the owner declares none
// either the run fails with a typed error at the transition, not silently.
func testHistoryWithoutRecordDefaultOrEntry(t *testing.T) {
	history := &ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"}
	outer := &ast.StateNode{
		Name:      "outer",
		Substates: []ast.Node{&ast.StateNode{Name: "first"}, history},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	_, err := exec.resolveAndFire(nil, transitionBetween(t, exec, "away", "H"), nil, nil)
	if !errors.Is(err, errHistoryWithoutEntry) {
		t.Fatalf("expected errHistoryWithoutEntry: nothing recorded, no default transition and outer has no entry transition; got %v", err)
	}
	for _, name := range []string{"H", "outer"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name %s, got: %v", name, err)
		}
	}
	if current := exec.getCurrentState(); current == nil || current.Name != "away" {
		t.Errorf("a failed history transition leaves the machine where it was, got %v", current)
	}
}

// testRegionPseudostateWithoutSatisfiedGuard: a completion through a junction
// with no enabled branch is dropped, leaving its source active.
func testRegionPseudostateWithoutSatisfiedGuard(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine parallel {
			attribute x : Integer = 9;

			state left {
				entry; then ls;
				state ls;
				state a;
				state b;
				succession first ls then a;
				transition first a then merge;
			}
			state right {
				entry; then rs;
				state rs;
				state c;
				succession first rs then c;
			}
			junction merge;

			transition first merge if x == 1 then b;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	var a *ast.StateNode
	for _, state := range exec.graph.States {
		if state.Name == "a" {
			a = state
			break
		}
	}
	if a == nil || !exec.inActiveConfiguration(a) {
		t.Fatalf("active states = %s, want a to remain active", activeStateNames(exec))
	}
}

// testRegionPseudostateCycle: pseudostates that route into each other never
// reach a state, so following the chain has to report the cycle instead of
// looping forever.
func testRegionPseudostateCycle(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine parallel {
			state left {
				entry; then ls;
				state ls;
				state a;
				succession first ls then a;
				transition first a then first;
			}
			state right {
				entry; then rs;
				state rs;
				state c;
				succession first rs then c;
			}
			junction first;
			junction second;

			transition first first then second;
			transition first second then first;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for pseudostates routing into each other")
	}
	if !strings.Contains(err.Error(), "form a cycle") {
		t.Errorf("expected a cycle error, got: %v", err)
	}
}

// testStateDanglingTransition: state with transition to nonexistent state
func testStateDanglingTransition(t *testing.T) {
	src := `
		package test {
			state Machine {
				entry; then init;
				state init;
				succession first init then nowhere; // 'nowhere' state doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	// Check diagnostics (resolver should catch missing state)
	// Note: resolver diagnostics accessed via resolver.Diagnostics field

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Broken state not found")
	}

	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Logf("CreateStateExecutor error (acceptable): %v", err)
		return
	}

	err = exec.ProcessNextEvent()
	if err != nil {
		t.Logf("ProcessNextEvent returned error (acceptable): %v", err)
		return
	}

	t.Log("ProcessNextEvent succeeded (dangling transition not exercised)")
}

// testSourcelessTransitionWithNothingBefore: a transition written without a
// source leaves the state declared before it (SysML v2 7.18.3); as the first
// member of its body it has none, which lowering reports.
func testSourcelessTransitionWithNothingBefore(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				accept go then active;
				entry; then init;
				state init;
				state active;
			}
		}
	`, "Machine")
	if !errors.Is(err, lower.ErrNoTransitionSource) {
		t.Fatalf("expected ErrNoTransitionSource, got %v", err)
	}
	if err.Error() != "create state executor: lower state machine: "+lower.NoTransitionSourceMessage {
		t.Fatalf("unexpected message: %v", err)
	}
}

// testSourcelessTransitionAfterANonState: the member before the shorthand is an
// entry action, a do action or an attribute rather than a state, which is not
// something a transition with a trigger can leave.
func testSourcelessTransitionAfterANonState(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"do action": {
			body: `entry; then init;
				state init;
				do action watch { }
				accept go then active;
				state active;`,
			want: "the do action",
		},
		"attribute": {
			body: `entry; then init;
				state init;
				attribute count : Integer = 0;
				accept go then active;
				state active;`,
			want: "the attribute usage count",
		},
		"choice pseudostate": {
			body: `entry; then init;
				state init;
				transition first init then pick;
				choice pick;
				accept go then active;
				state active;`,
			want: fmt.Sprintf(lower.TransitionSourcePseudostateFormat, "the choice pick", "pick"),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := stateExecutorError(t, `
				package test {
					state Machine {
						`+tc.body+`
					}
				}
			`, "Machine")
			var sourceErr *lower.TransitionSourceError
			if !errors.As(err, &sourceErr) {
				t.Fatalf("expected TransitionSourceError, got %v", err)
			}
			want := "create state executor: lower state machine: " + fmt.Sprintf(lower.TransitionSourceNotVertexFormat, tc.want)
			if _, ok := sourceErr.Source.(*ast.PseudostateNode); ok {
				want = "create state executor: lower state machine: " + tc.want
			}
			if err.Error() != want {
				t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
			}
		})
	}
}
