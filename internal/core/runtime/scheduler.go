package runtime

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
)

// A run's choice points — several steppable tokens in one step, several holding
// guards at a decision, several enabled transitions for one event, several
// regions reacting to one event, several executors due at one instant of the
// clock — are resolved by a scheduling policy; which of two same-step writes to
// one feature stands follows from the token order it chose. The default is what
// the executors always did; the others let a driver ask for another
// linearization of the run.

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
	// scheduleReplay follows a witness move for move, then behaves as reverse; a
	// move the run cannot make is refused (replay.go).
	scheduleReplay
)

// SchedulePolicy names how the executors resolve the choice points of a run.
// The zero value is the default policy, `reverse`.
type SchedulePolicy struct {
	kind   scheduleKind
	seed   uint64
	budget ExploreBudget
	replay *replayScript
}

// DefaultSchedulePolicy is the policy runs use unless one is set: `reverse`.
var DefaultSchedulePolicy = SchedulePolicy{kind: scheduleReverse}

// DefaultExploreSchedulePolicy is `explore` with no options: every linearization
// within the default budget.
var DefaultExploreSchedulePolicy = SchedulePolicy{kind: scheduleExplore, budget: DefaultExploreBudget}

// SchedulePolicyNames lists the policy spellings ParseSchedulePolicy accepts, for
// usage text; `seed:<n>` stands for any non-negative decimal seed and the
// bracketed options of `explore` are each optional; `replay:<file>` names a
// file of choice lines.
var SchedulePolicyNames = []string{"declared", "reverse", "seed:<n>", "explore[:runs=<n>,depth=<d>]", "replay:<file>"}

// exploreOptionsPrefix opens the `explore` spelling that carries options.
const exploreOptionsPrefix = "explore:"

// ParseSchedulePolicy reads `declared`, `reverse`, `seed:<n>`,
// `explore[:runs=<n>,depth=<d>]` (either option, either order) or `replay:<file>`,
// whose file is read here (see ParseChoices); "" is the default.
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
		return DefaultExploreSchedulePolicy, nil
	case strings.HasPrefix(spelling, exploreOptionsPrefix):
		budget, reason := parseExploreOptions(spelling[len(exploreOptionsPrefix):])
		if reason != "" {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: reason}
		}
		return SchedulePolicy{kind: scheduleExplore, budget: budget}, nil
	case spelling == "replay" || spelling == "replay:":
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: "replay: needs a file of choice lines"}
	case strings.HasPrefix(spelling, "replay:"):
		file := spelling[len("replay:"):]
		text, err := os.ReadFile(file) // #nosec G304 -- the policy names the witness file to follow.
		if err != nil {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: err.Error()}
		}
		choices, err := ParseChoices(string(text))
		if err != nil {
			return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling, Reason: err.Error()}
		}
		return SchedulePolicy{kind: scheduleReplay, replay: &replayScript{file: file, choices: choices}}, nil
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

// String returns the spelling ParseSchedulePolicy reads the policy back from; a
// replay of a witness held in memory names no file and spells `replay`.
func (p SchedulePolicy) String() string {
	switch p.kind {
	case scheduleDeclared:
		return "declared"
	case scheduleReplay:
		if p.replay.file == "" {
			return "replay"
		}
		return "replay:" + p.replay.file
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
		return exploreOptionsPrefix + strings.Join(options, ",")
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
	switch p.kind {
	case scheduleSeeded:
		s.pcg = rand.NewPCG(p.seed, 0)
		// #nosec G404 -- a replayable run needs a stated generator, not a cryptographic one.
		s.rng = rand.New(s.pcg)
	case scheduleReplay:
		s.replay = &replayRun{choices: p.replay.choices}
	}
	return s
}

// scheduler resolves the choice points of one run under a policy; a seeded one
// carries the generator state the run consumes choice by choice, an exploring
// one the exploration run the context takes part in, a replaying one its
// position in the witness.
type scheduler struct {
	policy  SchedulePolicy
	pcg     *rand.PCG
	rng     *rand.Rand
	explore *exploreRun
	replay  *replayRun
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
	replay  *replayMove
}

// Next is the token to try next; false once the step tried them all.
func (ts *tokenSchedule) Next() (int64, bool) {
	if ts.explore != nil {
		return ts.explore.next()
	}
	if ts.replay != nil {
		return ts.replay.nextToken()
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
	if ts.replay != nil {
		ts.replay.acted(acted)
	}
}

// Choice is the token-order pick an exploring or replaying step resolved, as the
// trace names the tokens able to act and the index of the one moved; false when
// it made none.
func (ts *tokenSchedule) Choice() (alternatives []string, taken int, ok bool) {
	if ts.replay != nil {
		return ts.replay.reported()
	}
	if ts.explore == nil || ts.explore.choice == nil {
		return nil, 0, false
	}
	return ts.explore.choice.labels, ts.explore.choice.taken, true
}

// oneMove reports whether a step is one token's move — the exploration's pick among
// every token able to act, or the witness's — rather than a sweep giving each token its turn.
func (s *scheduler) oneMove() bool {
	return (s.policy.kind == scheduleExplore && s.explore != nil) || s.replaying()
}

// replaying reports whether the run still has witness moves to follow.
func (s *scheduler) replaying() bool {
	return s.replay != nil && s.replay.following()
}

// scheduleStep fixes how the step tries its tokens: reversed, declared,
// seeded shuffle, or one at a time as the exploration or the witness picks them.
func (s *scheduler) scheduleStep(tokens stepTokens) *tokenSchedule {
	if s.replaying() {
		return &tokenSchedule{replay: s.replay.beginStep(tokens)}
	}
	if s.oneMove() {
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

// choose resolves the choice point c, whose Alternatives are canonical and whose
// Taken is not yet set, to the index taken: the first by default (the last for a
// due order), the first under declared, a draw under a seed, the exploration's
// turn under explore and the witness's move under replay. whereOf, when not nil,
// is how the run reports Where once alternative i is taken (a transition's names
// its trigger); an exploration's witness names the choice as the run reports it.
func (s *scheduler) choose(c ChoicePoint, whereOf func(i int) string) int {
	n := len(c.Alternatives)
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
			c.Taken = s.explore.pick(n)
			if whereOf != nil {
				c.Where = whereOf(c.Taken)
			}
			s.explore.describe(c)
			return c.Taken
		}
		return 0
	case scheduleReplay:
		if s.replaying() {
			return s.replay.choose(c, whereOf)
		}
	}
	if c.Kind == ChoiceDueOrder {
		return n - 1
	}
	return 0
}

// refusal is the witness move a replaying run could not follow, nil for none.
func (s *scheduler) refusal() error {
	if s.replay == nil {
		return nil
	}
	return s.replay.refused
}

// unfollowed is the refusal of a replaying run that ended, as how says, with
// witness moves left; nil for a run that followed its witness.
func (s *scheduler) unfollowed(how string) error {
	if s == nil || s.replay == nil {
		return nil
	}
	return s.replay.unfollowed(how)
}

// mark returns the state a probe restores, so previewing a run does not move
// the seeded generator, the exploration's position or the witness's.
func (s *scheduler) mark() func() {
	if s == nil {
		return func() {
			// No scheduler drove the run, so there is no state to restore.
		}
	}
	if s.explore != nil {
		return s.explore.mark()
	}
	if s.replay != nil {
		return s.replay.mark()
	}
	if s.pcg == nil {
		return func() {
			// An unseeded schedule draws nothing, so there is no state to restore.
		}
	}
	saved := *s.pcg
	return func() { *s.pcg = saved }
}
