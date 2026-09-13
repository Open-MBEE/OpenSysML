package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TypeCheckPass validates that each def/usage relationship target has a symbol
// kind compatible with the source node and relationship kind (spec §6.3), and
// types expressions (operands, bound values, invocation arguments) against the
// stdlib scalar lattice.
// It runs at LevelType, after name resolution; unresolved targets are skipped.
type TypeCheckPass struct{}

func (TypeCheckPass) Level() PassLevel { return LevelType }

func (TypeCheckPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	// Initialize model to enable inheritance-aware resolution
	model := ctx.Model()
	tc := &typeChecker{
		resolver: ctx.Resolver(),
		expr:     &exprChecker{resolver: ctx.Resolver(), model: model, lang: ctx.Kind},
		lang:     ctx.Kind,
	}
	tc.expr.walkMembers = tc.walk
	tc.walk(rootScope, root.Members)
	return append(tc.diags, tc.expr.diagnostics()...)
}

type typeChecker struct {
	resolver *resolve.Resolver
	expr     *exprChecker
	// lang is the document's language; a document of no known kind — the REPL
	// buffer — reads as SysML, the notation its prompt takes.
	lang  source.Kind
	diags []Diagnostic
	// sendPayloads are the payload bindings of send bodies, which the send-action
	// pass types so they are checked even when this pass is gated.
	sendPayloads map[*ast.Usage]bool
}

func (tc *typeChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		switch d := unwrapType(m).(type) {
		case *ast.Definition:
			tc.checkRelationships(scope, d.Relationships, declKind{
				lang: tc.lang, isDef: true, defKind: d.Kind, keyword: d.Keyword, span: d.Span(),
			})
			tc.expr.checkBoundOperators(scope, d.Multiplicity)
			tc.expr.checkRelationshipBounds(scope, d.Relationships)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Usage:
			tc.checkRelationships(scope, d.Relationships, declKind{
				lang:         tc.lang,
				useKind:      d.Kind,
				direction:    d.Direction,
				isReference:  d.IsReference,
				isEnd:        d.IsEnd,
				isIndividual: d.IsIndividual,
				portion:      d.Portion,
				keyword:      d.Keyword,
				hasType:      hasTypingRelationship(d.Relationships),
				span:         d.Span(),
			})
			tc.expr.checkUsageBounds(scope, d)
			if tc.sendPayloads[d] {
				tc.checkOneType(scope, usageDecl(d))
			} else {
				tc.checkFeatureDecl(scope, usageDecl(d))
			}
			if cross := d.CrossFeature; cross != nil {
				// A cross feature is a reference usage of its own (SysML.xtext
				// OwnedCrossFeatureMember), typed by any definition.
				tc.checkRelationships(scope, cross.Relationships, declKind{
					lang:        tc.lang,
					useKind:     ast.UsageAttribute,
					isReference: true,
					hasType:     hasTypingRelationship(cross.Relationships),
					span:        cross.Span(),
				})
			}
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.AssumeMember, *ast.RequireMember:
			tc.checkOwnedConstraint(scope, d)
			tc.checkBehaviorMember(scope, d)
		case *ast.MultiplicityDecl:
			tc.expr.checkBoundOperators(scope, d.Range)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.RelationshipMember:
			tc.checkRelationshipMember(scope, d)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Package:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Namespace:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		default:
			tc.checkBehaviorMember(scope, d)
		}
	}
}

// checkFeatureDecl checks what a feature declares beyond its relationships: a
// single type where the reference admits one, and a value it conforms to.
func (tc *typeChecker) checkFeatureDecl(scope *symbols.Scope, d featureDecl) {
	tc.checkOneType(scope, d)
	tc.expr.checkDeclValue(scope, d)
}

// checkOwnedConstraint checks the constraint usage an assume/require member
// declares as the usage it is; a reference or condition form declares none.
func (tc *typeChecker) checkOwnedConstraint(scope *symbols.Scope, m ast.Node) {
	d, ok := featureDeclOf(m)
	if !ok {
		return
	}
	tc.checkRelationships(scope, d.relationships, declKind{
		lang:    tc.lang,
		useKind: d.kind,
		keyword: d.keyword,
		hasType: d.hasTyping(),
		span:    d.span,
	})
	tc.checkFeatureDecl(scope, d)
}

