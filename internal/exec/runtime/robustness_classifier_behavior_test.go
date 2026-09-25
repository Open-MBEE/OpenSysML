package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessClassifierBehaviorStart exercises `perform obj.beh.start`, the
// start of a behavior an object's type declares for its objects to start: constructing
// the object runs nothing, the start runs the behavior with the object as this and
// leaves the object once the behavior is done, a parked behavior is woken by a later
// message, a second start is no second execution, and a start naming no object, no
// behavior, or a behavior that fails is a typed error that leaves the object untouched.
func TestRuntimeRobustnessClassifierBehaviorStart(t *testing.T) {
	t.Run("construction_starts_no_declared_behavior", testStartConstructionRunsNothing)
	t.Run("start_runs_the_behavior_as_the_object", testStartRunsAsTheObject)
	t.Run("started_behavior_is_woken_by_a_later_message", testStartWokenByMessage)
	t.Run("a_second_start_runs_nothing_more", testStartTwice)
	t.Run("an_inherited_behavior_starts_on_the_specialized_object", testStartInherited)
	t.Run("start_on_no_object_is_refused", testStartOnNoObject)
	t.Run("start_of_no_behavior_is_refused", testStartOfNoBehavior)
	t.Run("an_action_declared_as_start_is_performed_not_started", testStartNamedActionPerformed)
	t.Run("a_failing_start_is_undone_whole", testStartFailureRollsBack)
	t.Run("a_behavior_woken_by_a_start_runs_once_the_start_stands", testStartWakesOlderBehaviorAfterCommit)
	t.Run("start_is_traced_as_the_objects_own_execution", testStartTraced)
}

// starterModel is a Counter whose declared behavior Count writes its own n, with the
// given body, and a Starter action creating one, starting it as `steps` say and
// handing it out.
func starterModel(body, steps string) string {
	return `package test {
		private import ScalarValues::*;
		attribute def Tick;
		part def Counter {
			attribute n : Integer = 0;
			attribute seen : Integer = 0;
			action def Count {
				` + body + `
			}
			action count : Count;
		}
		action def Starter {
			out made : Counter;
			action create { out result : Counter = new Counter(); }
			` + steps + `
			bind create.result = made;
		}
	}`
}

// runStarter runs the Starter of src and hands back the context, the Counter it made
// and the error of the run, if any.
func runStarter(t *testing.T, src string) (*Context, *Instance, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Starter", ast.DefAction)
	if sym == nil {
		t.Fatal("action Starter not found")
	}
	results, err := ctx.ExecuteAction(sym)
	if err != nil {
		return ctx, nil, err
	}
	made, ok := results["made"]
	if !ok || made.Kind != ValInstance {
		t.Fatalf("made = %v, want the Counter", results)
	}
	inst, ok := ctx.Instance(made.Instance)
	if !ok {
		t.Fatalf("made names object #%d, which the context lacks", made.Instance)
	}
	return ctx, inst, nil
}

const countOnce = `first start; then action tally { assign n := n + 1; } then done;`

// testStartConstructionRunsNothing: `new Counter()` alone leaves Count unstarted; the
// object performs nothing and n keeps its default.
func testStartConstructionRunsNothing(t *testing.T) {
	ctx, counter, err := runStarter(t, starterModel(countOnce, `first start then create; first create then done;`))
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if bs := counter.Behaviors(); len(bs) != 0 {
		t.Errorf("the new Counter performs %v, want nothing until a start", bs)
	}
	if got := featureInt(t, ctx, counter, "n"); got != 0 {
		t.Errorf("n = %d after construction alone, want 0", got)
	}
}

// testStartRunsAsTheObject: the start runs Count once with the Counter as this, so its
// write lands on that object; the Counter outlives the behavior's completion.
func testStartRunsAsTheObject(t *testing.T) {
	ctx, counter, err := runStarter(t, starterModel(countOnce, `
		action kick { in target : Counter; perform target.count.start; }
		flow create.result to kick.target;
		first start then create; first create then kick; first kick then done;`))
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if got := featureInt(t, ctx, counter, "n"); got != 1 {
		t.Errorf("n = %d after the start, want 1", got)
	}
	b, ok := counter.Behavior("count")
	if !ok || b.Action == nil || b.Action.state != StateCompleted {
		t.Fatalf("count = %+v, %v; want the behavior complete on the object", b, ok)
	}
	if l, ok := ctx.OccurrenceLife(counter.ID); !ok || !l.Alive() {
		t.Errorf("OccurrenceLife(counter) = %v, %v; want the object alive once its behavior is done", l, ok)
	}
}

const countTicks = `
	first start;
	then action heard accept t : Tick;
	then action tally { assign seen := seen + 1; }
	then done;`

