package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
}

// withVerifications appends to a verdict the body verdict of every verification
// case in the session that verifies req, and of every subcase those cases
// perform. A requirement no case verifies adds nothing.
func (s *Session) withVerifications(v Verdict, ctx *runtime.Context, req *symbols.Symbol) Verdict {
	if ctx == nil || req == nil {
		return v
	}
	for _, scope := range s.docScopes() {
		for _, verdict := range ctx.VerificationVerdictsFor(scope, req) {
			v.Verifications = append(v.Verifications, VerificationVerdict{
				Case: verdict.Case, Kind: string(verdict.Kind), Detail: verdict.Detail,
				Subcase: verdict.Subcase,
			})
			v.Lines = append(v.Lines, verificationLine(verdict))
		}
	}
	return v
}

// verificationLine reports one body verdict the way the prompt reports every
// other one: a mark, what ran, and the verdict it answered.
func verificationLine(v runtime.VerificationVerdict) string {
	line := fmt.Sprintf("%s Verification %s verdict: %s", verdictMark(v.Kind), v.Case, v.Kind)
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