// checkBehaviorMember types the expressions carried by behavior body members
// (calc results, constraints, guards, conditions, assignments) and descends
// every body a state, transition or action node declares, in the scope the
// symbol builder gave that body.
func (tc *typeChecker) checkBehaviorMember(scope *symbols.Scope, n ast.Node) {
	switch m := n.(type) {
	case *ast.ConstraintMember:
		// `assert [not] c;` in a body states a reference to a constraint usage
		// (SysML.xtext AssertConstraintUsage), not a condition to be Boolean.
		if m.Keyword == "assert" && w8cIsReference(m.Expression) {
			tc.checkTypeTarget(scope, m.Expression, ast.RelReferences,
				declKind{lang: tc.lang, useKind: ast.UsageConstraint, keyword: m.Keyword, span: m.Span()})
		} else {
			tc.expr.checkBoolean(scope, m.Expression, "constraint expression")
		}
		tc.walk(symbols.ConstraintBodyScope(scope, m), m.Body)
	case *ast.AssumeMember:
		tc.expr.checkBoundOperators(scope, m.Multiplicity)
		tc.expr.checkBoolean(scope, m.Expression, "assume expression")
		tc.walk(symbols.ConstraintBodyScope(scope, m), m.Body)
	case *ast.RequireMember:
		tc.expr.checkBoundOperators(scope, m.Multiplicity)
		tc.expr.checkBoolean(scope, m.Expression, "require expression")
		tc.walk(symbols.ConstraintBodyScope(scope, m), m.Body)
	case *ast.IfActionNode:
		// The condition is evaluated before either branch is entered, so it is
		// checked outside them; each branch's body is checked in its own scope.
		tc.expr.checkBoolean(scope, m.Condition, "condition of 'if'")
		for _, branch := range m.Branches() {
			tc.checkBehaviorMember(scope, branch)
		}
	case *ast.IfBranchNode:
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.walk(body, m.Body)
	case *ast.WhileLoopActionNode:
		// The condition may read the loop's own body members, which live in the
		// scope the loop owns; the collection a `for` loop iterates over is
		// evaluated before the loop is entered, so it is checked outside it.
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.expr.infer(scope, m.Collection)
		tc.expr.checkBoolean(body, m.Condition, "condition of '"+m.Kind.String()+"'")
		tc.expr.checkBoolean(body, m.Until, "condition of 'until'")
		tc.walk(body, m.Body)
	case *ast.TransitionMember:
		// The effect and body see the parameters the trigger declares; the
		// trigger and guard are the trigger-argument and guard passes' own.
		body := symbols.TriggerScope(scope, m)
		tc.walk(body, m.Effect)
		tc.walk(body, m.Members)
	case *ast.EntryMember:
		tc.walk(scope, m.Actions)
	case *ast.DoMember:
		tc.walk(scope, m.Actions)
	case *ast.ExitMember:
		tc.walk(scope, m.Actions)
	case *ast.StateNode:
		body := childScopeOr(scope, m)
		tc.walk(body, m.Entry)
		tc.walk(body, m.Do)
		tc.walk(body, m.Exit)
		tc.walk(body, m.Substates)
		for _, region := range m.Regions {
			tc.checkBehaviorMember(body, region)
		}
	case *ast.StateRegion:
		tc.walk(childScopeOr(scope, m), m.States)
	case *ast.SendStatement:
		if payload := lower.SendPayloadParameter(m); payload != nil {
			if tc.sendPayloads == nil {
				tc.sendPayloads = map[*ast.Usage]bool{}
			}
			tc.sendPayloads[payload] = true
		}
		tc.walk(childScopeOr(scope, m), m.Members)
	case *ast.SuccessionEdge:
		tc.walk(childScopeOr(scope, m), m.Members)
	case *ast.InitialNode, *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		tc.walk(childScopeOr(scope, m), ast.NodeBodyMembers(m))
	case *ast.AssignmentActionNode:
		tc.expr.checkAssignmentValue(scope, m)
	case *ast.ActionExecutionNode:
		tc.expr.infer(scope, m.Expression)
	case *ast.PerformActionNode:
		tc.expr.checkPerform(scope, m)
	case *ast.SubjectMember:
		tc.checkSubjectMember(scope, m)
	default:
		// A body member that is an expression is a value the body computes — a
		// calc body whose result is its last expression — and is typed as one.
		tc.expr.infer(scope, n)
	}
}

// checkSubjectMember types a `subject` declared through the requirement body
// path, which yields a SubjectMember rather than a Usage and so would otherwise
// escape the usage-kind rules.
func (tc *typeChecker) checkSubjectMember(scope *symbols.Scope, m *ast.SubjectMember) {
	if m.TypeRef != nil {
		tc.checkTypeTarget(scope, m.TypeRef, ast.RelTyping, declKind{lang: tc.lang, useKind: ast.UsageSubject, span: m.Span()})
	}
	tc.checkRelationships(scope, m.Relationships, declKind{lang: tc.lang, useKind: ast.UsageSubject, span: m.Span()})
	tc.expr.checkBoundOperators(scope, m.Multiplicity)
	if m.BindingExpr != nil {
		tc.expr.infer(scope, m.BindingExpr)
	}
	if len(m.Body) > 0 {
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.walk(body, m.Body)
	}
}

// declKind describes the declaration a relationship is declared on: what the
// kind-compatibility rules are checked against.
type declKind struct {
	// lang is the language of the document the declaration is written in: the
	// KerML type layer has no definition/usage split to check against.
	lang        source.Kind
	isDef       bool
	defKind     ast.DefinitionKind
	useKind     ast.UsageKind
	direction   ast.FeatureDirection
	isReference bool
	hasType     bool
	// isEnd marks a feature declared with the `end` modifier, whose type is that
	// of the feature it connects and so escapes the usage-kind taxonomy.
	isEnd bool
	// isIndividual and portion carry the `individual` modifier and the
	// `snapshot`/`timeslice` portion prefix (SysML v2 §8.3.9.11:
	// `OccurrenceUsage::isIndividual` and `OccurrenceUsage::portionKind`).
	// Either makes the declaration an occurrence usage, whatever kind keyword
	// declares it.
	isIndividual bool
	portion      ast.PortionKind
	// keyword is the kind keyword as written, which tells apart the spellings a
	// single DefinitionKind carries (`classifier` and `class` are both DefClass).
	keyword string
	// span is the declaration itself, where the reference reports a wrong typing
	// (see pilotTypingMessage).
	span source.Span
	// conjugated marks a `~T` typing, which the conjugation rule owns.
	conjugated bool
}

// isPlainClassifier reports whether the declaration is written with `classifier`
// (or `subclassifier`), a KerML Classifier rather than the narrower `class`.
func (d declKind) isPlainClassifier() bool {
	return d.isDef && d.defKind == ast.DefClass &&
		(d.keyword == "classifier" || d.keyword == "subclassifier")
}

// isReferenceUsage reports whether the usage was written with no kind keyword at
// all: a ReferenceUsage (SysML.xtext DefaultReferenceUsage, `x : T;`, `ref x :
// T;`) or a plain Usage (ExtendedUsage, `#meta x : T;`). Neither is a usage of
// the default kind, so the usage-kind taxonomy does not constrain its type.
func (d declKind) isReferenceUsage() bool {
	return !d.isDef && d.keyword == "" && d.useKind == ast.UsageAttribute
}

// isAssertedReference reports whether the usage is the reference form of an
// assert constraint usage, `assert [not] c;`, rather than a declaration.
func (d declKind) isAssertedReference() bool {
	return !d.isDef && d.useKind == ast.UsageConstraint && d.keyword == "assert"
}

// isKerML reports whether the declaration is written in KerML, which has no
// definition/usage distinction to classify it against (KerML 1.0 §8.3).
func (d declKind) isKerML() bool { return d.lang == source.KindKerML }

// isOccurrenceUsage reports whether the `individual` modifier or a portion
// prefix makes the declaration an occurrence usage.
func (d declKind) isOccurrenceUsage() bool {
	return d.isIndividual || d.portion != ast.PortionNone
}

