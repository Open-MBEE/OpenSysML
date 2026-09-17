// Package lord plays the Legend of the Red Dragon model of examples/lord-demo:
// a Game runs the model in a runtime of its own, the menu's keys become the
// model's signals and actions, and the screen is a projection of the warrior's
// feature values. No rule of the game lives here.
package lord

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The model's names the client is written against.
const (
	modelDocName = "lord.sysml"
	heroFQN      = "LordPlay::hero"
	warriorFQN   = "LordPlay::Warrior"
	playPackage  = "LordPlay"
	lordPackage  = "Lord"
	maxSteps     = 1_000_000
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

// Game is one warrior's run of the model: a runtime of its own, the hero
// instantiated in it, and the day state machine the hero exhibits.
type Game struct {
	rt    *model.Runtime
	ctx   *runtime.Context
	hero  *runtime.Instance
	day   *runtime.StateExecutor
	scope *symbols.Scope
	// seed fixes the game's dice; deeds counts the direct actions performed, so
	// each is run under dice of its own that the seed still determines.
	seed, deeds uint64
}

// NewGame loads the model source into a fresh workspace, instantiates the hero
// and starts its day, with the schedule's dice seeded so two games differ.
func NewGame(modelSource []byte, seed uint64, character Character) (*Game, error) {
	ws := model.NewWorkspace()
	ws.Open(modelDocName, modelSource, 1)
	for _, d := range ws.Diagnostics(modelDocName) {
		if d.Blocking() {
			return nil, fmt.Errorf("%w: %s", ErrModelInvalid, d.Message)
		}
	}
	rt, err := ws.NewRuntime()
	if err != nil {
		return nil, err
	}
	heroSym := rt.Declared(modelDocName, heroFQN)
	if heroSym == nil {
		return nil, fmt.Errorf("%w: %s is not declared", ErrModelInvalid, heroFQN)
	}
	ctx := runtime.NewContext(rt.Model(), maxSteps)
	if err := seedDice(ctx, seed); err != nil {
		return nil, err
	}
	hero, err := ctx.Instantiate(heroSym)
	if err != nil {
		return nil, err
	}
	exhibited, ok := hero.ExhibitedState()
	if !ok || exhibited.State == nil {
		return nil, fmt.Errorf("%w: %s exhibits no state machine", ErrModelInvalid, heroFQN)
	}
	g := &Game{rt: rt, ctx: ctx, hero: hero, day: exhibited.State, scope: runtime.DeclScope(heroSym), seed: seed}
	if err := g.create(character); err != nil {
		return nil, err
	}
	return g, nil
}

// seedDice makes the context's next runs draw their dice from seed.
func seedDice(ctx *runtime.Context, seed uint64) error {
	policy, err := runtime.ParseSchedulePolicy("seed:" + strconv.FormatUint(seed, 10))
	if err != nil {
		return err
	}
	return ctx.SetSchedule(policy)
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
		if err := g.hero.SetFeatureValue(g.ctx, "name", runtime.NewStringValue(name)); err != nil {
			return err
		}
	}
	if c.Female {
		sex, err := g.literal("Sex", "female")
		if err != nil {
			return err
		}
		if err := g.hero.SetFeatureValue(g.ctx, "sex", sex); err != nil {
			return err
		}
	}
	if c.Class != "" {
		class, err := g.literal("CharacterClass", c.Class)
		if err != nil {
			return err
		}
		if err := g.hero.SetFeatureValue(g.ctx, "class", class); err != nil {
			return err
		}
	}
	return nil
}

// Location is the state of the day machine the warrior is in.
func (g *Game) Location() string {
	active := g.day.ActiveStates()
	if len(active) == 0 {
		return ""
	}
	names := make([]string, 0, len(active))
	for _, state := range active {
		names = append(names, state.Name)
	}
	return strings.Join(names, "|")
}

