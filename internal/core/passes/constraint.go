package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ConstraintPass runs the depth-C semantic constraint checks over a document's
// symbol tree: specialization-cycle detection, multiplicity-range validity, and
// subsetting multiplicity conformance (design §4.1). It runs at LevelConstraint,
// after type checking; it relies on the shared semantic model for the resolved
// specialization graph and extracted multiplicities.
type ConstraintPass struct{}

// CodeConnectorEnds marks a connector whose ends do not fit the link it
// specializes: too many for a binary link, or redefining no end of it.
const CodeConnectorEnds = "connector-ends"

func (ConstraintPass) Level() PassLevel { return LevelConstraint }

func (ConstraintPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	cc := &constraintChecker{
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
		seen:     make(map[*symbols.Symbol]bool),
	}
	cc.walk(rootScope)
	return cc.diags
}

type constraintChecker struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	seen     map[*symbols.Symbol]bool
	diags    []Diagnostic
}

// libraryDeclared reports whether sym is declared by bundled library content,
// which states the metamodel frame a model specializes rather than the model.
func (cc *constraintChecker) libraryDeclared(sym *symbols.Symbol) bool {
	if cc.resolver == nil {
		return false
	}
	idx := cc.resolver.Index()
	return idx != nil && idx.Library(sym)
}

// walk visits every symbol in the scope subtree, deduping by pointer (a decl
// with short+primary names registers the same *Symbol under two keys), and
// recurses into each symbol's owned child scope.
func (cc *constraintChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym == nil || cc.seen[sym] {
			return true
		}
		cc.seen[sym] = true
		cc.check(sym)
		cc.walk(sym.Scope)
		return true
	})
}

// check runs the per-symbol constraint rules.
func (cc *constraintChecker) check(sym *symbols.Symbol) {
	cc.checkSpecializationCycle(sym)
	cc.checkMultiplicityRange(sym)
	cc.checkMultiplicityConformance(sym)
	cc.checkConnectorEnds(sym)
	if !cc.checkBinaryConnectorEnds(sym) {
		cc.checkConnectorEndRedefinition(sym)
	}
	cc.checkFlowEndSubsetting(sym)
	cc.checkInterfaceEndConjugation(sym)
	cc.checkRedefinition(sym)
	cc.checkW10BRedefinition(sym)
	cc.checkW10BCrossFeatures(sym)
	cc.checkSubsettingFeaturingTypes(sym)
	cc.checkUnnamedRedefinitionValue(sym)
	cc.checkFeatureValueOverriding(sym)
	cc.checkViewSatisfyTarget(sym)
	cc.checkAtMostOneMember(sym)
	cc.checkReturnParameterOwner(sym)
	cc.checkAtMostOneConjugator(sym)
	cc.checkFeatureEndFeatureMultiplicity(sym)
}

// checkFlowEndSubsetting requires each declared flow end to name a payload
// feature of its participant with dot notation.
func (cc *constraintChecker) checkFlowEndSubsetting(sym *symbols.Symbol) {
	for _, attachment := range cc.model.FlowEndAttachments(sym) {
		if attachment.Attachment == nil {
			continue
		}
		if _, ok := cc.resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment); !ok {
			continue
		}
		target := attachment.Attachment
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || qn == nil || len(qn.Parts) != 1 {
			continue
		}
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     attachment.Attachment.Span(),
			Message:  "a flow end must name the feature the payload flows from or to using dot notation",
			Code:     "flow-end-subsetting",
			Source:   "constraint",
		})
	}
}

// checkViewSatisfyTarget flags a `satisfy` claiming a view's conformance to a
// requirement that is no viewpoint (SysML v2 §8.3.20): only a viewpoint frames
// concerns, so such a claim is one nothing can evaluate. A satisfy stating a
// subject asserts its requirement of that subject, not conformance, and stands.
func (cc *constraintChecker) checkViewSatisfyTarget(sym *symbols.Symbol) {
	if sym.OwnerScope == nil || !semantics.IsViewpointSatisfy(sym) {
		return
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !semantics.IsView(owner) {
		return
	}
	target, ref := cc.model.SatisfyTarget(sym)
	if target == nil || semantics.IsViewpoint(target) {
		return
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     sym.DeclSpan,
		Message: fmt.Sprintf(
			"satisfy in a view body must name a viewpoint: %s is a %s, which frames no concern for the view to conform to",
			ref, target.Kind.String()),
		Code:   "view-satisfy-viewpoint",
		Source: "constraint",
	})
}

