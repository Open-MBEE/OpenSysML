package runtime

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func readPending(_ *Context, inv *Invocation) (bool, error) {
	value := inv.action().Data()["pending"]
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		return false, errors.New("pending observation is missing or not Boolean")
	}
	return value.Const.Bool, nil
}

var pendingRequest = CheckProperty{Name: "accepted request awaiting response", Holds: readPending}

const respondingRequest = `package test {
	private import ScalarValues::*;

	action handle {
		attribute pending : Boolean = false;
		attribute outcome : Integer = 0;

		first start;
		action acceptRequest { assign pending := true; }
		merge ready;
		action complete { assign outcome := 1; assign pending := false; }
		action decline { assign outcome := 2; assign pending := false; }
		merge response;
		done;

		succession first start then acceptRequest;
		succession first acceptRequest then ready;
		succession first ready then respond;
		decide respond;
		if pending then complete;
		if pending then decline;
		succession first complete then response;
		succession first decline then response;
		succession first response then done;
	}
}`

func requestWithRetry() string {
	return strings.NewReplacer(
		"merge response;", "action retry { assign pending := true; } merge response;",
		"if pending then decline;", "if pending then decline; if pending then retry; succession first retry then ready;",
	).Replace(respondingRequest)
}

func TestCheckProgressRequestResponse(t *testing.T) {
	var outcomes []string
	for _, tc := range []struct {
		name, model string
		want        ProgressStatus
	}{
		{"respond", respondingRequest, ProgressHolds},
		{"retry", requestWithRetry(), ProgressStalled},
	} {
		model := parseLibraryModel(t, tc.model)
		start := starterOf(model.action(t, "handle"))
		stock := checkStart(t, model, start, CheckOptions{Diverge: []string{"pending"}, Reduce: true})
		if stock.Verdict != CheckExhaustive || len(stock.Finals) != 2 {
			t.Fatalf("%s stock %s, finals %d", tc.name, stock.Status(), len(stock.Finals))
		}
		finals := []string{stock.Finals[0].Outcome, stock.Finals[1].Outcome}
		if tc.want == ProgressHolds {
			outcomes = finals
		} else if !slices.Equal(finals, outcomes) {
			t.Fatalf("stock outcomes differ: %v versus %v", finals, outcomes)
		}
		r, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{Depth: 100, States: 200}, pendingRequest)
		if err != nil || r.Status != tc.want || !r.Activated || !r.Cleared || len(r.Search.Finals) != 2 {
			t.Fatalf("%s progress report %+v, err %v", tc.name, r, err)
		}
		t.Logf("%s: stock %s, progress %s, activated=%v cleared=%v finals=%d prefix=%d cycle=%d",
			tc.name, stock.Status(), r.Status, r.Activated, r.Cleared, len(r.Search.Finals), len(r.Prefix), len(r.Cycle))
		if tc.want == ProgressStalled {
			if len(r.Prefix) == 0 || len(r.Cycle) == 0 {
				t.Fatal("expected nonempty acceptance prefix and retry cycle")
			}
			reexecuteProgressWalk(t, model, start, r, pendingRequest)
			t.Logf("accepted prefix: %v; twice reexecuted native cycle: %v", r.Prefix, r.Cycle)
		}
	}
}

// Reexecute the selected moves on a fresh runtime, without graph collection
// or cycle search, and verify stable-state equality across two cycle traversals.
func reexecuteProgressWalk(t *testing.T, model *exploreModel, start Starter, report *ProgressReport, monitor CheckProperty) (deadlocked bool) {
	t.Helper()
	run := startProgressWalk(t, model, start, report.StartupPicks)
	ctx := run.ctx
	settle := func() {
		var err error
		deadlocked, err = run.settle()
		if err != nil && !deadlocked {
			t.Fatal(err)
		}
	}
	settle()
	if got := string(run.canonicalStateFor(true).key()); got != report.Root {
		t.Fatalf("startup digest = %s, want %s", got, report.Root)
	}
	c := &checker{run: run}
	move := func(edge ProgressStep) {
		form := run.canonicalStateFor(true)
		if string(form.key()) != edge.From {
			t.Fatalf("start digest differs for %s: %s", edge.Move, form.key())
		}
		moves := c.movesOf(form)
		index := slices.IndexFunc(moves, func(m searchMove) bool { return m.name == edge.Move })
		if index < 0 {
			index = slices.IndexFunc(moves, func(m searchMove) bool { return strings.HasPrefix(edge.Move, m.name+" pick ") })
		}
		if index < 0 {
			t.Fatalf("move %s not enabled", edge.Move)
		}
		selected := moves[index]
		if selected.name != edge.Move {
			extra := strings.TrimPrefix(edge.Move, selected.name+" pick ")
			for _, namedPick := range strings.Split(extra, " pick ") {
				pick, err := strconv.Atoi(namedPick)
				if err != nil || pick < 1 {
					t.Fatalf("invalid choice %q in move %s", namedPick, edge.Move)
				}
				selected.Picks = append(selected.Picks, pick-1)
			}
		}
		if _, err := run.makeMove(selected.enabledMove); err != nil {
			t.Fatal(err)
		}
		settle()
		if key := run.canonicalStateFor(true).key(); string(key) != edge.To {
			t.Fatalf("end digest differs for %s: %s", edge.Move, key)
		}
	}
	for _, edge := range report.Prefix {
		move(edge)
	}
	cycleStart := run.canonicalStateFor(true).key()
	for range 2 {
		for _, edge := range report.Cycle {
			if pending, err := monitor.Holds(ctx, run.inv); err != nil || !pending {
				t.Fatalf("obligation not pending before cycle edge %v: %v", edge, err)
			}
			move(edge)
		}
		if end := run.canonicalStateFor(true).key(); end != cycleStart {
			t.Fatalf("path is not a cycle: %s -> %s", cycleStart, end)
		}
	}
	if len(report.Cycle) == 0 && !deadlocked && !run.terminal() {
		t.Fatal("replayed stall did not end in a settled terminal")
	}
	pending, err := monitor.Holds(ctx, run.inv)
	if err != nil || !pending {
		t.Fatalf("obligation not pending at end of witness: %v", err)
	}
	return deadlocked
}

