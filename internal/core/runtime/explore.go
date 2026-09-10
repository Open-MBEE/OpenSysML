package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// `explore` replays a run from a fresh context per linearization: a recorded
// choice prefix, then the first untried alternative, depth-first, within a budget.

// ExploreBudget bounds an exploration: how many runs it may make and how many
// choice points one run may resolve before the rest take their first alternative.
type ExploreBudget struct {
	Runs  int
	Depth int
}

// DefaultExploreBudget is the budget `explore` has when its spelling names none.
var DefaultExploreBudget = ExploreBudget{Runs: 1024, Depth: 64}

// ErrNotExploring is the typed error Explore returns for a policy that is not `explore`.
var ErrNotExploring = errors.New("scheduling policy does not explore")

// ErrExploreUndriven is the typed error SetSchedule returns for `explore`, which
// Explore drives over fresh contexts rather than one.
var ErrExploreUndriven = errors.New("explore is not a policy one context runs under")

// ErrExplorationDiverged is the typed error for a replay that did not meet the
// choice points its prefix recorded, so no outcome set can be trusted.
var ErrExplorationDiverged = errors.New("exploration diverged")

// ChoiceTaken is one choice point of a run and the alternative it took, as the
// witness of an outcome lists them.
type ChoiceTaken struct {
	Kind ChoiceKind
	// Step is the action step the choice was made in, 0 for a state machine.
	Step int
	// Where names the decision node or the state and event; empty for a token order.
	Where string
	// Alternatives is how many the point had; Taken indexes the one taken.
	Alternatives int
	Taken        int
	// Among names the alternatives as a trace does; Took is the one taken: the
	// token tried next, a branch or a transition by declared position and target,
	// or the state whose transition fired first.
	Among []string
	Took  string
}

// String renders the choice for a table or a failure message.
func (c ChoiceTaken) String() string {
	switch c.Kind {
	case ChoiceTokenOrder:
		return fmt.Sprintf("step %d: %s first of %s", c.Step, c.Took, strings.Join(c.Among, ", "))
	case ChoiceDecisionBranch:
		return fmt.Sprintf("step %d: %s -> %s", c.Step, c.Where, c.Took)
	case ChoiceTransition:
		return fmt.Sprintf("%s -> %s", c.Where, c.Took)
	case ChoiceRegionOrder, ChoiceDueOrder:
		return fmt.Sprintf("%s: %s first of %s", c.Where, c.Took, strings.Join(c.Among, ", "))
	}
	return fmt.Sprintf("%s -> %s", c.Kind, c.Took)
}

// FormatChoices renders a witness as one line, its choices in run order.
func FormatChoices(choices []ChoiceTaken) string {
	if len(choices) == 0 {
		return "no choice points"
	}
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = c.String()
	}
	return strings.Join(parts, "; ")
}

// ExploredOutcome is one distinct outcome an exploration reached: how many
// linearizations reached it, and the choices of the first run that did.
type ExploredOutcome struct {
	Outcome        Outcome
	Linearizations int
	Witness        []ChoiceTaken
	// WitnessRun is the 1-based number of the run the witness is.
	WitnessRun int
}

// Exploration is what exploring a behavior found: its distinct outcomes, in
// canonical order, and whether every linearization within the budget was run.
type Exploration struct {
	Budget   ExploreBudget
	Runs     int
	Outcomes []ExploredOutcome
	// BudgetsHit names the budgets the exploration ran into, `runs` before
	// `depth`; none when it is complete.
	BudgetsHit []string
}

// Complete reports whether every linearization was run.
func (x *Exploration) Complete() bool { return len(x.BudgetsHit) == 0 }

// Status renders how the exploration ended: `complete (N runs)`, or which budget
// was hit after how many runs.
func (x *Exploration) Status() string {
	if x.Complete() {
		return fmt.Sprintf("complete (%d runs)", x.Runs)
	}
	named := make([]string, len(x.BudgetsHit))
	for i, budget := range x.BudgetsHit {
		limit := x.Budget.Runs
		if budget == "depth" {
			limit = x.Budget.Depth
		}
		named[i] = fmt.Sprintf("%s budget %d", budget, limit)
	}
	return fmt.Sprintf("incomplete: %s hit after %d runs", strings.Join(named, " and "), x.Runs)
}

