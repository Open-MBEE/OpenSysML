package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot SysMLValidator (2026-05) checkAtMostOneFeature and checkSubjectParameter:
// a requirement or case owns at most one subject, and it is the first parameter.
// Cases also own at most one objective.
const (
	msgOnlyOneSubject   = "Only one subject is allowed."
	msgOnlyOneObjective = "Only one objective is allowed."

	// codeOnlyOneObjective classifies the diagnostic reporting a second objective.
	codeOnlyOneObjective = "only-one-objective"

	msgSubjectParameterPosition = "Subject must be first parameter."

	// KerML 8.3.3.1.1: Type::multiplicity is single-valued, so a type states at
	// most one multiplicity (pilot KerMLValidator checkTypeMultiplicity).
	msgOnlyOneMultiplicity = "Only one multiplicity is allowed"
)

func (cc *constraintChecker) checkAtMostOneMember(sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	members := typeMembers(sym.Decl)
	cc.checkAtMostOneMultiplicity(sym.Decl, members)
	if subjectOwnerDecl(sym.Decl) {
		var subjects []ast.Node
		for _, m := range members {
			if isSubjectMemberNode(m) {
				subjects = append(subjects, m)
			}
		}
		_, inherited := cc.model.SubjectsOf(sym)
		cc.checkAtMostOneRole(sym, subjects, inherited, msgOnlyOneSubject, "only-one-subject")
		cc.checkSubjectParameterPosition(sym, members)
		if objectiveOwnerDecl(sym.Decl) {
			var objectives []ast.Node
			for _, m := range members {
				if isObjectiveMemberNode(m) {
					objectives = append(objectives, m)
				}
			}
			_, inherited := cc.model.ObjectivesOf(sym)
			cc.checkAtMostOneRole(sym, objectives, inherited, msgOnlyOneObjective, codeOnlyOneObjective)
		}
	}
}

// checkAtMostOneRole places the excess like the pilot's checkAtMostOneRelationship:
// owned ones after the first, the declaration when all are inherited, every owned one for a mix.
func (cc *constraintChecker) checkAtMostOneRole(sym *symbols.Symbol, owned []ast.Node, inherited []*symbols.Symbol, msg, code string) {
	switch {
	case len(owned)+len(inherited) <= 1:
	case len(owned) == 0:
		cc.diags = append(cc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     sym.Decl.Span(),
			Message:  msg,
			Code:     code,
			Source:   "constraint",
		})
	case len(inherited) == 0:
		cc.reportExtraMembers(owned, msg, code)
	default:
		cc.reportEachMember(owned, msg, code)
	}
}

// checkSubjectParameterPosition requires the first parameter of a requirement or
// case to be its subject (pilot checkSubjectParameter). It is reported on the
// owned subject when there is one, otherwise on the declaration; a declaration
// with no parameter at all is left to the reference's implicit subject.
func (cc *constraintChecker) checkSubjectParameterPosition(sym *symbols.Symbol, members []ast.Node) {
	localParameter := false
	for _, member := range members {
		if isSubjectMemberNode(member) {
			return
		}
		if u, ok := unwrapUsageMember(member); ok {
			if isInputParameterDecl(u) {
				cc.reportSubjectParameterPosition(sym, members)
				return
			}
			if isParameterDecl(u) {
				localParameter = true
			}
		}
	}
	if localParameter {
		return
	}

	var firstInput *symbols.Symbol
	for _, member := range cc.model.MembersOf(sym) {
		if member == nil {
			continue
		}
		if isSubjectDecl(member.Decl) || isInputParameterDecl(member.Decl) {
			firstInput = member
			break
		}
	}
	if firstInput == nil || isSubjectDecl(firstInput.Decl) {
		return
	}
	cc.reportSubjectParameterPosition(sym, members)
}

func isParameterDecl(u *ast.Usage) bool {
	return u != nil && (u.Direction != ast.DirNone || u.IsResult)
}

