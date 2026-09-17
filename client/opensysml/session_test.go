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
