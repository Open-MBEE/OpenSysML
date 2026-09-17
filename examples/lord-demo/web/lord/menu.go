package lord

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Choice is one line of the menu the player is shown: a key, what it does, and
// whether the day machine would take it right now.
type Choice struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Signal  string  `json:"signal,omitempty"`
	Action  string  `json:"action,omitempty"`
	Params  []Param `json:"params,omitempty"`
	Enabled bool    `json:"enabled"`
}

// Param is an input an action's parameter takes: a whole number, or one of the
// options the model declares for it (a weapon of the shop, a literal of an enumeration).
type Param struct {
	Name    string   `json:"name"`
	Prompt  string   `json:"prompt"`
	Integer bool     `json:"integer,omitempty"`
	Options []Option `json:"options,omitempty"`
}

// Option is one of a parameter's choices, named as the model names it.
type Option struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

// Screen is the menu for one state: its title, and the choices in the order shown.
type Screen struct {
	Title   string   `json:"title"`
	Choices []Choice `json:"choices"`
}

// signalMenu is how a menu signal is shown; the signals themselves come from the
// day machine's transitions, so a signal not listed here is shown by its name.
// Signals a direct action stands in for (actionMenus) need no entry.
type signalMenu struct {
	key, label string
}

var signalMenus = map[string]signalMenu{
	"EnterForest":            {"F", "Forest"},
	"LookForSomethingToKill": {"L", "Look for something to kill"},
	"SeekTheDragon":          {"D", "Seek the Red Dragon"},
	"MeetYourGuild":          {"G", "Meet your guild for a lesson"},
	"MeetTheOldHag":          {"H", "Meet the Old Hag (a gem for healing)"},
	"CatchAFairy":            {"C", "Catch a fairy"},
	"FindTheDarkCloakTavern": {"T", "Find the Dark Cloak Tavern"},
	"ReturnToTheForest":      {"R", "Return to the forest"},
	"ReturnToTown":           {"R", "Return to town"},
	"VisitTheHealer":         {"H", "Healer's Hut"},
	"VisitTheBank":           {"K", "King Arthur's Bank"},
	"RobTheBank":             {"B", "Rob the bank"},
	"VisitTheTrainingHall":   {"T", "Turgon's Warrior Training"},
	"VisitTheInn":            {"I", "The Inn"},
	"ListenToTheBard":        {"B", "Listen to Seth Able the bard"},
	"BribeTheBartender":      {"P", "Pay the bartender to keep quiet"},
	"BuyARoom":               {"S", "Sleep in a room for the night"},
	"SlaughterOtherPlayers":  {"S", "Slaughter other players"},
	"AttackAWarrior":         {"A", "Attack a warrior"},
	"OtherPlaces":            {"O", "Other places"},
	"NewDay":                 {"N", "Wait for midnight (a new day)"},
}

// stateTitles name the screens as the game did; a state not listed is shown by its name.
var stateTitles = map[string]string{
	"townSquare":      "The Town Square",
	"forest":          "The Forest",
	"darkCloakTavern": "The Dark Cloak Tavern",
	"healersHut":      "The Healer's Hut",
	"bank":            "King Arthur's Bank",
	"trainingHall":    "Turgon's Warrior Training",
	"inn":             "The Red Dragon Inn",
	"asleep":          "Asleep at the Inn",
	"slaughter":       "Slaughter Other Players",
	"otherPlaces":     "Other Places",
	"slain":           "The Grave",
}

// actionMenu is a direct action the menu offers in a state with the inputs it
// asks for. One that names a signal stands in for it: the machine's transition
// performs the action with its defaults, so the menu shows the action with its
// inputs instead, under the transition's own guard.
type actionMenu struct {
	state, key, label, action, signal string
	params                            []paramSpec
}

// paramSpec says where a parameter's options come from: the parts of a definition
// reached from the hero by a path (the shop's weapons), or an enumeration's literals.
type paramSpec struct {
	name, prompt string
	integer      bool
	sources      []optionSource
	// detail lists the attributes shown beside each option.
	detail []string
	// suitor keeps only the options whose attribute of that name is the hero's sex.
	suitor string
}

// optionSource is a Lord definition whose members are options, with the path of
// the part they belong to (`town.weapons`), or "" for an enumeration's literals.
type optionSource struct {
	definition, path string
}

func enumeration(name string) []optionSource { return []optionSource{{definition: name}} }

var moveParam = paramSpec{name: "favouredMove", prompt: "Fight with which move?", sources: enumeration("Move")}

