package runtime

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A default run resolves each choice point by a fixed rule. Whether that rule
// decided anything is read off the alternatives' footprints, the same
// projection the checker's reduction commutes moves by (check_reduce.go): two
// alternatives neither of which writes what the other reads or writes, nor meet
// on the bus, are one outcome in any order, and the choice is Independent. Any
// other pair leaves the features its alternatives write standing at a value
// another order may change; those are the choice's Touches.

// classifyChoice sets c.Independent and c.Touches for a choice of the order of
// its alternatives, from their footprints, one per alternative in order; rel is
// what is known of the executors alternatives i and j belong to, nil for one
// executor's. Footprints not matching the alternatives leave the choice
// conflicting, touching nothing named.
func classifyChoice(c *ChoicePoint, fps []lower.Footprint, rel func(i, j int) lower.Relation) {
	relation := func(i, j int) lower.Relation {
		if rel == nil {
			return lower.Relation{}
		}
		return rel(i, j)
	}
	classifyBy(c, fps, func(i, j int) bool {
		return onValues(fps[i]).DependentBy(onValues(fps[j]), relation(i, j))
	}, relation)
}

// classifyBy classifies a choice among moves by a dependence of its own — a
// statement order's, which also keeps the successions written among them —
// touching by the footprints as classifyChoice does.
func classifyBy(c *ChoicePoint, fps []lower.Footprint, dependent func(i, j int) bool, rel func(i, j int) lower.Relation) {
	if len(fps) != len(c.Alternatives) || len(fps) < 2 {
		return
	}
	c.serial = nextChoiceSerial()
	found := false
	for i := range fps {
		for j := i + 1; j < len(fps); j++ {
			if !dependent(i, j) {
				continue
			}
			found = true
			c.touchMeeting(fps[i], fps[j], rel(i, j))
		}
	}
	c.Independent = !found
}

// onValues is the footprint less what the order of two moves changes nothing of
// in a run's values: meeting at a control node (a join fires once whichever
// token arrives first) or both completing a performance.
func onValues(fp lower.Footprint) lower.Footprint {
	fp.Control, fp.Completion = nil, false
	return fp
}

// touchMeeting adds the places of two dependent alternatives whose values the
// order decides: a place both write, and what one writes from a read of a place
// the other writes, which sees it before or after. When no place meets (the
// alternatives meet on the bus, on control, or one is dynamic) every write of
// both may stand at an order-dependent value.
func (c *ChoicePoint) touchMeeting(f, g lower.Footprint, rel lower.Relation) {
	met := c.touchWritesMet(f, g, rel)
	met = c.touchWritesMet(g, f, rel) || met
	if !met {
		c.touchWrites(f, g)
	}
}

// touchWritesMet touches the writes of f that meet a write of g, and the writes
// of g standing on a read of g that a write of f meets; it reports whether any did.
func (c *ChoicePoint) touchWritesMet(f, g lower.Footprint, rel lower.Relation) bool {
	met := false
	for _, p := range f.Writes {
		if rel.PlaceMeets(p, g.Writes) {
			c.touch(p)
			met = true
		}
		if rel.PlaceMeets(p, g.Reads) {
			// A pin is each performance's own, delivered to, not raced for: a read of
			// one sees what arrived, whichever order the moves ran.
			if shared(p) {
				for _, q := range g.WritesReading(func(r lower.Place) bool { return rel.PlaceMeets(p, []lower.Place{r}) }) {
					c.touch(q)
				}
			}
			met = true
		}
	}
	return met
}

// shared reports whether a place is one slot every move names, which a choice
// point records: not a performance's pin or a node's own feature, nor a state.
func shared(p lower.Place) bool {
	return !(p.Local && p.Node != nil || p.State != nil)
}

// classifyExclusive marks a choice among alternatives of which one runs and the
// others do not (a decision's branches, a state's transitions): never
// independent, since the alternatives are different continuations, and touching
// whatever any of them writes.
func classifyExclusive(c *ChoicePoint, fps []lower.Footprint) {
	c.Independent = false
	c.serial = nextChoiceSerial()
	c.touchWrites(fps...)
}

func (c *ChoicePoint) touchWrites(fps ...lower.Footprint) {
	for _, fp := range fps {
		for _, p := range fp.Writes {
			c.touch(p)
		}
	}
}

func (c *ChoicePoint) touch(p lower.Place) {
	if !shared(p) {
		return
	}
	if !slices.ContainsFunc(c.touched, func(q lower.Place) bool { return q.Name == p.Name && q.Sym == p.Sym }) {
		c.touched = append(c.touched, p)
	}
	name := p.String()
	i := sort.SearchStrings(c.Touches, name)
	if i < len(c.Touches) && c.Touches[i] == name {
		return
	}
	c.Touches = slices.Insert(c.Touches, i, name)
}