// checkSpecializationCycle flags a symbol that participates in a specialization
// cycle. The diagnostic is anchored at the first generalization edge that leads
// back to sym so the error points at the offending clause.
func (cc *constraintChecker) checkSpecializationCycle(sym *symbols.Symbol) {
	selfSpan, selfLoop := cc.selfSpecialization(sym)
	if !cc.model.HasSpecializationCycle(sym) && !selfLoop {
		return
	}
	span := sym.DeclSpan
	if selfLoop {
		span = selfSpan
	} else {
		for _, rel := range semantics.RelationshipsOf(sym) {
			if rel == nil || rel.Target == nil || !semantics.GeneralizationKind(rel.Kind) {
				continue
			}
			span = rel.Target.Span()
			break
		}
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf("%s participates in a specialization cycle", sym.Name),
		Code:     "specialization-cycle",
		Source:   "constraint",
	})
}

// selfSpecialization reports a `part p :> p` edge, which the specialization
// graph drops: a same-named subsetting or redefinition with no inherited feature
// to retarget resolves back to sym itself.
func (cc *constraintChecker) selfSpecialization(sym *symbols.Symbol) (source.Span, bool) {
	if sym == nil || sym.OwnerScope == nil {
		return source.Span{}, false
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil ||
			(rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok || len(qn.Parts) != 1 || qn.Parts[0].Text != sym.Name {
			continue
		}
		if cc.inheritsFeatureNamed(sym, qn.Parts[0].Text) {
			continue // the name denotes the inherited feature, not sym
		}
		if target, ok := cc.resolver.ResolveQualified(sym.OwnerScope, qn); ok && target == sym {
			return rel.Target.Span(), true
		}
	}
	return source.Span{}, false
}

// inheritsFeatureNamed reports whether sym's owner inherits a feature named
// name from a supertype, skipping sym itself.
func (cc *constraintChecker) inheritsFeatureNamed(sym *symbols.Symbol, name string) bool {
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	for _, sup := range cc.model.AllSupertypes(owner) {
		if found, ok := cc.model.LookupMember(sup, name); ok && found != sym {
			return true
		}
	}
	return false
}

// checkMultiplicityRange flags a usage whose evaluable multiplicity has a lower
// bound greater than its upper bound. Non-evaluable bounds are skipped.
func (cc *constraintChecker) checkMultiplicityRange(sym *symbols.Symbol) {
	rng, ok := cc.model.MultiplicityOf(sym)
	if !ok {
		return
	}
	valid, evaluable := rng.LowerLeUpper()
	if !evaluable || valid {
		return
	}
	span := sym.DeclSpan
	if mult := semantics.UsageMultiplicityOf(sym); mult != nil {
		span = mult.Span()
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf("multiplicity lower bound exceeds upper bound on %s", sym.Name),
		Code:     "multiplicity-range",
		Source:   "constraint",
	})
}

// checkConnectorEnds validates the declared ends of connector-like usages
// (design §4.3/§4.6), using only the parsed end lists (no stdlib type model):
//
//   - a connection must declare at least two ends (adapts the pilot's
//     INVALID_CONNECTOR_RELATED_FEATURES rule);
//   - an interface or allocation is binary — exactly two ends when any are
//     declared (adapts the binary-specialization rules).
//
// Usages with no declared ends are treated as abstract and skipped. Flow ends
// are not checked: they are optional (SysML v2 §8.2.2.16) and must be absent for
// a message (§8.4.12.2), and a half-declared pair is a parse error.
func (cc *constraintChecker) checkConnectorEnds(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return
	}
	switch u.Kind {
	case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation:
		n := len(u.ConnectorEnds)
		if n == 0 {
			return // no connector clause: abstract connector
		}
		switch u.Kind {
		case ast.UsageConnection:
			if n < 2 {
				cc.addConnectorEndsDiag(sym, u, "a connection must have at least two ends")
			}
		case ast.UsageInterface:
			if n < 2 && cc.interfaceIsBinary(sym) {
				cc.addConnectorEndsDiag(sym, u, "an interface connection must be binary (exactly two ends)")
			}
		case ast.UsageAllocation:
			if n < 2 {
				cc.addConnectorEndsDiag(sym, u, "an allocation must be binary (exactly two ends)")
			}
		}
	}
}

