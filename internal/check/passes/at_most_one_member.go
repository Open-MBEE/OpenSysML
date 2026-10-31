package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
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
	members := ownedFeatureMembers(sym.Decl)
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

	if firstInput := cc.firstInputParameter(sym); firstInput != nil && !isSubjectSymbol(firstInput) {
		cc.reportSubjectParameterPosition(sym, members)
	}
}

// firstInputParameter is the first subject or input parameter in featureMembership
// order, owned then inherited (KerML 8.3.3.1); an anonymous `subject;` counts too.
func (cc *constraintChecker) firstInputParameter(sym *symbols.Symbol) *symbols.Symbol {
	var first *symbols.Symbol
	scan := func(scope *symbols.Scope) {
		scope.ForEachMember(func(member *symbols.Symbol) bool {
			if isSubjectSymbol(member) || isInputParameterSymbol(member) {
				first = member
			}
			return first == nil
		})
	}
	scan(sym.Scope)
	for _, src := range cc.model.MemberSources(sym) {
		if first != nil {
			break
		}
		scan(src.Scope)
	}
	return first
}

// isSubjectSymbol reads the declaration of a loaded symbol and the recorded facts
// of one whose document is held as a record.
func isSubjectSymbol(s *symbols.Symbol) bool {
	if !s.Recorded() {
		return isSubjectDecl(s.Decl)
	}
	return s.Facts.Node == symbols.NodeSubject || (s.Facts.Node == symbols.NodeUsage && s.Facts.UsageKind == ast.UsageSubject)
}

// isInputParameterSymbol is isInputParameterDecl over a loaded or recorded symbol.
func isInputParameterSymbol(s *symbols.Symbol) bool {
	if !s.Recorded() {
		return isInputParameterDecl(s.Decl)
	}
	if s.Facts.Node != symbols.NodeUsage {
		return false
	}
	switch s.Facts.UsageKind {
	case ast.UsageActor, ast.UsageStakeholder:
		return true
	}
	return s.Facts.Direction == ast.DirIn || s.Facts.Direction == ast.DirInOut
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
		if _, ok := kit.UnwrapMembership(m).(*ast.MultiplicityDecl); ok {
			mults = append(mults, kit.UnwrapMembership(m))
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

// ownedFeatureMembers returns feature memberships in featureMembership order.
func ownedFeatureMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	if usage, ok := decl.(*ast.Usage); ok && usage != nil && usage.Kind == ast.UsageSatisfy {
		for _, relationship := range usage.Relationships {
			if relationship != nil && relationship.Kind == ast.RelSubject {
				members = append(members, relationship)
			}
		}
	}
	return append(members, typeMembers(decl)...)
}

func isSubjectDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return d.Kind == ast.UsageSubject
	case *ast.Relationship:
		return d != nil && d.Kind == ast.RelSubject
	}
	return false
}

// isInputParameterDecl reports an `in`/`inout` usage, an actor or a stakeholder:
// Actor/StakeholderMembership are ParameterMemberships directed `in` (SysML 8.3.19).
func isInputParameterDecl(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	if !ok {
		return false
	}
	switch u.Kind {
	case ast.UsageActor, ast.UsageStakeholder:
		return true
	}
	return u.Direction == ast.DirIn || u.Direction == ast.DirInOut
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
	if relationship, ok := n.(*ast.Relationship); ok {
		return relationship != nil && relationship.Kind == ast.RelSubject
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

// subjectOwnerDecl reports a requirement or case declaration; concerns and
// viewpoints are requirements (SysML 8.3.19 ConcernDefinition, ViewpointDefinition).
func subjectOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return requirementLikeDefKind(d.Kind) || isCaseDefKind(d.Kind)
	case *ast.Usage:
		return requirementLikeUsageKind(d.Kind) || isCaseUsageKind(d.Kind)
	}
	return false
}

func requirementLikeDefKind(k ast.DefinitionKind) bool {
	switch k {
	case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint:
		return true
	}
	return false
}

func requirementLikeUsageKind(k ast.UsageKind) bool {
	switch k {
	case ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern, ast.UsageFramedConcern, ast.UsageViewpoint:
		return true
	}
	return false
}

// objectiveOwnerDecl reports whether a case declaration is judged by the objective
// cardinality rule: every case kind, analysis cases included (SysML 8.3.22.2-3).
func objectiveOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return isCaseDefKind(d.Kind)
	case *ast.Usage:
		return isCaseUsageKind(d.Kind)
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
