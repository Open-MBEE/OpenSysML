package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// RequirementVerifier runs the verification cases in scopes verifying a
// requirement: a Context, or a DeclaredReader's behavior-free one.
type RequirementVerifier interface {
	VerificationVerdictsIn(scopes []*symbols.Scope, req *symbols.Symbol) []VerificationVerdict
}

// RequirementVerdicts is the verdict overlay v answers over the cases in
// scopes: each case verifying a requirement, the subcases its body performs aside.
func RequirementVerdicts(v RequirementVerifier, scopes []*symbols.Scope) view.Verdicts {
	return func(req *symbols.Symbol) []view.Verdict {
		var out []view.Verdict
		for _, verdict := range v.VerificationVerdictsIn(scopes, req) {
			if !verdict.Subcase {
				out = append(out, view.Verdict{Case: verdict.Case, Kind: string(verdict.Kind), Detail: verdict.Detail})
			}
		}
		return out
	}
}