// checkBinaryConnectorEnds flags a connector or association that specializes a
// binary link yet has more than two ends, counting the ends of its `connect`
// clause, its `end` features and those it inherits (KerML 1.1 §8.3.3.5,
// §8.3.4.7). Each end past the second is reported; the declaration is when only
// inherited ends take the count past two. It reports whether it found any.
func (cc *constraintChecker) checkBinaryConnectorEnds(sym *symbols.Symbol) bool {
	excess, total := cc.model.BinaryConnectorExcessEnds(sym)
	if len(excess) == 0 {
		return false
	}
	name := sym.Name
	if name == "" {
		name = "this " + connectorKindName(sym)
	}
	for _, node := range excess {
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     node.Span(),
			Message: fmt.Sprintf("%s has %d ends but specializes a binary link (%s), which cannot have more than two; "+
				"drop the extra ends or specialize an n-ary link instead", name, total, semantics.BinaryConnectorBaseFQN),
			Code:   CodeConnectorEnds,
			Source: "constraint",
		})
	}
	return true
}

// connectorKindName names the kind of connector sym declares for a diagnostic.
func connectorKindName(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind.String() + " definition"
	case *ast.Usage:
		return d.Kind.String()
	}
	return "connector"
}

func (cc *constraintChecker) interfaceIsBinary(sym *symbols.Symbol) bool {
	return cc.model != nil && cc.model.IsBinaryConnector(sym)
}

// checkConnectorEndRedefinition flags an end a connector declares that redefines
// no end of the connector it specializes. Every end of a typed connector
// redefines the end at its own position of that type (SysML v2 §7.13.2), so an
// end beyond the last position of the type refines nothing.
func (cc *constraintChecker) checkConnectorEndRedefinition(sym *symbols.Symbol) {
	general, unmatched := cc.model.UnmatchedConnectorEnds(sym)
	if general == nil {
		return
	}
	declared := cc.model.ConnectorEndCount(general)
	for _, end := range unmatched {
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     end.DeclSpan,
			Message: fmt.Sprintf("end %s redefines no end of %s, which declares %d end(s)",
				end.Name, general.Name, declared),
			Code:   CodeConnectorEnds,
			Source: "constraint",
		})
	}
}

// checkInterfaceEndConjugation warns when the two ends of an interface are
// typed by ports whose features do not match with conjugate directions
// (SysML v2 §7.12.2): what one end sends the other cannot receive. It is a
// warning because an end may be typed through library ports this pass cannot
// see in full.
func (cc *constraintChecker) checkInterfaceEndConjugation(sym *symbols.Symbol) {
	first, second, mismatch := cc.model.InterfaceEndPortMismatch(sym)
	if !mismatch {
		return
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     sym.DeclSpan,
		Message: fmt.Sprintf(
			"interface %s connects ports %s and %s, whose directed features are not conjugate; one end usually names the conjugate port (~%s)",
			sym.Name, first.Name, second.Name, first.Name),
		Code:   "port-conjugation",
		Source: "constraint",
	})
}

// addConnectorEndsDiag records a connector-ends diagnostic anchored at the first
// offending end (the third end when there are too many, otherwise the first
// declared end), falling back to the declaration span.
func (cc *constraintChecker) addConnectorEndsDiag(sym *symbols.Symbol, u *ast.Usage, msg string) {
	span := sym.DeclSpan
	switch {
	case len(u.ConnectorEnds) > 2:
		span = u.ConnectorEnds[2].Span()
	case len(u.ConnectorEnds) >= 1:
		span = u.ConnectorEnds[0].Span()
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     CodeConnectorEnds,
		Source:   "constraint",
	})
}

// checkUnnamedRedefinitionValue warns about a value on a member that redefines
// more than one feature: it takes the first one's name, or none when it declares
// a short name (KerML 7.3.4.5), so the value is unreachable by the other names.
func (cc *constraintChecker) checkUnnamedRedefinitionValue(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil || u.Ident.Name != "" {
		return
	}
	var targets []string
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if name, _ := ast.TargetName(rel.Target); name != "" {
			targets = append(targets, name)
		}
	}
	if len(targets) < 2 {
		return
	}
	if naming := ast.NamingFeature(u); naming != nil && naming.Kind != ast.RelRedefines {
		return
	}
	var message string
	if u.Ident.ShortName != "" {
		message = fmt.Sprintf(
			"a member redefining %s derives no name, so this value is bound to the short name <%s> only; declare a name or redefine one feature",
			strings.Join(targets, " and "), u.Ident.ShortName)
	} else {
		message = fmt.Sprintf(
			"a member redefining %s takes the name %s only, so this value is not reachable by name as %s; declare a name or redefine one feature",
			strings.Join(targets, " and "), targets[0], strings.Join(targets[1:], " or "))
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     u.Value.Span(),
		Message:  message,
		Code:     "redefinition-no-derived-name",
		Source:   "constraint",
	})
}

