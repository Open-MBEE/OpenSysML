package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Schedule returns the policy runs started from here on resolve their choice
// points under.
func (s *Session) Schedule() runtime.SchedulePolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schedule
}

// SetSchedule sets the policy for runs started from here on. A debugging
// session already under way keeps the policy it started with.
func (s *Session) SetSchedule(policy runtime.SchedulePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setSchedule(policy)
}

func (s *Session) setSchedule(policy runtime.SchedulePolicy) {
	s.schedule = policy
	if s.rtCtx != nil {
		s.rtCtx.SetSchedule(policy)
	}
}

// doSchedule shows the scheduling policy, or sets it when one is named.
func (s *Session) doSchedule(args []string) []string {
	if len(args) > 0 {
		policy, err := runtime.ParseSchedulePolicy(args[0])
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
		s.setSchedule(policy)
	}
	return []string{fmt.Sprintf("schedule: %s", s.schedule)}
}
