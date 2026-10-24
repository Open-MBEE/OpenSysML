package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// VerificationVerdict is the verdict the body of a verification case answered,
// reported beside the requirement or satisfaction verdict of the requirement
// the case verifies. It does not decide that verdict: the specification leaves
// the evaluation of a verdict unspecified, so this is what running the body and
// the library's own PassIf calculation answer.
type VerificationVerdict struct {
	// Case is the qualified name of the verification case that ran.
	Case string
	// Kind is the VerdictKind literal the body produced.
	Kind string
	// Detail is why an error or inconclusive verdict decided nothing.
	Detail string
	// Subcase marks a case another case performed as a step of its body.
	Subcase bool
	// Object names the object the case ran as a feature of, by its usage and
	// id; empty for a package-level case.
	Object string
}

// withVerifications appends to a verdict about subject the body and subcase verdicts of every
// verification case verifying req, a type-owned case run on each object of it subject is or holds.
func (s *Session) withVerifications(v Verdict, ctx *runtime.Context, req *symbols.Symbol, subject *runtime.Instance) Verdict {
	if ctx == nil || req == nil {
		return v
	}
	for _, verdict := range ctx.VerificationVerdictsOn(s.docScopes(), req, subject) {
		v.Verifications = append(v.Verifications, verificationVerdictOf(ctx, verdict))
		v.Lines = append(v.Lines, verificationLine(ctx, verdict))
	}
	return v
}

// verificationVerdictOf reports one body verdict as data, naming the object the case ran on.
func verificationVerdictOf(ctx *runtime.Context, v runtime.VerificationVerdict) VerificationVerdict {
	return VerificationVerdict{
		Case: v.Case, Kind: string(v.Kind), Detail: v.Detail, Subcase: v.Subcase,
		Object: verificationObject(ctx, v),
	}
}

// verificationObject names the object a case ran as a feature of as objectText does, "" for none.
func verificationObject(ctx *runtime.Context, v runtime.VerificationVerdict) string {
	if v.Subject == nil {
		return ""
	}
	return objectText(ctx, runtime.Value{Kind: runtime.ValInstance, Instance: v.Subject.ID})
}

// verificationLine reports one body verdict the way the prompt reports every
// other one: a mark, what ran, the object it ran on, and the verdict it answered.
func verificationLine(ctx *runtime.Context, v runtime.VerificationVerdict) string {
	line := fmt.Sprintf("%s Verification %s", verdictMark(v.Kind), v.Case)
	if object := verificationObject(ctx, v); object != "" {
		line += " on " + object
	}
	line += " verdict: " + string(v.Kind)
	if v.Subcase {
		line += " (subcase)"
	}
	if v.Detail != "" {
		line += " — " + v.Detail
	}
	return line
}

// verificationStatus is what a body verdict reports on a surface that runs the
// case: a pass holds, a fail is the case answering false, and the other two
// decided nothing about the model.
func verificationStatus(kind runtime.VerdictKind) VerdictStatus {
	switch kind {
	case runtime.VerdictPass:
		return VerdictHolds
	case runtime.VerdictFail:
		return VerdictFails
	default:
		return VerdictUnresolved
	}
}

// verdictMark marks a verdict as the prompt marks a checked condition: a pass
// holds, a fail does not, and neither of the others decided anything.
func verdictMark(kind runtime.VerdictKind) string {
	switch kind {
	case runtime.VerdictPass:
		return "✓"
	case runtime.VerdictFail:
		return "✗"
	default:
		return "?"
	}
}