// Explore runs a behavior once per linearization within the policy's budget:
// fresh builds each run's context, run performs it and reports the outcome. A run
// that failed is an outcome; a caller that goes away between runs takes the
// exploration with it, its error being stop's.
func Explore(stop context.Context, policy SchedulePolicy, fresh func() (*Context, error), run func(*Context) (Outcome, error)) (*Exploration, error) {
	budget, ok := policy.Exploration()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotExploring, policy)
	}
	result := &Exploration{Budget: budget}
	reached := make(map[string]int)
	var prefix []exploreSlot
	depthHit := false
	for {
		if result.Runs == budget.Runs {
			result.BudgetsHit = append(result.BudgetsHit, "runs")
			break
		}
		if err := stop.Err(); err != nil {
			return nil, err
		}
		ctx, err := fresh()
		if err != nil {
			return nil, err
		}
		replay := &exploreRun{prefix: prefix, depth: budget.Depth}
		ctx.beginExploration(policy, replay)
		outcome, runErr := run(ctx)
		result.Runs++
		if runErr != nil {
			outcome = Outcome{Err: runErr}
		}
		if err := replay.followed(); err != nil {
			return nil, fmt.Errorf("%w: run %d: %v", ErrExplorationDiverged, result.Runs, err)
		}
		depthHit = depthHit || replay.depthHit
		key := outcome.identity()
		if i, seen := reached[key]; seen {
			if !replay.duplicate {
				result.Outcomes[i].Linearizations++
			}
		} else {
			reached[key] = len(result.Outcomes)
			result.Outcomes = append(result.Outcomes, ExploredOutcome{
				Outcome:        outcome,
				Linearizations: 1,
				Witness:        replay.choices(),
				WitnessRun:     result.Runs,
			})
		}
		next, more := replay.nextPrefix()
		if !more {
			break
		}
		prefix = next
	}
	if depthHit {
		result.BudgetsHit = append(result.BudgetsHit, "depth")
	}
	sort.SliceStable(result.Outcomes, func(i, j int) bool {
		a, b := result.Outcomes[i].Outcome, result.Outcomes[j].Outcome
		if as, bs := a.String(), b.String(); as != bs {
			return as < bs
		}
		return a.identity() < b.identity()
	})
	return result, nil
}

// slotKind is what an exploration slot resolves: a pick among alternatives given
// in declaration order, or which of the tokens able to act a step tries next.
type slotKind int

const (
	slotPick slotKind = iota
	slotTokens
)

// exploreSlot is one decision an exploring run made: the alternatives it had,
// the one taken, and how the run describes it.
type exploreSlot struct {
	kind         slotKind
	alternatives int
	taken        int
	beyond       bool // resolved past the depth budget, so not the exploration's to vary
	step         int
	tokens       []int64  // the tokens able to act, sorted by ID
	labels       []string // those tokens as the trace names them
	choice       ChoicePoint
	described    bool
}

// exploreRun is the plan and record of one exploring run: the slots it must
// follow and the slots it made.
type exploreRun struct {
	prefix    []exploreSlot
	depth     int
	record    []exploreSlot
	explored  int // choice points made that count against depth
	depthHit  bool
	duplicate bool // the frontier's alternative could not act, so the run repeats one made
	diverged  error
}

// pick resolves a choice among n alternatives: the planned one within the prefix,
// the first otherwise.
func (r *exploreRun) pick(n int) int {
	slot := exploreSlot{kind: slotPick, alternatives: n}
	r.resolve(&slot, fmt.Sprintf("%d alternatives to pick from", n))
	return slot.taken
}

// describe attaches to the last pick how the run reports it.
func (r *exploreRun) describe(c ChoicePoint) {
	for i := len(r.record) - 1; i >= 0; i-- {
		if slot := &r.record[i]; slot.kind == slotPick {
			slot.choice, slot.described = c, true
			return
		}
	}
}