// occurrenceModifier names the occurrence modifier the declaration carries, for
// diagnostics.
func (d declKind) occurrenceModifier() string {
	if d.isIndividual {
		return "individual"
	}
	return d.portion.Keyword()
}

func (tc *typeChecker) checkRelationships(scope *symbols.Scope, rels []*ast.Relationship, decl declKind) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		// Only a conjugated *typing* is a ConjugatedPortTyping (SysML.xtext:974); a
		// KerML Conjugation relates any two Types (KerML §7.4) and demands no port.
		conjugatedTyping := rel.Conjugated && rel.Kind == ast.RelTyping
		target := decl
		target.conjugated = conjugatedTyping
		tc.checkTypeTarget(scope, rel.Target, rel.Kind, target)
		if conjugatedTyping {
			tc.checkConjugatedTyping(scope, rel, decl)
		}
	}
}

// checkTypeTarget checks one relationship target against the kind rules for the
// declaration carrying it.
func (tc *typeChecker) checkTypeTarget(scope *symbols.Scope, target ast.Node, relKind ast.RelationshipKind, decl declKind) {
	// Unwrap FeatureReference if needed
	targetNode := target
	if fr, ok := targetNode.(*ast.FeatureReference); ok {
		targetNode = fr.Name
	}
	qn, isQN := targetNode.(*ast.QualifiedName)
	if !isQN {
		tc.checkChainSegments(scope, targetNode)
		tc.checkChainReferenceKind(scope, targetNode, relKind, decl)
		return
	}
	sym, ok := tc.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	// Resolve aliases to their underlying types for the relationships whose
	// check depends on the target's kind: a typing and a generalization.
	targetSym := sym
	aliasMatters := relKind == ast.RelSpecializes ||
		relKind == ast.RelSubsets ||
		relKind == ast.RelRedefines ||
		relKind == ast.RelTyping
	if aliasMatters && sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			targetSym = resolved
		}
	}
	if relKind == ast.RelRedefines || relKind == ast.RelSubsets {
		if tc.checkNearestDeclaredUsageTyping(targetSym, decl) {
			return
		}
	}
	if relKind == ast.RelTyping && decl.useKind == ast.UsageEnumeration &&
		tc.owningEnumerationConformsTo(scope, targetSym) {
		return
	}
	if relKind == ast.RelSpecializes && w11aFamilyRuleFires(decl, targetSym) {
		return // a supertype of the wrong classifier family is the family rules' finding
	}
	kind := targetSym.Kind
	if relKind == ast.RelReferences || relKind == ast.RelSubsets {
		kind = referentKind(targetSym)
	}
	msg := compatMessage(decl, relKind, kind)
	if (relKind == ast.RelReferences || relKind == ast.RelSubsets) &&
		kind == symbols.SymbolUnknown && targetSym.IsFeature() {
		msg = unclassifiedReferenceKindMessage(decl, relKind, targetSym)
	}
	if msg == "" {
		return
	}
	span, code := target.Span(), "type"
	// A wrong typing is the reference's per-kind message on the declaration.
	if relKind == ast.RelTyping {
		if pilot, pilotCode, ok := pilotTypingMessage(decl); ok {
			msg, span, code = pilot, decl.span, pilotCode
		}
	}
	tc.appendUnique(Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "type",
	})
}

// checkChainSegments reports a feature chain whose leading segments do not name
// features: a FeatureChaining chains Features (KerML 1.0 §8.3.4.7), so a chain
// cannot continue from a type declaration.
func (tc *typeChecker) checkChainSegments(scope *symbols.Scope, target ast.Node) {
	chain, ok := target.(*ast.FeatureChainExpr)
	if !ok {
		return
	}
	tc.checkChainSegments(scope, chain.Operand)
	sym, ok := tc.resolver.ResolveTarget(scope, chain.Operand)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	// An alias that resolves to nothing is the name-resolution tier's finding.
	if sym.Kind == symbols.SymbolAlias || endFeature.admits(sym.Kind) {
		return
	}
	tc.appendUnique(Diagnostic{
		Severity: SeverityError,
		Span:     chain.Operand.Span(),
		Message:  fmt.Sprintf("feature chain segment must be a feature, found %s", sym.Kind),
		Code:     "type",
		Source:   "type",
	})
}

// checkChainReferenceKind applies the referent-kind rule of a reference form to
// a feature chain, whose referent is its last chaining feature (KerML §8.3.4.7).
func (tc *typeChecker) checkChainReferenceKind(scope *symbols.Scope, target ast.Node, relKind ast.RelationshipKind, decl declKind) {
	if _, ok := target.(*ast.FeatureChainExpr); !ok {
		return
	}
	sym, ok := tc.resolver.ResolveTarget(scope, target)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	msg := referenceKindMessage(decl, relKind, referentKind(sym))
	if sym.Kind == symbols.SymbolUnknown && sym.IsFeature() {
		msg = unclassifiedReferenceKindMessage(decl, relKind, sym)
	}
	if msg == "" {
		return
	}
	tc.appendUnique(Diagnostic{
		Severity: SeverityError,
		Span:     target.Span(),
		Message:  msg,
		Code:     "type",
		Source:   "type",
	})
}

func (tc *typeChecker) checkNearestDeclaredUsageTyping(target *symbols.Symbol, decl declKind) bool {
	if _, ok := target.Decl.(*ast.Usage); !ok ||
		decl.isDef || decl.isReference || decl.keyword == "" ||
		decl.keyword == "feature" || decl.hasType ||
		w11aInheritedTypingKinds[decl.useKind] {
		return false
	}
	msg, code, ok := pilotTypingMessage(decl)
	if !ok {
		return false
	}
	for _, typ := range nearestDeclaredUsageTypesOf(tc.resolver, target, make(map[*symbols.Symbol]bool)) {
		if !isDefKind(typ.sym.Kind) {
			continue
		}
		if compatibleTyping(decl.useKind, decl.direction, typ.sym.Kind) {
			continue
		}
		tc.appendUnique(Diagnostic{
			Severity: SeverityError,
			Span:     decl.span,
			Message:  msg,
			Code:     code,
			Source:   "type",
		})
		return true
	}
	return false
}

