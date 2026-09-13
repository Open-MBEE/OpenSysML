package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
)

// Jobs returns how many runs of one plan the session lets go concurrently.
func (s *Session) Jobs() int {
	defer s.reading()()
	return s.jobs
}

// SetJobs sets how many runs of one plan go concurrently from here on. The held
// context, its objects and the debuggers keep going: jobs bound a plan's runs,
// not the session's context. A value below one is a typed error.
func (s *Session) SetJobs(jobs int) error {
	if jobs <= 0 {
		return &analysis.JobsError{Source: "%jobs", Value: fmt.Sprint(jobs)}
	}
	defer s.enter()()
	s.jobs = jobs
	return nil
}

// doJobs shows the jobs setting, or sets it when a count is given.
func (s *Session) doJobs(args []string) []string {
	if len(args) > 0 {
		jobs, err := analysis.ParseJobs("%jobs", args[0])
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
		s.jobs = jobs
	}
	return []string{fmt.Sprintf("jobs: %d", s.jobs)}
}
