package passes

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// HDBK1009Pass audits the elements a model classifies with the HDBK1009
// library against the relationships of the NASA-HDBK-1009A metamodel. Each
// rule fires only once the model declares the elements the rule relates, and
// a model that never uses the library gets no finding.
type HDBK1009Pass struct{}

// Diagnostic codes of the NASA-HDBK-1009A rules.
const (
	CodeHDBK1009RequirementNotDerived   = "hdbk1009-requirement-not-derived"
	CodeHDBK1009ExpectationNotTraced    = "hdbk1009-expectation-not-traced"
	CodeHDBK1009MeasureNotRefining      = "hdbk1009-measure-not-refining"
	CodeHDBK1009RequirementNotVerified  = "hdbk1009-requirement-not-verified"
	CodeHDBK1009VVRequirementNotRefined = "hdbk1009-vv-requirement-not-refined"
	CodeHDBK1009VVRequirementNoMethod   = "hdbk1009-vv-requirement-missing-method"
	CodeHDBK1009ObjectiveNotValidated   = "hdbk1009-objective-not-validated"
)

const hdbkSource = "hdbk1009"

func (HDBK1009Pass) Level() PassLevel { return LevelConstraint }

func (HDBK1009Pass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	if ctx.Index.DocumentLibraryTier(name).Library() {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	a := newHDBKAudit(ctx)
	if a == nil {
		return nil
	}
	a.union = hdbkUnionOf(ctx)
	if !ctx.Gathers().Has(name) {
		a.local = newHDBKFacts()
		a.facts = a.local
		a.gather(rootScope)
		a.facts = nil
	}
	a.check(rootScope)
	return a.diags
}

// hdbkKind is an element class of the Handbook's metamodel, told by
// conformance to its library definition; hdbkNone is no class.
type hdbkKind int

const (
	hdbkNone hdbkKind = iota
	hdbkStakeholder
	hdbkExpectation
	hdbkNeed
	hdbkGoal
	hdbkObjective
	hdbkSystemRequirement
	hdbkSubsystemRequirement
	hdbkComponentRequirement
	hdbkMOE
	hdbkMOP
	hdbkVerificationRequirement
	hdbkValidationRequirement
	hdbkVVCase
)

var hdbkKindDefinitions = []struct {
	kind   hdbkKind
	fqn    string
	name   string
	plural string
}{
	{hdbkStakeholder, "HDBK1009::Stakeholder", "stakeholder", "stakeholders"},
	{hdbkExpectation, "HDBK1009::StakeholderExpectationStatement", "stakeholder expectation statement", "stakeholder expectation statements"},
	{hdbkNeed, "HDBK1009::Need", "need", "needs"},
	{hdbkGoal, "HDBK1009::Goal", "goal", "goals"},
	{hdbkObjective, "HDBK1009::Objective", "objective", "objectives"},
	{hdbkSystemRequirement, "HDBK1009::SystemRequirement", "system requirement", "system requirements"},
	{hdbkSubsystemRequirement, "HDBK1009::SubsystemRequirement", "subsystem requirement", "subsystem requirements"},
	{hdbkComponentRequirement, "HDBK1009::ComponentRequirement", "component requirement", "component requirements"},
	{hdbkMOE, "HDBK1009::MeasureOfEffectiveness", "measure of effectiveness", "measures of effectiveness"},
	{hdbkMOP, "HDBK1009::MeasureOfPerformance", "measure of performance", "measures of performance"},
	{hdbkVerificationRequirement, "HDBK1009::VerificationRequirement", "verification requirement", "verification requirements"},
	{hdbkValidationRequirement, "HDBK1009::ValidationRequirement", "validation requirement", "validation requirements"},
	{hdbkVVCase, "HDBK1009::VVCase", "V&V case", "V&V cases"},
}

func hdbkKindName(k hdbkKind) string {
	for _, entry := range hdbkKindDefinitions {
		if entry.kind == k {
			return entry.name
		}
	}
	return ""
}

func hdbkKindPlural(k hdbkKind) string {
	for _, entry := range hdbkKindDefinitions {
		if entry.kind == k {
			return entry.plural
		}
	}
	return ""
}

// hdbkDerivesFrom lists the levels each level derives from in Figure 7-1.
var hdbkDerivesFrom = map[hdbkKind][]hdbkKind{
	hdbkGoal:                 {hdbkNeed},
	hdbkObjective:            {hdbkGoal},
	hdbkSystemRequirement:    {hdbkObjective},
	hdbkSubsystemRequirement: {hdbkSystemRequirement, hdbkSubsystemRequirement},
	hdbkComponentRequirement: {hdbkSystemRequirement, hdbkSubsystemRequirement},
	hdbkMOP:                  {hdbkMOE},
}

var hdbkTechnicalRequirements = []hdbkKind{hdbkSystemRequirement, hdbkSubsystemRequirement, hdbkComponentRequirement}

type hdbkAudit struct {
	ctx         *Context
	model       *semantics.Model
	definitions map[hdbkKind]*symbols.Symbol
	// facts is what a gather under way records; a check judges over union and
	// local, the facts of a root outside the gathered documents.
	facts     *hdbkFacts
	union     *hdbkUnion
	local     *hdbkFacts
	kinds     map[*symbols.Symbol]hdbkKind
	typeKinds map[*symbols.Symbol]hdbkKind
	diags     []diag.Diagnostic
}

// newHDBKAudit returns nil when the bundled HDBK1009 library is not loaded; a
// workspace package of the same name is not the Handbook's vocabulary.
func newHDBKAudit(ctx *Context) *hdbkAudit {
	a := &hdbkAudit{
		ctx:         ctx,
		model:       ctx.Model(),
		definitions: map[hdbkKind]*symbols.Symbol{},
		kinds:       map[*symbols.Symbol]hdbkKind{},
		typeKinds:   map[*symbols.Symbol]hdbkKind{},
	}
	for _, entry := range hdbkKindDefinitions {
		for _, def := range ctx.Index.LookupQualified(entry.fqn) {
			if def != nil && ctx.Index.Library(def) {
				a.definitions[entry.kind] = def
			}
		}
	}
	if len(a.definitions) == 0 {
		return nil
	}
	return a
}

// kindOf classifies sym: a usage by the types it has, a definition by itself.
func (a *hdbkAudit) kindOf(sym *symbols.Symbol) hdbkKind {
	if sym == nil {
		return hdbkNone
	}
	if kind, ok := a.kinds[sym]; ok {
		return kind
	}
	a.kinds[sym] = hdbkNone
	var types []*symbols.Symbol
	if sym.IsFeature() {
		types = a.model.FeatureTypeSet(sym)
	} else if sym.DeclaresDefinition() {
		types = []*symbols.Symbol{sym}
	}
	kind := hdbkNone
	for _, t := range types {
		if kind = a.kindOfType(t); kind != hdbkNone {
			break
		}
	}
	a.kinds[sym] = kind
	return kind
}

func (a *hdbkAudit) kindOfType(t *symbols.Symbol) hdbkKind {
	if kind, ok := a.typeKinds[t]; ok {
		return kind
	}
	kind := hdbkNone
	for _, entry := range hdbkKindDefinitions {
		if def := a.definitions[entry.kind]; def != nil && a.model.Conforms(t, def) {
			kind = entry.kind
			break
		}
	}
	a.typeKinds[t] = kind
	return kind
}

// declaredKind is the kind of a part, requirement or verification case usage
// the model declares, read from a recorded symbol's facts as from a body. A
// satisfy or verify is one only when it declares the requirement it names.
func (a *hdbkAudit) declaredKind(sym *symbols.Symbol) hdbkKind {
	kind, ok := sym.UsageKind()
	if !ok {
		return hdbkNone
	}
	switch kind {
	case ast.UsagePart, ast.UsageRequirement, ast.UsageVerificationCase:
		return a.kindOf(sym)
	case ast.UsageSatisfy:
		if declaresRequirement(sym) {
			return a.kindOf(sym)
		}
	}
	return hdbkNone
}

// declaresRequirement reports the `requirement` of `satisfy requirement r by v`.
func declaresRequirement(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModDeclaresRequirement)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.DeclaresRequirement
}