// futureFootprints is what the run's executors may still touch, made on first use.
func (ctx *Context) futureFootprints() *futureFootprints {
	if ctx.futures == nil {
		ctx.futures = newFutureFootprints(ctx)
	}
	return ctx.futures
}

// placeNamed reports whether a value reported under name is the one a write to
// the place may have left, among the performing objects selves. A bare name is a
// feature of the behavior's own, so a write to a performing object's feature of
// that name does not touch it; `this.<name>` is the one performing object's
// feature, touched only by a write to a feature it owns; any other qualifier (an
// object's name, a state path) is matched by the written feature's name alone,
// which over-marks rather than under-marks.
func (ctx *Context) placeNamed(p lower.Place, name string, selves []*Instance) bool {
	qualifier, leaf := "", name
	if i := strings.LastIndex(name, "."); i >= 0 {
		qualifier, leaf = name[:i], name[i+1:]
	}
	if p.Name != name && lastSegment(p.Name) != leaf {
		return false
	}
	if p.Local && p.Node != nil && qualifier == "" {
		return false // a pin or a node's own feature is not the bare name's
	}
	if p.Node != nil {
		if node := ActionNodeName(p.Node); node != "" {
			return node == lastSegment(qualifier)
		}
	}
	if p.Sym != nil && qualifier != "" && qualifier != "this" && ctx.run.marks.performed(lastSegment(qualifier)) {
		return declaredUnder(p.Sym, lastSegment(qualifier))
	}
	holder := ctx.performerOwning(selves, p.Sym)
	switch {
	case p.Sym == nil, len(selves) == 0:
		return true
	case qualifier == "":
		return holder == nil
	case qualifier != "this" && holder == nil:
		return true // a feature of an object a part of the action denotes, no performer's own
	default:
		return len(selves) == 1 && holder == selves[0]
	}
}

// declaredUnder reports whether the symbol may be the member named under owner:
// not when owner is declared beside it, a sibling whose members it is not among;
// otherwise the declarations do not say, and it may.
func declaredUnder(sym *symbols.Symbol, owner string) bool {
	if sym.OwnerScope == nil {
		return true
	}
	sibling, ok := sym.OwnerScope.LookupLocal(owner)
	return !ok || sibling == sym
}

// performerSelves are the distinct objects the run's executors perform or have
// performed on: those noted as they joined the clock, and any driven now.
func (ctx *Context) performerSelves() []*Instance {
	selves := slices.Clone(ctx.run.performers)
	for _, w := range ctx.clock.waiters {
		if self := executorSelf(w); self != nil && !slices.Contains(selves, self) {
			selves = append(selves, self)
		}
	}
	return selves
}

// performerOwning is the performing object whose own feature sym is, nil for none.
func (ctx *Context) performerOwning(selves []*Instance, sym *symbols.Symbol) *Instance {
	for _, self := range selves {
		if ctx.ownsFeature(self, sym) {
			return self
		}
	}
	return nil
}

// OrderDependenceText is the marker a value nothing fixed prints with: how many
// of the run's unresolved choice points touch it, and where the set it stands
// in is shown. Empty for a value no conflicting alternative wrote.
func OrderDependenceText(unresolved int) string {
	if unresolved <= 0 {
		return ""
	}
	return fmt.Sprintf("(order-dependent; %d %s — explore shows the set)", unresolved, pluralChoicePoints(unresolved))
}

func pluralChoicePoints(n int) string {
	if n == 1 {
		return "choice point"
	}
	return "choice points"
}

// OrderDependent reports how many unresolved choice points of the run the
// feature's value stands on, by its name alone or as the last segment of a
// written path: those whose alternatives touched it and those what was computed
// into it stood on (order_marks.go).
func (ctx *Context) OrderDependent(feature string) int {
	selves := ctx.performerSelves()
	return ctx.run.marks.standingOn(ctx.liveChoices(), func(p lower.Place) bool { return ctx.placeNamed(p, feature, selves) })
}

// OrderDependentAmong is, for each named value an unresolved choice point of the
// run touches, how many do; a value nothing order-dependent wrote is absent.
func (ctx *Context) OrderDependentAmong(names []string) map[string]int {
	var out map[string]int
	for _, name := range names {
		if n := ctx.OrderDependent(name); n > 0 {
			if out == nil {
				out = make(map[string]int)
			}
			out[name] = n
		}
	}
	return out
}

// UnresolvedChoices counts the run's choice points whose alternatives may not
// commute: the ones a default run's result stands only up to.
func (ctx *Context) UnresolvedChoices() int {
	count := 0
	for _, n := range ctx.run.notes {
		if c, ok := n.(ChoicePoint); ok && !c.Independent {
			count++
		}
	}
	return count
}
