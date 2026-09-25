package repl

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// Schedule returns the policy runs started from here on resolve their choice
// points under.
func (s *Session) Schedule() runtime.SchedulePolicy {
	defer s.reading()()
	return s.schedule
}

// SetSchedule sets the policy for runs started from here on; a debugger under
// way keeps its own, and `explore` runs on contexts of their own.
func (s *Session) SetSchedule(policy runtime.SchedulePolicy) error {
	defer s.enter()()
	return s.setSchedule(policy)
}

func (s *Session) setSchedule(policy runtime.SchedulePolicy) error {
	s.schedule = policy
	if s.rtCtx != nil {
		return s.rtCtx.SetSchedule(s.drivenSchedule())
	}
	return nil
}

// doSchedule shows the scheduling policy, or sets it when one is named. An
// `explore` policy is refused: the prompt's debuggers step one run at a time.
func (s *Session) doSchedule(args []string) []string {
	if len(args) > 0 {
		policy, err := runtime.ParseSchedulePolicy(args[0])
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
		if _, explores := policy.Exploration(); explores {
			return []string{errPrefix + (&ExploreAtPromptError{Policy: policy}).Error()}
		}
		if err := s.setSchedule(policy); err != nil {
			return []string{errPrefix + err.Error()}
		}
	}
	return []string{fmt.Sprintf("schedule: %s", s.schedule)}
}

// ModelSeed returns the seed runs started from here on draw their modeled
// randomness — weighted decisions, RandomFunctions — from, and whether one is set.
func (s *Session) ModelSeed() (uint64, bool) {
	defer s.reading()()
	return s.modelSeed.value, s.modelSeed.set
}

// SetModelSeed fixes the seed runs started from here on draw their modeled
// randomness from, whatever the scheduling policy; ClearModelSeed leaves it to
// the policy's own seed, so a `declared` run that draws is refused.
func (s *Session) SetModelSeed(seed uint64) {
	defer s.enter()()
	s.setModelSeed(sessionSeed{value: seed, set: true})
}

// ClearModelSeed unsets the seed SetModelSeed set.
func (s *Session) ClearModelSeed() {
	defer s.enter()()
	s.setModelSeed(sessionSeed{})
}

func (s *Session) setModelSeed(seed sessionSeed) {
	s.modelSeed = seed
	if s.rtCtx != nil {
		s.applyDraws(s.rtCtx)
	}
}

// applyDraws gives ctx the session's model seed, or none, its draw policy and its clock step.
func (s *Session) applyDraws(ctx *runtime.Context) {
	if s.modelSeed.set {
		ctx.SetModelSeed(s.modelSeed.value)
	} else {
		ctx.ClearModelSeed()
	}
	ctx.SetDrawPolicy(s.draws)
	_ = ctx.SetClockStep(s.clockStep) // checked when the session took it
}

// askedModelSeed is the session's model seed as a question to the engines carries it.
func (s *Session) askedModelSeed() analysis.ModelSeed {
	return analysis.ModelSeed{Seed: s.modelSeed.value, Set: s.modelSeed.set}
}

// Draws returns the policy runs started from here on resolve their RandomFunctions
// draws under: at random from the seeded stream, or at each call's min, max or average.
func (s *Session) Draws() runtime.DrawPolicy {
	defer s.reading()()
	return s.draws
}

// SetDraws sets the draw policy for runs started from here on, the debugger's
// included; a fixed policy draws without a seed.
func (s *Session) SetDraws(policy runtime.DrawPolicy) {
	defer s.enter()()
	s.setDraws(policy)
}

func (s *Session) setDraws(policy runtime.DrawPolicy) {
	s.draws = policy
	if s.rtCtx != nil {
		s.rtCtx.SetDrawPolicy(policy)
	}
}

// ClockStep returns the step the clock of runs started from here on ticks by, in
// seconds; 0 is a continuous clock.
func (s *Session) ClockStep() float64 {
	defer s.reading()()
	return s.clockStep
}

// SetClockStep sets the clock step for runs started from here on, the debugger's
// included: their waits come due at multiples of step seconds; 0 restores the
// continuous clock. A step that is not a finite, non-negative number is runtime.ErrClockStep.
func (s *Session) SetClockStep(step float64) error {
	defer s.enter()()
	return s.setClockStep(step)
}

func (s *Session) setClockStep(step float64) error {
	if err := runtime.CheckClockStep(step); err != nil {
		return err
	}
	s.clockStep = step
	if s.rtCtx != nil {
		_ = s.rtCtx.SetClockStep(step)
	}
	return nil
}

// doClockStep shows the clock step, or sets it when exactly one is named.
func (s *Session) doClockStep(args []string) []string {
	if len(args) > 1 {
		return []string{"usage: %clock-step [<seconds>]"}
	}
	if len(args) > 0 {
		step, err := runtime.ParseClockStep(args[0])
		if err == nil {
			err = s.setClockStep(step)
		}
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
	}
	if s.clockStep == 0 {
		return []string{"clock step: none (a continuous clock)"}
	}
	return []string{fmt.Sprintf("clock step: %s s", semantics.FormatReal(s.clockStep))}
}

// doDraws shows the draw policy, or sets it when exactly one is named.
func (s *Session) doDraws(args []string) []string {
	if len(args) > 1 {
		return []string{"usage: %draws [random|min|max|average]"}
	}
	if len(args) > 0 {
		policy, err := runtime.ParseDrawPolicy(args[0])
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
		s.setDraws(policy)
	}
	return []string{fmt.Sprintf("draws: %s", s.draws)}
}

// sessionSeed is a model seed and whether one is set.
type sessionSeed struct {
	value uint64
	set   bool
}

// doSeed shows the model seed, sets it when a number is given, or unsets it on `off`.
func (s *Session) doSeed(args []string) []string {
	if len(args) > 0 {
		switch {
		case args[0] == "off":
			s.setModelSeed(sessionSeed{})
		default:
			seed, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return []string{errPrefix + fmt.Sprintf("%q is not a seed: name a whole number, or off", args[0])}
			}
			s.setModelSeed(sessionSeed{value: seed, set: true})
		}
	}
	if !s.modelSeed.set {
		return []string{"seed: off (a run that draws needs %schedule seed:<n> or %seed <n>)"}
	}
	return []string{fmt.Sprintf("seed: %d", s.modelSeed.value)}
}
