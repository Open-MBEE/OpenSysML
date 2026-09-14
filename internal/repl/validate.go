package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// ValidateObject checks every assertion about a held object (a name, #<id> or a path
// such as car.engine) and the objects it holds, then adds a summary verdict for it.
func (s *Session) ValidateObject(ref string) []Verdict {
	defer s.enter()()
	return s.validateObject(ref)
}

// validateObject is ValidateObject for a caller already holding the session.
func (s *Session) validateObject(ref string) []Verdict {
	verdicts := s.validateVerdicts(ref)
	if len(verdicts) > 0 {
		verdicts[0] = s.withTrace(verdicts[0])
	}
	return verdicts
}

func (s *Session) validateVerdicts(ref string) []Verdict {
	if ref == "" {
		return []Verdict{unresolvedVerdict(ref, "usage: %validate <object>")}
	}
	inst, label, err := s.resolveObject(ref)
	if err != nil {
		return []Verdict{unresolvedVerdict(ref, err.Error())}
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []Verdict{unresolvedVerdict(ref, err.Error())}
	}
	// One question to the engines: the object as a whole, whose standing the
	// summary verdict carries.
	report, plan, err := evaluate(s.dispatched(), label, ctx, func(ctx *runtime.Context) (runtime.ValidationReport, error) {
		return ctx.ValidateObject(inst, s.docScopes())
	}, analysis.ValidationAnswer)
	if err != nil {
		return []Verdict{unresolvedVerdict(ref, err.Error())}
	}
	verdicts := make([]Verdict, 0, len(report.Verdicts)+1)
	for _, v := range report.Verdicts {
		verdicts = append(verdicts, s.withVerifications(objectVerdict(v, label), ctx, v.Requirement))
	}
	return append(verdicts, standing(validationSummary(report, label), plan))
}

// objectVerdict renders one assertion's verdict about one object, named by the
// path it was reached along from the validated object.
func objectVerdict(v runtime.ObjectVerdict, label string) Verdict {
	owner := label
	if len(v.Path) > 0 {
		owner = label + "." + strings.Join(v.Path, ".")
	}
	subject := v.Text + " on " + owner
	condition := "Assertion"
	if v.Kind != runtime.AssertionConstraint {
		condition = "Required condition"
	}
	switch v.Status {
	case runtime.ValidationHolds:
		return Verdict{Subject: subject, Status: VerdictHolds, Lines: []string{
			fmt.Sprintf("✓ %s holds%s", v.Text, onInstance(v.Subject, owner)),
		}}
	case runtime.ValidationViolated:
		return Verdict{Subject: subject, Status: VerdictFails, Lines: []string{
			fmt.Sprintf("✗ %s fails%s", v.Text, onInstance(v.Subject, owner)),
			"  " + verdictDetail(condition, v.Err),
		}}
	default:
		return unevaluableVerdict(subject, v.Text, v.Err, v.Subject, owner)
	}
}

// validationSummary is the verdict about the object as a whole: valid only when
// every assertion holds and every nested object was reached.
func validationSummary(report runtime.ValidationReport, label string) Verdict {
	total := len(report.Verdicts)
	fails, undecided := report.Count(runtime.ValidationViolated), report.Count(runtime.ValidationUndecided)
	assertions := fmt.Sprintf("%d %s", total, plural(total, "assertion", "assertions"))
	incomplete := !report.Complete()
	var lines []string
	status := VerdictHolds
	switch {
	case total == 0:
		lines = append(lines, fmt.Sprintf("? %s states no assertion to validate", label))
		status = VerdictUnresolved
	case fails > 0:
		line := fmt.Sprintf("✗ %s is not valid: %d of %s %s", label, fails, assertions, plural(fails, "fails", "fail"))
		if undecided > 0 {
			line += fmt.Sprintf(", %d undecided", undecided)
		}
		lines = append(lines, line)
		status = VerdictFails
	case undecided > 0:
		lines = append(lines, fmt.Sprintf("? %s is not shown valid: %d of %s undecided", label, undecided, assertions))
		status = VerdictUnresolved
	case incomplete:
		lines = append(lines, fmt.Sprintf("? %s is not shown valid: %s %s, but not every object it holds was reached", label, assertions, plural(total, "holds", "hold")))
		status = VerdictUnresolved
	default:
		lines = append(lines, fmt.Sprintf("✓ %s is valid: %s %s %s", label, plural(total, "its", "all"), assertions, plural(total, "holds", "hold")))
	}
	if report.Bounded {
		lines = append(lines, "  Objects nested beyond the depth the validation descends were not validated")
	}
	for _, err := range report.Unread {
		lines = append(lines, fmt.Sprintf("  Not validated: %v", err))
	}
	return Verdict{Subject: label, Status: status, Lines: lines}
}
