package opensysml_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// sessionSource is a small interactive model: a hero whose exhibited machine
// moves on signals, with a guarded transition, a completion transition, an
// action that may turn its caller away and one that rolls dice.
const sessionSource = `package Play {
	private import ScalarValues::*;
	attribute def Go;
	attribute def Rest;
	item def Fight;
	enum def Mood { calm; wild; }
	part def Hero {
		attribute gold : Integer = 10;
		attribute strength : Integer = 3;
		attribute mood : Mood = Mood::calm;
		attribute rested : Boolean = false;
		calc twice { gold * 2 }
		exhibit state day {
			entry; then town;
			state town;
			state road;
			state camp;
			state home;
			transition town_road first town accept Go then road;
			transition road_camp first road accept Rest if gold > 5 then camp;
			transition road_fight first road accept Fight then town;
			transition camp_home first camp then home;
			transition home_town first home accept Go then town;
		}
		action pay {
			in cost : Integer = 1;
			first start;
			then decide canPay;
			if gold >= cost then paying;
			else done;
			action paying { assign gold := gold - cost; }
			succession first paying then done;
		}
		action gamble {
			in wager : Integer = 1;
			out won : Boolean = false;
			first start;
			then decide dice;
			first dice if true then win;
			first dice if true then lose;
			action win { assign gold := gold + wager; assign won := true; }
			action lose { assign gold := gold - wager; }
			succession first win then done;
			succession first lose then done;
		}
	}
	part hero : Hero;
}`

func openSession(t *testing.T) (*opensysml.Session, opensysml.InstanceID) {
	t.Helper()
	client := newClient(t)
	model := parse(t, client, sessionSource)
	session, err := opensysml.OpenSession(client, model)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	hero, err := session.Instantiate("Play::hero")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return session, hero
}

func activeStates(t *testing.T, session *opensysml.Session, hero opensysml.InstanceID) []string {
	t.Helper()
	states, err := session.ActiveStates(hero)
	if err != nil {
		t.Fatalf("ActiveStates: %v", err)
	}
	return states
}

func TestSessionRetainsTheObjectAcrossCalls(t *testing.T) {
	session, hero := openSession(t)

	gold, err := session.Feature(hero, "gold")
	if err != nil || gold.Value != opensysml.Int(10) {
		t.Fatalf("Feature gold = %#v, %v; want Int(10)", gold, err)
	}
	if err := session.SetFeature(hero, "gold", opensysml.Int(3)); err != nil {
		t.Fatalf("SetFeature: %v", err)
	}
	value, err := session.Evaluate("hero.gold + 1", opensysml.WithContextSymbol("Play"))
	if err != nil || value != opensysml.Int(4) {
		t.Fatalf("Evaluate hero.gold + 1 = %#v, %v; want Int(4)", value, err)
	}
	value, err = session.Evaluate("Mood::wild", opensysml.WithContextSymbol("Play"))
	if err != nil {
		t.Fatalf("Evaluate Mood::wild: %v", err)
	}
	if literal, ok := value.(opensysml.EnumLiteral); !ok || literal.LiteralID != "Play::Mood::wild" {
		t.Errorf("Evaluate Mood::wild = %#v, want the enumeration literal", value)
	}
	members, err := session.Members("Play::Mood")
	if err != nil {
		t.Fatalf("Members: %v", err)
	}
	var names []string
	for _, m := range members {
		names = append(names, m.Name)
	}
	if want := []string{"calm", "wild"}; !reflect.DeepEqual(names, want) {
		t.Errorf("Members(Play::Mood) = %v, want %v", names, want)
	}
}

