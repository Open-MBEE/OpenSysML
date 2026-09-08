package runtime

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// A run's choice points — several steppable tokens in one step, several holding
// guards at a decision, several enabled transitions for one event — are resolved
// by a scheduling policy; which of two same-step writes to one feature stands
// follows from the token order it chose. The default is what the executors
// always did; the others let a driver ask for another linearization of the run.

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
)

// SchedulePolicy names how the executors resolve the choice points of a run.
// The zero value is the default policy, `reverse`.
type SchedulePolicy struct {
	kind scheduleKind
	seed uint64
}

// DefaultSchedulePolicy is the policy runs use unless one is set: `reverse`.
var DefaultSchedulePolicy = SchedulePolicy{kind: scheduleReverse}

// SchedulePolicyNames lists the policy spellings ParseSchedulePolicy accepts, for
// usage text; `seed:<n>` stands for any non-negative decimal seed.
var SchedulePolicyNames = []string{"declared", "reverse", "seed:<n>"}

// ParseSchedulePolicy reads a policy spelling: `declared`, `reverse` or `seed:<n>`
// with n a non-negative decimal integer. The empty spelling is the default policy.
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
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling,
			Reason: "reserved for bounded exploration, which is not available yet; want one of " + strings.Join(SchedulePolicyNames, ", ")}
	default:
		return SchedulePolicy{}, &SchedulePolicyError{Spelling: spelling,
			Reason: "want one of " + strings.Join(SchedulePolicyNames, ", ")}
	}
}

// String returns the spelling ParseSchedulePolicy reads the policy back from.
func (p SchedulePolicy) String() string {
	switch p.kind {
	case scheduleDeclared:
		return "declared"
	case scheduleSeeded:
		return "seed:" + strconv.FormatUint(p.seed, 10)
	default:
		return "reverse"
	}
}

// IsDefault reports whether the policy is the one runs use unless told otherwise.
func (p SchedulePolicy) IsDefault() bool { return p == DefaultSchedulePolicy }

// start begins the sequence of resolutions one run draws under the policy.
func (p SchedulePolicy) start() *scheduler {
	s := &scheduler{policy: p}
	if p.kind == scheduleSeeded {
		s.pcg = rand.NewPCG(p.seed, 0)
		s.rng = rand.New(s.pcg)
	}
	return s
}

// scheduler resolves the choice points of one run under a policy; a seeded one
// carries the generator state the run consumes choice by choice.
type scheduler struct {
	policy SchedulePolicy
	pcg    *rand.PCG
	rng    *rand.Rand
}

// orderTokens permutes the IDs of the tokens one step may move, spawn order
// given, into the order the step tries them.
func (s *scheduler) orderTokens(ids []int64) {
	if len(ids) < 2 {
		return
	}
	switch s.policy.kind {
	case scheduleDeclared:
	case scheduleSeeded:
		s.rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	default:
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
	}
}

// pick chooses one of n alternatives given in declaration order.
func (s *scheduler) pick(n int) int {
	if n < 2 || s.policy.kind != scheduleSeeded {
		return 0
	}
	return s.rng.IntN(n)
}

// mark returns the generator state a probe restores, so previewing a run does
// not move the choices the run itself goes on to make.
func (s *scheduler) mark() func() {
	if s == nil || s.pcg == nil {
		return func() {}
	}
	saved := *s.pcg
	return func() { *s.pcg = saved }
}
