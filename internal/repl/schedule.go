package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
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