// checkRedefinition flags a usage that redefines a member without proper
// inheritance, type conformance, or multiplicity bounds. SysML constraints:
// (1) redefined member must be inherited, (2) redefining usage must have a type
// that conforms to the redefined usage's type, (3) multiplicity bounds must be
// compatible (lower >= redefined.lower, upper <= redefined.upper).
func (cc *constraintChecker) checkRedefinition(sym *symbols.Symbol) {
	rels := semantics.RelationshipsOf(sym)
	if len(rels) == 0 {
		return // No relationships
	}

	// sym is declared in the scope its owning definition or usage owns; a scope
	// with no owner is the root or internal, and redefines nothing.
	var owner *symbols.Symbol
	if sym.OwnerScope != nil {
		owner = sym.OwnerScope.Owner()
	}
	if owner == nil {
		return
	}

	// Extract all redefines relationships
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.Kind != ast.RelRedefines {
			continue
		}
		if rel.Target == nil {
			continue
		}

		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		featuringOwners, hasFeaturing := cc.featuringOwners(sym)
		if !hasFeaturing {
			if owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
				continue
			}
			featuringOwners = []*symbols.Symbol{owner}
		}
		var redefined *symbols.Symbol
		inherited := false
		for _, featuringOwner := range featuringOwners {
			candidate, resolved := cc.resolveInheritedMember(featuringOwner, qn)
			if !resolved || candidate == nil {
				continue
			}
			if isPackageLevelFeature(candidate) {
				continue
			}
			redefined = candidate
			inherited = cc.isInheritedMember(featuringOwner, candidate, qn.Parts[len(qn.Parts)-1].Text)
			if inherited {
				break
			}
		}
		if redefined == nil {
			continue
		}

		// A target that is not an inherited member may still be reachable
		// through the featuring context (KerML validateRedefinitionFeaturingTypes).
		if !inherited {
			inherited = cc.redefinedAccessible(sym, redefined, map[*symbols.Symbol]bool{})
		}

		if !inherited {
			cc.diags = append(cc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message: fmt.Sprintf(
					"%s redefines %s, but %s is not an inherited member of %s",
					sym.Name, redefined.Name, redefined.Name, owner.Name),
				Code:   "redefinition-no-inherited",
				Source: "constraint",
			})
			continue
		}

		// A redefining feature's declared type is added to the redefined one's,
		// not checked against it (KerML 8.3.3.3.4), so an unrelated type is advisory.
		usageType := extractUsageType(cc, sym)
		redefinedType := extractUsageType(cc, redefined)

		if usageType != nil && redefinedType != nil {
			if !cc.model.Conforms(usageType, redefinedType) {
				cc.diags = append(cc.diags, Diagnostic{
					Severity: SeverityWarning,
					Span:     rel.Target.Span(),
					Message: fmt.Sprintf(
						"%s (typed by %s) redefines %s (typed by %s): types do not conform",
						sym.Name, usageType.Name, redefined.Name, redefinedType.Name),
					Code:   "redefinition-type-mismatch",
					Source: "constraint",
				})
			}
		}
	}
}

func (cc *constraintChecker) featuringOwners(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	owners := make([]*symbols.Symbol, 0, 1)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelFeaturedBy || rel.Target == nil {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok {
			continue
		}
		resolveScope := sym.OwnerScope
		if sym.Scope != nil {
			resolveScope = sym.Scope
		}
		target, ok := cc.resolver.ResolveQualified(resolveScope, qn)
		if !ok || target == nil {
			continue
		}
		owners = append(owners, target)
	}
	return owners, len(owners) > 0
}

// redefinedAccessible reports whether redefined is reachable from sym's
// featuring contexts, walking outward through feature-valued contexts
// (KerML 1.0 §8.3.3.3 validateRedefinitionFeaturingTypes).
func (cc *constraintChecker) redefinedAccessible(sym, redefined *symbols.Symbol, visited map[*symbols.Symbol]bool) bool {
	if sym == nil || visited[sym] {
		return false
	}
	visited[sym] = true
	for _, t := range cc.featuringContexts(sym) {
		if cc.featuredWithin(redefined, t) {
			return true
		}
		if isUsageKind(t.Kind) && cc.redefinedAccessible(t, redefined, visited) {
			return true
		}
	}
	return false
}

// featuringContexts returns sym's explicit featured-by targets, or its owning
// type when it declares none.
func (cc *constraintChecker) featuringContexts(sym *symbols.Symbol) []*symbols.Symbol {
	if owners, ok := cc.featuringOwners(sym); ok {
		return owners
	}
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return nil
	}
	return []*symbols.Symbol{owner}
}