func TestSessionEvaluatesEachExpressionAfresh(t *testing.T) {
	t.Setenv("OPENSYSML_MAX_STEPS", "200")
	session, hero := openSession(t)

	value, err := session.Evaluate("hero.twice", opensysml.WithContextSymbol("Play"))
	if err != nil || value != opensysml.Int(20) {
		t.Fatalf("Evaluate hero.twice = %#v, %v; want Int(20)", value, err)
	}
	if err := session.SetFeature(hero, "gold", opensysml.Int(7)); err != nil {
		t.Fatalf("SetFeature: %v", err)
	}
	value, err = session.Evaluate("hero.twice", opensysml.WithContextSymbol("Play"))
	if err != nil || value != opensysml.Int(14) {
		t.Fatalf("Evaluate hero.twice after SetFeature = %#v, %v; want Int(14)", value, err)
	}
	for i := 0; i < 50; i++ {
		if _, err := session.Evaluate("hero.gold + hero.strength * 2 - hero.twice", opensysml.WithContextSymbol("Play")); err != nil {
			t.Fatalf("Evaluate #%d under a per-run step budget: %v", i, err)
		}
	}
}

func TestSessionReportsTransitionsAndAcceptance(t *testing.T) {
	session, hero := openSession(t)
	if got := activeStates(t, session, hero); !reflect.DeepEqual(got, []string{"town"}) {
		t.Fatalf("ActiveStates = %v, want [town]", got)
	}
	transitions, err := session.Transitions(hero)
	if err != nil {
		t.Fatalf("Transitions: %v", err)
	}
	want := []opensysml.Transition{{Name: "town_road", Source: "town", Target: "road", Trigger: opensysml.TriggerSignal, Signal: "Go"}}
	if !reflect.DeepEqual(transitions, want) {
		t.Errorf("Transitions = %+v, want %+v", transitions, want)
	}

	if _, err := session.Send(hero, "Play::Go", nil); err != nil {
		t.Fatalf("Send Go: %v", err)
	}
	if _, err := session.Advance(0); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := activeStates(t, session, hero); !reflect.DeepEqual(got, []string{"road"}) {
		t.Fatalf("ActiveStates after Go = %v, want [road]", got)
	}
	transitions, err = session.Transitions(hero)
	if err != nil {
		t.Fatalf("Transitions: %v", err)
	}
	if len(transitions) != 2 || transitions[0].Signal != "Rest" || !transitions[0].Guarded || transitions[1].Signal != "Fight" || transitions[1].Guarded {
		t.Errorf("Transitions out of road = %+v, want guarded Rest then Fight", transitions)
	}

	// The guard on Rest holds with gold 10 and fails with gold 1.
	acceptance, err := session.Accepts(hero, "Play::Rest", nil)
	if err != nil || !acceptance.Accepted || !acceptance.Enabled() || len(acceptance.Fires) != 1 || acceptance.Fires[0].Name != "road_camp" {
		t.Fatalf("Accepts Rest = %+v, %v; want road_camp enabled", acceptance, err)
	}
	if err := session.SetFeature(hero, "gold", opensysml.Int(1)); err != nil {
		t.Fatal(err)
	}
	acceptance, err = session.Accepts(hero, "Play::Rest", nil)
	if err != nil || !acceptance.Accepted || acceptance.Enabled() {
		t.Fatalf("Accepts Rest with gold 1 = %+v, %v; want accepted but not enabled", acceptance, err)
	}
	if _, err := session.Send(hero, "Play::Rest", nil); !hasCode(err, opensysml.CodeFailedPrecondition) {
		t.Errorf("Send Rest with its guard false: %v, want CodeFailedPrecondition", err)
	}
	if got := activeStates(t, session, hero); !reflect.DeepEqual(got, []string{"road"}) {
		t.Errorf("ActiveStates after the refused Send = %v, want [road]", got)
	}

	// The completion transition out of camp fires within the same Advance.
	if err := session.SetFeature(hero, "gold", opensysml.Int(10)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Send(hero, "Play::Rest", nil); err != nil {
		t.Fatalf("Send Rest: %v", err)
	}
	if _, err := session.Advance(0); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := activeStates(t, session, hero); !reflect.DeepEqual(got, []string{"home"}) {
		t.Errorf("ActiveStates after Rest = %v, want [home] through the completion transition", got)
	}
}

// nestedSource exhibits two machines, one with a composite state whose own
// transition leaves from whichever nested state is active.
const nestedSource = `package Nest {
	attribute def Abort;
	attribute def Step;
	attribute def Spin;
	part def Unit {
		exhibit state work {
			entry; then working;
			state working {
				entry; then step1;
				state step1;
				state step2;
				transition step1_step2 first step1 accept Step then step2;
			}
			state done;
			transition working_done first working accept Abort then done;
		}
		exhibit state fan {
			entry; then off;
			state off;
			state on;
			transition off_on first off accept Spin then on;
		}
	}
	part unit : Unit;
}`

func TestSessionSeesEnclosingStatesAndEveryMachine(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, nestedSource)
	session, err := opensysml.OpenSession(client, model)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	unit, err := session.Instantiate("Nest::unit")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := activeStates(t, session, unit); !reflect.DeepEqual(got, []string{"step1", "off"}) {
		t.Fatalf("ActiveStates = %v, want [step1 off], one per machine", got)
	}
	transitions, err := session.Transitions(unit)
	if err != nil {
		t.Fatalf("Transitions: %v", err)
	}
	want := []opensysml.Transition{
		{Name: "step1_step2", Source: "step1", Target: "step2", Trigger: opensysml.TriggerSignal, Signal: "Step"},
		{Name: "working_done", Source: "working", Target: "done", Trigger: opensysml.TriggerSignal, Signal: "Abort"},
		{Name: "off_on", Source: "off", Target: "on", Trigger: opensysml.TriggerSignal, Signal: "Spin"},
	}
	if !reflect.DeepEqual(transitions, want) {
		t.Errorf("Transitions = %+v, want the nested state's, then its enclosing state's, then the second machine's", transitions)
	}

	// The enclosing state's transition is accepted while a nested state is active.
	acceptance, err := session.Accepts(unit, "Nest::Abort", nil)
	if err != nil || !acceptance.Accepted || len(acceptance.Fires) != 1 || acceptance.Fires[0].Name != "working_done" {
		t.Fatalf("Accepts Abort = %+v, %v; want working_done", acceptance, err)
	}
	// The second machine's signal is accepted and dispatched to it alone.
	acceptance, err = session.Accepts(unit, "Nest::Spin", nil)
	if err != nil || !acceptance.Accepted || len(acceptance.Fires) != 1 || acceptance.Fires[0].Name != "off_on" {
		t.Fatalf("Accepts Spin = %+v, %v; want off_on", acceptance, err)
	}
	if _, err := session.Send(unit, "Nest::Spin", nil); err != nil {
		t.Fatalf("Send Spin: %v", err)
	}
	if _, err := session.Advance(0); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := activeStates(t, session, unit); !reflect.DeepEqual(got, []string{"step1", "on"}) {
		t.Errorf("ActiveStates after Spin = %v, want [step1 on]", got)
	}
	if _, err := session.Send(unit, "Nest::Abort", nil); err != nil {
		t.Fatalf("Send Abort: %v", err)
	}
	if _, err := session.Advance(0); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := activeStates(t, session, unit); !reflect.DeepEqual(got, []string{"done", "on"}) {
		t.Errorf("ActiveStates after Abort = %v, want [done on]", got)
	}
}