func nearestDeclaredUsageTypesOf(resolver *resolve.Resolver, sym *symbols.Symbol, visited map[*symbols.Symbol]bool) []w8dUsageType {
	if sym == nil || visited[sym] {
		return nil
	}
	visited[sym] = true
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	scope := w8dScopeOf(sym)
	var inherited []*symbols.Symbol
	for _, rel := range decl.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		target, ok := resolver.ResolveTarget(scope, rel.Target)
		if !ok || target == nil {
			continue
		}
		if rel.Kind == ast.RelTyping {
			inherited = append(inherited, target)
		}
	}
	if len(inherited) > 0 {
		if !usageDeclCanBeTypedByDefinition(decl) {
			return nil
		}
		types := make([]w8dUsageType, 0, len(inherited))
		for _, target := range inherited {
			types = append(types, w8dUsageType{sym: target, declared: true})
		}
		return types
	}
	var types []w8dUsageType
	for _, rel := range decl.Relationships {
		if rel == nil || rel.Target == nil ||
			(rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines && rel.Kind != ast.RelReferences) {
			continue
		}
		target, ok := resolver.ResolveTarget(scope, rel.Target)
		if !ok || target == nil {
			continue
		}
		types = append(types, nearestDeclaredUsageTypesOf(resolver, target, visited)...)
	}
	return types
}

func usageDeclCanBeTypedByDefinition(decl *ast.Usage) bool {
	if decl == nil || decl.IsReference || decl.Direction != ast.DirNone ||
		decl.Keyword == "" || decl.Keyword == "feature" {
		return false
	}
	return usageKindCanBeTypedByDefinition(decl.Kind)
}

func usageKindCanBeTypedByDefinition(kind ast.UsageKind) bool {
	switch kind {
	case ast.UsageAttribute, ast.UsagePart, ast.UsageItem, ast.UsageOccurrence,
		ast.UsagePort, ast.UsageAction, ast.UsageState, ast.UsageConnection,
		ast.UsageInterface, ast.UsageAllocation, ast.UsageFlow, ast.UsageCalc,
		ast.UsageConstraint, ast.UsageRequirement, ast.UsageCase,
		ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
		ast.UsageEnumeration, ast.UsageRendering, ast.UsageViewpoint, ast.UsageView,
		ast.UsageMetadata:
		return true
	}
	return false
}

func hasTypingRelationship(rels []*ast.Relationship) bool {
	for _, rel := range rels {
		if rel != nil && rel.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// appendUnique records d unless the same message was already reported there: a
// declaration with two wrong types states its one rule once.
func (tc *typeChecker) appendUnique(d Diagnostic) {
	for _, have := range tc.diags {
		if have.Span.Offset == d.Span.Offset && have.Message == d.Message {
			return
		}
	}
	tc.diags = append(tc.diags, d)
}

// checkConjugatedTyping checks a `~T` typing: a ConjugatedPortTyping names the
// conjugated port definition of a port definition, so T must be one (SysML v2
// §7.12.3). A KerML declaration conjugation is a Conjugation, not a typing.
func (tc *typeChecker) checkConjugatedTyping(scope *symbols.Scope, rel *ast.Relationship, decl declKind) {
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, isQN := target.(*ast.QualifiedName)
	if !isQN {
		return
	}
	sym, ok := tc.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	switch sym.Kind {
	case symbols.SymbolPortDef, symbols.SymbolPortUsage:
	default:
		tc.diags = append(tc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     target.Span(),
			Message: fmt.Sprintf(
				"'~' names the conjugated port definition of a port definition, found %s", sym.Kind),
			Code:   "type",
			Source: "type",
		})
		return
	}
	if decl.isDef || (decl.useKind != ast.UsagePort && !decl.isEnd && !decl.isReferenceUsage()) {
		tc.diags = append(tc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     target.Span(),
			Message:  "only a port usage or a connector end may be typed by a conjugated port definition",
			Code:     "type",
			Source:   "type",
		})
	}
}

