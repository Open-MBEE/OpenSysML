package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkedAction is a one-action invocation moved move by move under the check policy.
type checkedAction struct {
	ctx  *Context
	run  *invocationRun
	exec *ActionExecutor
}

func checkedActionOf(ctx *Context, exec *ActionExecutor) *checkedAction {
	inv := &Invocation{Actions: []*ActionExecutor{exec}}
	return &checkedAction{ctx: ctx, run: &invocationRun{ctx: ctx, inv: inv}, exec: exec}
}

func startChecked(t *testing.T, m *exploreModel, name string) *checkedAction {
	t.Helper()
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	return checkedOn(t, ctx, m.action(t, name))
}

func checkedOn(t *testing.T, ctx *Context, action *symbols.Symbol) *checkedAction {
	t.Helper()
	mustSchedule(t, ctx, checkPolicy(&checkScript{due: -1}))
	exec, err := ctx.CreateActionExecutor(action)
	if err != nil {
		t.Fatalf("create %s: %v", symbolText(action), err)
	}
	return checkedActionOf(ctx, exec)
}

func moveLabels(moves []enabledMove) string {
	labels := make([]string, len(moves))
	for i, m := range moves {
		labels[i] = m.String() + ":" + m.Kind.String()
	}
	return strings.Join(labels, " ")
}

// make makes the move, failing the test if the run refused it, and returns the
// choice points the move drew past its picks.
func (a *checkedAction) make(t *testing.T, m enabledMove) []ChoicePoint {
	t.Helper()
	drawn, err := a.run.makeMove(m)
	if err != nil {
		t.Fatalf("move %s: %v", m, err)
	}
	return drawn
}

// only asserts the state has exactly the moves labelled and returns them.
func (a *checkedAction) only(t *testing.T, want string) []enabledMove {
	t.Helper()
	moves := a.run.enabledMoves()
	if got := moveLabels(moves); got != want {
		t.Fatalf("moves %q, want %q", got, want)
	}
	return moves
}

func (a *checkedAction) result(t *testing.T, name string) string {
	t.Helper()
	v, ok := a.exec.Results()[name]
	if !ok {
		t.Fatalf("no result %s in %v", name, a.exec.Results())
	}
	return FormatValue(v)
}

// The moves of a state are the tokens able to act, one each; making one leaves
// the others for the next state, and the last writer's value stands.
func TestEnabledMovesAreTheTokensAbleToAct(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	a := startChecked(t, m, "race")
	a.make(t, a.only(t, "1@start:plain")[0])
	a.make(t, a.only(t, "1@split:plain")[0])
	writers := a.only(t, "2@a:plain 3@b:plain 4@c:plain")
	a.make(t, writers[1])
	rest := a.only(t, "2@a:plain 4@c:plain")
	a.make(t, rest[1])
	a.make(t, a.only(t, "2@a:plain")[0])
	if got := a.result(t, "x"); got != "1" {
		t.Fatalf("x = %s after b, c, a; want 1", got)
	}
	// The three arrived tokens fuse into one performance of the join.
	joined := a.run.enabledMoves()
	if len(joined) != 1 || joined[0].Kind != moveJoin {
		t.Fatalf("at the join: %s, want one join move", moveLabels(joined))
	}
	a.make(t, joined[0])
	a.make(t, a.only(t, "5@done:plain")[0])
	if a.exec.State() != StateCompleted || len(a.run.enabledMoves()) != 0 {
		t.Fatalf("state %v with moves %s, want completed with none", a.exec.State(), moveLabels(a.run.enabledMoves()))
	}
	for _, c := range a.ctx.Choices() {
		if c.Kind == ChoiceTokenOrder && c.Step == 3 && c.Taken != 1 {
			t.Errorf("step 3 noted %s, want b taken among three", c.Choice())
		}
	}
}

// A token not among those able to act is refused as the typed error, and the
// refused step moves nothing.
func TestMakeMoveRefusesATokenUnableToAct(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	a := startChecked(t, m, "race")
	a.make(t, a.only(t, "1@start:plain")[0])
	a.make(t, a.only(t, "1@split:plain")[0])
	_, err := a.run.makeMove(enabledMove{Owner: a.exec, Token: 9, Label: "9@nowhere"})
	var refused *CheckMoveError
	if !errors.As(err, &refused) || !errors.Is(err, ErrCheckRefused) || !strings.Contains(refused.Move, "token 9") {
		t.Fatalf("moving token 9: %v, want a CheckMoveError for token 9", err)
	}
	a.only(t, "2@a:plain 3@b:plain 4@c:plain")
}

