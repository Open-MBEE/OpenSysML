package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func deadAlternativeModel(t *testing.T) *exploreModel {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_entry_route_dead_alternative.sysml"))
	if err != nil {
		t.Fatalf("read conformance model: %v", err)
	}
	return parseExploreModel(t, string(data))
}

func TestDefaultEntryRouteAvailableWhenAnyTargetIsViable(t *testing.T) {
	m := deadAlternativeModel(t)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	exec, err := newStateExecutor(ctx, m.state(t, "Machine"), nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var available bool
	for source, transitions := range exec.graph.Transitions {
		state, ok := source.(*ast.StateNode)
		if !ok || state.Name != "idle" {
			continue
		}
		for _, transition := range transitions {
			target, ok := transition.Target.(*ast.StateNode)
			if ok && target.Name == "outer" {
				available = exec.routeAvailable(transition, nil)
			}
		}
	}
	if !available {
		t.Fatal("transition into outer is unavailable although its default entry can reach good")
	}
}

func TestExploreDefaultEntryRouteReachesViableAlternative(t *testing.T) {
	m := deadAlternativeModel(t)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		exec.SendSignal("Go", nil)
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	}
	exploration, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !exploration.Complete() || exploration.Runs != 2 {
		t.Fatalf("exploration %q over %d runs; want complete over both junction alternatives", exploration.Status(), exploration.Runs)
	}
	reachedGood := false
	var badErr error
	for _, explored := range exploration.Outcomes {
		if explored.Outcome.Err != nil {
			if !errors.Is(explored.Outcome.Err, errNoWayThrough) {
				t.Fatalf("bad alternative error = %v, want errNoWayThrough", explored.Outcome.Err)
			}
			badErr = explored.Outcome.Err
			continue
		}
		if explored.Outcome.FinalState == "good" {
			reachedGood = true
		}
	}
	if !reachedGood {
		t.Fatalf("exploration outcomes %v; want the good alternative reached", outcomeTexts(exploration))
	}
	if badErr == nil {
		t.Fatal("exploration did not report the bad alternative's dead default route")
	}
	t.Logf("bad alternative error: %v", badErr)
}
