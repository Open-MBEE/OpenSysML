// Package lord plays the Legend of the Red Dragon model of examples/lord-demo:
// a Game runs the model in a persistent session of the public API, the menu's
// keys become the model's signals and actions, and the screen is a projection
// of the warrior's feature values. No rule of the game lives here.
package lord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// The model's names the client is written against.
const (
	heroFQN     = "LordPlay::hero"
	warriorFQN  = "LordPlay::Warrior"
	playPackage = "LordPlay"
	lordPackage = "Lord"
)

var (
	// ErrModelInvalid is a model with analysis errors, which cannot be played.
	ErrModelInvalid = errors.New("the model has errors")
	// ErrNoSuchCommand is a signal or action the model does not declare.
	ErrNoSuchCommand = errors.New("no such command")
	// ErrNotHere is a signal no transition out of the current state accepts.
	ErrNotHere = errors.New("that is not a choice here")
	// ErrRefused is a signal the current state accepts whose guard does not hold.
	ErrRefused = errors.New("the game refuses that now")
	// ErrBadArgument is an argument the action's parameter does not take.
	ErrBadArgument = errors.New("bad argument")
)

// Character is what a player chooses before the first day: the warrior's name,
// sex and skill guild, written to the hero's declared attributes.
type Character struct {
	Name   string
	Female bool
	// Class is a literal of Lord::CharacterClass: deathKnight, mysticalSkills or thievingSkills.
	Class string
}

// Game is one warrior's run of the model: a session of its own on the public
// API, the hero instantiated in it, and the day state machine the hero exhibits.
type Game struct {
	client  opensysml.Client
	model   *opensysml.Model
	session *opensysml.Session
	hero    opensysml.InstanceID
	// seed fixes the game's dice; deeds counts the direct actions performed, so
	// each is run under dice of its own that the seed still determines.
	seed, deeds uint64
}

// NewGame parses the model source in a client of its own, opens a session on
// it, instantiates the hero and starts its day, with the schedule's dice seeded
// so two games differ. Close releases the game.
func NewGame(modelSource []byte, seed uint64, character Character) (*Game, error) {
	client, err := opensysml.New()
	if err != nil {
		return nil, err
	}
	g, err := openGame(client, modelSource, seed, character)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return g, nil
}

// openGame parses the source in client, opens the session and starts the day.
func openGame(client opensysml.Client, modelSource []byte, seed uint64, character Character) (*Game, error) {
	model, err := client.ParseSource(context.Background(), string(modelSource))
	if err != nil {
		return nil, err
	}
	for _, d := range model.Diagnostics {
		if d.Severity == opensysml.SeverityError {
			return nil, fmt.Errorf("%w: %s", ErrModelInvalid, d.Message)
		}
	}
	session, err := opensysml.OpenSession(client, model)
	if err != nil {
		return nil, err
	}
	g := &Game{client: client, model: model, session: session, seed: seed}
	if err := g.start(character); err != nil {
		_ = session.Close()
		return nil, err
	}
	return g, nil
}

// start seeds the dice, instantiates the hero and writes the player's choices.
func (g *Game) start(character Character) error {
	if err := g.declared(heroFQN); err != nil {
		return err
	}
	if err := seedDice(g.session, g.seed); err != nil {
		return err
	}
	hero, err := g.session.Instantiate(heroFQN)
	if err != nil {
		return err
	}
	g.hero = hero
	if _, err := g.session.ActiveStates(hero); err != nil {
		return fmt.Errorf("%w: %s exhibits no state machine: %v", ErrModelInvalid, heroFQN, err)
	}
	return g.create(character)
}

// Close releases the session and the client the game plays in.
func (g *Game) Close() error {
	err := g.session.Close()
	if closeErr := g.client.Close(); err == nil {
		err = closeErr
	}
	return err
}

// declared checks the model declares fqn; a missing one is ErrModelInvalid.
func (g *Game) declared(fqn string) error {
	if _, err := g.client.LookupSymbol(context.Background(), g.model, fqn); err != nil {
		if errors.Is(err, opensysml.ErrFailure) {
			return fmt.Errorf("%w: %s is not declared", ErrModelInvalid, fqn)
		}
		return err
	}
	return nil
}

// declares reports whether the model declares fqn.
func (g *Game) declares(fqn string) (bool, error) {
	_, err := g.client.LookupSymbol(context.Background(), g.model, fqn)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, opensysml.ErrFailure):
		return false, nil
	default:
		return false, err
	}
}

// seedDice makes the session's next runs draw their dice from seed.
func seedDice(session *opensysml.Session, seed uint64) error {
	return session.SetSchedule("seed:" + strconv.FormatUint(seed, 10))
}