// gather records the kinds a document declares and the relationships it states.
func (a *hdbkAudit) gather(root *symbols.Scope) {
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		if kind := a.declaredKind(sym); kind != hdbkNone {
			if a.facts.present[kind] {
				a.facts.several[kind] = true
			}
			a.facts.present[kind] = true
		}
		switch decl := sym.Decl.(type) {
		case *ast.Dependency:
			for _, e := range a.model.RelationshipEdgesOf(sym, semantics.RelationshipDependency) {
				a.note(hdbkTracedTo, e.Source, e.Target)
			}
			for _, e := range a.model.RelationshipEdgesOf(sym, semantics.RelationshipRefinement) {
				a.noteBoth(hdbkRefines, hdbkRefinedBy, e.Source, e.Target)
			}
		case *ast.Usage:
			switch {
			case decl.Kind == ast.UsageConnection:
				for _, e := range a.model.RelationshipEdgesOf(sym, semantics.RelationshipDerivation) {
					a.noteBoth(hdbkDerivedFrom, hdbkOriginOf, e.Target, e.Source)
				}
			case decl.Kind == ast.UsageSatisfy && decl.Keyword == "verify":
				for _, e := range a.model.RelationshipEdgesOf(sym, semantics.RelationshipVerification) {
					a.note(hdbkVerifiedBy, e.Target, e.Source)
				}
			case decl.Kind == ast.UsageSatisfy && !decl.IsNegated:
				for _, e := range a.model.RelationshipEdgesOf(sym, semantics.RelationshipSatisfaction) {
					a.noteBoth(hdbkSatisfies, hdbkSatisfiedBy, e.Source, e.Target)
				}
			}
		}
	})
	// A recorded document's body is gone; its record kept what it stated.
	rec := a.ctx.Index.Gathered(root.DocName())
	if rec == nil {
		return
	}
	each := func(ends []symbols.GatheredEnds, note func(source, target *symbols.Symbol)) {
		for _, e := range ends {
			for _, s := range a.ctx.Index.Elements(e.Sources) {
				for _, t := range a.ctx.Index.Elements(e.Targets) {
					note(s, t)
				}
			}
		}
	}
	each(rec.RequirementDerivations, func(s, t *symbols.Symbol) { a.noteBoth(hdbkDerivedFrom, hdbkOriginOf, t, s) })
	each(rec.Dependencies, func(s, t *symbols.Symbol) { a.note(hdbkTracedTo, s, t) })
	each(rec.Refinements, func(s, t *symbols.Symbol) { a.noteBoth(hdbkRefines, hdbkRefinedBy, s, t) })
	each(rec.Verifications, func(s, t *symbols.Symbol) { a.note(hdbkVerifiedBy, t, s) })
	for _, s := range rec.Satisfactions {
		for _, by := range a.ctx.Index.Elements(s.Satisfiers) {
			for _, req := range a.ctx.Index.Elements(s.Requirements) {
				a.noteBoth(hdbkSatisfies, hdbkSatisfiedBy, by, req)
			}
		}
	}
}