// testStartWokenByMessage: a started behavior parked at an accept is quiescence, not a
// deadlock; the Tick sent afterwards wakes it and it writes its object.
func testStartWokenByMessage(t *testing.T) {
	ctx, counter, err := runStarter(t, starterModel(countTicks, `
		action kick { in target : Counter; perform target.count.start; }
		action poke { in target : Counter; send new Tick() to target; }
		flow create.result to kick.target;
		flow create.result to poke.target;
		first start then create; first create then kick; first kick then poke; first poke then done;`))
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if got := featureInt(t, ctx, counter, "seen"); got != 1 {
		t.Errorf("seen = %d, want 1 once the Tick woke the started behavior", got)
	}
	b, ok := counter.Behavior("count")
	if !ok || b.Action == nil || b.Action.state != StateCompleted {
		t.Errorf("count = %+v, %v; want the woken behavior complete", b, ok)
	}
}

// testStartTwice: a second start of a behavior the object already runs is no second
// execution, whether the first is still parked or done.
func testStartTwice(t *testing.T) {
	ctx, counter, err := runStarter(t, starterModel(countTicks, `
		action kick { in target : Counter; perform target.count.start; }
		action again { in target : Counter; perform target.count.start; }
		action poke { in target : Counter; send new Tick() to target; }
		action once { in target : Counter; perform target.count.start; }
		flow create.result to kick.target;
		flow create.result to again.target;
		flow create.result to poke.target;
		flow create.result to once.target;
		first start then create; first create then kick; first kick then again;
		first again then poke; first poke then once; first once then done;`))
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if bs := counter.Behaviors(); len(bs) != 1 {
		t.Errorf("the Counter performs %d behaviors after three starts, want the one", len(bs))
	}
	if got := featureInt(t, ctx, counter, "seen"); got != 1 {
		t.Errorf("seen = %d after three starts and one Tick, want 1", got)
	}
}

// testStartInherited: an object of a specialization starts the behavior its general
// declares, with itself as this, so the write lands on the specialized object.
func testStartInherited(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		part def Counter {
			attribute n : Integer = 0;
			action def Count { first start; then action tally { assign n := n + 1; } then done; }
			action count : Count;
		}
		part def Clicker :> Counter { attribute label : String = "c"; }
		action def Starter {
			out made : Clicker;
			action create { out result : Clicker = new Clicker(); }
			action kick { in target : Clicker; perform target.count.start; }
			flow create.result to kick.target;
			first start then create; first create then kick; first kick then done;
			bind create.result = made;
		}
	}`
	ctx, clicker, err := runStarter(t, src)
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if clicker.Type == nil || clicker.Type.Name != "Clicker" {
		t.Fatalf("made a %s, want a Clicker", symbolText(clicker.Type))
	}
	if got := featureInt(t, ctx, clicker, "n"); got != 1 {
		t.Errorf("n = %d after the inherited start, want 1", got)
	}
	if b, ok := clicker.Behavior("count"); !ok || b.Action == nil || b.Action.state != StateCompleted {
		t.Errorf("count = %+v, %v; want the inherited behavior complete on the Clicker", b, ok)
	}
}

// testStartOnNoObject: a start whose object is an empty optional names no object to
// start the behavior on, and is refused saying so.
func testStartOnNoObject(t *testing.T) {
	_, _, err := runStarter(t, starterModel(countOnce, `
		action kick { in target : Counter[0..1] = (); perform target.count.start; }
		first start then create; first create then kick; first kick then done;`))
	if !errors.Is(err, ErrPerformerNotObject) || !strings.Contains(err.Error(), "target.count is started on null, which is no one object") {
		t.Fatalf("error = %v, want ErrPerformerNotObject over an empty target", err)
	}
}

// testStartOfNoBehavior: a start naming an attribute of the object names no behavior
// of it, and is refused with the object and the member named.
func testStartOfNoBehavior(t *testing.T) {
	_, _, err := runStarter(t, starterModel(countOnce, `
		action kick { in target : Counter; perform target.n.start; }
		flow create.result to kick.target;
		first start then create; first create then kick; first kick then done;`))
	if !errors.Is(err, ErrNoSuchBehavior) || !strings.Contains(err.Error(), "n is no behavior of object #") {
		t.Fatalf("error = %v, want ErrNoSuchBehavior naming n", err)
	}
}

// testStartNamedActionPerformed: `perform target.start` naming an action the object's
// type declares under the name start performs that action as a step, once, as any other
// perform of it would; it is no start of a behavior called `target`.
func testStartNamedActionPerformed(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		part def Vehicle {
			attribute n : Integer = 0;
			action def Launch { first start; then action tally { assign n := n + 1; } then done; }
			action start : Launch;
		}
		action def Starter {
			out made : Vehicle;
			action create { out result : Vehicle = new Vehicle(); }
			action kick { in target : Vehicle; perform target.start; }
			flow create.result to kick.target;
			first start then create; first create then kick; first kick then done;
			bind create.result = made;
		}
	}`
	ctx, vehicle, err := runStarter(t, src)
	if err != nil {
		t.Fatalf("Starter: %v", err)
	}
	if got := featureInt(t, ctx, vehicle, "n"); got != 1 {
		t.Errorf("n = %d after performing the action named start, want 1", got)
	}
	if bs := vehicle.Behaviors(); len(bs) != 0 {
		t.Errorf("the Vehicle performs %v, want no behavior started on it by a perform of its start action", bs)
	}
}

