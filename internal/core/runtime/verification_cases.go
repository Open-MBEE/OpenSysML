package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// VerificationsOf returns the verification cases declared in scope, and in the
// scopes nested within it, whose objective verifies req (SysML v2 §8.3.24), in
// declaration order. A nil requirement matches every verification case.
func (ctx *Context) VerificationsOf(scope *symbols.Scope, req *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	ctx.collectVerifications(scope, req, seen, &out)
	return out
}

func (ctx *Context) collectVerifications(scope *symbols.Scope, req *symbols.Symbol, seen map[*symbols.Symbol]bool, out *[]*symbols.Symbol) {
	if scope == nil {
		return
	}
	for _, sym := range scopeMemberSymbols(scope) {
		if !IsVerificationCaseSymbol(sym) || seen[sym] {
			continue
		}
		if req == nil || verifies(ctx.VerifiedRequirements(sym), req) {
			seen[sym] = true
			*out = append(*out, sym)
		}
	}
	for _, child := range scope.Children() {
		ctx.collectVerifications(child, req, seen, out)
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

// VerifiedRequirements returns the requirements the objective of a verification
// case verifies, its inherited objectives included, in declaration order.
func (ctx *Context) VerifiedRequirements(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, carrier := range append([]*symbols.Symbol{sym}, ctx.model.AllSupertypes(sym)...) {
		for _, objective := range objectiveSymbols(carrier) {
			for _, req := range ctx.verifiedIn(objective) {
				if !seen[req] {
					seen[req] = true
					out = append(out, req)
				}
			}
		}
	}
	return out
}

// objectiveSymbols are the objectives a case declares.
func objectiveSymbols(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range scopeMemberSymbols(sym.Scope) {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageObjective {
			out = append(out, member)
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
			if types := ctx.model.FeatureTypes(member); len(types) > 0 {
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
			if target, ok := ctx.resolver.ResolveTarget(member.OwnerScope, rel.Target); ok && target != nil {
				out = append(out, target)
			}
		}
	}
	return out
}

// VerificationVerdictsFor runs the verification cases in scope whose objective
// verifies req and reports the verdict of each body run, the verdict of every
// subcase it performs beside it. A case nested in another is not run on its own:
// its verdict is reported as the subcase of the run that performs it.
func (ctx *Context) VerificationVerdictsFor(scope *symbols.Scope, req *symbols.Symbol) []VerificationVerdict {
	var out []VerificationVerdict
	for _, sym := range ctx.VerificationsOf(scope, req) {
		if !isVerificationUsage(sym) || nestedInCase(sym) {
			continue
		}
		out = append(out, ctx.VerificationVerdicts(sym)...)
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
