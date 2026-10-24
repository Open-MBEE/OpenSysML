package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/selfcheck"
)

// ErrSelfCheckPackageNotFound is wrapped by SelfCheckPackages when a
// -self-check-package name resolves to no package.
var ErrSelfCheckPackageNotFound = selfcheck.ErrPackageNotFound

// SelfCheck applies the SysMLValidation constraints to every element of the
// loaded model and reports what fails.
func (s *Session) SelfCheck() []Verdict {
	defer s.enter()()
	verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
	return verdicts
}

// ResolveSelfCheckPackages reports the -self-check-package names no loaded model
// or library declares a package for, or nil when every name resolves.
func (s *Session) ResolveSelfCheckPackages(pkgs []string) error {
	defer s.enter()()
	_, err := selfcheck.ResolvePackages(s.browseIndex(), pkgs)
	return err
}

// SelfCheckPackages applies the SysMLValidation constraints together with the
// constraint defs of each named rule package. A name nothing declares is an
// error wrapping ErrSelfCheckPackageNotFound and nothing is evaluated.
func (s *Session) SelfCheckPackages(pkgs []string) ([]Verdict, error) {
	defer s.enter()()
	verdicts, _, err := s.selfCheckRun("SysMLValidation", false, pkgs)
	return verdicts, err
}

func (s *Session) selfCheckWithCounts(pkg string, includeWorkspacePackages bool) ([]Verdict, selfcheck.Counts) {
	verdicts, counts, _ := s.selfCheckRun(pkg, includeWorkspacePackages, nil)
	return verdicts, counts
}

// selfCheckRun runs the shared check over the session's model and spells each
// finding as the verdict the REPL prints, the summary last.
func (s *Session) selfCheckRun(pkg string, includeWorkspacePackages bool, rulePackages []string) ([]Verdict, selfcheck.Counts, error) {
	idx := s.browseIndex()
	groups, err := selfcheck.ResolvePackages(idx, rulePackages)
	if err != nil {
		return nil, selfcheck.Counts{Applications: make(map[string]int)}, err
	}
	in := selfcheck.Input{Index: idx, Runtime: s.getOrCreateRuntime, Package: pkg,
		IncludeWorkspacePackages: includeWorkspacePackages, RulePackages: groups}
	if idx != nil {
		sem := passes.NewTypedModel(resolve.New(idx))
		sem.SetSourceText(s.sessionSourceText())
		in.Model = sem
		in.Sources = selfcheck.Sources(s.sessionDocs())
	}
	report := selfcheck.Run(in)
	verdicts := make([]Verdict, 0, len(report.Findings)+1)
	for _, f := range report.Findings {
		verdicts = append(verdicts, selfCheckVerdict(f))
	}
	status := VerdictHolds
	if report.Counts.Violations > 0 {
		status = VerdictFails
	} else if report.Counts.EvaluationErrors > 0 {
		status = VerdictUnresolved
	}
	return append(verdicts, Verdict{Subject: "Self-model check", Status: status, Lines: []string{report.Counts.Summary()}}), report.Counts, nil
}

func selfCheckVerdict(f selfcheck.Finding) Verdict {
	switch f.Status {
	case selfcheck.Fails:
		return Verdict{Subject: f.Subject, Status: VerdictFails, Lines: []string{f.Line()}}
	case selfcheck.Unevaluated:
		v := unevaluableVerdict(f.Subject, "Constraint "+f.Constraint, f.Err, nil, "")
		v.Lines[0] = f.Line()
		return v
	}
	return Verdict{Subject: f.Subject, Status: VerdictUnresolved, Lines: []string{f.Line()}}
}
