package runtime

import (
	"errors"
	"strings"
	"testing"
)

// orderDependenceModel forks to three writers of x (conflicting) and, in a
// second action, to two writers of different features (commuting).
const orderDependenceModel = `package test {
	private import ScalarValues::*;
	action def Writers {
		out x : Integer = 0;
		out aRan : Boolean = false;
		out bRan : Boolean = false;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		first split then c;
		action a { assign x := 1; assign aRan := true; }
		action b { assign x := 2; assign bRan := true; }
		action c { assign x := 3; }
		first a then sync;
		first b then sync;
		first c then sync;
		join sync;
		first sync then done;
	}
	action def Apart {
		out x : Integer = 0;
		out y : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign y := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then done;
	}
}`

// TestRuntimeRobustnessOrderDependence: a default run marks the values its
// conflicting choice points touch and nothing else; commuting alternatives are
// one outcome and mark nothing; the marker is typed, not parsed from prose; a
// replay that refuses a recorded choice is the typed error, not a marked value.
func TestRuntimeRobustnessOrderDependence(t *testing.T) {
	t.Run("conflicting_writers_mark_the_feature_they_write", testConflictingWritersMarkTheFeatureTheyWrite)
	t.Run("commuting_branches_mark_nothing", testCommutingBranchesMarkNothing)
	t.Run("marker_text_counts_unresolved_choice_points", testMarkerTextCountsUnresolvedChoicePoints)
	t.Run("replay_of_an_invalid_choice_is_the_typed_error", testReplayOfAnInvalidChoiceIsTheTypedError)
	t.Run("qualified_names_are_matched_by_their_last_segment", testQualifiedNamesAreMatchedByTheirLastSegment)
	t.Run("a_performer_feature_is_marked_only_by_writes_to_it", testAPerformerFeatureIsMarkedOnlyByWritesToIt)
}

// performerShadowModel is a machine with an attribute x of its own, performed by
// a part with an x of its own: the junction's branches write the machine's.
const performerShadowModel = `package test {
	private import ScalarValues::*;
	attribute def Go;
	part def Holder {
		attribute x : Integer = 7;
		exhibit state machine {
			attribute x : Integer = 0;
			entry; then idle;
			state idle;
			junction split;
			state left;
			state right;
			transition first idle accept Go then split;
			transition first split do assign x := 1 then left;
			transition first split do assign x := 2 then right;
		}
	}
	part holder : Holder;
}
`