// featuredWithin reports whether feature b is featured within type t: every
// featuring context of b accepts t (KerML 1.0 §8.3.3.3, Feature::isFeaturedWithin).
func (cc *constraintChecker) featuredWithin(b, t *symbols.Symbol) bool {
	ctxs := cc.featuringContexts(b)
	if len(ctxs) == 0 {
		return false
	}
	for _, f := range ctxs {
		if !cc.featuringContextConforms(t, f) {
			return false
		}
	}
	return true
}

// featuringContextConforms reports whether featuring context t conforms to f:
// by specialization, or because both redefine a common feature and t's own
// contexts conform to f's (the variable-feature snapshot encoding).
func (cc *constraintChecker) featuringContextConforms(t, f *symbols.Symbol) bool {
	if t == nil || f == nil {
		return false
	}
	if t == f || cc.model.Conforms(t, f) {
		return true
	}
	if !cc.shareRedefinedTarget(t, f) {
		return false
	}
	fCtxs := cc.featuringContexts(f)
	for _, ct := range cc.featuringContexts(t) {
		for _, cf := range fCtxs {
			if ct == cf || cc.model.Conforms(ct, cf) {
				return true
			}
		}
	}
	return false
}

// shareRedefinedTarget reports whether t and f redefine a common feature,
// directly or transitively.
func (cc *constraintChecker) shareRedefinedTarget(t, f *symbols.Symbol) bool {
	tTargets := make(map[*symbols.Symbol]bool)
	cc.collectRedefined(t, tTargets)
	if len(tTargets) == 0 {
		return false
	}
	fTargets := make(map[*symbols.Symbol]bool)
	cc.collectRedefined(f, fTargets)
	for g := range fTargets {
		if tTargets[g] {
			return true
		}
	}
	return false
}

// collectRedefined resolves sym's redefinition targets into `into`, transitively.
func (cc *constraintChecker) collectRedefined(sym *symbols.Symbol, into map[*symbols.Symbol]bool) {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		target := cc.model.RelationshipTarget(sym, rel)
		if target == nil || into[target] {
			continue
		}
		into[target] = true
		cc.collectRedefined(target, into)
	}
}

func (cc *constraintChecker) isInheritedMember(
	owner, candidate *symbols.Symbol,
	name string,
) bool {
	if owner == nil || candidate == nil {
		return false
	}

	for _, supertype := range cc.model.AllSupertypes(owner) {
		if found, ok := cc.model.LookupMember(supertype, name); ok && found == candidate {
			return true
		}
		for scope := supertype.OwnerScope; scope != nil; scope = scope.Parent() {
			if scope.Owner() == candidate {
				return true
			}
		}
	}

	return false
}

func isPackageLevelFeature(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return false
	}
	switch sym.OwnerScope.Owner().Kind {
	case symbols.SymbolPackage, symbols.SymbolNamespace:
	default:
		return false
	}
	// A package-level feature has no featuring type to inherit its redefined member.
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelFeaturedBy && rel.Target != nil {
			return false
		}
	}
	return true
}

// extractUsageType extracts the type of a usage via its RelTyping relationship.
// Returns nil if no explicit type is found.
func extractUsageType(cc *constraintChecker, sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelTyping {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		if qn, ok := targetNode.(*ast.QualifiedName); ok {
			resolved, ok := cc.resolver.ResolveQualified(sym.OwnerScope, qn)
			if ok && resolved != nil {
				if canonical, aliasOK := cc.resolver.ResolveAliasTarget(resolved); aliasOK {
					resolved = canonical
				} else {
					continue
				}
				return resolved
			}
		}
	}
	return nil
}

// resolveInheritedMember resolves a qualified name from owner's inherited scopes only,
// excluding locally declared members. Used for redefines validation where target
// must be inherited, not local.
func (cc *constraintChecker) resolveInheritedMember(owner *symbols.Symbol, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	// For single-part names, search inherited scopes directly
	if len(qn.Parts) == 1 {
		name := qn.Parts[0].Text
		// Search all supertypes for the member
		for _, supertype := range cc.model.AllSupertypes(owner) {
			if supertype.Scope != nil {
				if members := supertype.Scope.LookupLocalAll(name); len(members) > 0 {
					return members[0], true
				}
			}
		}
		return nil, false
	}

	// For multi-part names, resolve normally (qualifiers won't be local members)
	return cc.resolver.ResolveQualified(owner.Scope, qn)
}
