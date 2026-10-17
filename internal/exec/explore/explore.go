package explore

import (
	"context"
	"errors"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"slices"
	"sort"
	"strings"
)

// `explore` replays a run from a fresh context per linearization: a recorded
// choice prefix, then the first untried alternative, earliest first, within a budget.

// ErrNotExploring is the typed error Explore returns for a policy that is not `explore`.
var ErrNotExploring = errors.New("scheduling policy does not explore")

// ErrExplorationDiverged is the typed error for a replay that did not meet the
// choice points its prefix recorded, so no outcome set can be trusted.
var ErrExplorationDiverged = errors.New("exploration diverged")

// ProbabilityRange is the probability the model's draws give an outcome,
// least to greatest over schedulers that resolve its scheduling choices.
type ProbabilityRange struct{ Min, Max float64 }

// Exact reports whether the range is a single probability, within floating-point tolerance.
func (r ProbabilityRange) Exact() bool { return r.Max-r.Min <= 1e-12 }

// ExploredOutcome is one distinct outcome an exploration reached: how many
// linearizations reached it, its model probability range when weighted, and
// the choices of the first run that did.
type ExploredOutcome struct {
	Outcome        runtime.Outcome
	Linearizations int
	// Probability is nil unless the exploration made a weighted choice.
	Probability *ProbabilityRange
	Witness     []runtime.ChoiceTaken
	// WitnessRun is the 1-based number of the run the witness is.
	WitnessRun int
}

// Exploration is what exploring a behavior found: its distinct outcomes, in
// canonical order, and whether every linearization within the budget was run.
type Exploration struct {
	Budget   runtime.ExploreBudget
	Runs     int
	Outcomes []ExploredOutcome
	// BudgetsHit names the budgets the exploration ran into, `runs` before
	// `depth`; none when it is complete.
	BudgetsHit []string
	// Unenumerated are the constructs whose unordered alternatives the runs took
	// in one order without a choice point, so fewer orders were run than the
	// library admits; none when the exploration is complete.
	Unenumerated []runtime.CoverageGap
	// Scope is the distinct reasons the runs' outcomes were observed short of quiescence.
	Scope    []runtime.ObservationReason
	weighted bool
}

// Complete reports whether every linearization the library admits was run: no
// budget stopped the exploration and no construct was left unenumerated.
func (x *Exploration) Complete() bool { return len(x.BudgetsHit) == 0 && len(x.Unenumerated) == 0 }

// Weighted reports whether a committed run made a weighted choice.
func (x *Exploration) Weighted() bool { return x.weighted }

// FailedLinearizations counts the runs whose outcomes carry runtime errors.
func (x *Exploration) FailedLinearizations() int {
	total := 0
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			total += o.Linearizations
		}
	}
	return total
}

// ProbabilitiesBounded reports whether the outcomes' probabilities are lower
// bounds while weighted probabilities have unexplored linearizations.
func (x *Exploration) ProbabilitiesBounded() bool { return !x.Complete() && x.Weighted() }

// Status renders how the exploration ended: `complete (N runs)`, or which budget
// was hit after how many runs and what was left unenumerated.
func (x *Exploration) Status() string {
	if x.Complete() {
		return fmt.Sprintf("complete (%d runs)", x.Runs)
	}
	status := fmt.Sprintf("incomplete after %d runs", x.Runs)
	if len(x.BudgetsHit) > 0 {
		named := make([]string, len(x.BudgetsHit))
		for i, budget := range x.BudgetsHit {
			limit := x.Budget.Runs
			if budget == "depth" {
				limit = x.Budget.Depth
			} else if budget == runtime.BoundStatementOrders {
				limit = runtime.MaxStatementOrderEvaluations
			}
			named[i] = fmt.Sprintf("%s budget %d", budget, limit)
		}
		status = fmt.Sprintf("incomplete: %s hit after %d runs", strings.Join(named, " and "), x.Runs)
	}
	if len(x.Unenumerated) > 0 {
		status += "; not enumerated: " + runtime.DescribeGaps(x.Unenumerated)
	}
	if x.ProbabilitiesBounded() {
		status += "; probabilities are lower bounds"
	}
	return status
}

// Explore runs a behavior once per linearization within the policy's budget, one run
// at a time in plan order. A run that fails after it begins is an outcome; setup
// failures and a caller that goes away fail the exploration.
func Explore(stop context.Context, policy runtime.SchedulePolicy, fresh func() (*runtime.Context, error), run func(*runtime.Context) (runtime.Outcome, error)) (*Exploration, error) {
	return ExploreWith(stop, policy, 1, func(int) (*runtime.Context, error) { return fresh() }, run)
}