const twinSource = `package Twin {
	private import ScalarValues::*;
	attribute def Ping;
	attribute def Go;
	part def Pair {
		attribute leftArmed : Boolean = false;
		attribute rightArmed : Boolean = true;
		exhibit state left {
			entry; then idle;
			state idle;
			transition left_go first idle accept Ping if leftArmed then done;
			state done;
		}
		exhibit state right {
			entry; then busy;
			state busy { defer Ping; }
			transition right_ready first busy accept Go then ready;
			state ready;
			transition right_go first ready accept Ping if rightArmed then done;
			state done;
		}
	}
	part pair : Pair;
}`

// Accepts reads the machines dispatch would let take the signal, so what it
// reports is what Send and Advance then do.
func TestSessionAcceptsMatchesDispatchAcrossMachines(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, twinSource)
	session, err := opensysml.OpenSession(client, model)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	pair, err := session.Instantiate("Twin::pair")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	ping := func() *opensysml.Acceptance {
		t.Helper()
		acceptance, err := session.Accepts(pair, "Twin::Ping", nil)
		if err != nil {
			t.Fatalf("Accepts Ping: %v", err)
		}
		return acceptance
	}
	send := func(signal string) {
		t.Helper()
		if _, err := session.Send(pair, "Twin::"+signal, nil); err != nil {
			t.Fatalf("Send %s: %v", signal, err)
		}
		if _, err := session.Advance(0); err != nil {
			t.Fatalf("Advance: %v", err)
		}
	}
	states := func(want ...string) {
		t.Helper()
		if got := activeStates(t, session, pair); !reflect.DeepEqual(got, want) {
			t.Fatalf("ActiveStates = %v, want %v", got, want)
		}
	}

	// left's guard is false and right only defers: the signal is taken, deferred,
	// and no transition is triggered by it, distinct facts Send agrees with.
	acceptance := ping()
	if acceptance.Accepted || !acceptance.Deferred || len(acceptance.Fires) != 0 || !acceptance.Taken() || !acceptance.Enabled() {
		t.Fatalf("Accepts Ping with left disarmed and right busy = %+v; want deferred only", acceptance)
	}
	send("Ping")
	states("idle", "busy")

	// Go readies right, which then fires on the deferred Ping alone; left, whose
	// guard is false, yields it rather than dropping it.
	send("Go")
	states("idle", "done")

	// Back in ready with the guards swapped, left would fire and right, whose
	// guard is false, yields: only left_go is reported, and only left moves.
	pair, err = session.Instantiate("Twin::pair")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := session.SetFeature(pair, "leftArmed", opensysml.Bool(true)); err != nil {
		t.Fatalf("SetFeature leftArmed: %v", err)
	}
	if err := session.SetFeature(pair, "rightArmed", opensysml.Bool(false)); err != nil {
		t.Fatalf("SetFeature rightArmed: %v", err)
	}
	send("Go")
	states("idle", "ready")
	acceptance = ping()
	if !acceptance.Accepted || acceptance.Deferred || len(acceptance.Fires) != 1 || acceptance.Fires[0].Name != "left_go" {
		t.Fatalf("Accepts Ping with only left armed = %+v; want left_go alone", acceptance)
	}
	send("Ping")
	states("done", "ready")

	// With both guards holding, both would take it and the schedule's due order
	// decides which consumes it: both fires are reported, exactly one machine
	// moves, and Advance notes the choice.
	pair, err = session.Instantiate("Twin::pair")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := session.SetFeature(pair, "leftArmed", opensysml.Bool(true)); err != nil {
		t.Fatalf("SetFeature leftArmed: %v", err)
	}
	send("Go")
	acceptance = ping()
	if !acceptance.Accepted || len(acceptance.Fires) != 2 || acceptance.Fires[0].Name != "left_go" || acceptance.Fires[1].Name != "right_go" {
		t.Fatalf("Accepts Ping with both armed = %+v; want left_go then right_go", acceptance)
	}
	if _, err := session.Send(pair, "Twin::Ping", nil); err != nil {
		t.Fatalf("Send Ping: %v", err)
	}
	advanced, err := session.Advance(0)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	dueOrder := 0
	for _, choice := range advanced.Choices {
		if choice.Kind == "due order" && len(choice.Alternatives) == 2 {
			dueOrder++
		}
	}
	if dueOrder != 1 {
		t.Fatalf("Advance chose the due order %d times among %+v; want once between the two machines", dueOrder, advanced.Choices)
	}
	if got := activeStates(t, session, pair); !reflect.DeepEqual(got, []string{"done", "ready"}) && !reflect.DeepEqual(got, []string{"idle", "done"}) {
		t.Fatalf("ActiveStates = %v, want exactly one machine to have consumed Ping", got)
	}

	// With neither guard holding, both machines are triggered and would drop it:
	// accepted, not enabled, and Send refuses it as Advance would do nothing.
	pair, err = session.Instantiate("Twin::pair")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := session.SetFeature(pair, "rightArmed", opensysml.Bool(false)); err != nil {
		t.Fatalf("SetFeature rightArmed: %v", err)
	}
	send("Go")
	acceptance = ping()
	if !acceptance.Accepted || acceptance.Enabled() {
		t.Fatalf("Accepts Ping with neither armed = %+v; want accepted but not enabled", acceptance)
	}
	if _, err := session.Send(pair, "Twin::Ping", nil); !hasCode(err, opensysml.CodeFailedPrecondition) {
		t.Fatalf("Send Ping with neither armed: %v, want CodeFailedPrecondition", err)
	}
	states("idle", "ready")
}

