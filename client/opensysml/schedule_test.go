package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const scheduleSource = `package Sched {
	private import ScalarValues::*;

	action race {
		attribute winner : Integer = 0;
		first start;
		fork split;
		action left { assign winner := 1; }
		action right { assign winner := 2; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}

	state Dispatcher {
		attribute level : Integer = 8;
		entry; then idle;
		state idle;
		state low;
		state high;
		transition first idle accept Go if level > 5 then low;
		transition first idle accept Go if level > 7 then high;
	}

	action def Race {
		out winner : Integer = 0;
		first start;
		fork split;
		action left { assign winner := 1; }
		action right { assign winner := 2; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}

	analysis raced {
		out winner : Integer;
		perform action race : Race;
		return : Integer = winner;
	}
}`

func winner(t *testing.T, outputs map[string]opensysml.Value) opensysml.Int {
	t.Helper()
	got, ok := outputs["winner"].(opensysml.Int)
	if !ok {
		t.Fatalf("winner = %#v, want an Int", outputs["winner"])
	}
	return got
}

// WithSchedule selects which branch's write stands: the default runs the
// later-declared branch first, declared the first-declared one.
func TestWithScheduleSelectsTheOrderARunTakes(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, scheduleSource)

	for _, test := range []struct {
		schedule string
		want     opensysml.Int
	}{
		{"", 1},
		{"reverse", 1},
		{"declared", 2},
	} {
		var opts []opensysml.ExecuteOption
		if test.schedule != "" {
			opts = append(opts, opensysml.WithSchedule(test.schedule))
		}
		run, err := client.ExecuteAction(ctx, model, "Sched::race", nil, opts...)
		if err != nil {
			t.Fatalf("ExecuteAction under %q: %v", test.schedule, err)
		}
		if got := winner(t, run.Outputs); got != test.want {
			t.Errorf("ExecuteAction under %q: winner = %d, want %d", test.schedule, got, test.want)
		}
	}

	for _, test := range []struct{ schedule, visited string }{
		{"declared", "idle,low"},
		{"seed:3", "idle,high"},
	} {
		run, err := client.ExecuteState(ctx, model, "Sched::Dispatcher", []string{"Go"}, opensysml.WithSchedule(test.schedule))
		if err != nil {
			t.Fatalf("ExecuteState under %q: %v", test.schedule, err)
		}
		if got := strings.Join(run.Visited, ","); got != test.visited {
			t.Errorf("ExecuteState under %q visited %q, want %q", test.schedule, got, test.visited)
		}
	}

	analysis, err := client.RunAnalysis(ctx, model, "Sched::raced", opensysml.Schedule("declared"))
	if err != nil {
		t.Fatalf("RunAnalysis under declared: %v", err)
	}
	if got, ok := analysis.Output("winner"); !ok || got != opensysml.Int(2) {
		t.Errorf("RunAnalysis under declared: winner = %#v, want Int(2)", got)
	}
}

// The same seed orders a run the same way each time it is asked for.
func TestASeededScheduleIsReproducible(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, scheduleSource)

	var first *opensysml.Int
	for i := 0; i < 3; i++ {
		run, err := client.ExecuteAction(ctx, model, "Sched::race", nil, opensysml.WithSchedule("seed:11"))
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		got := winner(t, run.Outputs)
		if first == nil {
			first = &got
		} else if got != *first {
			t.Fatalf("run %d under seed:11 answered %d, the first %d", i, got, *first)
		}
	}
}

// A spelling naming no policy is refused by the service as an invalid
// argument on each call, before the run starts.
func TestAnUnknownScheduleIsInvalidArgument(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, scheduleSource)

	for _, spelling := range []string{"random", "seed", "seed:", "seed:-1", "seed:abc"} {
		_, err := client.ExecuteAction(ctx, model, "Sched::race", nil, opensysml.WithSchedule(spelling))
		if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), spelling) {
			t.Errorf("ExecuteAction under %q: err = %v, want CodeInvalidArgument naming it", spelling, err)
		}
		_, err = client.ExecuteState(ctx, model, "Sched::Dispatcher", nil, opensysml.WithSchedule(spelling))
		if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), spelling) {
			t.Errorf("ExecuteState under %q: err = %v, want CodeInvalidArgument naming it", spelling, err)
		}
		_, err = client.RunAnalysis(ctx, model, "Sched::raced", opensysml.Schedule(spelling))
		if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), spelling) {
			t.Errorf("RunAnalysis under %q: err = %v, want CodeInvalidArgument naming it", spelling, err)
		}
	}
}