// exploreStep tries a step's tokens one at a time until one acts: that move is
// the step, a choice among the tokens able to act at that moment.
type exploreStep struct {
	run       *exploreRun
	tokens    stepTokens
	remaining []int64      // sorted by ID, not yet tried
	held      []int64      // tried last, as they never act on their own
	slot      int          // index in run.record of the slot open, -1 between picks
	dupBefore bool         // run.duplicate when the slot opened, restored if the slot is dropped
	moved     bool         // a token acted, so the step is over
	choice    *exploreSlot // the pick the move resolved, nil when one token alone could act
}

// beginStep opens the step's picks.
func (r *exploreRun) beginStep(tokens stepTokens) *exploreStep {
	s := &exploreStep{run: r, tokens: tokens, slot: -1}
	for _, id := range tokens.ids {
		if tokens.held[id] {
			s.held = append(s.held, id)
		} else {
			s.remaining = append(s.remaining, id)
		}
	}
	slices.Sort(s.remaining)
	slices.Sort(s.held)
	return s
}

// next picks the token to try: the planned or first of those able to act when at
// least two are, the only one when one is, else the first left, which will not act.
func (s *exploreStep) next() (int64, bool) {
	if s.moved {
		return 0, false
	}
	if len(s.remaining) == 0 {
		if len(s.held) == 0 {
			return 0, false
		}
		id := s.held[0]
		s.held = s.held[1:]
		return id, true
	}
	if s.slot >= 0 {
		slot := &s.run.record[s.slot]
		return slot.tokens[slot.taken], true
	}
	enabled := make([]int64, 0, len(s.remaining))
	for _, id := range s.remaining {
		if s.tokens.enabled(id) {
			enabled = append(enabled, id)
		}
	}
	switch len(enabled) {
	case 0:
		return s.remaining[0], true
	case 1:
		return enabled[0], true
	}
	slot := exploreSlot{kind: slotTokens, alternatives: len(enabled), step: s.tokens.step, tokens: enabled}
	s.dupBefore = s.run.duplicate
	s.run.resolve(&slot, fmt.Sprintf("step %d: tokens %v able to act", s.tokens.step, enabled))
	s.slot = len(s.run.record) - 1
	slot = s.run.record[s.slot]
	slot.labels = make([]string, len(slot.tokens))
	for i, id := range slot.tokens {
		slot.labels[i] = s.tokens.label(id)
	}
	s.run.record[s.slot] = slot
	return slot.tokens[slot.taken], true
}

// acted ends the step when the token acted, closing the pick; one that did not
// was no alternative, so it leaves the slot and the pick is made again.
func (s *exploreStep) acted(id int64, acted bool) {
	if i := slices.Index(s.remaining, id); i >= 0 {
		s.remaining = slices.Delete(s.remaining, i, i+1)
	}
	if acted {
		s.moved = true
		if s.slot >= 0 {
			slot := s.run.record[s.slot]
			s.choice = &slot
			s.slot = -1
		}
		return
	}
	if s.slot < 0 {
		return
	}
	slot := &s.run.record[s.slot]
	if i := slices.Index(slot.tokens, id); i >= 0 {
		slot.tokens = slices.Delete(slices.Clone(slot.tokens), i, i+1)
		slot.labels = slices.Delete(slices.Clone(slot.labels), i, i+1)
		slot.alternatives--
	}
	if len(slot.tokens) == 0 {
		// No token of the pick acted, so it was no choice point and repeated nothing.
		s.run.record = s.run.record[:s.slot]
		s.run.explored--
		s.run.duplicate = s.dupBefore
		s.slot = -1
		return
	}
	if slot.taken >= len(slot.tokens) {
		// The plan's untried alternative could not act: the run repeats the last one made.
		slot.taken = len(slot.tokens) - 1
		s.run.duplicate = true
	}
}