func TestSessionPerformReportsBranchesAndChoices(t *testing.T) {
	session, hero := openSession(t)

	paid, err := session.Perform(hero, "Play::Hero::pay", map[string]opensysml.Value{"cost": opensysml.Int(4)})
	if err != nil {
		t.Fatalf("Perform pay: %v", err)
	}
	if paid.TurnedAway() || len(paid.Branches) != 1 || paid.Branches[0] != (opensysml.Branch{Decision: "canPay", Target: "paying", Opening: true}) {
		t.Errorf("pay(4) branches = %+v, want the opening decision left for paying", paid.Branches)
	}
	gold, err := session.Feature(hero, "gold")
	if err != nil || gold.Value != opensysml.Int(6) {
		t.Errorf("gold after pay(4) = %#v, %v; want Int(6)", gold, err)
	}

	refused, err := session.Perform(hero, "Play::Hero::pay", map[string]opensysml.Value{"cost": opensysml.Int(100)})
	if err != nil {
		t.Fatalf("Perform pay(100): %v", err)
	}
	if !refused.TurnedAway() || len(refused.Branches) != 1 || !refused.Branches[0].Else || !refused.Branches[0].Opening {
		t.Errorf("pay(100) branches = %+v, want the opening decision left by else", refused.Branches)
	}
	gold, err = session.Feature(hero, "gold")
	if err != nil || gold.Value != opensysml.Int(6) {
		t.Errorf("gold after the refused pay = %#v, %v; want Int(6) unchanged", gold, err)
	}

	outcomes := func(seed string) (choices []opensysml.ChoicePoint, gold opensysml.Value) {
		t.Helper()
		if err := session.SetSchedule(seed); err != nil {
			t.Fatalf("SetSchedule(%s): %v", seed, err)
		}
		if err := session.SetFeature(hero, "gold", opensysml.Int(10)); err != nil {
			t.Fatal(err)
		}
		gambled, err := session.Perform(hero, "Play::Hero::gamble", map[string]opensysml.Value{"wager": opensysml.Int(3)})
		if err != nil {
			t.Fatalf("Perform gamble: %v", err)
		}
		if _, ok := gambled.Outputs["won"].(opensysml.Bool); !ok {
			t.Errorf("gamble outputs = %#v, want a Bool won", gambled.Outputs)
		}
		fv, err := session.Feature(hero, "gold")
		if err != nil {
			t.Fatal(err)
		}
		return gambled.Choices, fv.Value
	}
	firstChoices, firstGold := outcomes("seed:7")
	if len(firstChoices) != 1 || firstChoices[0].Kind != opensysml.ChoiceDecisionBranch || firstChoices[0].Where != "decision dice" || len(firstChoices[0].Alternatives) != 2 {
		t.Fatalf("gamble choices = %+v, want one decision branch at decision dice with two alternatives", firstChoices)
	}
	if firstGold != opensysml.Int(13) && firstGold != opensysml.Int(7) {
		t.Errorf("gold after gamble = %#v, want 13 or 7", firstGold)
	}
	againChoices, againGold := outcomes("seed:7")
	if !reflect.DeepEqual(againChoices, firstChoices) || againGold != firstGold {
		t.Errorf("the same seed chose %+v (gold %v), want %+v (gold %v) again", againChoices, againGold, firstChoices, firstGold)
	}
	sawOther := false
	for _, seed := range []string{"seed:1", "seed:2", "seed:3", "seed:4", "seed:5", "seed:6", "seed:8", "seed:9"} {
		if choices, _ := outcomes(seed); choices[0].Taken != firstChoices[0].Taken {
			sawOther = true
			break
		}
	}
	if !sawOther {
		t.Error("every seed made the same dice choice; want the seed to matter")
	}
	if err := session.SetSchedule("explore[all]"); !hasCode(err, opensysml.CodeInvalidArgument) {
		t.Errorf("SetSchedule(explore[all]) = %v, want CodeInvalidArgument", err)
	}
}