func compatMessage(decl declKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	isDef, defKind, useKind, direction := decl.isDef, decl.defKind, decl.useKind, decl.direction
	switch rel {
	case ast.RelSpecializes:
		want := defSymbolKind(defKind)
		if !isDef {
			if !decl.isKerML() {
				return "only a definition may specialize; found a usage"
			}
			// Every KerML declaration is a Type and specializes a Type; the
			// definition/usage taxonomy does not apply (KerML 1.0 §8.3.3).
			if !endType.admits(target) {
				return fmt.Sprintf("a KerML type may specialize only a type, found %s", target)
			}
			return ""
		}
		if target == symbols.SymbolUnknown || target == symbols.SymbolKerMLType {
			return "" // a KerML type or an unclassified kind constrains nothing
		}
		if !isDefKind(target) {
			return fmt.Sprintf("%s cannot specialize %s (target is not a definition)", defKind, target)
		}
		// Enums can specialize attribute defs (per SysML v2 spec: GradePoints :> Real)
		if defKind == ast.DefEnumeration && target == symbols.SymbolAttributeDef {
			return ""
		}
		// Metadata defs can specialize metaclasses (per SysML v2 spec: situation :> SemanticMetadata)
		if defKind == ast.DefMetadata && target == symbols.SymbolMetaclass {
			return ""
		}
		// `individual def X` is an occurrence definition (equivalent to
		// `individual occurrence def X`) that individuates the definition it
		// specializes, so it may specialize an occurrence definition of any kind
		// (SysML v2 §7.9.4). It may not specialize an attribute definition:
		// Occurrences::Occurrence is disjoint with Base::DataValues (§8.4.5.1).
		if defKind == ast.DefIndividual && isOccurrenceDefKind(target) {
			return ""
		}
		// Every definition is a Classifier, a DataType among them (KerML §8.3.2), so a
		// classifier specializes any kind; `class`/`struct` stay constrained (§8.4.4.1).
		if decl.isPlainClassifier() {
			return ""
		}
		if !defKindsComparable(target, want) {
			return fmt.Sprintf("%s cannot specialize %s (kind mismatch)", defKind, target)
		}
	case ast.RelSubsets, ast.RelRedefines:
		if isDef {
			return fmt.Sprintf("a definition may not %s a feature", rel)
		}
		if target == symbols.SymbolUnknown {
			return "" // an unclassified target constrains nothing
		}
		if msg := referenceKindMessage(decl, rel, target); msg != "" {
			return msg
		}
		// Usages can subset/redefine other usages OR definitions
		// Example: datatype MyReal :>> Real (usage redefines attributeDef)
		// The check for isUsageKind OR isDefKind allows both patterns
		if !isUsageKind(target) && !isDefKind(target) {
			return fmt.Sprintf("%s target must be a usage or definition, found %s", rel, target)
		}
	case ast.RelTyping:
		if isDef {
			return "" // typing on a definition is not produced by the parser; ignore
		}
		// A KerML FeatureTyping's type is any Type, a Feature among them (KerML
		// 1.0 §8.3.4.4); KerML has no usage-kind taxonomy to check further.
		if decl.isKerML() {
			if !endType.admits(target) {
				return fmt.Sprintf("type must be a type, found %s", target)
			}
			return ""
		}
		if !isDefKind(target) {
			return fmt.Sprintf("type must be a definition, found %s", target)
		}
		// An end feature is a plain KerML feature typed by whatever the feature it
		// connects is typed by (`end supplierPort : FuelOutPort`), so the usage-kind
		// taxonomy does not constrain it.
		if decl.isEnd {
			return ""
		}
		// Nor does it constrain a usage written with no kind keyword: that is a
		// ReferenceUsage, whose type may be any definition.
		if decl.isReferenceUsage() {
			return ""
		}
		// Every SysML definition specializes a KerML type (a part def is a
		// Structure, an action def a Behavior …), so a KerML type may type a usage
		// of any kind (KerML 1.0 §8.3.4).
		if target == symbols.SymbolKerMLType {
			return ""
		}
		// An `individual` or `snapshot` usage is an occurrence usage, and an
		// occurrence is disjoint with the data values an attribute or enumeration
		// definition classifies (SysML v2 §8.4.5.1), so name the modifier that
		// makes the typing wrong rather than the kind keyword.
		if decl.isOccurrenceUsage() && isDataTypeDefKind(target) {
			return fmt.Sprintf("%s usage cannot be typed by %s (an occurrence usage may not be typed by a data type)", decl.occurrenceModifier(), target)
		}
		if !isCompatibleTyping(useKind, direction, target, decl.isOccurrenceUsage()) {
			return fmt.Sprintf("%s cannot be typed by %s (kind mismatch)", useKind, target)
		}
	case ast.RelReferences, ast.RelCrosses, ast.RelVia, ast.RelSubject:
		if msg := referenceKindMessage(decl, rel, target); msg != "" {
			return msg
		}
		if !isDef && !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	case ast.RelAnnotates:
		// An Annotation's annotatedElement is any Element (SysML v2 §7.9):
		// definitions, packages and usages are all legal `about` targets.
		return ""
	}
	return ""
}

// referenceKindMessage is the referent-kind rule shared by the reference forms
// `satisfy r` (a requirement usage, SysML v2 §7.20) and `assert c` (a constraint usage, §7.19).
func referenceKindMessage(decl declKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	if target == symbols.SymbolUnknown {
		return ""
	}
	return referentKindMessage(decl, rel, target, target.String())
}

// referentKind is the kind the referent-kind rule judges sym by. An objective is
// a requirement usage (SysML v2 §8.3.22.4) though the builder files it as a part.
func referentKind(sym *symbols.Symbol) symbols.SymbolKind {
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageObjective {
		return symbols.SymbolRequirementUsage
	}
	return sym.Kind
}

// unclassifiedReferenceKindMessage judges a referent the builder leaves without a
// kind (a named binding): a feature of no constraint kind, named by its notation.
func unclassifiedReferenceKindMessage(decl declKind, rel ast.RelationshipKind, sym *symbols.Symbol) string {
	return referentKindMessage(decl, rel, sym.Kind, sym.Notation())
}

func referentKindMessage(decl declKind, rel ast.RelationshipKind, target symbols.SymbolKind, found string) string {
	if decl.isDef {
		return ""
	}
	switch {
	// `satisfy`/`verify <name>` is a reference subsetting of an existing
	// requirement usage; viewpoint and concern usages are requirement usages.
	case decl.useKind == ast.UsageSatisfy && rel == ast.RelSubsets:
		if !isRequirementUsageKind(target) {
			return fmt.Sprintf("satisfy target must be a requirement usage, found %s", found)
		}
	// `assert [not] <name>` is a reference subsetting of an existing constraint
	// usage; a requirement usage is a constraint usage.
	case decl.isAssertedReference() && rel == ast.RelReferences:
		if !isConstraintUsageKind(target) {
			return fmt.Sprintf("assert target must be a constraint usage, found %s", found)
		}
	}
	return ""
}