func startProgressWalk(t *testing.T, model *exploreModel, start Starter, picks []int) *invocationRun {
	t.Helper()
	ctx, err := model.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.SetSchedule(checkPolicy(&checkScript{due: -1, picks: picks})); err != nil {
		t.Fatal(err)
	}
	run, err := beginInvocation(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(run.inv.Release)
	if err := run.inv.started(ctx); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestCheckProgressBoundAndTerminal(t *testing.T) {
	normal := respondingRequest
	for _, tc := range []struct {
		name   string
		text   string
		budget CheckBudget
		want   ProgressStatus
	}{
		{"depth", normal, CheckBudget{Depth: 1}, ProgressInconclusive},
		{"states", normal, CheckBudget{States: 1}, ProgressInconclusive},
		{"never accepted", strings.NewReplacer("assign pending := true;", "assign pending := false;", "if pending then", "if not pending then").Replace(normal), CheckBudget{}, ProgressVacuous},
		{"silent terminal", strings.NewReplacer("assign outcome := 1; assign pending := false;", "assign pending := true;", "assign outcome := 2; assign pending := false;", "assign pending := true;").Replace(normal), CheckBudget{}, ProgressStalled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := parseLibraryModel(t, tc.text)
			r, err := CheckProgress(context.Background(), model.fresh, starterOf(model.action(t, "handle")), tc.budget, pendingRequest)
			if err != nil || r.Status != tc.want {
				t.Fatalf("progress report %+v, err %v", r, err)
			}
			if tc.want == ProgressStalled && (len(r.Prefix) == 0 || len(r.Cycle) != 0) {
				t.Fatalf("expected witness to pending terminal, got %+v", r)
			}
			if tc.want == ProgressInconclusive && len(r.Search.BoundsHit) != 1 {
				t.Fatalf("expected exactly one exhausted budget, got %v", r.Search.BoundsHit)
			}
			t.Logf("%s: %s, bounds=%v prefix=%d cycle=%d", tc.name, r.Status, r.Search.BoundsHit, len(r.Prefix), len(r.Cycle))
		})
	}
}

func TestRuntimeRobustnessProgress(t *testing.T) {
	cappedMonitor := CheckProperty{Name: "unknown monitor", Holds: func(*Context, *Invocation) (bool, error) {
		return true, errStatementOrderSweepLimit
	}}
	modelForMonitor := parseLibraryModel(t, respondingRequest)
	if _, err := CheckProgress(context.Background(), modelForMonitor.fresh, starterOf(modelForMonitor.action(t, "handle")), CheckBudget{}, cappedMonitor); !errors.Is(err, errStatementOrderSweepLimit) {
		t.Fatalf("progress treated an incomplete monitor as evidence: %v", err)
	}

	const timed = "package test { private import SI::*; action delayed { first start; then action wait accept after 1 [s]; then done; } }"
	model := parseLibraryModel(t, timed)
	start := starterOf(model.action(t, "delayed"))
	never := CheckProperty{Name: "no request", Holds: func(*Context, *Invocation) (bool, error) { return false, nil }}
	if _, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{}, never); !errors.Is(err, ErrProgressTimed) {
		t.Fatalf("timed progress error = %v", err)
	}
	if _, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{}, CheckProperty{}); !errors.Is(err, ErrProgressMonitor) {
		t.Fatalf("invalid monitor error = %v", err)
	}
}

func pendingMachine() CheckProperty {
	return CheckProperty{Name: "accepted request awaiting response", Holds: func(_ *Context, inv *Invocation) (bool, error) {
		return inv.States[0].StateData()["pending"].Const.Bool, nil
	}}
}