// resolve settles the slot's alternative: the planned one within the prefix, the
// first past it or past the depth budget; the slot is recorded as made.
func (r *exploreRun) resolve(slot *exploreSlot, faced string) {
	i := len(r.record)
	switch {
	case i < len(r.prefix):
		planned := r.prefix[i]
		if !planned.matches(*slot) {
			r.diverge(fmt.Sprintf("choice %d: %s, planned %s", i+1, faced, planned.describePlan()))
		} else {
			slot.taken, slot.beyond = planned.taken, planned.beyond
			if slot.kind == slotTokens {
				slot.tokens, slot.alternatives = planned.tokens, planned.alternatives
			}
		}
	case r.explored >= r.depth:
		slot.beyond = true
	}
	if slot.beyond {
		r.depthHit = true
	}
	r.explored++
	r.record = append(r.record, *slot)
}

// matches reports whether the slot faced is the one planned: the same pick, or
// a step whose tokens able to act include those the plan found so.
func (planned exploreSlot) matches(faced exploreSlot) bool {
	if planned.kind != faced.kind {
		return false
	}
	if planned.kind == slotPick {
		return planned.alternatives == faced.alternatives
	}
	for _, id := range planned.tokens {
		if !slices.Contains(faced.tokens, id) {
			return false
		}
	}
	return true
}

// diverge records the first way the run left its plan.
func (r *exploreRun) diverge(reason string) {
	if r.diverged == nil {
		r.diverged = errors.New(reason)
	}
}

// followed reports whether the run made every choice its prefix planned, as planned.
func (r *exploreRun) followed() error {
	if r.diverged != nil {
		return r.diverged
	}
	if len(r.record) < len(r.prefix) {
		return fmt.Errorf("%d choice points reached, %d planned", len(r.record), len(r.prefix))
	}
	return nil
}

// choices lists the choice points the run made, in order, as a witness.
func (r *exploreRun) choices() []ChoiceTaken {
	out := make([]ChoiceTaken, len(r.record))
	for i, slot := range r.record {
		out[i] = slot.asChoice()
	}
	return out
}

// asChoice renders the slot as the choice the run took.
func (s exploreSlot) asChoice() ChoiceTaken {
	if s.kind == slotTokens {
		return ChoiceTaken{
			Kind:         ChoiceTokenOrder,
			Step:         s.step,
			Alternatives: s.alternatives,
			Taken:        s.taken,
			Among:        s.labels,
			Took:         s.labels[s.taken],
		}
	}
	c := ChoiceTaken{Kind: ChoiceDecisionBranch, Alternatives: s.alternatives, Taken: s.taken}
	if s.described {
		c.Kind, c.Step, c.Where, c.Among = s.choice.Kind, s.choice.Step, s.choice.Where, s.choice.Alternatives
		if s.taken < len(s.choice.Alternatives) {
			c.Took = s.choice.Alternatives[s.taken]
		}
	}
	if c.Took == "" {
		c.Took = fmt.Sprintf("alternative %d of %d", s.taken+1, s.alternatives)
	}
	return c
}

// describePlan renders the slot as a plan: what was faced and what was taken.
func (s exploreSlot) describePlan() string {
	if s.kind == slotTokens {
		return fmt.Sprintf("step %d: token %d of %v", s.step, s.tokens[s.taken], s.tokens)
	}
	return fmt.Sprintf("alternative %d of %d", s.taken+1, s.alternatives)
}

// nextPrefix is the record up to the last choice with an untried alternative,
// taking the next one; nil when every alternative within depth was tried.
func (r *exploreRun) nextPrefix() ([]exploreSlot, bool) {
	for i := len(r.record) - 1; i >= 0; i-- {
		slot := r.record[i]
		if slot.beyond || slot.taken+1 >= slot.alternatives {
			continue
		}
		next := make([]exploreSlot, i+1)
		copy(next, r.record[:i+1])
		next[i].taken++
		return next, true
	}
	return nil, false
}

// mark returns what a probe restores: the run's position, so previewing does
// not make choices the run itself goes on to make.
func (r *exploreRun) mark() func() {
	record, explored := len(r.record), r.explored
	depthHit, diverged := r.depthHit, r.diverged
	return func() {
		r.record = r.record[:record]
		r.explored = explored
		r.depthHit, r.diverged = depthHit, diverged
	}
}