func defSymbolKind(k ast.DefinitionKind) symbols.SymbolKind {
	switch k {
	case ast.DefPart:
		return symbols.SymbolPartDef
	case ast.DefAttribute:
		return symbols.SymbolAttributeDef
	case ast.DefItem:
		return symbols.SymbolItemDef
	case ast.DefOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.DefIndividual:
		return symbols.SymbolIndividualDef
	case ast.DefMetadata:
		return symbols.SymbolMetadataDef
	case ast.DefMetaclass:
		// A Metaclass is a Class (KerML 1.0 §8.4.4), so it specializes a metaclass.
		return symbols.SymbolMetaclass
	case ast.DefEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.DefView:
		return symbols.SymbolViewDef
	case ast.DefViewpoint:
		return symbols.SymbolViewpointDef
	case ast.DefRendering:
		return symbols.SymbolRenderingDef
	case ast.DefConcern:
		return symbols.SymbolConcernDef
	case ast.DefConnection:
		return symbols.SymbolConnectionDef
	case ast.DefFlow:
		return symbols.SymbolFlowDef
	case ast.DefPort:
		return symbols.SymbolPortDef
	case ast.DefInterface:
		return symbols.SymbolInterfaceDef
	case ast.DefAllocation:
		return symbols.SymbolAllocationDef
	case ast.DefAction:
		return symbols.SymbolActionDef
	case ast.DefState:
		return symbols.SymbolStateDef
	case ast.DefCalc:
		return symbols.SymbolCalcDef
	case ast.DefConstraint:
		return symbols.SymbolConstraintDef
	case ast.DefRequirement:
		return symbols.SymbolRequirementDef
	case ast.DefCase:
		return symbols.SymbolCaseDef
	case ast.DefAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.DefVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.DefUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

func usageWantsDefKind(k ast.UsageKind) symbols.SymbolKind {
	switch k {
	case ast.UsagePart:
		return symbols.SymbolPartDef
	case ast.UsageAttribute:
		return symbols.SymbolAttributeDef
	case ast.UsageItem:
		return symbols.SymbolItemDef
	case ast.UsageOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.UsageIndividual:
		return symbols.SymbolIndividualDef
	case ast.UsageMetadata:
		return symbols.SymbolMetadataDef
	case ast.UsageEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.UsageView:
		return symbols.SymbolViewDef
	case ast.UsageViewpoint:
		return symbols.SymbolViewpointDef
	case ast.UsageRendering, ast.UsageViewRendering:
		return symbols.SymbolRenderingDef
	case ast.UsageConcern, ast.UsageFramedConcern:
		return symbols.SymbolConcernDef
	case ast.UsageActor, ast.UsageStakeholder:
		// An actor and a stakeholder are part usages, so a part definition types
		// them (SysML v2 §8.3.19).
		return symbols.SymbolPartDef
	case ast.UsageConnection:
		return symbols.SymbolConnectionDef
	case ast.UsageFlow:
		return symbols.SymbolFlowDef
	case ast.UsagePort:
		return symbols.SymbolPortDef
	case ast.UsageInterface:
		return symbols.SymbolInterfaceDef
	case ast.UsageAllocation:
		return symbols.SymbolAllocationDef
	case ast.UsageAction:
		return symbols.SymbolActionDef
	case ast.UsageState:
		return symbols.SymbolStateDef
	case ast.UsageCalc:
		return symbols.SymbolCalcDef
	case ast.UsageConstraint:
		return symbols.SymbolConstraintDef
	case ast.UsageRequirement:
		return symbols.SymbolRequirementDef
	case ast.UsageCase:
		return symbols.SymbolCaseDef
	case ast.UsageAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.UsageVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.UsageUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

// defSymbolKinds is the set of SymbolKinds that classify a definition.
var defSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartDef:             true,
	symbols.SymbolAttributeDef:        true,
	symbols.SymbolItemDef:             true,
	symbols.SymbolOccurrenceDef:       true,
	symbols.SymbolIndividualDef:       true,
	symbols.SymbolMetadataDef:         true,
	symbols.SymbolMetaclass:           true, // KerML metaclass definitions
	symbols.SymbolEnumerationDef:      true,
	symbols.SymbolViewDef:             true,
	symbols.SymbolViewpointDef:        true,
	symbols.SymbolRenderingDef:        true,
	symbols.SymbolConcernDef:          true,
	symbols.SymbolConnectionDef:       true,
	symbols.SymbolFlowDef:             true,
	symbols.SymbolPortDef:             true,
	symbols.SymbolInterfaceDef:        true,
	symbols.SymbolAllocationDef:       true,
	symbols.SymbolActionDef:           true,
	symbols.SymbolStateDef:            true,
	symbols.SymbolCalcDef:             true,
	symbols.SymbolConstraintDef:       true,
	symbols.SymbolRequirementDef:      true,
	symbols.SymbolCaseDef:             true,
	symbols.SymbolAnalysisCaseDef:     true,
	symbols.SymbolVerificationCaseDef: true,
	symbols.SymbolUseCaseDef:          true,
	symbols.SymbolKerMLType:           true, // KerML class/struct/assoc/behavior/predicate
	symbols.SymbolAlias:               true, // Aliases can be used as types
}

// usageSymbolKinds is the set of SymbolKinds that classify a usage.
var usageSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartUsage:               true,
	symbols.SymbolAttributeUsage:          true,
	symbols.SymbolItemUsage:               true,
	symbols.SymbolOccurrenceUsage:         true,
	symbols.SymbolIndividualUsage:         true,
	symbols.SymbolMetadataUsage:           true,
	symbols.SymbolEnumerationUsage:        true,
	symbols.SymbolViewUsage:               true,
	symbols.SymbolViewpointUsage:          true,
	symbols.SymbolRenderingUsage:          true,
	symbols.SymbolConcernUsage:            true,
	symbols.SymbolConnectionUsage:         true,
	symbols.SymbolSuccessionUsage:         true,
	symbols.SymbolFlowUsage:               true,
	symbols.SymbolPortUsage:               true,
	symbols.SymbolInterfaceUsage:          true,
	symbols.SymbolAllocationUsage:         true,
	symbols.SymbolActionUsage:             true,
	symbols.SymbolStateUsage:              true,
	symbols.SymbolCalcUsage:               true,
	symbols.SymbolConstraintUsage:         true,
	symbols.SymbolRequirementUsage:        true,
	symbols.SymbolSatisfyRequirementUsage: true,
	symbols.SymbolCaseUsage:               true,
	symbols.SymbolAnalysisCaseUsage:       true,
	symbols.SymbolVerificationCaseUsage:   true,
	symbols.SymbolUseCaseUsage:            true,
	symbols.SymbolConnectorEnd:            true, // An end of a connect clause is a feature
	symbols.SymbolCrossFeature:            true,
	symbols.SymbolAlias:                   true, // Aliases can be subsetting targets
}

func isDefKind(k symbols.SymbolKind) bool {
	return defSymbolKinds[k]
}

func isUsageKind(k symbols.SymbolKind) bool {
	return usageSymbolKinds[k]
}

// typeSymbolKinds is the set of SymbolKinds that classify a Type: every
// definition and usage kind, a KerML type declaration, and a named multiplicity,
// which is a Feature. Enumerated so a kind added later is rejected until classified.
var typeSymbolKinds = func() map[symbols.SymbolKind]bool {
	m := map[symbols.SymbolKind]bool{
		symbols.SymbolKerMLType:    true,
		symbols.SymbolMultiplicity: true,
	}
	for k := range defSymbolKinds {
		m[k] = true
	}
	for k := range usageSymbolKinds {
		m[k] = true
	}
	// An alias is a naming, not a Type; a resolvable one is resolved upstream.
	delete(m, symbols.SymbolAlias)
	return m
}()

// isTypeKind reports whether k classifies a Type, which alone may be a KerML
// specialization or typing target — a Feature is one too (KerML 1.0 §8.3.3).
// An unclassified kind constrains nothing, as on the definition path.
func isTypeKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolUnknown || typeSymbolKinds[k]
}

// occurrenceDefSymbolKinds is the set of SymbolKinds that classify a definition
// that is an OccurrenceDefinition in the SysML v2 abstract syntax (§8.3.9.3):
// the occurrence definition itself, an individual definition, and every kind
// whose metaclass directly or indirectly specializes OccurrenceDefinition —
// items and parts (§8.3.10.2, §8.3.11.2), ports (§8.3.12.5), connections with
// their interface and allocation specializations (§8.3.13.3, §8.3.14.2,
// §8.3.15.2), actions and everything derived from them (§8.3.16.2, §8.3.17.3,
// §8.3.18.5, §8.3.19.2, §8.3.22.2, §8.3.23.2, §8.3.24.3, §8.3.25.3),
// constraints and requirements (§8.3.20.3, §8.3.21.3, §8.3.21.8, §8.3.26.8),
// views and renderings (§8.3.26.7, §8.3.26.5) and metadata definitions
// (§8.3.27.2). Attribute and enumeration definitions are data types, not
// occurrence definitions (§8.3.7.2, §8.3.8.2).
var occurrenceDefSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolOccurrenceDef:       true,
	symbols.SymbolIndividualDef:       true,
	symbols.SymbolItemDef:             true,
	symbols.SymbolPartDef:             true,
	symbols.SymbolPortDef:             true,
	symbols.SymbolConnectionDef:       true,
	symbols.SymbolInterfaceDef:        true,
	symbols.SymbolAllocationDef:       true,
	symbols.SymbolFlowDef:             true,
	symbols.SymbolActionDef:           true,
	symbols.SymbolStateDef:            true,
	symbols.SymbolCalcDef:             true,
	symbols.SymbolCaseDef:             true,
	symbols.SymbolAnalysisCaseDef:     true,
	symbols.SymbolVerificationCaseDef: true,
	symbols.SymbolUseCaseDef:          true,
	symbols.SymbolConstraintDef:       true,
	symbols.SymbolRequirementDef:      true,
	symbols.SymbolConcernDef:          true,
	symbols.SymbolViewpointDef:        true,
	symbols.SymbolViewDef:             true,
	symbols.SymbolRenderingDef:        true,
	symbols.SymbolMetadataDef:         true,
}

