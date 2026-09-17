package lordweb

import (
	"errors"
	"strings"
	"testing"
)

func menu(t *testing.T, g *Game) *Screen {
	t.Helper()
	s, err := g.Menu()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func find(t *testing.T, s *Screen, key string) Choice {
	t.Helper()
	for _, c := range s.Choices {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("no choice %q on %s: %v", key, s.Title, keys(s))
	return Choice{}
}

func keys(s *Screen) []string {
	var out []string
	for _, c := range s.Choices {
		out = append(out, c.Key)
	}
	return out
}

func play(t *testing.T, g *Game, key string, inputs map[string]string) *Outcome {
	t.Helper()
	o, err := g.Play(key, inputs)
	if err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return o
}

func TestMenuShowsTheStatesSignalsAndActions(t *testing.T) {
	g := newGame(t, 1, Character{})
	town := menu(t, g)
	if town.Title != "The Town Square" {
		t.Fatalf("title = %q", town.Title)
	}
	if got, want := strings.Join(keys(town), ""), "FHKTISONWAM"; got != want {
		t.Fatalf("town keys = %s, want %s", got, want)
	}
	for _, key := range "FHKTISON" {
		if c := find(t, town, string(key)); c.Signal == "" || !c.Enabled {
			t.Fatalf("%s should be an enabled signal: %+v", string(key), c)
		}
	}
	if c := find(t, town, "W"); c.Action != "buyWeapon" || len(c.Params) != 1 || len(c.Params[0].Options) != 13 {
		t.Fatalf("weapons = %+v", c)
	}
	if c := find(t, town, "M"); c.Action != "" || c.Signal != "" || len(c.Params[0].Options) != 9 {
		t.Fatalf("favoured move = %+v", c)
	}
	play(t, g, "F", nil)
	forest := menu(t, g)
	if c := find(t, forest, "D"); c.Signal != "SeekTheDragon" || c.Enabled {
		t.Fatalf("a level-1 warrior may seek the dragon: %+v", c)
	}
	if c := find(t, forest, "A"); c.Action != "askTheFairies" || c.Signal != "" || !c.Enabled || len(c.Params[0].Options) != 4 {
		t.Fatalf("the fairies' blessing should stand in for the signal with its options: %+v", c)
	}
}

func TestMenuFollowsTheGuards(t *testing.T) {
	g := newGame(t, 1, Character{})
	play(t, g, "K", nil)
	if c := find(t, menu(t, g), "B"); c.Enabled {
		t.Fatalf("an untrained warrior may rob the bank: %+v", c)
	}
	play(t, g, "R", nil)
	play(t, g, "I", nil)
	inn := menu(t, g)
	if c := find(t, inn, "G"); c.Action != "tradeGems" || c.Enabled {
		t.Fatalf("gems traded with none held: %+v", c)
	}
	if c := find(t, inn, "F"); c.Action != "flirt" || !c.Enabled {
		t.Fatalf("flirting = %+v", c)
	}
	if _, err := g.Play("G", map[string]string{"stat": "Stat::defense"}); !errors.Is(err, ErrRefused) {
		t.Fatalf("playing a disabled choice: err = %v, want ErrRefused", err)
	}
	if c := find(t, inn, "S"); c.Signal != "BuyARoom" || !c.Enabled {
		t.Fatalf("a warrior with 500 gold gets no room: %+v", c)
	}
	o := play(t, g, "S", nil)
	if o.To != "asleep" || !o.After.InnRoom || o.After.Gold >= 500 {
		t.Fatalf("the room: %s %+v", o.To, *o.After)
	}
	asleep := menu(t, g)
	if asleep.Title != "Asleep at the Inn" || strings.Join(keys(asleep), "") != "N" {
		t.Fatalf("asleep = %s %v", asleep.Title, keys(asleep))
	}
	if c := find(t, menu(t, g), "N"); c.Signal != "NewDay" || !c.Enabled {
		t.Fatalf("midnight = %+v", c)
	}
	if o := play(t, g, "N", nil); o.To != "townSquare" || o.After.InnRoom {
		t.Fatalf("midnight left the sleeper in %s: %+v", o.To, *o.After)
	}
}

func TestFlirtOptionsSuitTheWarrior(t *testing.T) {
	for _, tc := range []struct {
		female bool
		want   string
	}{{false, "town.inn.violet."}, {true, "town.inn.sethAble."}} {
		g := newGame(t, 1, Character{Female: tc.female})
		play(t, g, "I", nil)
		flirt := find(t, menu(t, g), "F")
		if len(flirt.Params[0].Options) != 7 {
			t.Fatalf("female=%v: %d favours", tc.female, len(flirt.Params[0].Options))
		}
		for _, o := range flirt.Params[0].Options {
			if !strings.HasPrefix(o.Value, tc.want) || o.Detail == "" {
				t.Fatalf("female=%v: favour %+v", tc.female, o)
			}
		}
	}
}

func TestPlayBindsTheOptionsAndNumbers(t *testing.T) {
	g := newGame(t, 1, Character{})
	o := play(t, g, "W", map[string]string{"weapon": "town.weapons.stick"})
	if o.After.WeaponTier != 1 || o.After.Gold != 300 {
		t.Fatalf("after the stick: %+v", *o.After)
	}
	if lines := strings.Join(o.Narrate(g), "\n"); !strings.Contains(lines, "You spend 200 gold pieces.") || !strings.Contains(lines, "new weapon") {
		t.Fatalf("narration = %q", lines)
	}
	o = play(t, g, "W", map[string]string{"weapon": "town.weapons.nirasTeeth"})
	if o.After.WeaponTier != 1 || o.After.Gold != 300 || !o.Refused() {
		t.Fatalf("a weapon beyond the purse was sold: %+v", *o.After)
	}
	o = play(t, g, "M", map[string]string{"favouredMove": "Move::deathKnight"})
	if o.After.FavouredMove != "deathKnight" {
		t.Fatalf("favoured move = %s", o.After.FavouredMove)
	}
	play(t, g, "K", nil)
	o = play(t, g, "D", map[string]string{"amount": " 250 "})
	if o.After.Gold != 50 || o.After.BankGold != 250 {
		t.Fatalf("after depositing: %+v", *o.After)
	}
	o = play(t, g, "W", map[string]string{"amount": "100"})
	if o.After.Gold != 150 || o.After.BankGold != 150 || o.Refused() {
		t.Fatalf("after withdrawing: %+v", *o.After)
	}
	o = play(t, g, "W", map[string]string{"amount": "9999"})
	if o.After.Gold != 150 || o.After.BankGold != 150 || !o.Refused() {
		t.Fatalf("overdrawing the bank: %+v refused=%v", *o.After, o.Refused())
	}
}

func TestTheTrainingHallNamesTheMaster(t *testing.T) {
	g := newGame(t, 1, Character{})
	o := play(t, g, "T", nil)
	want := "Halder, master of the Short Sword, teaches level 2 to warriors of 100 experience. You have 0; come back stronger."
	if lines := o.Narrate(g); o.To != "trainingHall" || o.After.Level != 1 || len(lines) != 1 || lines[0] != want {
		t.Fatalf("training at level 1 with no experience: %s %q", o.To, lines)
	}
	play(t, g, "R", nil)
	if err := g.SetPreference("experience", IntValue(100)); err != nil {
		t.Fatal(err)
	}
	o = play(t, g, "T", nil)
	lines := strings.Join(o.Narrate(g), "\n")
	switch {
	case !o.After.TrainedToday:
		t.Fatalf("training at level 1 with 100 experience: no bout %q", lines)
	case o.After.Level == 2 && strings.Contains(lines, "Halder bows: you have learned all the Short Sword can teach."):
	case o.After.Level == 1 && strings.Contains(lines, "Halder bests you"):
	default:
		t.Fatalf("training at level 1 with 100 experience: level %d %q", o.After.Level, lines)
	}
}

func TestPlayRefusesWhatItCannotBind(t *testing.T) {
	g := newGame(t, 1, Character{})
	for _, tc := range []struct {
		key    string
		inputs map[string]string
		want   error
	}{
		{"Z", nil, ErrNoSuchCommand},
		{"W", nil, ErrBadArgument},
		{"W", map[string]string{"weapon": "town.weapons.excalibur"}, ErrBadArgument},
		{"W", map[string]string{"weapon": "gold"}, ErrBadArgument},
		{"M", map[string]string{"favouredMove": "Move::attack; assign gold := 1"}, ErrBadArgument},
	} {
		if _, err := g.Play(tc.key, tc.inputs); !errors.Is(err, tc.want) {
			t.Errorf("Play(%q, %v): err = %v, want %v", tc.key, tc.inputs, err, tc.want)
		}
	}
	play(t, g, "K", nil)
	if _, err := g.Play("D", map[string]string{"amount": "lots"}); !errors.Is(err, ErrBadArgument) {
		t.Fatalf("a word for an amount: err = %v, want ErrBadArgument", err)
	}
	if s := snapshot(t, g); s.Gold != 500 || s.BankGold != 0 || s.WeaponTier != 0 {
		t.Fatalf("refused plays changed the warrior: %+v", *s)
	}
}

func TestPlayReadsTheMenuFromTheModel(t *testing.T) {
	g := newGame(t, 1, Character{})
	if _, err := g.Play("l", nil); !errors.Is(err, ErrNoSuchCommand) {
		t.Fatalf("hunting from town: err = %v, want ErrNoSuchCommand", err)
	}
	play(t, g, "f", nil)
	if g.Location() != "forest" {
		t.Fatalf("a lower-case key did not enter the forest: %s", g.Location())
	}
	play(t, g, "T", nil)
	tavern := menu(t, g)
	if tavern.Title != "The Dark Cloak Tavern" || strings.Join(keys(tavern), "") != "GPR" {
		t.Fatalf("tavern = %s %v", tavern.Title, keys(tavern))
	}
	o := play(t, g, "G", map[string]string{"wager": "100"})
	if o.After.Gold == 500 {
		t.Fatalf("a wager changed nothing: %+v", *o.After)
	}
	o = play(t, g, "P", map[string]string{"profession": "CharacterClass::thievingSkills"})
	if o.After.Class != "thievingSkills" {
		t.Fatalf("class = %s", o.After.Class)
	}
}
