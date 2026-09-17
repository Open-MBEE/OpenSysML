package lord

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// gambleTwice wagers 10 gold twice in the tavern and reports each wager's dice.
func gambleTwice(t *testing.T, seed uint64) (first, second []opensysml.ChoicePoint, gold int64) {
	t.Helper()
	g := newGame(t, seed, Character{})
	if _, err := g.Send("EnterForest"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Send("FindTheDarkCloakTavern"); err != nil {
		t.Fatal(err)
	}
	wager := map[string]opensysml.Value{"wager": IntValue(10)}
	a, err := g.Invoke("gamble", wager)
	if err != nil {
		t.Fatal(err)
	}
	b, err := g.Invoke("gamble", wager)
	if err != nil {
		t.Fatal(err)
	}
	return a.Choices, b.Choices, b.After.Gold
}

func TestDirectActionsDrawSuccessiveDice(t *testing.T) {
	same := 0
	for seed := uint64(1); seed <= 32; seed++ {
		first, second, _ := gambleTwice(t, seed)
		if len(first) != 1 || len(second) != 1 {
			t.Fatalf("seed %d: choices %d and %d, want one each", seed, len(first), len(second))
		}
		if first[0].Taken == second[0].Taken {
			same++
		}
	}
	if same == 32 {
		t.Fatal("two identical wagers always fall the same way: the dice restart on every direct action")
	}
}

func TestDirectActionsAreReproducible(t *testing.T) {
	_, _, gold := gambleTwice(t, 7)
	_, _, again := gambleTwice(t, 7)
	if gold != again {
		t.Fatalf("seed 7 left %d then %d gold", gold, again)
	}
}