// note records element as related to other: to its kind, and to any element.
func (a *hdbkAudit) note(relation hdbkRelation, element, other *symbols.Symbol) {
	if element == nil || other == nil {
		return
	}
	key := symbols.KeyOf(element)
	if key == symbols.KeyOf(other) {
		return // an element related to itself is related to nothing
	}
	a.facts.links[hdbkLink{relation, key, hdbkNone}] = true
	if kind := a.kindOf(other); kind != hdbkNone {
		a.facts.links[hdbkLink{relation, key, kind}] = true
	}
}

func (a *hdbkAudit) noteBoth(forward, backward hdbkRelation, source, target *symbols.Symbol) {
	a.note(forward, source, target)
	a.note(backward, target, source)
}

// check judges the elements the document declares.
func (a *hdbkAudit) check(root *symbols.Scope) {
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageRequirement && !usage.DeclaresRequirement {
			return
		}
		kind := a.declaredKind(sym)
		if kind == hdbkNone {
			return
		}
		key := symbols.KeyOf(sym)
		a.checkDerivation(sym, key, kind)
		a.checkTrace(sym, key, kind)
		a.checkMeasure(sym, key, kind)
		a.checkVerification(sym, key, kind)
		a.checkVVRequirement(sym, key, kind)
		a.checkValidation(sym, key, kind)
	})
}

// checkDerivation expects a requirement to derive from the level above it once that
// exists, and otherwise from its own level unless it is where that level starts.
func (a *hdbkAudit) checkDerivation(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	from, derives := hdbkDerivesFrom[kind]
	if !derives || a.linked(hdbkDerivedFrom, key, from...) {
		return
	}
	above := slices.DeleteFunc(slices.Clone(from), func(k hdbkKind) bool { return k == kind })
	origin := from[0]
	if !a.hasAnyKind(above) {
		if len(above) == len(from) || !a.hasPeers(kind) || a.linked(hdbkOriginOf, key, kind) {
			return
		}
		origin = kind
	}
	a.report(sym, CodeHDBK1009RequirementNotDerived, fmt.Sprintf(
		"This %s derives from no %s: the model declares %s, so Figure 7-1 of NASA-HDBK-1009A expects a #derivation connection naming it at a #derive end and a %s at an #original end.",
		hdbkKindName(kind), hdbkKindName(origin), hdbkKindPlural(origin), hdbkKindName(origin)))
}