func TestSessionRefusesMisuseWithTypedErrors(t *testing.T) {
	session, hero := openSession(t)

	if _, err := session.Send(hero, "Play::Rest", nil); !hasCode(err, opensysml.CodeFailedPrecondition) {
		t.Errorf("Send a signal no transition out of town accepts: %v, want CodeFailedPrecondition", err)
	}
	if _, err := session.Send(hero, "Play::Nothing", nil); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Send an undeclared signal: %v, want a FailureError", err)
	}
	if _, err := session.Send(hero, "Play::Hero::pay", nil); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Send an action as a signal: %v, want a FailureError", err)
	}
	if _, err := session.Perform(hero, "Play::Hero::sing", nil); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Perform an unknown action: %v, want a FailureError", err)
	}
	if _, err := session.Perform(opensysml.InstanceID(999), "Play::Hero::pay", nil); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Perform on an object the session does not hold: %v, want a FailureError", err)
	}
	if _, err := session.Feature(hero, "mana"); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Feature the type lacks: %v, want a FailureError", err)
	}
	if _, err := session.Evaluate("hero.gold +", opensysml.WithContextSymbol("Play")); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Evaluate a malformed expression: %v, want a FailureError", err)
	}
	if _, err := session.Evaluate("1", opensysml.WithSubject("Play::hero")); !hasCode(err, opensysml.CodeInvalidArgument) {
		t.Errorf("Evaluate with a subject: %v, want CodeInvalidArgument", err)
	}
	if _, err := session.Advance(-1); !hasCode(err, opensysml.CodeInvalidArgument) {
		t.Errorf("Advance(-1): %v, want CodeInvalidArgument", err)
	}
	if err := session.SetSchedule("nonsense"); !hasCode(err, opensysml.CodeInvalidArgument) {
		t.Errorf("SetSchedule(nonsense): %v, want CodeInvalidArgument", err)
	}
	if _, err := session.Instantiate("Play::Nowhere"); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Instantiate an undeclared part: %v, want a FailureError", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if _, err := session.ActiveStates(hero); !hasCode(err, opensysml.CodeUnavailable) {
		t.Errorf("ActiveStates on a closed session: %v, want CodeUnavailable", err)
	}
	if _, err := session.Perform(hero, "Play::Hero::pay", nil); !hasCode(err, opensysml.CodeUnavailable) {
		t.Errorf("Perform on a closed session: %v, want CodeUnavailable", err)
	}
	if _, err := session.Send(hero, "Play::Go", nil); !hasCode(err, opensysml.CodeUnavailable) {
		t.Errorf("Send on a closed session: %v, want CodeUnavailable", err)
	}
}

