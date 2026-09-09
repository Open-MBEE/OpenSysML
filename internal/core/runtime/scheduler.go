package runtime

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// A run's choice points — several steppable tokens in one step, several holding
// guards at a decision, several enabled transitions for one event, several
// executors due at one instant of the clock — are resolved by a scheduling
// policy; which of two same-step writes to one feature stands follows from the
// token order it chose. The default is what the executors always did; the others
// let a driver ask for another linearization of the run.

// ErrInvalidSchedulePolicy is the typed error every unparseable policy spelling wraps.
var ErrInvalidSchedulePolicy = errors.New("invalid scheduling policy")

// SchedulePolicyError reports a policy spelling that names no policy, with why.
type SchedulePolicyError struct {
	Spelling string
	Reason   string
}

func (e *SchedulePolicyError) Error() string {
	return fmt.Sprintf("%v %q: %s", ErrInvalidSchedulePolicy, e.Spelling, e.Reason)
}

// Is makes every SchedulePolicyError match ErrInvalidSchedulePolicy.
func (e *SchedulePolicyError) Is(target error) bool { return target == ErrInvalidSchedulePolicy }

type scheduleKind int

const (
	// scheduleReverse steps tokens in reverse index order and takes the first
	// holding guard and the first enabled transition: the policy every run had.
	scheduleReverse scheduleKind = iota
	// scheduleDeclared steps tokens in the order they were spawned and takes the
	// first alternative in declaration order.
	scheduleDeclared
	// scheduleSeeded draws every resolution from a pseudo-random sequence a seed fixes.
	scheduleSeeded
	// scheduleExplore replays runs under Explore, each following a recorded prefix of
	// choices and taking the first untried alternative at its frontier.
	scheduleExplore
)

// SchedulePolicy names how the executors resolve the choice points of a run.
// The zero value is the default policy, `reverse`.
type SchedulePolicy struct {
	kind   scheduleKind
	seed   uint64
	budget ExploreBudget
}

// DefaultSchedulePolicy is the policy runs use unless one is set: `reverse`.
var DefaultSchedulePolicy = SchedulePolicy{kind: scheduleReverse}

// SchedulePolicyNames lists the policy spellings ParseSchedulePolicy accepts, for
// usage text; `seed:<n>` stands for any non-negative decimal seed and the
// bracketed options of `explore` are each optional.
var SchedulePolicyNames = []string{"declared", "reverse", "seed:<n>", "explore[:runs=<n>,depth=<d>]"}

// ParseSchedulePolicy reads `declared`, `reverse`, `seed:<n>` or
// `explore[:runs=<n>,depth=<d>]` (either option, either order); "" is the default.
func ParseSchedulePolicy(spelling string) (SchedulePolicy, error) {
	switch {
	case spelling == "":
		return DefaultSchedulePolicy, nil
	case spelling == "reverse":
		return SchedulePolicy{kind: scheduleReverse}, nil
	case spelling == "declared":
		return SchedulePolicy{kind: scheduleDeclared}, nil
	case strings.HasPrefix(spelling, "seed:"):
		digits := spelling[len("seed:"):]
		if digits == "" {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: "seed: needs a number"}
		}
		seed, err := strconv.ParseUint(digits, 10, 64)
		if err != nil {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling,
				Reason: fmt.Sprintf("seed %q is not a non-negative decimal integer", digits)}
		}
		return SchedulePolicy{kind: scheduleSeeded, seed: seed}, nil
	case spelling == "seed":
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: "seed: needs a number"}
	case spelling == "explore":
		return SchedulePolicy{kind: scheduleExplore, budget: DefaultExploreBudget}, nil
	case strings.HasPrefix(spelling, "explore:"):
		budget, reason := parseExploreOptions(spelling[len("explore:"):])
		if reason != "" {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: reason}
		}
		return SchedulePolicy{kind: scheduleExplore, budget: budget}, nil
	default:
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling,
			Reason: "want one of " + strings.Join(SchedulePolicyNames, ", ")}
	}
}

// parseExploreOptions reads the `runs=<n>,depth=<d>` options of an explore
// spelling, returning the budget or why the options name none.
func parseExploreOptions(options string) (ExploreBudget, string) {
	budget := DefaultExploreBudget
	if options == "" {
		return budget, "explore: needs runs=<n> and/or depth=<d> after the colon, or no colon"
	}
	seen := make(map[string]bool)
	for _, option := range strings.Split(options, ",") {
		name, digits, assigned := strings.Cut(option, "=")
		if !assigned || (name != "runs" && name != "depth") {
			return budget, fmt.Sprintf("explore option %q is not runs=<n> or depth=<d>", option)
		}
		if seen[name] {
			return budget, fmt.Sprintf("explore option %s is given twice", name)
		}
		seen[name] = true
		value, err := strconv.ParseUint(digits, 10, 31)
		if err != nil || (name == "runs" && value < 1) {
			least := "0"
			if name == "runs" {
				least = "1"
			}
			return budget, fmt.Sprintf("explore %s %q is not a decimal integer of at least %s", name, digits, least)
		}
		if name == "runs" {
			budget.Runs = int(value)
		} else {
			budget.Depth = int(value)
		}
	}
	return budget, ""
}

// ExplorePolicy is the `explore` policy under the given budget: at least one
// run, and a depth of at least zero.
func ExplorePolicy(budget ExploreBudget) (SchedulePolicy, error) {
	if budget.Runs < 1 || budget.Depth < 0 {
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: fmt.Sprintf("explore:runs=%d,depth=%d", budget.Runs, budget.Depth),
			Reason: "explore runs must be at least 1 and depth at least 0"}
	}
	return SchedulePolicy{kind: scheduleExplore, budget: budget}, nil
}