var actionMenus = []actionMenu{
	{state: "townSquare", key: "W", label: "King Arthur's Weapons", action: "buyWeapon", params: []paramSpec{
		{name: "weapon", prompt: "Which weapon?", sources: []optionSource{{"WeaponShop", "town.weapons"}}, detail: []string{"strength", "price"}}}},
	{state: "townSquare", key: "A", label: "Abdul's Armour", action: "buyArmour", params: []paramSpec{
		{name: "armour", prompt: "Which armour?", sources: []optionSource{{"ArmourShop", "town.armour"}}, detail: []string{"defense", "price"}}}},
	{state: "townSquare", key: "M", label: "Choose your favoured move", params: []paramSpec{moveParam}},
	{state: "forest", key: "M", label: "Choose your favoured move", params: []paramSpec{moveParam}},
	{state: "slaughter", key: "M", label: "Choose your favoured move", params: []paramSpec{moveParam}},
	{state: "forest", key: "A", label: "Ask the fairies for a blessing", action: "askTheFairies", signal: "AskTheFairies", params: []paramSpec{
		{name: "blessing", prompt: "Which blessing?", sources: enumeration("Blessing")}}},
	{state: "bank", key: "D", label: "Deposit gold", action: "deposit", params: []paramSpec{{name: "amount", prompt: "How much to deposit?", integer: true}}},
	{state: "bank", key: "W", label: "Withdraw gold", action: "withdraw", params: []paramSpec{{name: "amount", prompt: "How much to withdraw?", integer: true}}},
	{state: "healersHut", key: "H", label: "Heal your wounds", action: "heal"},
	{state: "darkCloakTavern", key: "G", label: "Gamble with the old man", action: "gamble", signal: "Gamble", params: []paramSpec{
		{name: "wager", prompt: "How much to wager?", integer: true}}},
	{state: "darkCloakTavern", key: "P", label: "Let Chance change your profession", action: "changeProfession", signal: "ChangeProfession", params: []paramSpec{
		{name: "profession", prompt: "Which skills?", sources: enumeration("CharacterClass")}}},
	{state: "inn", key: "F", label: "Flirt", action: "flirt", signal: "FlirtAtTheInn", params: []paramSpec{
		{name: "favour", prompt: "How?", sources: []optionSource{{"Barmaid", "town.inn.violet"}, {"Bard", "town.inn.sethAble"}}, detail: []string{"charmNeeded"}, suitor: "suitor"}}},
	{state: "inn", key: "G", label: "Trade gems for a stat", action: "tradeGems", signal: "TradeGems", params: []paramSpec{
		{name: "stat", prompt: "Which stat?", sources: enumeration("Stat")}}},
	{state: "inn", key: "D", label: "Divorce", action: "divorce"},
}

// standIn is the direct action shown in place of a signal in a state, if any.
func standIn(state, signal string) (actionMenu, bool) {
	for _, menu := range actionMenus {
		if menu.state == state && menu.signal == signal {
			return menu, true
		}
	}
	return actionMenu{}, false
}

// Menu is the screen for the state the day machine is in: the signals its
// transitions accept, each decided against the warrior as it stands, and the
// direct actions the menu offers there.
func (g *Game) Menu() (*Screen, error) {
	active := g.day.ActiveStates()
	if len(active) == 0 {
		return nil, fmt.Errorf("%w: the day machine has no active state", ErrModelInvalid)
	}
	state := active[0]
	screen := &Screen{Title: stateTitles[state.Name]}
	if screen.Title == "" {
		screen.Title = state.Name
	}
	taken := map[string]bool{}
	for _, trans := range g.day.Graph().Transitions[state] {
		accept, ok := trans.Trigger.(*ast.AcceptEvent)
		if !ok || accept.SignalType == nil || len(accept.SignalType.Parts) == 0 {
			continue
		}
		signal := accept.SignalType.Parts[len(accept.SignalType.Parts)-1].Text
		enabled, err := g.accepts(signal)
		if err != nil {
			return nil, err
		}
		menu, known := signalMenus[signal]
		if !known {
			menu = signalMenu{label: spaced(signal)}
		}
		if direct, ok := standIn(state.Name, signal); ok {
			params, err := g.params(direct.params)
			if err != nil {
				return nil, err
			}
			screen.Choices = append(screen.Choices, Choice{Key: freeKey(taken, direct.key, direct.label), Label: direct.label, Action: direct.action, Params: params, Enabled: enabled})
			continue
		}
		screen.Choices = append(screen.Choices, Choice{Key: freeKey(taken, menu.key, signal), Label: menu.label, Signal: signal, Enabled: enabled})
	}
	for _, menu := range actionMenus {
		if menu.state != state.Name || menu.signal != "" {
			continue
		}
		params, err := g.params(menu.params)
		if err != nil {
			return nil, err
		}
		screen.Choices = append(screen.Choices, Choice{Key: freeKey(taken, menu.key, menu.label), Label: menu.label, Action: menu.action, Params: params, Enabled: true})
	}
	return screen, nil
}

// accepts reports whether the day machine would take the signal now.
func (g *Game) accepts(signal string) (bool, error) {
	sym := g.rt.Declared(modelDocName, playPackage+"::"+signal)
	if sym == nil {
		return false, fmt.Errorf("%w: %s accepts a signal %s the model does not declare", ErrModelInvalid, g.Location(), signal)
	}
	msg, err := g.ctx.SignalMessage(sym, nil, g.hero)
	if err != nil {
		return false, err
	}
	accepted, err := g.day.AcceptsMessage(msg)
	if err != nil || !accepted {
		return false, err
	}
	decision, err := g.day.Decide(msg)
	if err != nil {
		return false, err
	}
	return decision.Enabled(), nil
}

