package queryexec

import (
	"errors"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Verdict is one row of Verdicts: an assertion checked about one object, and
// how it came out. Immutable once built.
type Verdict struct {
	kind         runtime.AssertionKind
	assertion    *symbols.Symbol
	requirement  *symbols.Symbol
	text         string
	carrier      *runtime.Instance
	carrierDecl  *symbols.Symbol
	path         string
	status       runtime.ValidationStatus
	condition    string
	reason       string
	verification []runtime.VerdictKind
	held         bool
}

// AssertionVerification is the kind of a verdict row reporting a verification
// case's run, following the requirement verdict it verifies.
const AssertionVerification runtime.AssertionKind = "verification"

// Kind is the kind of assertion checked: constraint, requirement, satisfaction
// or verification.
func (v Verdict) Kind() runtime.AssertionKind { return v.kind }

// Assertion is the asserting element: the constraint, requirement, satisfy
// usage or verification case.
func (v Verdict) Assertion() *symbols.Symbol { return v.assertion }

// Requirement is the requirement a requirement, satisfaction or verification
// verdict is about; nil for a constraint.
func (v Verdict) Requirement() *symbols.Symbol { return v.requirement }

// Text is the assertion as written (`constraint massLimit`), naming an anonymous one.
func (v Verdict) Text() string { return v.text }

// Carrier is the object the assertion was checked on, and whether the query's
// session holds it rather than the check having materialized it as declared.
func (v Verdict) Carrier() (*runtime.Instance, bool) { return v.carrier, v.held }

// CarrierDeclaration is the element the carrier stands for in the model.
func (v Verdict) CarrierDeclaration() *symbols.Symbol { return v.carrierDecl }

// Path names the carrier from the row checked: its label, then the features
// walked to the carrier (`car.wheels[2]`).
func (v Verdict) Path() string { return v.path }

// Status is how the assertion came out.
func (v Verdict) Status() runtime.ValidationStatus { return v.status }

// Condition is the condition that evaluated to false on a violated verdict.
func (v Verdict) Condition() string { return v.condition }

// Reason is why the assertion is violated or undecided; empty when it holds.
func (v Verdict) Reason() string { return v.reason }

// Verification is the verdict kinds of the verification cases the row reports:
// its own case on a verification row, those verifying its requirement otherwise.
func (v Verdict) Verification() []runtime.VerdictKind {
	return append([]runtime.VerdictKind(nil), v.verification...)
}

// Label names the verdict: the assertion as written on its carrier's path.
func (v Verdict) Label() string {
	if v.path == "" {
		return v.text
	}
	return v.text + " on " + v.path
}

// Summary is the verdict in one line: its label and status.
func (v Verdict) Summary() string {
	return v.Label() + ": " + v.status.String()
}

// VerdictValue constructs a verdict value; its declaration is the assertion.
func VerdictValue(verdict Verdict) Value {
	return Value{kind: ValueVerdict, verdict: &verdict, origin: verdict.assertion.Origin()}
}

// Verdict returns the value's verdict and whether it is a verdict value.
func (v Value) Verdict() (Verdict, bool) {
	if v.kind != ValueVerdict || v.verdict == nil {
		return Verdict{}, false
	}
	return *v.verdict, true
}

// assertionVerdict converts one verdict of a validation report asked of the row
// labelled label; held is whether the session holds the objects checked.
func assertionVerdict(v runtime.ObjectVerdict, label string, held bool, verification []runtime.VerdictKind) Verdict {
	out := Verdict{
		kind:         v.Kind,
		assertion:    v.Element,
		requirement:  v.Requirement,
		text:         v.Text,
		carrier:      v.Subject,
		path:         carrierPath(label, v.Path),
		status:       v.Status,
		verification: append([]runtime.VerdictKind(nil), verification...),
		held:         held,
	}
	if v.Subject != nil {
		out.carrierDecl = objectDeclaration(v.Subject)
	}
	if v.Err != nil {
		out.reason = v.Err.Error()
		var violation *runtime.ViolationError
		if errors.As(v.Err, &violation) {
			out.condition = violation.Condition
		}
	}
	if out.status == runtime.ValidationUndecided && out.reason == "" {
		out.reason = "the assertion could not be evaluated"
	}
	return out
}

// verificationVerdict converts a verification case's verdict to a row beside
// the verdict about the requirement it verifies.
func verificationVerdict(about Verdict, v runtime.VerificationVerdict) Verdict {
	out := Verdict{
		kind:         AssertionVerification,
		assertion:    v.Symbol,
		requirement:  about.requirement,
		text:         "verification " + v.Case,
		carrier:      about.carrier,
		carrierDecl:  about.carrierDecl,
		path:         about.path,
		reason:       v.Detail,
		verification: []runtime.VerdictKind{v.Kind},
		held:         about.held,
	}
	if v.Subcase {
		out.text += " (subcase)"
	}
	switch v.Kind {
	case runtime.VerdictPass:
		out.status = runtime.ValidationHolds
	case runtime.VerdictFail:
		out.status = runtime.ValidationViolated
		if out.reason == "" {
			out.reason = "the verification case answered fail"
		}
	default:
		out.status = runtime.ValidationUndecided
		if out.reason == "" {
			out.reason = "the verification case answered " + string(v.Kind)
		}
	}
	return out
}

// carrierPath labels a carrier by the row it was reached from and the features walked to it.
func carrierPath(label string, path []string) string {
	if len(path) == 0 {
		return label
	}
	return label + "." + strings.Join(path, ".")
}