// A decision several of whose guards hold is one move per holding branch: the
// first move reveals the branch choice it drew, and a pick past them is refused.
func TestMakeMoveTakesTheSelectedBranch(t *testing.T) {
	m := parseExploreModel(t, decisionLoopModel)
	a := startChecked(t, m, "count")
	a.make(t, a.only(t, "1@start:plain")[0])
	pick := a.only(t, "1@pick:decision")[0]
	before, err := a.exec.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	drawn := a.make(t, pick)
	if len(drawn) != 1 || drawn[0].Kind != ChoiceDecisionBranch || len(drawn[0].Alternatives) != 2 || drawn[0].Taken != 0 {
		t.Fatalf("the first branch drew %+v, want one branch choice of two taking the first", drawn)
	}
	a.only(t, "1@left:plain")

	before.Restore()
	pick.Picks = []int{1}
	if drawn := a.make(t, pick); len(drawn) != 0 {
		t.Fatalf("branch 2 drew %+v, want nothing past its pick", drawn)
	}
	a.only(t, "1@right:plain")

	before.Restore()
	pick.Picks = []int{2}
	_, err = a.run.makeMove(pick)
	if !errors.Is(err, ErrCheckRefused) || !strings.Contains(err.Error(), "pick 3 is not among them") {
		t.Fatalf("pick 3 of two: %v, want a refusal naming it", err)
	}
	before.Release()
}

// A token at an accept is no move until a message it takes is in flight; settling
// a state with no move parks it and reports the accept deadlock.
func TestParkedAcceptIsAMoveOnceAnswered(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action reader {
			attribute got : Integer = 0;
			first start;
			then action take accept n : Integer;
			then action keep assign got := n;
			then done;
		}
	}`)
	a := startChecked(t, m, "reader")
	a.make(t, a.only(t, "1@start:plain")[0])
	a.only(t, "")
	if err := a.run.stabilize(); !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("settle with nothing in flight: %v, want %v", err, ErrAcceptDeadlock)
	}
	if tok := a.exec.Tokens(); len(tok) != 1 || tok[0].Wait == nil {
		t.Fatalf("tokens after settling %+v, want one parked at take", tok)
	}
	seven := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}}
	a.ctx.PostMessage(Message{SignalType: "Integer", Value: &seven})
	a.make(t, a.only(t, "1@take:accept")[0])
	a.make(t, a.only(t, "1@keep:plain")[0])
	if got := a.result(t, "got"); got != "7" {
		t.Fatalf("got = %s, want 7", got)
	}
}

// Settling a state whose only wait is on the clock advances it to the wait: time
// is captured in the state, not a move of the checker's, and the trigger is the move.
func TestSettlingOnTheClockAdvancesItWithoutAMove(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import SI::*;
		private import ScalarValues::*;
		action timed {
			attribute count : Integer = 0;
			first start;
			then action wait accept after 5 [s];
			then action tick assign count := count + 1;
			then done;
		}
	}`))
	a := checkedOn(t, ctx, findSymbolByName(idx.DocumentRoot("<test>"), "timed", ast.DefAction))
	a.make(t, a.only(t, "1@start:plain")[0])
	a.only(t, "")
	if err := a.run.stabilize(); err != nil {
		t.Fatalf("settle on the clock: %v", err)
	}
	if now := a.ctx.Clock().Now(); now != 5 {
		t.Fatalf("clock at %v after settling, want 5", now)
	}
	a.make(t, a.only(t, "1@wait:trigger")[0])
	a.make(t, a.only(t, "1@tick:plain")[0])
	a.make(t, a.only(t, "1@done:plain")[0])
	if got := a.result(t, "count"); got != "1" {
		t.Fatalf("count = %s, want 1", got)
	}
}