// params reads a menu's parameter options from the model.
func (g *Game) params(specs []paramSpec) ([]Param, error) {
	var params []Param
	for _, spec := range specs {
		p := Param{Name: spec.name, Prompt: spec.prompt, Integer: spec.integer}
		for _, source := range spec.sources {
			options, err := g.options(spec, source)
			if err != nil {
				return nil, err
			}
			p.Options = append(p.Options, options...)
		}
		params = append(params, p)
	}
	return params, nil
}

// options lists a source's members as a parameter's options, each valued by the
// model expression that names it: the part reached by the path with its named
// attributes beside it, or the enumeration's literal.
func (g *Game) options(spec paramSpec, source optionSource) ([]Option, error) {
	names, err := g.Members(lordPackage + "::" + source.definition)
	if err != nil {
		return nil, err
	}
	sex, err := g.Literal("sex")
	if err != nil {
		return nil, err
	}
	var options []Option
	for _, name := range names {
		if source.path == "" {
			options = append(options, Option{Value: source.definition + "::" + name, Label: spaced(name)})
			continue
		}
		option := Option{Value: source.path + "." + name, Label: spaced(name)}
		v, err := g.Eval(option.Value)
		if err != nil {
			return nil, err
		}
		inst, ok := g.Instance(v)
		if !ok {
			continue
		}
		if spec.suitor != "" {
			suitor, err := g.Feature(inst, spec.suitor)
			if err != nil {
				return nil, err
			}
			if lit := suitor.EnumerationLiteral(); lit == nil || lit.Name != sex {
				continue
			}
		}
		if label, err := g.Feature(inst, "name"); err == nil && label.Kind == runtime.ValString {
			option.Label = label.Str()
		}
		var details []string
		for _, attribute := range spec.detail {
			v, err := g.Feature(inst, attribute)
			if err != nil {
				return nil, err
			}
			details = append(details, attribute+" "+runtime.FormatValue(v))
		}
		option.Detail = strings.Join(details, ", ")
		options = append(options, option)
	}
	return options, nil
}

// Play takes the key the player pressed on the current screen with the inputs
// they gave, binds them as the model's values and runs the choice.
func (g *Game) Play(key string, inputs map[string]string) (*Outcome, error) {
	screen, err := g.Menu()
	if err != nil {
		return nil, err
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	for _, choice := range screen.Choices {
		if choice.Key != key {
			continue
		}
		if !choice.Enabled {
			return nil, fmt.Errorf("%w: %s", ErrRefused, choice.Label)
		}
		if choice.Signal != "" {
			return g.Send(choice.Signal)
		}
		return g.perform(choice, inputs)
	}
	return nil, fmt.Errorf("%w: %q is not on this menu", ErrNoSuchCommand, key)
}

// perform binds a direct action's inputs and invokes it, or writes a preference.
func (g *Game) perform(choice Choice, inputs map[string]string) (*Outcome, error) {
	args := map[string]runtime.Value{}
	for _, param := range choice.Params {
		text, given := inputs[param.Name]
		if !given {
			return nil, fmt.Errorf("%w: %s needs %s", ErrBadArgument, choice.Label, param.Name)
		}
		v, err := g.bind(param, strings.TrimSpace(text))
		if err != nil {
			return nil, err
		}
		args[param.Name] = v
	}
	if choice.Action != "" {
		return g.Invoke(choice.Action, args)
	}
	return g.run(func() (deed, error) {
		for name, v := range args {
			if err := g.SetPreference(name, v); err != nil {
				return deed{}, err
			}
		}
		return deed{}, nil
	})
}

// bind turns the player's text into the model value the parameter takes: a
// whole number, or the value of the option's expression.
func (g *Game) bind(param Param, text string) (runtime.Value, error) {
	if param.Integer {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("%w: %s must be a whole number, not %q", ErrBadArgument, param.Name, text)
		}
		return IntValue(n), nil
	}
	for _, option := range param.Options {
		if option.Value == text {
			return g.Eval(option.Value)
		}
	}
	return runtime.Value{}, fmt.Errorf("%w: %q is not a choice for %s", ErrBadArgument, text, param.Name)
}

// freeKey gives a choice its preferred key, or the first letter of its name not
// yet taken on the screen, or a digit when every letter is.
func freeKey(taken map[string]bool, preferred, name string) string {
	candidates := []string{preferred}
	for _, r := range name {
		if unicode.IsLetter(r) {
			candidates = append(candidates, strings.ToUpper(string(r)))
		}
	}
	for _, key := range candidates {
		if key != "" && !taken[key] {
			taken[key] = true
			return key
		}
	}
	for d := 1; d <= 9; d++ {
		if key := strconv.Itoa(d); !taken[key] {
			taken[key] = true
			return key
		}
	}
	return "?"
}

// spaced writes a camel-case model name as words: largeGreenRat → Large Green Rat.
func spaced(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i == 0 {
			b.WriteRune(unicode.ToUpper(r))
			continue
		}
		if unicode.IsUpper(r) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
