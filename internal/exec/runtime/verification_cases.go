package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// verificationsOf returns the verification cases declared in scope, and in the
// scopes nested within it, whose objective verifies req (SysML v2 §8.3.24), in
// declaration order. A nil requirement matches every verification case.
func (ctx *Context) verificationsOf(scope *symbols.Scope, req *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, sym := range ctx.model.verificationCasesIn(scope) {
		if seen[sym] {
			continue
		}
		if req == nil || verifies(ctx.verifiedRequirements(sym), req) {
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

// verifies reports whether req is among the requirements verified, whichever
// scope tree each symbol was reached through.
func verifies(verified []*symbols.Symbol, req *symbols.Symbol) bool {
	for _, sym := range verified {
		if symbols.SameElement(sym, req) {
			return true
		}
	}
	return false
}

// verifiedRequirements returns the requirements the objectives of a verification
// case verify, in declaration order. The objectives are the ones the case states
// as it runs them, so an objective a case redeclares verifies what the
// redeclaration says and not also what the inherited one said.
func (ctx *Context) verifiedRequirements(sym *symbols.Symbol) []*symbols.Symbol {
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

// VerificationVerdictsIn runs the verification cases of every scope whose objective verifies req
// and reports each body and subcase verdict; a case a type owns runs once per object of that type.
func (ctx *Context) VerificationVerdictsIn(scopes []*symbols.Scope, req *symbols.Symbol) []VerificationVerdict {
	return ctx.VerificationVerdictsOn(scopes, req, nil)
}

// VerificationVerdictsOn is VerificationVerdictsIn for a verdict about subject: a case a type owns
// runs once on each object of that type subject is or holds, every held one when subject is nil.
func (ctx *Context) VerificationVerdictsOn(scopes []*symbols.Scope, req *symbols.Symbol, subject *Instance) []VerificationVerdict {
	return ctx.VerificationVerdictsOnce(scopes, req, subject, nil)
}

// VerificationRuns records the runs a report has made, so a case verifying a
// requirement several checks are about runs once per object it is a feature of.
type VerificationRuns struct {
	done map[verificationRunKey]bool
}

type verificationRunKey struct {
	requirement, kase *symbols.Symbol
	self              *Instance
}

// VerificationVerdictsOnce is VerificationVerdictsOn skipping the runs recorded
// in runs, and recording the ones it makes; a nil runs records nothing.
func (ctx *Context) VerificationVerdictsOnce(scopes []*symbols.Scope, req *symbols.Symbol, subject *Instance, runs *VerificationRuns) []VerificationVerdict {
	var out []VerificationVerdict
	for _, result := range ctx.VerificationResultsOnce(scopes, req, subject, runs) {
		out = append(out, result.Verdicts()...)
	}
	return out
}

// VerificationResultsOn is VerificationVerdictsOn reporting each run whole: the body
// verdict with the objective and assertion verdicts of the same run and its subcases.
func (ctx *Context) VerificationResultsOn(scopes []*symbols.Scope, req *symbols.Symbol, subject *Instance) []VerificationResult {
	return ctx.VerificationResultsOnce(scopes, req, subject, nil)
}

// VerificationResultsOnce is VerificationResultsOn skipping the runs recorded in
// runs, and recording the ones it makes; a nil runs records nothing.
func (ctx *Context) VerificationResultsOnce(scopes []*symbols.Scope, req *symbols.Symbol, subject *Instance, runs *VerificationRuns) []VerificationResult {
	var out []VerificationResult
	ran := make(map[*symbols.Symbol]bool)
	for _, scope := range scopes {
		for _, sym := range ctx.verificationsOf(scope, req) {
			if ran[sym] || !isVerificationUsage(sym) || nestedInCase(sym) {
				continue
			}
			ran[sym] = true
			for _, self := range ctx.caseCarriers(sym, subject) {
				if runs != nil {
					key := verificationRunKey{requirement: req, kase: sym, self: self}
					if runs.done[key] {
						continue
					}
					if runs.done == nil {
						runs.done = make(map[verificationRunKey]bool)
					}
					runs.done[key] = true
				}
				out = append(out, ctx.verificationResultOn(sym, self))
			}
		}
	}
	return out
}

// verificationResultOn runs one verification case as a feature of self (nil for none) and
// reports the run; one that could not be made is an error verdict about self.
func (ctx *Context) verificationResultOn(sym *symbols.Symbol, self *Instance) VerificationResult {
	scope := sym.OwnerScope
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, scope, self)
	if err != nil {
		return VerificationResult{Verdict: VerificationVerdict{
			Case: ctx.QualifiedName(sym), Symbol: sym, Kind: verdictError, Detail: err.Error(), Subject: self,
		}}
	}
	if result.Verdict.Subject == nil {
		result.Verdict.Subject = self
	}
	return result
}

// caseCarriers are the objects a type-owned case runs on for a verdict about subject: subject when
// of that type, else each object of the type under it (none when it holds none), or under every
// root when subject is nil, where no carrier leaves the case to run on no object — a run that
// reports the case needs one (ErrNeedsObject) rather than a body reading nothing. The case is a
// feature of every object of the type, so objects a multiplicity repeats each get their run.
func (ctx *Context) caseCarriers(sym *symbols.Symbol, subject *Instance) []*Instance {
	owner := declaringType(sym)
	if owner == nil {
		return []*Instance{nil}
	}
	roots := []*Instance{subject}
	if subject == nil {
		roots = ctx.rootInstances()
	} else if ctx.modelConforms(subject.Type, owner) {
		return []*Instance{subject}
	}
	carriers := ctx.objectsUnder(roots, owner)
	if len(carriers) == 0 {
		if subject != nil {
			return nil
		}
		return []*Instance{nil}
	}
	out := make([]*Instance, len(carriers))
	for i, c := range carriers {
		out[i] = c.instance
	}
	return out
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