// sortOutcomes puts an exploration's outcomes in canonical order.
func sortOutcomes(outcomes []ExploredOutcome) {
	sort.SliceStable(outcomes, func(i, j int) bool {
		a, b := outcomes[i].Outcome, outcomes[j].Outcome
		if as, bs := a.String(), b.String(); as != bs {
			return as < bs
		}
		return a.Identity() < b.Identity()
	})
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
	choice       runtime.ChoicePoint
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
func (r *exploreRun) Pick(n int) int {
	slot := exploreSlot{kind: slotPick, alternatives: n}
	r.resolve(&slot, fmt.Sprintf("%d alternatives to pick from", n))
	return slot.taken
}

// describe attaches to the last pick how the run reports it.
func (r *exploreRun) Describe(c runtime.ChoicePoint) {
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
	tokens    runtime.StepTokens
	remaining []int64      // sorted by ID, not yet tried
	held      []int64      // tried last, as they never act on their own
	slot      int          // index in run.record of the slot open, -1 between picks
	dupBefore bool         // run.duplicate when the slot opened, restored if the slot is dropped
	moved     bool         // a token acted, so the step is over
	choice    *exploreSlot // the pick the move resolved, nil when one token alone could act
}

// Choice is the pick the step's move resolved, nil when one token alone could act.
func (s *exploreStep) Choice() (alternatives []string, taken int, ok bool) {
	if s.choice == nil {
		return nil, 0, false
	}
	return s.choice.labels, s.choice.taken, true
}

// BeginStep opens the step's picks.
func (r *exploreRun) BeginStep(tokens runtime.StepTokens) runtime.StepChooser {
	s := &exploreStep{run: r, tokens: tokens, slot: -1}
	for _, id := range tokens.IDs() {
		if tokens.Held(id) {
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
func (s *exploreStep) Next() (int64, bool) {
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
		if s.tokens.Enabled(id) {
			enabled = append(enabled, id)
		}
	}
	switch len(enabled) {
	case 0:
		return s.remaining[0], true
	case 1:
		return enabled[0], true
	}
	slot := exploreSlot{kind: slotTokens, alternatives: len(enabled), step: s.tokens.Step(), tokens: enabled}
	s.dupBefore = s.run.duplicate
	s.run.resolve(&slot, fmt.Sprintf("step %d: tokens %v able to act", s.tokens.Step(), enabled))
	s.slot = len(s.run.record) - 1
	slot = s.run.record[s.slot]
	slot.labels = make([]string, len(slot.tokens))
	for i, id := range slot.tokens {
		slot.labels[i] = s.tokens.Label(id)
	}
	s.run.record[s.slot] = slot
	return slot.tokens[slot.taken], true
}

// acted ends the step when the token acted, closing the pick; one that did not
// was no alternative, so it leaves the slot and the pick is made again.
func (s *exploreStep) Acted(id int64, acted bool) {
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
func (r *exploreRun) choices() []runtime.ChoiceTaken {
	out := make([]runtime.ChoiceTaken, len(r.record))
	for i, slot := range r.record {
		out[i] = slot.asChoice()
	}
	return out
}

// asChoice renders the slot as the choice the run took.
func (s exploreSlot) asChoice() runtime.ChoiceTaken {
	if s.kind == slotTokens {
		return runtime.ChoiceTaken{
			Kind:         runtime.ChoiceTokenOrder,
			Step:         s.step,
			Alternatives: s.alternatives,
			Taken:        s.taken,
			Among:        s.labels,
			Took:         s.labels[s.taken],
		}
	}
	c := runtime.ChoiceTaken{Kind: runtime.ChoiceDecisionBranch, Alternatives: s.alternatives, Taken: s.taken}
	if s.described {
		c.Kind, c.Step, c.Where, c.Among = s.choice.Kind, s.choice.Step, s.choice.Where, s.choice.Alternatives
		if s.taken < len(s.choice.Alternatives) {
			c.Took = s.choice.Alternatives[s.taken]
		}
		if s.choice.Weighted() {
			c.Weights, c.Drew, c.Drawn = slices.Clone(s.choice.Weights), s.choice.Drew, s.choice.Drawn
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

// unexplored is the prefixes this run leaves to explore, earliest first: for each choice
// the run owns — the last its prefix planned and every one it made below — the record up
// to that choice taking its next alternative, when one is untried within depth. The
// choices before the last planned one belong to the runs that planned them.
func (r *exploreRun) unexplored() [][]exploreSlot {
	var next [][]exploreSlot
	for i := max(len(r.prefix)-1, 0); i < len(r.record); i++ {
		slot := r.record[i]
		if slot.beyond || slot.taken+1 >= slot.alternatives {
			continue
		}
		prefix := make([]exploreSlot, i+1)
		copy(prefix, r.record[:i+1])
		prefix[i].taken++
		next = append(next, prefix)
	}
	return next
}

// mark returns what a probe restores: the run's position, so previewing does
// not make choices the run itself goes on to make.
func (r *exploreRun) Mark() func() {
	record, explored := len(r.record), r.explored
	depthHit, diverged := r.depthHit, r.diverged
	return func() {
		r.record = r.record[:record]
		r.explored = explored
		r.depthHit, r.diverged = depthHit, diverged
	}
}