// A parked accept whose `via` port fails to materialize is a move once a message
// it would take is in flight: making it raises the port's error, not a deadlock.
func TestAFailingAcceptIsAMoveThatRaisesItsError(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan = 1 / 0;
			action listen {
				first start;
				action reader accept v : Integer via in;
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, checkPolicy(&checkScript{due: -1}))
	exec, err := ctx.CreateActionExecutorFor(oneSymbol(t, idx, "P::Listener::listen"), listener)
	if err != nil {
		t.Fatal(err)
	}
	a := checkedActionOf(ctx, exec)
	a.make(t, a.only(t, "1@start:plain")[0])
	a.only(t, "")
	if err := a.run.stabilize(); !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("settle with nothing in flight: %v, want %v", err, ErrAcceptDeadlock)
	}
	four := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Port: "other", Object: listener.ID, PortID: -1,
		Delivery: DeliverPort, Value: &four})
	moves := a.only(t, "1@reader:accept")
	if !errors.Is(moves[0].Fails, ErrDivisionByZero) {
		t.Fatalf("the move fails with %v, want the port's %v", moves[0].Fails, ErrDivisionByZero)
	}
	if _, err := a.run.makeMove(moves[0]); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("making the move: %v, want %v", err, ErrDivisionByZero)
	}
}

// A run not under the check policy cannot be moved by the checker.
func TestMakeMoveNeedsTheCheckPolicy(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	exec, err := ctx.CreateActionExecutor(m.action(t, "race"))
	if err != nil {
		t.Fatal(err)
	}
	a := checkedActionOf(ctx, exec)
	_, err = a.run.makeMove(a.run.enabledMoves()[0])
	if !errors.Is(err, ErrCheckRefused) {
		t.Fatalf("under the declared policy: %v, want %v", err, ErrCheckRefused)
	}
}

// A move of an invocation with several executors due draws the due order exactly
// once, taken at the move's owner, who then holds the turn while it has a move;
// one with a single executor due draws none. The order once drawn, a second draw
// within the move is a refusal, not a second choice.
func TestMakeMoveDrawsTheDueOrderOnce(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action worker {
			out x : Integer = 0;
			first start;
			then assign x := 1;
			then assign x := 2;
			then done;
		}
		state Machine {
			attribute y : Integer = 0;
			entry; then a;
			state a;
			state b;
			state c;
			transition first a do assign y := 1 then b;
			transition first b do assign y := 2 then c;
		}
	}`)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, checkPolicy(&checkScript{due: -1}))
	action, err := ctx.CreateActionExecutor(m.action(t, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	machine, err := ctx.CreateStateExecutor(m.state(t, "Machine"))
	if err != nil {
		t.Fatal(err)
	}
	inv := &Invocation{Actions: []*ActionExecutor{action}, States: []*StateExecutor{machine}}
	if err := inv.started(ctx); err != nil {
		t.Fatal(err)
	}
	run := &invocationRun{ctx: ctx, inv: inv}
	dueOrders := func() []ChoiceTaken {
		var drawn []ChoiceTaken
		for _, c := range ctx.ChoicesTaken() {
			if c.Kind == ChoiceDueOrder {
				drawn = append(drawn, c)
			}
		}
		return drawn
	}
	// The machine's move is taken first, so the one draw gives it the turn until
	// it rests, and the action alone moves after.
	moves := 0
	for {
		if err := run.stabilize(); err != nil {
			t.Fatalf("settling after %d moves: %v", moves, err)
		}
		if run.terminal() {
			break
		}
		enabled := run.enabledMoves()
		execs := owners(enabled)
		move := enabled[len(enabled)-1]
		before := len(dueOrders())
		if _, err := run.makeMove(move); err != nil {
			t.Fatalf("move %d, %s: %v", moves+1, move, err)
		}
		moves++
		drawn := dueOrders()[before:]
		switch {
		case len(execs) < 2 && len(drawn) != 0:
			t.Fatalf("move %d, %s, alone due: drew %v, want no due order", moves, move, drawn)
		case len(execs) >= 2 && len(drawn) != 1:
			t.Fatalf("move %d, %s, %d due: drew %v, want one due order", moves, move, len(execs), drawn)
		case len(execs) >= 2 && (drawn[0].Taken != slices.Index(execs, move.Owner) || !slices.Equal(drawn[0].Among, executorLabels(execs))):
			t.Fatalf("move %d, %s: drew %s, want the owner among %v", moves, move, drawn[0], executorLabels(execs))
		}
	}
	if moves != 6 {
		t.Fatalf("made %d moves, want the machine's two dispatches and the action's four steps", moves)
	}
	if orders := dueOrders(); len(orders) != 1 {
		t.Fatalf("drew %d due orders, want the one move both executors were due for", len(orders))
	}
	if x := FormatValue(action.Results()["x"]); x != "2" {
		t.Fatalf("x = %s, want 2", x)
	}
	if y := FormatValue(machine.StateData()["y"]); y != "2" {
		t.Fatalf("y = %s, want 2", y)
	}
}