// deedSeed is the seed the nth direct action rolls under: a seeded run starts its
// dice over, so each deed gets a stream of its own, mixed from the game's seed.
func deedSeed(seed, n uint64) uint64 {
	z := seed + (n+1)*0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// create writes the player's choices to the hero before the day begins.
func (g *Game) create(c Character) error {
	name := strings.TrimSpace(c.Name)
	if name != "" {
		if err := g.session.SetFeature(g.hero, "name", opensysml.String(name)); err != nil {
			return err
		}
	}
	if c.Female {
		sex, err := g.literal("Sex", "female")
		if err != nil {
			return err
		}
		if err := g.session.SetFeature(g.hero, "sex", sex); err != nil {
			return err
		}
	}
	if c.Class != "" {
		class, err := g.literal("CharacterClass", c.Class)
		if err != nil {
			return err
		}
		if err := g.session.SetFeature(g.hero, "class", class); err != nil {
			return err
		}
	}
	return nil
}

// Location is the state of the day machine the warrior is in.
func (g *Game) Location() string {
	active, err := g.session.ActiveStates(g.hero)
	if err != nil {
		return ""
	}
	return strings.Join(active, "|")
}

// Send offers the day machine a signal of the LordPlay package and, where the
// current state takes it and its guard holds, dispatches it and runs what follows.
func (g *Game) Send(signal string) (*Outcome, error) {
	signalID := playPackage + "::" + signal
	acceptance, err := g.session.Accepts(g.hero, signalID, nil)
	if err != nil {
		if errors.Is(err, opensysml.ErrFailure) {
			return nil, fmt.Errorf("%w: signal %s", ErrNoSuchCommand, signal)
		}
		return nil, err
	}
	if !acceptance.Taken() {
		return nil, fmt.Errorf("%w: %s in %s", ErrNotHere, signal, g.Location())
	}
	if !acceptance.Enabled() {
		return nil, fmt.Errorf("%w: %s in %s", ErrRefused, signal, g.Location())
	}
	return g.run(func() (deed, error) {
		if _, err := g.session.Send(g.hero, signalID, nil); err != nil {
			return deed{}, err
		}
		advanced, err := g.session.Advance(1)
		if err != nil {
			return deed{}, err
		}
		return deed{choices: advanced.Choices}, nil
	})
}

// Invoke performs one of the warrior's actions directly with the given
// arguments, then lets the day machine take any completion transition it enables.
// The deed is refused when the model turns the warrior away at its opening decision.
func (g *Game) Invoke(action string, args map[string]opensysml.Value) (*Outcome, error) {
	actionID := warriorFQN + "::" + action
	known, err := g.declares(actionID)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, fmt.Errorf("%w: action %s", ErrNoSuchCommand, action)
	}
	return g.run(func() (deed, error) {
		if err := seedDice(g.session, deedSeed(g.seed, g.deeds)); err != nil {
			return deed{}, err
		}
		g.deeds++
		performed, err := g.session.Perform(g.hero, actionID, args)
		if err != nil {
			return deed{}, err
		}
		done := deed{choices: performed.Choices, refused: performed.TurnedAway()}
		advanced, err := g.session.Advance(1)
		if err != nil {
			return deed{}, err
		}
		done.choices = append(done.choices, advanced.Choices...)
		return done, nil
	})
}

// SetPreference writes one of the warrior's own preference attributes, such as
// the favoured move the forest fights are fought with.
func (g *Game) SetPreference(attribute string, value opensysml.Value) error {
	return g.session.SetFeature(g.hero, attribute, value)
}

// deed is what a command's run reported: the choices it made, and whether the model refused it.
type deed struct {
	choices []opensysml.ChoicePoint
	refused bool
}

// run executes a command and reports what the model made of it: the states it
// went through, the warrior before and after, whether it was refused, and the
// choices the run noted.
func (g *Game) run(command func() (deed, error)) (*Outcome, error) {
	before, err := g.Snapshot()
	if err != nil {
		return nil, err
	}
	from := g.Location()
	done, err := command()
	if err != nil {
		return nil, err
	}
	after, err := g.Snapshot()
	if err != nil {
		return nil, err
	}
	return &Outcome{From: from, To: g.Location(), Before: before, After: after, Choices: done.choices, Refused: done.refused}, nil
}

// Int reads an Integer attribute of the warrior.
func (g *Game) Int(attribute string) (int64, error) {
	v, err := g.value(attribute)
	if err != nil {
		return 0, err
	}
	return whole(attribute, v)
}

// Bool reads a Boolean attribute of the warrior.
func (g *Game) Bool(attribute string) (bool, error) {
	v, err := g.value(attribute)
	if err != nil {
		return false, err
	}
	b, ok := v.(opensysml.Bool)
	if !ok {
		return false, fmt.Errorf("%s is %s, not a boolean", attribute, spell(v))
	}
	return bool(b), nil
}