// The machine's x is order-dependent on the branch drawn; the performer's x,
// reported as this.x, is not, since nothing wrote it.
func testAPerformerFeatureIsMarkedOnlyByWritesToIt(t *testing.T) {
	m := parseExploreModel(t, performerShadowModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	self, err := ctx.Instantiate(oneSymbol(t, m.idx, "test::holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	exec, err := ctx.PerformState(m.state(t, "Holder::machine"), self, []QueuedEvent{{Signal: "Go"}})
	if err != nil {
		t.Fatalf("PerformState: %v", err)
	}
	if got := FormatValue(exec.StateData()["x"]); got != "1" && got != "2" {
		t.Fatalf("x = %s, want a branch's value", got)
	}
	if n := ctx.OrderDependent("x"); n != 1 {
		t.Errorf("order-dependent x = %d choice points, want the junction's draw", n)
	}
	if n := ctx.OrderDependent("this.x"); n != 0 {
		t.Errorf("order-dependent this.x = %d choice points, want 0: the branches write the machine's x, not the performer's", n)
	}
	// An outcome is reported after the executor has left the clock: the
	// performer is still known.
	ctx.clock.detach(exec)
	if n := ctx.OrderDependent("this.x"); n != 0 {
		t.Errorf("order-dependent this.x = %d choice points after the executor left the clock, want 0", n)
	}
	if n := ctx.OrderDependent("x"); n != 1 {
		t.Errorf("order-dependent x = %d choice points after the executor left the clock, want 1", n)
	}
}

// A value reported under a qualified key (`this.x`, a state path) is marked as
// the bare feature the choice touched is.
func testQualifiedNamesAreMatchedByTheirLastSegment(t *testing.T) {
	ctx, _, err := runOrderDependence(t, "Writers", nil)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	for _, name := range []string{"x", "this.x", "Writers.x"} {
		if n := ctx.OrderDependent(name); n != 2 {
			t.Errorf("order-dependent %s = %d choice points, want 2", name, n)
		}
	}
	for _, name := range []string{"this.aRan", "y"} {
		if n := ctx.OrderDependent(name); n != 0 {
			t.Errorf("order-dependent %s = %d choice points, want 0", name, n)
		}
	}
}

// runOrderDependence runs the named action of orderDependenceModel under policy.
func runOrderDependence(t *testing.T, name string, policy *SchedulePolicy) (*Context, *ActionExecutor, error) {
	t.Helper()
	m := parseExploreModel(t, orderDependenceModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if policy != nil {
		mustSchedule(t, ctx, *policy)
	}
	exec, err := ctx.CreateActionExecutor(m.action(t, name))
	if err != nil {
		t.Fatalf("CreateActionExecutor(%s): %v", name, err)
	}
	return ctx, exec, exec.RunToCompletion()
}

func testConflictingWritersMarkTheFeatureTheyWrite(t *testing.T) {
	ctx, exec, err := runOrderDependence(t, "Writers", nil)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := FormatValue(exec.Results()["x"]); got != "1" && got != "2" && got != "3" {
		t.Fatalf("x = %s, want a writer's value", got)
	}
	if n := ctx.UnresolvedChoices(); n != 2 {
		t.Errorf("unresolved choice points = %d, want the two token orders", n)
	}
	if n := ctx.OrderDependent("x"); n != 2 {
		t.Errorf("order-dependent x = %d choice points, want 2", n)
	}
	for _, flag := range []string{"aRan", "bRan"} {
		if n := ctx.OrderDependent(flag); n != 0 {
			t.Errorf("order-dependent %s = %d choice points, want 0: one writer alone writes it", flag, n)
		}
	}
	for _, c := range ctx.Choices() {
		if c.Kind != ChoiceTokenOrder {
			continue
		}
		if c.Independent {
			t.Errorf("%s classified independent; its alternatives all write x", c)
		}
		if strings.Join(c.Touches, ",") != "x" {
			t.Errorf("%s touches %v, want x alone", c, c.Touches)
		}
	}
	marked := ctx.OrderDependentAmong([]string{"x", "aRan", "bRan"})
	if len(marked) != 1 || marked["x"] != 2 {
		t.Errorf("OrderDependentAmong = %v, want x: 2 alone", marked)
	}
}

func testCommutingBranchesMarkNothing(t *testing.T) {
	ctx, exec, err := runOrderDependence(t, "Apart", nil)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if x, y := FormatValue(exec.Results()["x"]), FormatValue(exec.Results()["y"]); x != "1" || y != "2" {
		t.Fatalf("x, y = %s, %s, want 1, 2", x, y)
	}
	if n := ctx.UnresolvedChoices(); n != 0 {
		t.Errorf("unresolved choice points = %d, want none: the branches commute", n)
	}
	orders := 0
	for _, c := range ctx.Choices() {
		if c.Kind != ChoiceTokenOrder {
			continue
		}
		orders++
		if !c.Independent || len(c.Touches) != 0 {
			t.Errorf("%s: independent=%v touches=%v, want independent and none", c, c.Independent, c.Touches)
		}
		if !strings.HasSuffix(c.String(), "; independent)") {
			t.Errorf("%s does not state its standing", c)
		}
	}
	if orders == 0 {
		t.Fatal("the fork recorded no token order to classify")
	}
	if marked := ctx.OrderDependentAmong([]string{"x", "y"}); len(marked) != 0 {
		t.Errorf("OrderDependentAmong = %v, want nothing marked", marked)
	}
}

func testMarkerTextCountsUnresolvedChoicePoints(t *testing.T) {
	if got := OrderDependenceText(0); got != "" {
		t.Errorf("OrderDependenceText(0) = %q, want nothing: the value is fixed", got)
	}
	if got, want := OrderDependenceText(1), "(order-dependent; 1 choice point — explore shows the set)"; got != want {
		t.Errorf("OrderDependenceText(1) = %q, want %q", got, want)
	}
	if got, want := OrderDependenceText(2), "(order-dependent; 2 choice points — explore shows the set)"; got != want {
		t.Errorf("OrderDependenceText(2) = %q, want %q", got, want)
	}
}

func testReplayOfAnInvalidChoiceIsTheTypedError(t *testing.T) {
	ctx, _, err := runOrderDependence(t, "Writers", nil)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	taken := ctx.ChoicesTaken()
	if len(taken) == 0 {
		t.Fatal("the default run recorded no replayable choice")
	}
	taken[0].Taken = 99
	replay := ReplayPolicy(taken)
	_, _, err = runOrderDependence(t, "Writers", &replay)
	if !errors.Is(err, ErrInvalidChoice) && !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("replay of an out-of-range choice: err = %v, want ErrInvalidChoice or ErrReplayRefused", err)
	}
}