func isOccurrenceDefKind(k symbols.SymbolKind) bool {
	return occurrenceDefSymbolKinds[k]
}

// defKindParents is the definition metaclass taxonomy (SysML v2 §8.3): each kind
// maps to the kinds it specializes.
var defKindParents = map[symbols.SymbolKind][]symbols.SymbolKind{
	symbols.SymbolItemDef:             {symbols.SymbolOccurrenceDef},
	symbols.SymbolIndividualDef:       {symbols.SymbolOccurrenceDef},
	symbols.SymbolPartDef:             {symbols.SymbolItemDef},
	symbols.SymbolMetadataDef:         {symbols.SymbolItemDef},
	symbols.SymbolConnectionDef:       {symbols.SymbolPartDef},
	symbols.SymbolInterfaceDef:        {symbols.SymbolConnectionDef},
	symbols.SymbolAllocationDef:       {symbols.SymbolConnectionDef},
	symbols.SymbolViewDef:             {symbols.SymbolPartDef},
	symbols.SymbolRenderingDef:        {symbols.SymbolPartDef},
	symbols.SymbolActionDef:           {symbols.SymbolOccurrenceDef},
	symbols.SymbolFlowDef:             {symbols.SymbolActionDef, symbols.SymbolConnectionDef},
	symbols.SymbolStateDef:            {symbols.SymbolActionDef},
	symbols.SymbolCalcDef:             {symbols.SymbolActionDef},
	symbols.SymbolCaseDef:             {symbols.SymbolCalcDef},
	symbols.SymbolAnalysisCaseDef:     {symbols.SymbolCaseDef},
	symbols.SymbolVerificationCaseDef: {symbols.SymbolCaseDef},
	symbols.SymbolUseCaseDef:          {symbols.SymbolCaseDef},
	symbols.SymbolConstraintDef:       {symbols.SymbolOccurrenceDef},
	symbols.SymbolRequirementDef:      {symbols.SymbolConstraintDef},
	symbols.SymbolConcernDef:          {symbols.SymbolRequirementDef},
	symbols.SymbolViewpointDef:        {symbols.SymbolRequirementDef},
	symbols.SymbolPortDef:             {symbols.SymbolOccurrenceDef},
	symbols.SymbolEnumerationDef:      {symbols.SymbolAttributeDef},
}

// defKindSpecializes reports whether kind k is want or one of its
// specializations in that taxonomy.
func defKindSpecializes(k, want symbols.SymbolKind) bool {
	if k == want {
		return true
	}
	for _, parent := range defKindParents[k] {
		if defKindSpecializes(parent, want) {
			return true
		}
	}
	return false
}

// defKindsComparable reports whether one of two definition kinds specializes the
// other, as an item and a part definition do and a part and an attribute
// definition do not.
func defKindsComparable(a, b symbols.SymbolKind) bool {
	return defKindSpecializes(a, b) || defKindSpecializes(b, a)
}

// isDataTypeDefKind reports whether k classifies data values rather than
// occurrences: an attribute definition is a DataType and an enumeration
// definition an AttributeDefinition (SysML v2 §8.3.7.2, §8.3.8.2). Occurrences
// are disjoint with data values (§8.4.5.1).
func isDataTypeDefKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolAttributeDef || k == symbols.SymbolEnumerationDef
}

// isConstraintUsageKind reports whether k is a ConstraintUsage or one of its
// specializations, the requirement usage kinds among them (SysML v2 §8.3.19).
func isConstraintUsageKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolConstraintUsage || isRequirementUsageKind(k)
}