func (cc *constraintChecker) reportSubjectParameterPosition(sym *symbols.Symbol, members []ast.Node) {
	span := sym.Decl.Span()
	for _, member := range members {
		if isSubjectMemberNode(member) {
			span = member.Span()
			break
		}
	}
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msgSubjectParameterPosition,
		Code:     "subject-parameter-position",
		Source:   "constraint",
	})
}

// checkAtMostOneMultiplicity reports every multiplicity member of a type after
// the first, the one a multiplicity written in the declaration itself already is
// (KerML 8.3.3.1.1, Type::multiplicity).
func (cc *constraintChecker) checkAtMostOneMultiplicity(decl ast.Node, members []ast.Node) {
	var mults []ast.Node
	if declMultiplicity(decl) != nil {
		mults = append(mults, decl)
	}
	for _, m := range members {
		if _, ok := unwrapMembership(m).(*ast.MultiplicityDecl); ok {
			mults = append(mults, unwrapMembership(m))
		}
	}
	cc.reportExtraMembers(mults, msgOnlyOneMultiplicity, "only-one-multiplicity")
}

// declMultiplicity is the multiplicity stated in a declaration itself.
func declMultiplicity(decl ast.Node) *ast.Multiplicity {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Multiplicity
	case *ast.Usage:
		return d.Multiplicity
	}
	return nil
}

// typeMembers is the body of a type declaration, in either language.
func typeMembers(decl ast.Node) []ast.Node {
	if members := ast.DeclMembers(decl); members != nil {
		return members
	}
	if ns, ok := decl.(*ast.Namespace); ok {
		return ns.Members
	}
	return nil
}

func isSubjectDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return d.Kind == ast.UsageSubject
	}
	return false
}

func isInputParameterDecl(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	return ok && (u.Direction == ast.DirIn || u.Direction == ast.DirInOut)
}

// reportExtraMembers errors on every member after the first, the reference's
// behavior when all the competing memberships are owned.
func (cc *constraintChecker) reportExtraMembers(members []ast.Node, msg, code string) {
	if len(members) > 1 {
		cc.reportEachMember(members[1:], msg, code)
	}
}

func (cc *constraintChecker) reportEachMember(members []ast.Node, msg, code string) {
	for _, m := range members {
		cc.diags = append(cc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     m.Span(),
			Message:  msg,
			Code:     code,
			Source:   "constraint",
		})
	}
}

// unwrapUsageMember returns the usage a body member declares, through the
// membership wrapper the parser keeps.
func unwrapUsageMember(n ast.Node) (*ast.Usage, bool) {
	if mem, ok := n.(*ast.Membership); ok && mem != nil {
		n = mem.Member
	}
	u, ok := n.(*ast.Usage)
	return u, ok && u != nil
}

func isSubjectMemberNode(n ast.Node) bool {
	if _, ok := n.(*ast.SubjectMember); ok {
		return true
	}
	u, ok := unwrapUsageMember(n)
	return ok && u.Kind == ast.UsageSubject
}

func isObjectiveMemberNode(n ast.Node) bool {
	u, ok := unwrapUsageMember(n)
	return ok && u.Kind == ast.UsageObjective
}

func stateLikeDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefState
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	return false
}

func subjectOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefRequirement || isCaseDefKind(d.Kind)
	case *ast.Usage:
		return d.Kind == ast.UsageRequirement || isCaseUsageKind(d.Kind)
	}
	return false
}

// objectiveOwnerDecl reports whether a case declaration is judged by the objective
// cardinality rule; analysis cases are exempt, see internal/core/solve.
func objectiveOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return isCaseDefKind(d.Kind) && d.Kind != ast.DefAnalysisCase
	case *ast.Usage:
		return isCaseUsageKind(d.Kind) && d.Kind != ast.UsageAnalysisCase
	}
	return false
}

func isCaseDefKind(k ast.DefinitionKind) bool {
	switch k {
	case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
		return true
	}
	return false
}

func isCaseUsageKind(k ast.UsageKind) bool {
	switch k {
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		return true
	}
	return false
}