// Send offers the day machine a signal of the LordPlay package and, where the
// current state takes it and its guard holds, dispatches it and runs what follows.
func (g *Game) Send(signal string) (*Outcome, error) {
	sym := g.rt.Declared(modelDocName, playPackage+"::"+signal)
	if sym == nil || !runtime.IsSignalDefinition(sym) {
		return nil, fmt.Errorf("%w: signal %s", ErrNoSuchCommand, signal)
	}
	msg, err := g.ctx.SignalMessage(sym, nil, g.hero)
	if err != nil {
		return nil, err
	}
	accepted, err := g.day.AcceptsMessage(msg)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, fmt.Errorf("%w: %s in %s", ErrNotHere, signal, g.Location())
	}
	decision, err := g.day.Decide(msg)
	if err != nil {
		return nil, err
	}
	if !decision.Enabled() {
		return nil, fmt.Errorf("%w: %s in %s", ErrRefused, signal, g.Location())
	}
	return g.run(func() (deed, error) {
		g.ctx.PostMessage(msg)
		report, err := g.ctx.Advance(1)
		return deed{notes: report.Notes}, err
	})
}

// Invoke performs one of the warrior's actions directly with the given
// arguments, then lets the day machine take any completion transition it enables.
// The deed is refused when the model turns the warrior away at its opening decision.
func (g *Game) Invoke(action string, args map[string]runtime.Value) (*Outcome, error) {
	sym := g.rt.Declared(modelDocName, warriorFQN+"::"+action)
	if sym == nil {
		return nil, fmt.Errorf("%w: action %s", ErrNoSuchCommand, action)
	}
	return g.run(func() (deed, error) {
		if err := seedDice(g.ctx, deedSeed(g.seed, g.deeds)); err != nil {
			return deed{}, err
		}
		g.deeds++
		exec, err := g.ctx.CreateActionExecutorWithInputs(sym, g.hero, args)
		if err != nil {
			return deed{}, err
		}
		defer exec.Release()
		exec.KeepTraversals(true)
		if err := exec.RunToCompletion(); err != nil {
			return deed{}, err
		}
		done := deed{notes: exec.Notes(), refused: turnedAway(exec)}
		report, err := g.ctx.Advance(1)
		done.notes = append(done.notes, report.Notes...)
		return done, err
	})
}

// turnedAway reports whether the deed left its opening decision, the one its
// start leads to, by the else branch: how every deed of the model refuses.
func turnedAway(exec *runtime.ActionExecutor) bool {
	graph := exec.Graph()
	var opening ast.Node
	for _, node := range graph.Nodes {
		if _, ok := node.(*ast.InitialNode); !ok {
			continue
		}
		for _, edge := range graph.Edges[node] {
			if _, ok := edge.Target.(*ast.DecisionNode); ok {
				opening = edge.Target
			}
		}
	}
	if opening == nil {
		return false
	}
	for _, t := range exec.Traversals() {
		if len(t.Within) > 0 || t.Edge.Source != opening {
			continue
		}
		if branch, ok := t.Edge.Decl.(*ast.ControlFlowEdge); ok && branch.IsElse {
			return true
		}
	}
	return false
}

// SetPreference writes one of the warrior's own preference attributes, such as
// the favoured move the forest fights are fought with.
func (g *Game) SetPreference(attribute string, value runtime.Value) error {
	return g.hero.SetFeatureValue(g.ctx, attribute, value)
}

// deed is what a command's run reported: what it noted, and whether the model refused it.
type deed struct {
	notes   []runtime.RunNote
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
	var choices []runtime.ChoicePoint
	for _, note := range done.notes {
		if c, ok := note.(runtime.ChoicePoint); ok {
			choices = append(choices, c)
		}
	}
	return &Outcome{From: from, To: g.Location(), Before: before, After: after, Choices: choices, Refused: done.refused}, nil
}

// Int reads an Integer attribute of the warrior.
func (g *Game) Int(attribute string) (int64, error) {
	v, err := g.value(attribute)
	if err != nil {
		return 0, err
	}
	n, ok := v.Const.WholeNumber()
	if v.Kind != runtime.ValConst || !ok {
		return 0, fmt.Errorf("%s is %s, not an integer", attribute, runtime.FormatValue(v))
	}
	return n, nil
}

// Bool reads a Boolean attribute of the warrior.
func (g *Game) Bool(attribute string) (bool, error) {
	v, err := g.value(attribute)
	if err != nil {
		return false, err
	}
	if v.Kind != runtime.ValConst || v.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%s is %s, not a boolean", attribute, runtime.FormatValue(v))
	}
	return v.Const.Bool, nil
}

