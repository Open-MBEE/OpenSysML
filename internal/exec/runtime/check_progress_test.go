package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func readPending(_ *Context, inv *Invocation) (bool, error) {
	value := inv.action().Data()["pending"]
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		return false, errors.New("pending observation is missing or not Boolean")
	}
	return value.Const.Bool, nil
}

var pendingCharge = CheckProperty{Name: "accepted request awaiting response", Holds: readPending}

func progressModel(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "examples", "charging-response-progress", name+".sysml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func chargeResponseStarter(t *testing.T, model *exploreModel) Starter {
	t.Helper()
	return starterOf(namedOrFoundSymbol(t, model.idx, "ChargingProgress::serve", model.idx.DocumentRoot(model.path), ast.DefAction, ast.UsageAction))
}

func TestCheckProgressChargingResponse(t *testing.T) {
	var outcomes []string
	for _, tc := range []struct {
		fixture string
		want    ProgressStatus
	}{
		{"station", ProgressHolds},
		{"retry", ProgressStalled},
	} {
		text := progressModel(t, tc.fixture)
		model := parseLibraryModel(t, text)
		start := chargeResponseStarter(t, model)
		stock := checkStart(t, model, start, CheckOptions{Diverge: []string{"pending"}, Reduce: true})
		if stock.Verdict != CheckExhaustive || len(stock.Finals) != 2 {
			t.Fatalf("%s stock %s, finals %d", tc.fixture, stock.Status(), len(stock.Finals))
		}
		finals := []string{stock.Finals[0].Outcome, stock.Finals[1].Outcome}
		if tc.fixture == "station" {
			outcomes = finals
		} else if !slices.Equal(finals, outcomes) {
			t.Fatalf("stock outcomes differ: %v versus %v", finals, outcomes)
		}
		r, err := CheckProgress(context.Background(), model.fresh, start, CheckBudget{Depth: 100, States: 200}, pendingCharge)
		if err != nil || r.Status != tc.want || !r.Activated || !r.Cleared || len(r.Search.Finals) != 2 {
			t.Fatalf("%s progress report %+v, err %v", tc.fixture, r, err)
		}
		t.Logf("%s: stock %s, progress %s, activated=%v cleared=%v finals=%d prefix=%d cycle=%d",
			tc.fixture, stock.Status(), r.Status, r.Activated, r.Cleared, len(r.Search.Finals), len(r.Prefix), len(r.Cycle))
		if tc.want == ProgressStalled {
			if len(r.Prefix) == 0 || len(r.Cycle) == 0 {
				t.Fatal("expected nonempty acceptance prefix and retry cycle")
			}
			reexecuteProgressWalk(t, model, start, r)
			t.Logf("accepted prefix: %v; twice reexecuted native cycle: %v", r.Prefix, r.Cycle)
		}
	}
}

// Reexecute the selected moves on a fresh runtime, without graph collection
// or cycle search, and verify stable-state equality across two cycle traversals.
func reexecuteProgressWalk(t *testing.T, model *exploreModel, start Starter, report *ProgressReport) {
	t.Helper()
	ctx, err := model.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.SetSchedule(checkPolicy(&checkScript{due: -1})); err != nil {
		t.Fatal(err)
	}
	run, err := beginInvocation(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	defer run.inv.Release()
	if err := run.inv.started(ctx); err != nil {
		t.Fatal(err)
	}
	if err := run.stabilize(); err != nil {
		t.Fatal(err)
	}
	c := &checker{run: run}
	move := func(edge ProgressStep) {
		form := run.canonicalState()
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
		if err := run.stabilize(); err != nil {
			t.Fatal(err)
		}
		if key := run.canonicalState().key(); string(key) != edge.To {
			t.Fatalf("end digest differs for %s: %s", edge.Move, key)
		}
	}
	for _, edge := range report.Prefix {
		move(edge)
	}
	cycleStart := run.canonicalState().key()
	for range 2 {
		for _, edge := range report.Cycle {
			if pending, err := readPending(ctx, run.inv); err != nil || !pending {
				t.Fatalf("obligation not pending before cycle edge %v: %v", edge, err)
			}
			move(edge)
		}
		if end := run.canonicalState().key(); end != cycleStart {
			t.Fatalf("path is not a cycle: %s -> %s", cycleStart, end)
		}
	}
}

func TestCheckProgressBoundAndTerminal(t *testing.T) {
	normal := progressModel(t, "station")
	for _, tc := range []struct {
		name   string
		text   string
		budget CheckBudget
		want   ProgressStatus
	}{
		{"depth", normal, CheckBudget{Depth: 1}, ProgressInconclusive},
		{"states", normal, CheckBudget{States: 1}, ProgressInconclusive},
		{"never accepted", strings.ReplaceAll(normal, "assign pending := true;", "assign pending := false;"), CheckBudget{}, ProgressVacuous},
		{"silent terminal", strings.NewReplacer("assign energized := true; assign pending := false;", "assign pending := true;", "assign rejected := true; assign pending := false;", "assign pending := true;").Replace(normal), CheckBudget{}, ProgressStalled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := parseLibraryModel(t, tc.text)
			r, err := CheckProgress(context.Background(), model.fresh, chargeResponseStarter(t, model), tc.budget, pendingCharge)
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
	modelForMonitor := parseLibraryModel(t, progressModel(t, "station"))
	if _, err := CheckProgress(context.Background(), modelForMonitor.fresh, chargeResponseStarter(t, modelForMonitor), CheckBudget{}, cappedMonitor); !errors.Is(err, errStatementOrderSweepLimit) {
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