func TestSessionRefusesToPassTheObjectBound(t *testing.T) {
	t.Setenv("OPENSYSML_GRPC_MAX_HELD_OBJECTS", "4")
	session, _ := openSession(t)
	for i := 0; i < 8; i++ {
		_, err := session.Instantiate("Play::hero")
		if err == nil {
			continue
		}
		if !hasCode(err, opensysml.CodeResourceExhausted) {
			t.Fatalf("Instantiate past the bound: %v, want CodeResourceExhausted", err)
		}
		return
	}
	t.Fatal("Instantiate never reached the bound of 4 held objects")
}

// eventSource accepts by the event feature an accept trigger subsets, not by a
// signal type: the transition fact names the feature, and dispatch matches it.
const eventSource = `package Watch {
	private import ScalarValues::*;
	item def Ping;
	part def Unit {
		item alert : Ping;
		exhibit state duty {
			entry; then standingBy;
			state standingBy;
			state working;
			transition wake first standingBy accept :> alert then working;
		}
	}
	part unit : Unit;
}`

func TestSessionNamesTheEventAnAcceptSubsets(t *testing.T) {
	client := newClient(t)
	session, err := opensysml.OpenSession(client, parse(t, client, eventSource))
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	unit, err := session.Instantiate("Watch::unit")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	transitions, err := session.Transitions(unit)
	if err != nil {
		t.Fatalf("Transitions: %v", err)
	}
	want := []opensysml.Transition{{Name: "wake", Source: "standingBy", Target: "working", Trigger: opensysml.TriggerSignal, Event: "alert"}}
	if !reflect.DeepEqual(transitions, want) {
		t.Errorf("Transitions = %+v, want %+v", transitions, want)
	}
}