// String reads a String attribute of the warrior.
func (g *Game) String(attribute string) (string, error) {
	v, err := g.value(attribute)
	if err != nil {
		return "", err
	}
	s, ok := v.(opensysml.String)
	if !ok {
		return "", fmt.Errorf("%s is %s, not a string", attribute, spell(v))
	}
	return string(s), nil
}

// Literal reads an enumeration-typed attribute of the warrior as its literal's name.
func (g *Game) Literal(attribute string) (string, error) {
	v, err := g.value(attribute)
	if err != nil {
		return "", err
	}
	name, ok := literalName(v)
	if !ok {
		return "", fmt.Errorf("%s is %s, not an enumeration literal", attribute, spell(v))
	}
	return name, nil
}

func (g *Game) value(attribute string) (opensysml.Value, error) {
	return g.Feature(g.hero, attribute)
}

// literal evaluates `<enum>::<name>` of the Lord package to its literal value.
func (g *Game) literal(enum, name string) (opensysml.Value, error) {
	if err := g.declared(lordPackage + "::" + enum); err != nil {
		return nil, err
	}
	known, err := g.declares(lordPackage + "::" + enum + "::" + name)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, fmt.Errorf("%w: %s is no literal of %s", ErrBadArgument, name, enum)
	}
	return g.Eval(enum + "::" + name)
}

// Eval evaluates a model expression in the hero's scope: a part of the town
// (`town.weapons.dagger`), a literal (`Stat::strength`) or an attribute.
func (g *Game) Eval(expr string) (opensysml.Value, error) {
	v, err := g.session.Evaluate(expr, opensysml.WithContextSymbol(heroFQN))
	if err != nil {
		if errors.Is(err, opensysml.ErrFailure) {
			return nil, fmt.Errorf("%w: %s: %v", ErrBadArgument, expr, err)
		}
		return nil, err
	}
	return v, nil
}

// Instance reads a value that is an object of the session, such as a shop's weapon.
func (g *Game) Instance(v opensysml.Value) (opensysml.InstanceID, bool) {
	id, ok := v.(opensysml.InstanceID)
	return id, ok
}

// Feature reads an attribute of any object the session holds.
func (g *Game) Feature(inst opensysml.InstanceID, attribute string) (opensysml.Value, error) {
	fv, err := g.session.Feature(inst, attribute)
	if err != nil {
		return nil, err
	}
	if fv.Error != "" {
		return nil, fmt.Errorf("%s: %s", attribute, fv.Error)
	}
	if fv.Value == nil {
		return opensysml.Sequence(fv.Values), nil
	}
	return fv.Value, nil
}

// Members lists the named members a definition or part of the Lord package
// declares, in declaration order: the shop's weapons, the enumeration's literals.
func (g *Game) Members(fqn string) ([]string, error) {
	members, err := g.session.Members(fqn)
	if err != nil {
		if errors.Is(err, opensysml.ErrFailure) {
			return nil, fmt.Errorf("%w: %s is not declared", ErrModelInvalid, fqn)
		}
		return nil, err
	}
	var names []string
	for _, member := range members {
		if member.Kind != attributeUsageKind {
			names = append(names, member.Name)
		}
	}
	return names, nil
}

// attributeUsageKind is the Member.Kind of an attribute usage, which a
// definition's members list leaves out: the shop's weapons, not its prices.
const attributeUsageKind = "attributeUsage"

// IntValue is an Integer argument for an action's parameter.
func IntValue(n int64) opensysml.Value {
	return opensysml.Int(n)
}

// whole reads an Integer attribute's value, a Real that is whole included.
func whole(attribute string, v opensysml.Value) (int64, error) {
	switch n := v.(type) {
	case opensysml.Int:
		return int64(n), nil
	case opensysml.Real:
		if float64(int64(n)) == float64(n) {
			return int64(n), nil
		}
	}
	return 0, fmt.Errorf("%s is %s, not an integer", attribute, spell(v))
}

// literalName is the simple name of an enumeration literal value.
func literalName(v opensysml.Value) (string, bool) {
	lit, ok := v.(opensysml.EnumLiteral)
	if !ok {
		return "", false
	}
	if i := strings.LastIndex(lit.LiteralID, "::"); i >= 0 {
		return lit.LiteralID[i+2:], true
	}
	return lit.LiteralID, true
}

// spell writes a value as the screen shows it: a scalar by itself, a string bare.
func spell(v opensysml.Value) string {
	switch v := v.(type) {
	case nil:
		return "nothing"
	case opensysml.String:
		return string(v)
	case opensysml.EnumLiteral:
		return v.Name
	}
	return fmt.Sprint(v)
}