// checkTrace expects an expectation statement to trace to a stakeholder, and a
// need, goal or objective to an expectation statement, once those exist.
func (a *hdbkAudit) checkTrace(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	var to hdbkKind
	switch kind {
	case hdbkExpectation:
		to = hdbkStakeholder
	case hdbkNeed, hdbkGoal, hdbkObjective:
		to = hdbkExpectation
	default:
		return
	}
	if !a.hasKind(to) || a.linked(hdbkTracedTo, key, to) {
		return
	}
	a.report(sym, CodeHDBK1009ExpectationNotTraced, fmt.Sprintf(
		"This %s is traced to no %s: the model declares %s, so Figure 7-1 of NASA-HDBK-1009A expects a dependency from it to one.",
		hdbkKindName(kind), hdbkKindName(to), hdbkKindPlural(to)))
}

// checkMeasure expects a measure of effectiveness to refine an objective and a
// measure of performance a technical requirement, once those exist.
func (a *hdbkAudit) checkMeasure(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	switch kind {
	case hdbkMOE:
		if a.hasKind(hdbkObjective) && !a.linked(hdbkRefines, key, hdbkObjective) {
			a.report(sym, CodeHDBK1009MeasureNotRefining,
				"This measure of effectiveness refines no objective: the model declares objectives, so Figure 7-1 of NASA-HDBK-1009A expects a #refinement dependency from it to one.")
		}
	case hdbkMOP:
		if a.hasAnyKind(hdbkTechnicalRequirements) && !a.linked(hdbkRefines, key, hdbkTechnicalRequirements...) {
			a.report(sym, CodeHDBK1009MeasureNotRefining,
				"This measure of performance refines no requirement: the model declares technical requirements, so Figure 7-1 of NASA-HDBK-1009A expects a #refinement dependency from it to one.")
		}
	}
}

// checkVerification expects a technical requirement to be verified once the
// model declares verification requirements.
func (a *hdbkAudit) checkVerification(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	switch kind {
	case hdbkSystemRequirement, hdbkSubsystemRequirement, hdbkComponentRequirement:
	default:
		return
	}
	if !a.hasKind(hdbkVerificationRequirement) || a.linked(hdbkVerifiedBy, key, hdbkNone) {
		return
	}
	a.report(sym, CodeHDBK1009RequirementNotVerified, fmt.Sprintf(
		"This %s is verified by nothing: the model declares verification requirements, so Figure 7-2 of NASA-HDBK-1009A expects a V&V case whose objective states `verify` of it.",
		hdbkKindName(kind)))
}

// checkVVRequirement expects a verification or validation requirement to state
// its method and, once the model declares V&V cases, to be refined by one.
func (a *hdbkAudit) checkVVRequirement(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	if kind != hdbkVerificationRequirement && kind != hdbkValidationRequirement {
		return
	}
	if values, present, err := a.model.DeclaredFeatureValues(sym, "verifyMethod"); err == nil && (!present || len(values) == 0) {
		a.report(sym, CodeHDBK1009VVRequirementNoMethod, fmt.Sprintf(
			"This %s states no verifyMethod: the matrices of NASA-HDBK-1009A sections 8.17 and 8.22 show the method of each.",
			hdbkKindName(kind)))
	}
	if a.hasKind(hdbkVVCase) && !a.linked(hdbkRefinedBy, key, hdbkVVCase) {
		a.report(sym, CodeHDBK1009VVRequirementNotRefined, fmt.Sprintf(
			"This %s is refined by no V&V case: the model declares V&V cases, so Figure 7-2 of NASA-HDBK-1009A expects a #refinement dependency from one to it.",
			hdbkKindName(kind)))
	}
}

// checkValidation expects a validation requirement to satisfy an objective,
// and an objective to be satisfied by one once the model declares any.
func (a *hdbkAudit) checkValidation(sym *symbols.Symbol, key symbols.ElementKey, kind hdbkKind) {
	switch kind {
	case hdbkValidationRequirement:
		if a.hasKind(hdbkObjective) && !a.linked(hdbkSatisfies, key, hdbkObjective) {
			a.report(sym, CodeHDBK1009ObjectiveNotValidated,
				"This validation requirement satisfies no objective: the model declares objectives, so Figure 7-2 of NASA-HDBK-1009A expects a `satisfy` of one by it.")
		}
	case hdbkObjective:
		if a.hasKind(hdbkValidationRequirement) && !a.linked(hdbkSatisfiedBy, key, hdbkValidationRequirement) {
			a.report(sym, CodeHDBK1009ObjectiveNotValidated,
				"This objective is satisfied by no validation requirement: the model declares validation requirements, so Figure 7-2 of NASA-HDBK-1009A expects a `satisfy` of it by one.")
		}
	}
}

func (a *hdbkAudit) hasAnyKind(kinds []hdbkKind) bool {
	for _, k := range kinds {
		if a.hasKind(k) {
			return true
		}
	}
	return false
}

func (a *hdbkAudit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   hdbkSource,
	})
}
