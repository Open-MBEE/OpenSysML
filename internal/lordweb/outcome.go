package lordweb

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Outcome is what one command made of the model: where the day machine went,
// the warrior before and after, and the dice the run drew on the way.
type Outcome struct {
	From, To      string
	Before, After *Snapshot
	Choices       []runtime.ChoicePoint
}

// Moved reports whether the day machine changed state.
func (o *Outcome) Moved() bool { return o.From != o.To }

// Refused reports that the model turned the deed down: every deed opens with a
// decision whose else branch is done, so a run that stayed put, drew no dice and
// changed nothing took that branch.
func (o *Outcome) Refused() bool {
	return !o.Moved() && len(o.Choices) == 0 && *o.Before == *o.After
}

// Narrate tells the outcome as the game's screens did: the foe the forest
// served, the blows, and every change to the warrior's standing.
func (o *Outcome) Narrate(g *Game) []string {
	var lines []string
	if foe := o.foeName(g); foe != "" {
		lines = append(lines, fmt.Sprintf("You have encountered %s!", foe))
	}
	if hits, misses := o.blows(); hits+misses > 0 {
		lines = append(lines, blowsLine(hits, misses))
	}
	lines = append(lines, o.changes()...)
	if master := o.masterLine(g); master != "" {
		lines = append(lines, master)
	}
	return lines
}

// masterLine tells whom the training hall put before the warrior on arrival: the
// master whose class is the next level, read from the hall's parts.
func (o *Outcome) masterLine(g *Game) string {
	if o.To != "trainingHall" || o.From == "trainingHall" {
		return ""
	}
	names, err := g.Members(lordPackage + "::TrainingHall")
	if err != nil {
		return ""
	}
	level := o.Before.Level
	for _, name := range names {
		v, err := g.Eval("town.training." + name)
		if err != nil {
			continue
		}
		inst, ok := g.Instance(v)
		if !ok {
			continue
		}
		teaches, err := wholeFeature(g, inst, "teaches")
		if err != nil || teaches != level+1 {
			continue
		}
		master, err := g.Feature(inst, "name")
		if err != nil {
			return ""
		}
		weapon, err := g.Feature(inst, "weapon")
		if err != nil {
			return ""
		}
		required, err := wholeFeature(g, inst, "experienceRequired")
		if err != nil {
			return ""
		}
		if o.After.Level > level {
			return fmt.Sprintf("%s bows: you have learned all the %s can teach.", master.Str(), weapon.Str())
		}
		if o.After.TrainedToday && !o.Before.TrainedToday {
			return fmt.Sprintf("%s bests you and sends you off to rest; try again tomorrow.", master.Str())
		}
		line := fmt.Sprintf("%s, master of the %s, teaches level %d to warriors of %d experience.", master.Str(), weapon.Str(), level+1, required)
		if o.After.Experience < required {
			line += fmt.Sprintf(" You have %d; come back stronger.", o.After.Experience)
		}
		return line
	}
	return "No master has anything left to teach you."
}

// wholeFeature reads an Integer attribute of a model part.
func wholeFeature(g *Game, inst *runtime.Instance, attribute string) (int64, error) {
	v, err := g.Feature(inst, attribute)
	if err != nil {
		return 0, err
	}
	n, ok := v.Const.WholeNumber()
	if v.Kind != runtime.ValConst || !ok {
		return 0, fmt.Errorf("%s is %s, not an integer", attribute, runtime.FormatValue(v))
	}
	return n, nil
}

// foeName names the monster the forest's roll picked, read from the level's part;
// a roll that is no monster's (the fairies', the dice) names none.
func (o *Outcome) foeName(g *Game) string {
	for _, c := range o.Choices {
		if c.Kind != runtime.ChoiceDecisionBranch || c.Where != "decision roll" || c.Taken >= len(c.Alternatives) {
			continue
		}
		_, part, ok := strings.Cut(c.Alternatives[c.Taken], "->")
		if !ok {
			continue
		}
		v, err := g.Eval(fmt.Sprintf("town.forest.level%d.%s.name", o.Before.Level, part))
		if err == nil && v.Kind == runtime.ValString {
			return v.Str()
		}
	}
	return ""
}

// blows counts the sword swings the run decided, landed and missed.
func (o *Outcome) blows() (hits, misses int) {
	for _, c := range o.Choices {
		if c.Kind != runtime.ChoiceDecisionBranch || c.Where != "decision swing" || c.Taken >= len(c.Alternatives) {
			continue
		}
		switch {
		case strings.HasSuffix(c.Alternatives[c.Taken], "->strike"):
			hits++
		case strings.HasSuffix(c.Alternatives[c.Taken], "->miss"):
			misses++
		}
	}
	return hits, misses
}

func blowsLine(hits, misses int) string {
	return fmt.Sprintf("You swing %s: %s land, %s go wide.", plural(int64(hits+misses), "time"), count(hits), count(misses))
}