// String returns the spelling ParseSchedulePolicy reads the policy back from.
func (p SchedulePolicy) String() string {
	switch p.kind {
	case scheduleDeclared:
		return "declared"
	case scheduleSeeded:
		return "seed:" + strconv.FormatUint(p.seed, 10)
	case scheduleExplore:
		var options []string
		if p.budget.Runs != DefaultExploreBudget.Runs {
			options = append(options, "runs="+strconv.Itoa(p.budget.Runs))
		}
		if p.budget.Depth != DefaultExploreBudget.Depth {
			options = append(options, "depth="+strconv.Itoa(p.budget.Depth))
		}
		if len(options) == 0 {
			return "explore"
		}
		return "explore:" + strings.Join(options, ",")
	default:
		return "reverse"
	}
}

// IsDefault reports whether the policy is the one runs use unless told otherwise.
func (p SchedulePolicy) IsDefault() bool { return p == DefaultSchedulePolicy }

// Exploration returns the budget of an `explore` policy, and whether the policy
// is one: such a policy is driven by Explore rather than set on a context.
func (p SchedulePolicy) Exploration() (ExploreBudget, bool) {
	return p.budget, p.kind == scheduleExplore
}

// start begins the sequence of resolutions one run draws under the policy.
func (p SchedulePolicy) start() *scheduler {
	s := &scheduler{policy: p}
	if p.kind == scheduleSeeded {
		s.pcg = rand.NewPCG(p.seed, 0)
		// #nosec G404 -- a replayable run needs a stated generator, not a cryptographic one.
		s.rng = rand.New(s.pcg)
	}
	return s
}

// scheduler resolves the choice points of one run under a policy; a seeded one
// carries the generator state the run consumes choice by choice, an exploring
// one the exploration run the context takes part in.
type scheduler struct {
	policy  SchedulePolicy
	pcg     *rand.PCG
	rng     *rand.Rand
	explore *exploreRun
}

// stepTokens are the tokens one step may try, in spawn order; parked ones cannot
// act yet and held ones collapse into another's synchronization.
type stepTokens struct {
	step    int
	ids     []int64
	parked  map[int64]bool
	held    map[int64]bool
	enabled func(id int64) bool
	label   func(id int64) string
}

// tokenSchedule hands a step its tokens one at a time in the order the policy
// tries them, and is told after each whether it acted.
type tokenSchedule struct {
	order   []int64
	next    int
	explore *exploreStep
}

// Next is the token to try next; false once the step tried them all.
func (ts *tokenSchedule) Next() (int64, bool) {
	if ts.explore != nil {
		return ts.explore.next()
	}
	if ts.next >= len(ts.order) {
		return 0, false
	}
	id := ts.order[ts.next]
	ts.next++
	return id, true
}

// Acted tells the schedule whether the token last handed out did something.
func (ts *tokenSchedule) Acted(id int64, acted bool) {
	if ts.explore != nil {
		ts.explore.acted(id, acted)
	}
}

// scheduleStep fixes how the step tries its tokens: reversed, declared,
// seeded shuffle, or one at a time as the exploration picks them.
func (s *scheduler) scheduleStep(tokens stepTokens) *tokenSchedule {
	if s.policy.kind == scheduleExplore && s.explore != nil {
		return &tokenSchedule{explore: s.explore.beginStep(tokens)}
	}
	ids := tokens.ids
	if len(ids) < 2 {
		return &tokenSchedule{order: ids}
	}
	switch s.policy.kind {
	case scheduleDeclared, scheduleExplore:
	case scheduleSeeded:
		slots := make([]int, 0, len(ids))
		for i, id := range ids {
			if !tokens.parked[id] {
				slots = append(slots, i)
			}
		}
		if len(slots) < 2 {
			break
		}
		s.rng.Shuffle(len(slots), func(i, j int) {
			ids[slots[i]], ids[slots[j]] = ids[slots[j]], ids[slots[i]]
		})
	default:
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
	}
	return &tokenSchedule{order: ids}
}

// pick chooses one of n alternatives given in declaration order.
func (s *scheduler) pick(n int) int {
	if n < 2 {
		return 0
	}
	switch s.policy.kind {
	case scheduleSeeded:
		return s.rng.IntN(n)
	case scheduleExplore:
		if s.explore != nil {
			return s.explore.pick(n)
		}
	}
	return 0
}

// pickDue chooses which of n executors due at one instant (in creation order)
// runs first: the last by default, the first under declared, a draw under a
// seed, and the exploration's turn under explore.
func (s *scheduler) pickDue(n int) int {
	if n < 2 {
		return 0
	}
	switch s.policy.kind {
	case scheduleDeclared:
		return 0
	case scheduleSeeded:
		return s.rng.IntN(n)
	case scheduleExplore:
		if s.explore != nil {
			return s.explore.pick(n)
		}
		return 0
	default:
		return n - 1
	}
}

// describe tells the scheduler how the run reports the choice its last pick made,
// so an exploration's witness names the alternative as the trace does.
func (s *scheduler) describe(c ChoicePoint) {
	if s.policy.kind == scheduleExplore && s.explore != nil {
		s.explore.describe(c)
	}
}

// mark returns the state a probe restores, so previewing a run does not move
// the seeded generator or the exploration's position.
func (s *scheduler) mark() func() {
	if s == nil {
		return func() {
			// No scheduler drove the run, so there is no state to restore.
		}
	}
	if s.explore != nil {
		return s.explore.mark()
	}
	if s.pcg == nil {
		return func() {
			// An unseeded schedule draws nothing, so there is no state to restore.
		}
	}
	saved := *s.pcg
	return func() { *s.pcg = saved }
}
