package lord

import (
	"crypto/rand"
	"fmt"
)

// View is what the page renders after every turn: the warrior as the model
// holds them, the screen the day machine is on, and what just happened.
type View struct {
	Character bool      `json:"character"`
	Location  string    `json:"location"`
	Warrior   *Snapshot `json:"warrior,omitempty"`
	Screen    *Screen   `json:"screen,omitempty"`
	Lines     []string  `json:"lines"`
	Refused   bool      `json:"refused,omitempty"`
}

// CharacterView is the screen shown while no warrior walks the realm.
func CharacterView(lines ...string) *View {
	return &View{Character: true, Lines: lines}
}

// View projects the game for the page, with the given lines as what just happened.
func (g *Game) View(lines ...string) (*View, error) {
	warrior, err := g.Snapshot()
	if err != nil {
		return nil, err
	}
	screen, err := g.Menu()
	if err != nil {
		return nil, err
	}
	return &View{Location: g.Location(), Warrior: warrior, Screen: screen, Lines: lines}, nil
}

// Welcome is the first view of a new warrior.
func (g *Game) Welcome() (*View, error) {
	warrior, err := g.Snapshot()
	if err != nil {
		return nil, err
	}
	return g.View(fmt.Sprintf("Welcome to the realm, %s. It is a fine day to slay the dragon.", warrior.Name))
}

// Played is the view after a turn: the outcome narrated, or when there is
// nothing to tell, where the warrior went, that the model refused, or that
// nothing came of it.
func (g *Game) Played(outcome *Outcome) (*View, error) {
	view, err := g.View(outcome.Narrate(g)...)
	if err != nil {
		return nil, err
	}
	if len(view.Lines) == 0 {
		switch {
		case outcome.Moved():
			view.Lines = []string{fmt.Sprintf("You make your way to %s.", view.Screen.Title)}
		case outcome.Refused():
			view.Refused = true
			view.Lines = []string{"You cannot do that as things stand."}
		default:
			view.Lines = []string{"Nothing comes of it."}
		}
	}
	return view, nil
}

// RandomSeed draws the dice a new game is played with.
func RandomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // the platform's random source failed; nothing sensible remains
	}
	var seed uint64
	for _, x := range b {
		seed = seed<<8 | uint64(x)
	}
	return seed
}