func TestOpenSessionIsInProcessOnly(t *testing.T) {
	remote := dialClient(t, startService(t))
	model := parse(t, remote, sessionSource)
	if _, err := opensysml.OpenSession(remote, model); !hasCode(err, opensysml.CodeUnimplemented) {
		t.Errorf("OpenSession over Dial: %v, want CodeUnimplemented", err)
	}

	local := newClient(t)
	if _, err := opensysml.OpenSession(local, model); !hasCode(err, opensysml.CodeNotFound) {
		t.Errorf("OpenSession over a model another client parsed: %v, want CodeNotFound", err)
	}
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opensysml.OpenSession(local, model); !hasCode(err, opensysml.CodeUnavailable) {
		t.Errorf("OpenSession from a closed client: %v, want CodeUnavailable", err)
	}
}

func hasCode(err error, code opensysml.Code) bool {
	var status *opensysml.StatusError
	return errors.As(err, &status) && status.Code == code
}

// forkSource is a machine whose one signal enables two transitions, so which
// fires is the schedule's choice: a seeded run's first draw.
const forkSource = `package Fork {
	attribute def Go;
	part def Chooser {
		exhibit state pick {
			entry; then start;
			state start;
			state first;
			state second;
			transition to_first first start accept Go then first;
			transition to_second first start accept Go then second;
			transition back_first first first accept Go then start;
			transition back_second first second accept Go then start;
		}
	}
	part chooser : Chooser;
}`

// A policy set after the object was instantiated and its clock advanced governs
// the choices the later turns make, as it would a fresh session's first.
func TestSessionSetScheduleGovernsTheLaterTurns(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, forkSource)
	open := func() (*opensysml.Session, opensysml.InstanceID) {
		t.Helper()
		session, err := opensysml.OpenSession(client, model)
		if err != nil {
			t.Fatalf("OpenSession: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })
		chooser, err := session.Instantiate("Fork::chooser")
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		return session, chooser
	}
	go_ := func(session *opensysml.Session, chooser opensysml.InstanceID) string {
		t.Helper()
		if _, err := session.Send(chooser, "Fork::Go", nil); err != nil {
			t.Fatalf("Send Go: %v", err)
		}
		if _, err := session.Advance(0); err != nil {
			t.Fatalf("Advance: %v", err)
		}
		states := activeStates(t, session, chooser)
		if len(states) != 1 {
			t.Fatalf("ActiveStates = %v, want one", states)
		}
		return states[0]
	}
	seeds := []string{"seed:1", "seed:2", "seed:3", "seed:4", "seed:5", "seed:6", "seed:7", "seed:8"}

	// What each seed's first draw picks, read from a fresh session per seed.
	first := make(map[string]string, len(seeds))
	picked := make(map[string]bool)
	for _, seed := range seeds {
		session, chooser := open()
		if err := session.SetSchedule(seed); err != nil {
			t.Fatalf("SetSchedule(%s): %v", seed, err)
		}
		first[seed] = go_(session, chooser)
		picked[first[seed]] = true
	}
	if len(picked) != 2 {
		t.Fatalf("the seeds' first draws all pick %v; want both transitions among them", picked)
	}

	// One session, its clock already advanced under the default policy: each
	// seed set from then on starts its draws over.
	session, chooser := open()
	if state := go_(session, chooser); state == "start" {
		t.Fatalf("Go under the default policy left the machine in %s", state)
	}
	for _, seed := range seeds {
		if state := go_(session, chooser); state != "start" {
			t.Fatalf("Go back left the machine in %s, want start", state)
		}
		if err := session.SetSchedule(seed); err != nil {
			t.Fatalf("SetSchedule(%s): %v", seed, err)
		}
		if got := go_(session, chooser); got != first[seed] {
			t.Errorf("Go under %s set after earlier turns went to %s; a fresh session's first draw goes to %s", seed, got, first[seed])
		}
	}
}
