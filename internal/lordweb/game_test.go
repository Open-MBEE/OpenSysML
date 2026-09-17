package lordweb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// modelSource reads the LORD model the client plays.
func modelSource(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "lord-demo", "lord.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newGame(t *testing.T, seed uint64, c Character) *Game {
	t.Helper()
	g, err := NewGame(modelSource(t), seed, c)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func snapshot(t *testing.T, g *Game) *Snapshot {
	t.Helper()
	s, err := g.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewGameStartsADayInTheTownSquare(t *testing.T) {
	g := newGame(t, 1, Character{})
	if got := g.Location(); got != "townSquare" {
		t.Fatalf("location = %q, want townSquare", got)
	}
	s := snapshot(t, g)
	want := Snapshot{
		Name: "Sir Devin", Sex: "male", Class: "deathKnight", Level: 1, HitPoints: 10, MaxHitPoints: 10,
		Strength: 10, Defense: 1, Gold: 500, Charm: 1, FavouredMove: "attack", ForestFightsLeft: 15,
		PlayerFightsLeft: 3, Alive: true, Spouse: "nobody",
	}
	if *s != want {
		t.Fatalf("snapshot = %+v, want %+v", *s, want)
	}
}

func TestNewGameWritesTheCharacterToTheHero(t *testing.T) {
	g := newGame(t, 1, Character{Name: "  Lady Jane ", Female: true, Class: "mysticalSkills"})
	s := snapshot(t, g)
	if s.Name != "Lady Jane" || s.Sex != "female" || s.Class != "mysticalSkills" {
		t.Fatalf("character = %s/%s/%s", s.Name, s.Sex, s.Class)
	}
	if _, err := NewGame(modelSource(t), 1, Character{Class: "wizard"}); err == nil {
		t.Fatal("a class the model does not declare was accepted")
	}
}

func TestNewGameRejectsAModelThatDoesNotPlay(t *testing.T) {
	if _, err := NewGame([]byte("package Nothing { part x; }"), 1, Character{}); !errors.Is(err, ErrModelInvalid) {
		t.Fatalf("err = %v, want ErrModelInvalid", err)
	}
	if _, err := NewGame([]byte("package {{{"), 1, Character{}); !errors.Is(err, ErrModelInvalid) {
		t.Fatalf("err = %v, want ErrModelInvalid", err)
	}
}

func TestSendDrivesTheDayMachine(t *testing.T) {
	g := newGame(t, 42, Character{})
	o, err := g.Send("EnterForest")
	if err != nil {
		t.Fatal(err)
	}
	if o.From != "townSquare" || o.To != "forest" || !o.Moved() {
		t.Fatalf("EnterForest went %s -> %s", o.From, o.To)
	}
	o, err = g.Send("LookForSomethingToKill")
	if err != nil {
		t.Fatal(err)
	}
	if o.After.ForestFightsLeft != 14 {
		t.Fatalf("forest fights left = %d, want 14", o.After.ForestFightsLeft)
	}
	if len(o.Choices) == 0 {
		t.Fatal("a fight rolled no dice")
	}
	lines := o.Narrate(g)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "You have encountered ") {
		t.Fatalf("narration = %q, want the foe named first", lines)
	}
	if o.After.Alive && (o.After.Gold <= 500 || o.After.Experience <= 0) {
		t.Fatalf("a won fight left the warrior at %d gold, %d experience", o.After.Gold, o.After.Experience)
	}
	if !o.After.Alive && (o.To != "slain" || o.After.Gold != 0) {
		t.Fatalf("a lost fight left the warrior in %s with %d gold", o.To, o.After.Gold)
	}
}

func TestSendRefusesWhatTheStateDoesNot(t *testing.T) {
	g := newGame(t, 1, Character{})
	if _, err := g.Send("LookForSomethingToKill"); !errors.Is(err, ErrNotHere) {
		t.Fatalf("hunting from town: err = %v, want ErrNotHere", err)
	}
	if _, err := g.Send("Teleport"); !errors.Is(err, ErrNoSuchCommand) {
		t.Fatalf("an undeclared signal: err = %v, want ErrNoSuchCommand", err)
	}
	if _, err := g.Send("hero"); !errors.Is(err, ErrNoSuchCommand) {
		t.Fatalf("a usage, which is no signal: err = %v, want ErrNoSuchCommand", err)
	}
	if _, err := g.Send("Warrior"); !errors.Is(err, ErrNotHere) {
		t.Fatalf("a definition no transition accepts: err = %v, want ErrNotHere", err)
	}
	if _, err := g.Send("EnterForest"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Send("SeekTheDragon"); !errors.Is(err, ErrRefused) {
		t.Fatalf("a level-1 warrior seeking the dragon: err = %v, want ErrRefused", err)
	}
	if g.Location() != "forest" {
		t.Fatalf("a refused signal moved the machine to %s", g.Location())
	}
}

func TestInvokeBindsTypedArguments(t *testing.T) {
	g := newGame(t, 1, Character{})
	weapon, err := g.Eval("town.weapons.stick")
	if err != nil {
		t.Fatal(err)
	}
	o, err := g.Invoke("buyWeapon", map[string]runtime.Value{"weapon": weapon})
	if err != nil {
		t.Fatal(err)
	}
	if o.After.Gold != 300 || o.After.Strength != 15 || o.After.WeaponTier != 1 {
		t.Fatalf("after the stick: gold %d, strength %d, tier %d", o.After.Gold, o.After.Strength, o.After.WeaponTier)
	}
	o, err = g.Invoke("deposit", map[string]runtime.Value{"amount": IntValue(120)})
	if err != nil {
		t.Fatal(err)
	}
	if o.After.Gold != 180 || o.After.BankGold != 120 {
		t.Fatalf("after depositing: gold %d, bank %d", o.After.Gold, o.After.BankGold)
	}
	o, err = g.Invoke("deposit", map[string]runtime.Value{"amount": IntValue(1000)})
	if err != nil {
		t.Fatal(err)
	}
	if o.After.Gold != 180 || o.After.BankGold != 120 {
		t.Fatalf("depositing more than is held moved gold: gold %d, bank %d", o.After.Gold, o.After.BankGold)
	}
	if _, err := g.Invoke("cheat", nil); !errors.Is(err, ErrNoSuchCommand) {
		t.Fatalf("an undeclared action: err = %v, want ErrNoSuchCommand", err)
	}
	if _, err := g.Invoke("deposit", map[string]runtime.Value{"amount": runtime.NewStringValue("lots")}); err == nil {
		t.Fatal("a String bound to an Integer parameter was accepted")
	}
}

func TestGamesAreIsolated(t *testing.T) {
	a := newGame(t, 1, Character{Name: "A"})
	b := newGame(t, 1, Character{Name: "B"})
	if _, err := a.Invoke("deposit", map[string]runtime.Value{"amount": IntValue(500)}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Send("EnterForest"); err != nil {
		t.Fatal(err)
	}
	sa, sb := snapshot(t, a), snapshot(t, b)
	if sa.Gold != 0 || sa.BankGold != 500 || a.Location() != "forest" {
		t.Fatalf("game a: %+v in %s", *sa, a.Location())
	}
	if sb.Gold != 500 || sb.BankGold != 0 || sb.Name != "B" || b.Location() != "townSquare" {
		t.Fatalf("game b saw game a's play: %+v in %s", *sb, b.Location())
	}
}

func TestSeedsDrawDifferentDice(t *testing.T) {
	outcomes := map[string]bool{}
	for seed := uint64(1); seed <= 12; seed++ {
		g := newGame(t, seed, Character{})
		if _, err := g.Send("EnterForest"); err != nil {
			t.Fatal(err)
		}
		o, err := g.Send("LookForSomethingToKill")
		if err != nil {
			t.Fatal(err)
		}
		outcomes[strings.Join(o.Narrate(g), "\n")] = true
	}
	if len(outcomes) < 2 {
		t.Fatalf("twelve seeds fought the same fight: %v", outcomes)
	}
}

func TestADayEndsAtMidnight(t *testing.T) {
	g := newGame(t, 3, Character{})
	if _, err := g.Send("EnterForest"); err != nil {
		t.Fatal(err)
	}
	for g.Location() == "forest" {
		s := snapshot(t, g)
		if s.ForestFightsLeft == 0 {
			break
		}
		if _, err := g.Send("LookForSomethingToKill"); err != nil {
			t.Fatal(err)
		}
	}
	if g.Location() == "forest" {
		if _, err := g.Send("LookForSomethingToKill"); !errors.Is(err, ErrRefused) {
			t.Fatalf("hunting with no fights left: err = %v, want ErrRefused", err)
		}
		if _, err := g.Send("ReturnToTown"); err != nil {
			t.Fatal(err)
		}
	}
	o, err := g.Send("NewDay")
	if err != nil {
		t.Fatal(err)
	}
	if o.To != "townSquare" || !o.After.Alive || o.After.ForestFightsLeft != 15 || o.After.HitPoints != o.After.MaxHitPoints {
		t.Fatalf("midnight left the warrior in %s: %+v", o.To, *o.After)
	}
	if lines := o.Narrate(g); !strings.Contains(strings.Join(lines, "\n"), "A new day dawns") {
		t.Fatalf("midnight narrated %q", lines)
	}
}