// changes lists every difference between the warrior before and after.
func (o *Outcome) changes() []string {
	b, a := o.Before, o.After
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	if b.Alive && !a.Alive {
		add("You have been slain! Your gold is lost, and the day with it.")
	}
	if !b.Alive && a.Alive {
		add("You awaken, alive again, your wounds healed.")
	}
	if a.Level > b.Level {
		add("You are now level %d!", a.Level)
	} else if a.Level < b.Level {
		add("You start over at level %d.", a.Level)
	}
	if a.DragonKills > b.DragonKills {
		add("You have slain the Red Dragon! The realm rejoices.")
	}
	if a.PlayerKills > b.PlayerKills {
		add("You have killed another warrior!")
	}
	if d := a.Gold - b.Gold; d > 0 {
		add("You gain %s.", plural(d, "gold piece"))
	} else if d < 0 && a.Alive {
		add("You spend %s.", plural(-d, "gold piece"))
	}
	if d := a.BankGold - b.BankGold; d != 0 {
		add("Your bank account now holds %s.", plural(a.BankGold, "gold piece"))
	}
	if d := a.Experience - b.Experience; d > 0 {
		add("You gain %s.", plural(d, "experience point"))
	} else if d < 0 {
		add("You lose %s.", plural(-d, "experience point"))
	}
	if d := a.HitPoints - b.HitPoints; d < 0 && a.Alive {
		add("You lose %s.", plural(-d, "hit point"))
	} else if d > 0 {
		add("You are healed for %s.", plural(d, "hit point"))
	}
	if a.MaxHitPoints != b.MaxHitPoints {
		add("Your hit points now peak at %d.", a.MaxHitPoints)
	}
	if a.Strength != b.Strength {
		add("Your strength is now %d.", a.Strength)
	}
	if a.Defense != b.Defense {
		add("Your defense is now %d.", a.Defense)
	}
	if a.WeaponTier != b.WeaponTier {
		add("You wield a new weapon.")
	}
	if a.ArmourTier != b.ArmourTier {
		add("You wear new armour.")
	}
	if d := a.Gems - b.Gems; d > 0 {
		add("You find %s!", plural(d, "gem"))
	} else if d < 0 {
		add("You part with %s.", plural(-d, "gem"))
	}
	if a.Charm != b.Charm {
		add("Your charm is now %d.", a.Charm)
	}
	if a.Class != b.Class {
		add("You now follow the way of the %s.", className(a.Class))
	}
	if a.FavouredMove != b.FavouredMove {
		add("In combat you will favour %s.", spaced(a.FavouredMove))
	}
	for _, skill := range []struct {
		name   string
		before int64
		after  int64
	}{
		{"Death Knight", b.DeathKnightSkill, a.DeathKnightSkill},
		{"Mystical", b.MysticalSkill, a.MysticalSkill},
		{"Thieving", b.ThievingSkill, a.ThievingSkill},
	} {
		if skill.after > skill.before {
			add("You learn more of the %s skills: %d points.", skill.name, skill.after)
		}
	}
	if a.Spouse != b.Spouse {
		if a.Spouse == "nobody" {
			add("You are divorced.")
		} else {
			add("You are married to %s!", spouseName(a.Spouse))
		}
	}
	if a.Children > b.Children {
		add("A child is born to you!")
	} else if a.Children < b.Children {
		add("One of your children took the blow meant for you.")
	}
	if !b.Horse && a.Horse {
		add("The fairies grant you a horse.")
	}
	if !b.Fairy && a.Fairy {
		add("You catch a fairy and pocket it.")
	} else if b.Fairy && !a.Fairy {
		add("The fairy in your pocket flies free.")
	}
	if !b.InnRoom && a.InnRoom {
		add("You take a room at the inn for the night.")
	}
	if !b.Bribed && a.Bribed {
		add("The bartender pockets your gold and forgets your name.")
	}
	if a.ForestFightsLeft > b.ForestFightsLeft || a.PlayerFightsLeft > b.PlayerFightsLeft {
		add("A new day dawns. You have %s in the forest and %s against other warriors.",
			plural(a.ForestFightsLeft, "fight"), plural(a.PlayerFightsLeft, "fight"))
	}
	if a.News != b.News && a.News != "" {
		add("The town crier: %s", a.News)
	}
	return lines
}

func plural(n int64, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func count(n int) string {
	if n == 0 {
		return "none"
	}
	return fmt.Sprint(n)
}

// className spells a CharacterClass literal as the guild names itself.
func className(literal string) string {
	switch literal {
	case "deathKnight":
		return "Death Knight"
	case "mysticalSkills":
		return "Mystical Skills"
	case "thievingSkills":
		return "Thieving Skills"
	}
	return literal
}

// spouseName spells a Spouse literal as the inn knows them.
func spouseName(literal string) string {
	switch literal {
	case "violet":
		return "Violet"
	case "sethAble":
		return "Seth Able"
	case "nobody":
		return "nobody"
	}
	return literal
}
