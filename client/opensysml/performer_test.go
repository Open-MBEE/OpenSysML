package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const performerSource = `package Wire {
	private import ScalarValues::*;
	item def Ping;
	port def Link { in item ping : Ping; }
	part def Ground {
		port p : ~Link;
		exhibit state hail { entry; then go; state go { entry send new Ping() via p; } }
	}
	part def Craft {
		port p : Link;
		attribute pinged : Boolean = false;
		exhibit state modes {
			entry; then waiting;
			state waiting;
			transition first waiting accept Ping via p then active;
			state active { entry assign pinged := true; }
		}
		action look { out seen : Boolean; first start; then action read assign seen := pinged; then done; }
	}
	part def Pair {
		part ground : Ground;
		part craft : Craft;
		connect craft.p to ground.p;
	}
	part pair : Pair;
}`

// PerformedBy runs a behavior on the object a declaration-rooted path reaches
// inside its assembly, so a machine the part exhibits hears its siblings over
// the connector; the declaration alone makes the part on its own.
func TestPerformedByRunsOnTheObjectAPathReaches(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, performerSource)

	for _, test := range []struct{ performer, final string }{
		{"Wire::pair.craft", "active"},
		{"Wire::Craft", "waiting"},
	} {
		run, err := client.ExecuteState(ctx, model, "Wire::Craft::modes", nil, opensysml.PerformedBy(test.performer))
		if err != nil {
			t.Fatalf("ExecuteState on %s: %v", test.performer, err)
		}
		if got := run.Visited[len(run.Visited)-1]; got != test.final {
			t.Errorf("ExecuteState on %s visited %v, want to end in %s", test.performer, run.Visited, test.final)
		}
		exploration, err := client.ExploreState(ctx, model, "Wire::Craft::modes", nil, opensysml.PerformedBy(test.performer))
		if err != nil {
			t.Fatalf("ExploreState on %s: %v", test.performer, err)
		}
		if len(exploration.Outcomes) != 1 || exploration.Outcomes[0].FinalState != test.final {
			t.Errorf("ExploreState on %s: %+v, want one outcome ending in %s", test.performer, exploration.Outcomes, test.final)
		}
	}

	run, err := client.ExecuteAction(ctx, model, "Wire::Craft::look", nil, opensysml.PerformedBy("Wire::pair.craft"))
	if err != nil || run.Outputs["seen"] != opensysml.Bool(true) {
		t.Errorf("ExecuteAction look on pair.craft: %v %#v, want seen true", err, run)
	}
	exploration, err := client.ExploreAction(ctx, model, "Wire::Craft::look", nil, opensysml.PerformedBy("Wire::pair.craft"))
	if err != nil || len(exploration.Outcomes) != 1 || exploration.Outcomes[0].Outputs["seen"] != opensysml.Bool(true) {
		t.Errorf("ExploreAction look on pair.craft: %v %+v, want one outcome with seen true", err, exploration)
	}

	_, err = client.ExecuteState(ctx, model, "Wire::Craft::modes", nil, opensysml.PerformedBy("Wire::pair.tug"))
	var failure *opensysml.FailureError
	if !errors.As(err, &failure) || !strings.Contains(failure.Message, `Wire::pair has no feature "tug"`) {
		t.Errorf("ExecuteState on pair.tug: %v, want the missing feature reported", err)
	}
}