// isRequirementUsageKind reports whether k is a RequirementUsage or one of its
// specializations (ViewpointUsage, ConcernUsage).
func isRequirementUsageKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolRequirementUsage, symbols.SymbolSatisfyRequirementUsage,
		symbols.SymbolViewpointUsage, symbols.SymbolConcernUsage:
		return true
	}
	return false
}

// isRequirementDefKind reports whether k is a RequirementDefinition or one of
// its specializations (ConcernDefinition, ViewpointDefinition).
func isRequirementDefKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolRequirementDef, symbols.SymbolConcernDef, symbols.SymbolViewpointDef:
		return true
	}
	return false
}

// isCompatibleTyping checks if a usage kind can be typed by a definition kind.
// Allows structural compatibility: part/attribute/item/occurrence can cross-type
// since they're all structural classifiers in SysML.
// isOccurrenceUsage marks a usage carrying the `individual` or `snapshot`
// modifier. Such a usage is an OccurrenceUsage whatever kind keyword declares it
// (SysML v2 §8.3.9.11), so it may be typed by an occurrence definition of any
// kind, on top of whatever its kind keyword admits.
func isCompatibleTyping(useKind ast.UsageKind, direction ast.FeatureDirection, defKind symbols.SymbolKind, isOccurrenceUsage bool) bool {
	if isOccurrenceUsage && isOccurrenceDefKind(defKind) {
		return true
	}
	if compatibleTyping(useKind, direction, defKind) {
		return true
	}
	// An individual definition is an occurrence definition that individuates the
	// definition it specializes (SysML v2 §7.9.4), so a usage may be typed by
	// one wherever it may be typed by an occurrence definition.
	if defKind == symbols.SymbolIndividualDef {
		return compatibleTyping(useKind, direction, symbols.SymbolOccurrenceDef)
	}
	return false
}

func compatibleTyping(useKind ast.UsageKind, direction ast.FeatureDirection, defKind symbols.SymbolKind) bool {
	// Exact match always allowed
	if defKind == usageWantsDefKind(useKind) {
		return true
	}

	// SysML v2 §7.27.2: a MetadataDefinition is an ItemDefinition, so it types
	// whatever an item definition types (`:> annotatedElement : SysML::PartDefinition`).
	if defKind == symbols.SymbolMetadataDef {
		defKind = symbols.SymbolItemDef
	}

	// An attribute is typed by data types: an attribute or enumeration
	// definition (SysML v2 §8.3.9.4 validateAttributeUsageType). A directed one
	// is a parameter and crosses to structural definitions below.
	if useKind == ast.UsageAttribute && direction == ast.DirNone {
		return defKind == symbols.SymbolAttributeDef ||
			defKind == symbols.SymbolEnumerationDef
	}

	// Parameters (in/out/inout) can cross-type to any structural def
	// Also allow enumDef for typed enumerations
	// This allows: in power : PowerValue (part : attributeDef), out verdict : VerdictKind (part : enumDef)
	hasDirection := direction != ast.DirNone
	if hasDirection {
		return defKind == symbols.SymbolPartDef ||
			defKind == symbols.SymbolAttributeDef ||
			defKind == symbols.SymbolItemDef ||
			defKind == symbols.SymbolOccurrenceDef ||
			defKind == symbols.SymbolEnumerationDef
	}

	// An occurrence, item or part is typed by occurrence definitions only: its
	// types are Classes, and a data type is not one (SysML v2 §8.3.9.7
	// validateOccurrenceUsageType).
	if useKind == ast.UsagePart || useKind == ast.UsageItem {
		return isOccurrenceDefKind(defKind)
	}
	// An `occurrence`/`individual` usage's data typing is W8D's, which reports
	// it whether declared or inherited.
	if useKind == ast.UsageOccurrence || useKind == ast.UsageIndividual {
		return isOccurrenceDefKind(defKind) || defKind == symbols.SymbolAttributeDef
	}

	// An action must be typed by action definitions, i.e. Behaviors (SysML v2
	// §8.3.16.6 validateActionUsageType), so any behavior-family def works.
	if useKind == ast.UsageAction {
		return defKindSpecializes(defKind, symbols.SymbolActionDef)
	}

	// A case may be typed by a case definition of any kind (SysML v2 §8.3.24.4
	// validateCaseUsageType); analysis and verification keep their exact kinds.
	if useKind == ast.UsageCase {
		return defKindSpecializes(defKind, symbols.SymbolCaseDef)
	}

	// A constraint is typed by a Predicate (SysML v2 §8.3.19.3): a requirement,
	// concern or viewpoint definition is a constraint definition, so it qualifies.
	if useKind == ast.UsageConstraint {
		return defKindSpecializes(defKind, symbols.SymbolConstraintDef)
	}

	// Successions and bindings type through a plain UsageDeclaration
	// (SysML.xtext:1033, :1020), so any definition types them.
	if useKind == ast.UsageSuccession || useKind == ast.UsageBinding {
		return true
	}

	// SysML v2 §8.3.22.4: an ObjectiveMembership's ownedObjectiveRequirement is a
	// RequirementUsage, so an objective is typed by a RequirementDefinition or one
	// of its specializations (`objective : MaximizeObjective`, a requirement def in
	// Domain Libraries/Analysis/TradeStudies.sysml).
	if useKind == ast.UsageObjective {
		return isRequirementDefKind(defKind)
	}

	// A SatisfyRequirementUsage is a RequirementUsage (SysML v2 §8.3.19), so the
	// declaration form `satisfy requirement r : Req1 by v` is typed by a
	// RequirementDefinition or one of its specializations.
	if useKind == ast.UsageSatisfy {
		return isRequirementDefKind(defKind)
	}

	// A SubjectMembership's ownedSubjectParameter is an unconstrained Usage (SysML
	// v2 §8.3.21), so any definition types a subject — the OMG training models
	// subject a `port def` and an `action def` as well as structural definitions.
	if useKind == ast.UsageSubject {
		return true
	}

	return false
}

func unwrapType(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func childScopeOf(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	return scope.ChildFor(decl)
}
