package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// VerificationsOf returns the verification cases declared in scope, and in the
// scopes nested within it, whose objective verifies req (SysML v2 §8.3.24), in
// declaration order. A nil requirement matches every verification case.
func (ctx *Context) VerificationsOf(scope *symbols.Scope, req *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, sym := range ctx.model.verificationCasesIn(scope) {
		if seen[sym] {
			continue
		}
		if req == nil || verifies(ctx.VerifiedRequirements(sym), req) {
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}

// verificationCasesIn returns the verification cases declared in scope and the
// scopes nested within it, in declaration order, memoized per scope.
func (m *Model) verificationCasesIn(scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	if m == nil {
		var out []*symbols.Symbol
		collectVerificationCases(scope, &out)
		return out
	}
	if cases, done := m.verificationCases[scope]; done {
		return cases
	}
	var out []*symbols.Symbol
	collectVerificationCases(scope, &out)
	m.verificationCases[scope] = out
	return out
}

func collectVerificationCases(scope *symbols.Scope, out *[]*symbols.Symbol) {
	for _, sym := range scopeMemberSymbols(scope) {
		if IsVerificationCaseSymbol(sym) {
			*out = append(*out, sym)
		}
	}
	for _, child := range scope.Children() {
		collectVerificationCases(child, out)
	}
}

// verifies reports whether req is among the requirements verified.
func verifies(verified []*symbols.Symbol, req *symbols.Symbol) bool {
	for _, sym := range verified {
		if sym == req {
			return true
		}
	}
	return false
}

// VerifiedRequirements returns the requirements the objectives of a verification
// case verify, in declaration order. The objectives are the ones the case states
// as it runs them, so an objective a case redeclares verifies what the
// redeclaration says and not also what the inherited one said.
func (ctx *Context) VerifiedRequirements(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, objective := range ctx.ObjectivesOf(sym, nil) {
		for _, req := range ctx.verifiedIn(objective.Symbol) {
			if !seen[req] {
				seen[req] = true
				out = append(out, req)
			}
		}
	}
	return out
}

// verifiedIn resolves the requirements an objective's `verify r;` members name:
// the requirement a reference form names, and the definition a declaration form
// (`verify requirement : R;`) types its own requirement by.
func (ctx *Context) verifiedIn(objective *symbols.Symbol) []*symbols.Symbol {
	if objective == nil || objective.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range scopeMemberSymbols(objective.Scope) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !usage.IsVerifiedRequirement() {
			continue
		}
		if usage.DeclaresRequirement {
			if types := ctx.model.semantics.FeatureTypes(member); len(types) > 0 {
				out = append(out, types...)
			} else {
				out = append(out, member)
			}
			continue
		}
		for _, rel := range usage.Relationships {
			if rel == nil || rel.Kind != ast.RelSubsets || rel.Target == nil {
				continue
			}
			if target, ok := ctx.resolveTarget(member.OwnerScope, rel.Target); ok && target != nil {
				out = append(out, target)
			}
		}
	}
	return out
}

// VerificationVerdictsIn runs the verification cases of every scope whose
// objective verifies req, each once, and reports its body verdict and the verdict
// of every subcase it performs. A nested case answers only as a subcase.
func (ctx *Context) VerificationVerdictsIn(scopes []*symbols.Scope, req *symbols.Symbol) []VerificationVerdict {
	var out []VerificationVerdict
	ran := make(map[*symbols.Symbol]bool)
	for _, scope := range scopes {
		for _, sym := range ctx.VerificationsOf(scope, req) {
			if ran[sym] || !isVerificationUsage(sym) || nestedInCase(sym) {
				continue
			}
			ran[sym] = true
			out = append(out, ctx.VerificationVerdicts(sym)...)
		}
	}
	return out
}

// VerificationVerdicts runs one verification case and reports its body verdict
// followed by the verdict of each subcase it performs.
func (ctx *Context) VerificationVerdicts(sym *symbols.Symbol) []VerificationVerdict {
	scope := sym.OwnerScope
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, scope, nil)
	if err != nil {
		return []VerificationVerdict{{
			Case: ctx.qualifiedSymbolName(sym), Symbol: sym, Kind: VerdictError, Detail: err.Error(),
		}}
	}
	return append([]VerificationVerdict{result.Verdict}, result.Subcases...)
}

// isVerificationUsage reports whether sym declares a verification case usage,
// the form that carries a run: a definition is a type, invoked by a usage of it.
func isVerificationUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		return usage.Kind == ast.UsageVerificationCase
	}
	return sym.Decl == nil && sym.Kind == symbols.SymbolVerificationCaseUsage
}

// nestedInCase reports whether a case is declared inside another case, whose run
// performs it as a step.
func nestedInCase(sym *symbols.Symbol) bool {
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil && IsRunnableCaseSymbol(owner) {
			return true
		}
	}
	return false
}

// VerifyingCase returns the verification case whose objective states a
// `verify requirement` assertion, and nil for an assertion stated elsewhere:
// a `satisfy`, or a `verify` outside an objective.
func (ctx *Context) VerifyingCase(a *SatisfyAssertion) *symbols.Symbol {
	if a == nil || a.Symbol == nil {
		return nil
	}
	usage, ok := a.Symbol.Decl.(*ast.Usage)
	if !ok || !usage.IsVerifiedRequirement() || a.Symbol.OwnerScope == nil {
		return nil
	}
	objective := a.Symbol.OwnerScope.Owner()
	if objective == nil || objective.OwnerScope == nil {
		return nil
	}
	if owner, ok := objective.Decl.(*ast.Usage); !ok || owner.Kind != ast.UsageObjective {
		return nil
	}
	if owner := objective.OwnerScope.Owner(); IsVerificationCaseSymbol(owner) {
		return owner
	}
	return nil
}