// String reads a String attribute of the warrior.
func (g *Game) String(attribute string) (string, error) {
	v, err := g.value(attribute)
	if err != nil {
		return "", err
	}
	if v.Kind != runtime.ValString {
		return "", fmt.Errorf("%s is %s, not a string", attribute, runtime.FormatValue(v))
	}
	return v.Str(), nil
}

// Literal reads an enumeration-typed attribute of the warrior as its literal's name.
func (g *Game) Literal(attribute string) (string, error) {
	v, err := g.value(attribute)
	if err != nil {
		return "", err
	}
	lit := v.EnumerationLiteral()
	if lit == nil {
		return "", fmt.Errorf("%s is %s, not an enumeration literal", attribute, runtime.FormatValue(v))
	}
	return lit.Name, nil
}

func (g *Game) value(attribute string) (runtime.Value, error) {
	fv, err := g.hero.GetFeatureValue(g.ctx, attribute)
	if err != nil {
		return runtime.Value{}, err
	}
	return fv.Value, nil
}

// literal evaluates `<enum>::<name>` of the Lord package to its literal value.
func (g *Game) literal(enum, name string) (runtime.Value, error) {
	def := g.rt.Declared(modelDocName, lordPackage+"::"+enum)
	if def == nil || def.Scope == nil {
		return runtime.Value{}, fmt.Errorf("%w: %s::%s is not declared", ErrModelInvalid, lordPackage, enum)
	}
	if _, ok := def.Scope.LookupLocal(name); !ok {
		return runtime.Value{}, fmt.Errorf("%w: %s is no literal of %s", ErrBadArgument, name, enum)
	}
	return g.Eval(enum + "::" + name)
}

// Eval evaluates a model expression in the hero's scope: a part of the town
// (`town.weapons.dagger`), a literal (`Stat::strength`) or an attribute.
func (g *Game) Eval(expr string) (runtime.Value, error) {
	node, err := parseExpression(expr)
	if err != nil {
		return runtime.Value{}, err
	}
	return g.ctx.EvalWithScope(node, g.scope)
}

// Instance dereferences a value that is an object, such as a shop's weapon.
func (g *Game) Instance(v runtime.Value) (*runtime.Instance, bool) {
	id, ok := v.Object()
	if !ok {
		return nil, false
	}
	return g.ctx.Instance(id)
}

// Feature reads an attribute of any instance, formatted as the REPL prints it.
func (g *Game) Feature(inst *runtime.Instance, attribute string) (runtime.Value, error) {
	fv, err := inst.GetFeatureValue(g.ctx, attribute)
	if err != nil {
		return runtime.Value{}, err
	}
	return fv.Value, nil
}

// Members lists the named members a definition or part of the Lord package
// declares, in declaration order: the shop's weapons, the enumeration's literals.
func (g *Game) Members(fqn string) ([]string, error) {
	sym := g.rt.Declared(modelDocName, fqn)
	if sym == nil || sym.Scope == nil {
		return nil, fmt.Errorf("%w: %s is not declared", ErrModelInvalid, fqn)
	}
	var names []string
	for _, member := range sym.Scope.Members() {
		if member.Name != "" && member.Kind != symbols.SymbolAttributeUsage {
			names = append(names, member.Name)
		}
	}
	return names, nil
}

// IntValue is an Integer argument for an action's parameter.
func IntValue(n int64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

// parseExpression parses one expression as the REPL does: as the value of a throwaway attribute.
func parseExpression(expr string) (ast.Node, error) {
	const prefix = "attribute __lit__ = "
	p := parser.New(source.New("expression", []byte(prefix+expr+";")))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		return nil, fmt.Errorf("%w: %s: %s", ErrBadArgument, expr, p.Diagnostics[0].Message)
	}
	if len(root.Members) != 1 {
		return nil, fmt.Errorf("%w: %s is not one expression", ErrBadArgument, expr)
	}
	member := root.Members[0]
	if m, ok := member.(*ast.Membership); ok {
		member = m.Member
	}
	usage, ok := member.(*ast.Usage)
	if !ok || usage.Value == nil {
		return nil, fmt.Errorf("%w: %s is not an expression", ErrBadArgument, expr)
	}
	return usage.Value, nil
}