// testStartFailureRollsBack: a behavior whose start fails is undone whole; the error
// names the failure, and no object of the run keeps a write of it.
func testStartFailureRollsBack(t *testing.T) {
	ctx, _, err := runStarter(t, starterModel(`
		first start;
		then action tally { assign n := n + 1; }
		then action blow { assign seen := 1 / 0; }
		then done;`, `
		action kick { in target : Counter; perform target.count.start; }
		flow create.result to kick.target;
		first start then create; first create then kick; first kick then done;`))
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("error = %v, want the division by zero of the started behavior", err)
	}
	for _, inst := range ctx.instances {
		if inst.Type == nil || inst.Type.Name != "Counter" {
			continue
		}
		if bs := inst.Behaviors(); len(bs) != 0 {
			t.Errorf("the Counter performs %v after the failed start, want nothing", bs)
		}
		fv, err := inst.GetFeatureValue(ctx, "n")
		if err != nil {
			t.Fatalf("n after the failed start: %v", err)
		}
		if got := fv.HeldValue(); got.Kind == ValConst && got.Const.Int != 0 {
			t.Errorf("n = %v after the failed start, want the write undone", FormatValue(got))
		}
	}
}

// testStartWakesOlderBehaviorAfterCommit: a message the started behavior sends wakes an
// older parked behavior only once the start is kept, so the older one's failure is the
// run's, not the start's: the start stands and the message is consumed, not restored.
func testStartWakesOlderBehaviorAfterCommit(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		attribute def Tick;
		part def Listener {
			attribute seen : Integer = 0;
			action def Listen {
				first start;
				then action heard accept t : Tick;
				then action blow { assign seen := 1 / 0; }
				then done;
			}
			action listen : Listen;
		}
		part def Pinger {
			ref part peer : Listener;
			action def Ping { first start; then action fire send new Tick() to peer; then done; }
			action ping : Ping;
		}
		action def Starter {
			action makeA { out result : Listener = new Listener(); }
			action makeB { in target : Listener; out result : Pinger = new Pinger(peer = target); }
			action kickA { in target : Listener; perform target.listen.start; }
			action kickB { in target : Pinger; perform target.ping.start; }
			flow makeA.result to makeB.target;
			flow makeA.result to kickA.target;
			flow makeB.result to kickB.target;
			first start then makeA; first makeA then kickA; first kickA then makeB;
			first makeB then kickB; first kickB then done;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Starter", ast.DefAction)
	if sym == nil {
		t.Fatal("action Starter not found")
	}
	_, err := ctx.ExecuteAction(sym)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("error = %v, want the division by zero of the woken Listener", err)
	}
	for _, inst := range ctx.instances {
		if inst.Type == nil {
			continue
		}
		switch inst.Type.Name {
		case "Pinger":
			b, ok := inst.Behavior("ping")
			if !ok || b.Action == nil || b.Action.state != StateCompleted {
				t.Errorf("ping = %+v, %v; want the start kept and its behavior complete", b, ok)
			}
		case "Listener":
			b, ok := inst.Behavior("listen")
			if !ok || b.Action == nil || b.Action.state == StateWaiting {
				t.Errorf("listen = %+v, %v; want the woken behavior past its accept", b, ok)
			}
		}
	}
	if n := len(ctx.messages); n != 0 {
		t.Errorf("%d messages in flight, want the Tick consumed rather than restored", n)
	}
}

// testStartTraced: the trace records the start as the object's own execution of its
// performed action count, once, as it records a start at construction.
func testStartTraced(t *testing.T) {
	src := starterModel(countOnce, `
		action kick { in target : Counter; perform target.count.start; }
		flow create.result to kick.target;
		first start then create; first create then kick; first kick then done;`)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Starter", ast.DefAction)
	if sym == nil {
		t.Fatal("action Starter not found")
	}
	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("Starter: %v", err)
	}
	var starts []string
	for _, line := range tr.Entries() {
		if strings.HasPrefix(line, "start: ") {
			starts = append(starts, line)
		}
	}
	if len(starts) != 1 || !strings.HasPrefix(starts[0], "start: performed action count of #") {
		t.Errorf("trace starts = %q, want one start of the performed action count on the object", starts)
	}
}