func TestCheckProgressStateRetry(t *testing.T) {
	model := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		state handle {
			attribute pending : Boolean = false;
			entry; then ready;
			state ready { entry action { assign pending := true; } }
			state retry;
			transition first ready then retry;
			transition first retry then ready;
		}
	}`)
	start := invocationOf(nil, []*symbols.Symbol{model.state(t, "handle")})
	budget := CheckBudget{Depth: 12, States: 20}
	r, err := CheckProgress(context.Background(), model.fresh, start, budget, pendingMachine())
	if err != nil || r.Status != ProgressStalled || len(r.Cycle) != 2 {
		t.Fatalf("want a two-transition stall, got %+v, %v", r, err)
	}
	reexecuteProgressWalk(t, model, start, r, pendingMachine())
	stock, err := Check(context.Background(), model.fresh, start, budget, CheckOptions{}, nil)
	if err != nil || !slices.Contains(stock.BoundsHit, "depth") {
		t.Fatalf("stock check must keep visit history, report %+v, err %v", stock, err)
	}
}

func TestCheckProgressPendingDeadlock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		prefix bool
		want   ProgressStatus
	}{
		{"after move", "attribute pending : Boolean = false; first start; then action request { assign pending := true; } then action response accept n : Integer; then done;", true, ProgressStalled},
		{"initial", "attribute pending : Boolean = true; first start; then action response accept n : Integer; then done;", false, ProgressStalled},
		{"initial no pending", "attribute pending : Boolean = false; first start; then action response accept n : Integer; then done;", false, ProgressInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := parseLibraryModel(t, "package test { private import ScalarValues::*; action handle { "+tc.body+" } }")
			start := starterOf(model.action(t, "handle"))
			if !tc.prefix {
				start = startProgressAtAccept(start)
			}
			r, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{}, pendingRequest)
			if err != nil || r.Status != tc.want || len(r.Cycle) != 0 || (len(r.Prefix) > 0) != tc.prefix {
				t.Fatalf("want %s (prefix %t), got %+v, %v", tc.want, tc.prefix, r, err)
			}
			if len(r.Search.Finals) != 0 || len(r.Search.Violations) != 1 || !errors.Is(r.Search.Violations[0].Err, ErrAcceptDeadlock) {
				t.Fatalf("stock diagnostics should retain the deadlock, not produce a final: %+v", r.Search)
			}
			if tc.want == ProgressStalled && !reexecuteProgressWalk(t, model, start, r, pendingRequest) {
				t.Fatal("expected witness to replay to a settled deadlock")
			}
		})
	}
}

// Start at a waiting accept instead of exposing the fixture's initial start-node move.
func startProgressAtAccept(start Starter) Starter {
	return func(ctx *Context) (*Invocation, error) {
		inv, err := start(ctx)
		if err != nil {
			return nil, err
		}
		run := &invocationRun{ctx: ctx, inv: inv, state: ctx.run}
		if _, err := run.makeMove(run.enabledMoves()[0]); err != nil {
			inv.Release()
			return nil, err
		}
		return inv, nil
	}
}

func TestCheckProgressStartupWitness(t *testing.T) {
	model := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		state handle {
			attribute armed : Boolean = false;
			attribute pending : Boolean = false;
			entry; then parallelWork;
			state parallelWork parallel {
				state arm {
					entry; then armedState;
					state armedState { entry action { assign armed := true; } }
				}
				state acceptRequest {
					entry; then ready;
					state ready { entry action { assign pending := not armed; } }
				}
			}
		}
	}`)
	start := invocationOf(nil, []*symbols.Symbol{model.state(t, "handle")})
	monitor := pendingMachine()
	r, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{}, monitor)
	if err != nil || r.Status != ProgressStalled || !slices.Equal(r.StartupPicks, []int{1}) || len(r.Prefix)+len(r.Cycle) != 0 {
		t.Fatalf("want a stall only after the alternate startup, got %+v, %v", r, err)
	}
	reexecuteProgressWalk(t, model, start, r, monitor)
	for _, picks := range [][]int{nil, r.StartupPicks} {
		run := startProgressWalk(t, model, start, picks)
		if err := run.stabilize(); err != nil {
			t.Fatal(err)
		}
		pending, err := monitor.Holds(run.ctx, run.inv)
		if !run.terminal() || err != nil || pending != (picks != nil) {
			t.Fatalf("startup %v: want terminal and pending=%t; got pending=%t, %v", picks, picks != nil, pending, err)
		}
	}
}

func TestCheckProgressPendingExecutionFailure(t *testing.T) {
	model := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action handle {
			attribute pending : Boolean = true;
			attribute denominator : Integer = 0;
			attribute outcome : Integer = 0;
			first start;
			then action attempt { assign outcome := 1 / denominator; }
			then done;
		}
	}`)
	r, err := CheckProgress(context.Background(), model.fresh, starterOf(model.action(t, "handle")), CheckBudget{}, pendingRequest)
	if err != nil || r.Status != ProgressInconclusive || !r.Activated || r.Root != "" {
		t.Fatalf("want an inconclusive execution failure, not a stalled witness, got %+v, %v", r, err)
	}
	if len(r.Search.Violations) != 1 || !errors.Is(r.Search.Violations[0].Err, ErrDivisionByZero) {
		t.Fatalf("wanted original division failure, got %+v", r.Search)
	}
}
