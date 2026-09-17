package lord

import (
	"strings"
	"testing"
)

func TestWelcomeShowsTheTownSquare(t *testing.T) {
	g := newGame(t, 1, Character{Name: "Sir Wasm", Class: "deathKnight"})
	v, err := g.Welcome()
	if err != nil {
		t.Fatal(err)
	}
	if v.Character || v.Location != "townSquare" || v.Warrior == nil || v.Warrior.Name != "Sir Wasm" || v.Screen == nil {
		t.Fatalf("welcome = %+v", v)
	}
	if len(v.Lines) != 1 || !strings.Contains(v.Lines[0], "Welcome to the realm, Sir Wasm") {
		t.Fatalf("welcome lines = %q", v.Lines)
	}
	if !strings.Contains(v.Screen.Title, "Town Square") {
		t.Fatalf("screen = %+v", v.Screen)
	}
}

func TestPlayedTellsWhereTheWarriorWent(t *testing.T) {
	g := newGame(t, 1, Character{Name: "Sir Wasm", Class: "deathKnight"})
	o, err := g.Play("K", nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := g.Played(o)
	if err != nil {
		t.Fatal(err)
	}
	if v.Refused || v.Location != "bank" || len(v.Lines) != 1 || !strings.Contains(v.Lines[0], "You make your way to") {
		t.Fatalf("after K: refused=%v %s %q", v.Refused, v.Location, v.Lines)
	}
}

func TestPlayedMarksARefusal(t *testing.T) {
	g := newGame(t, 1, Character{Name: "Sir Wasm", Class: "deathKnight"})
	if _, err := g.Play("K", nil); err != nil {
		t.Fatal(err)
	}
	o, err := g.Play("W", map[string]string{"amount": "999"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := g.Played(o)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Refused || v.Warrior.Gold != 500 || v.Warrior.BankGold != 0 || len(v.Lines) != 1 || !strings.Contains(v.Lines[0], "cannot") {
		t.Fatalf("overdrawing the bank: refused=%v %+v %q", v.Refused, *v.Warrior, v.Lines)
	}
	o, err = g.Play("D", map[string]string{"amount": "100"})
	if err != nil {
		t.Fatal(err)
	}
	if v, err = g.Played(o); err != nil {
		t.Fatal(err)
	}
	if v.Refused || v.Warrior.Gold != 400 || v.Warrior.BankGold != 100 {
		t.Fatalf("a deposit within the purse: refused=%v %+v %q", v.Refused, *v.Warrior, v.Lines)
	}
}

func TestPlayedTellsANoOpFromARefusal(t *testing.T) {
	g := newGame(t, 1, Character{})
	if _, err := g.Play("K", nil); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"D", "W"} {
		o, err := g.Play(key, map[string]string{"amount": "0"})
		if err != nil {
			t.Fatal(err)
		}
		v, err := g.Played(o)
		if err != nil {
			t.Fatal(err)
		}
		if o.Refused || v.Refused || *o.Before != *o.After || len(v.Lines) != 1 || !strings.Contains(v.Lines[0], "Nothing comes of it") {
			t.Fatalf("%s of nothing: refused=%v %q", key, v.Refused, v.Lines)
		}
	}
	o, err := g.Play("W", map[string]string{"amount": "-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Refused || *o.Before != *o.After {
		t.Fatalf("withdrawing a negative sum: refused=%v %+v", o.Refused, *o.After)
	}
}

func TestARefusalNeedNotEndAtDone(t *testing.T) {
	g := newGame(t, 1, Character{})
	if _, err := g.Send("VisitTheTrainingHall"); err != nil {
		t.Fatal(err)
	}
	o, err := g.Invoke("train", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Refused || o.After.Level != 1 || o.After.TrainedToday {
		t.Fatalf("an untried warrior was trained: refused=%v %+v", o.Refused, *o.After)
	}
}

func TestARefusalMayFollowAReckoning(t *testing.T) {
	g := newGame(t, 1, Character{})
	if _, err := g.Send("VisitTheInn"); err != nil {
		t.Fatal(err)
	}
	o, err := g.Play("F", map[string]string{"favour": "town.inn.violet.wink"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := g.Played(o)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Refused || !v.Refused || o.After.Charm != o.Before.Charm || *o.Before != *o.After {
		t.Fatalf("a penniless wink: refused=%v %+v %q", v.Refused, *o.After, v.Lines)
	}
}

func TestCharacterViewAsksForAWarrior(t *testing.T) {
	v := CharacterView("Create one.")
	if !v.Character || v.Warrior != nil || v.Screen != nil || len(v.Lines) != 1 {
		t.Fatalf("character view = %+v", v)
	}
}

func TestRandomSeedVaries(t *testing.T) {
	seen := map[uint64]bool{}
	for i := 0; i < 8; i++ {
		seen[RandomSeed()] = true
	}
	if len(seen) < 2 {
		t.Fatalf("eight seeds, %d distinct", len(seen))
	}
}
