package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// NASASTD7009Pass audits the elements a model classifies with the NASASTD7009 library against
// what NASA-STD-7009B states of them; a model that never touches the library gets no finding.
type NASASTD7009Pass struct{}

// Diagnostic codes of the NASA-STD-7009B rules.
const (
	CodeNASASTD7009AssessmentMissingFactor = "nasastd7009-assessment-missing-factor"
	CodeNASASTD7009ReportMissingContent    = "nasastd7009-report-missing-content"
	CodeNASASTD7009ReportMissingWarning    = "nasastd7009-report-missing-warning"
	CodeNASASTD7009WarningMissingImpact    = "nasastd7009-warning-missing-impact"
	CodeNASASTD7009CriticalityRating       = "nasastd7009-criticality-rating-differs-from-matrix"
	CodeNASASTD7009EntryIncomplete         = "nasastd7009-requirement-entry-incomplete"
	CodeNASASTD7009MatrixMissingReq        = "nasastd7009-matrix-missing-requirement"
)

const nasastd7009Source = "nasastd7009"

// Level reports the tier the pass runs at.
func (NASASTD7009Pass) Level() PassLevel { return LevelConstraint }

// Run audits one document against the NASASTD7009 library it uses.
func (NASASTD7009Pass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
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
	a := newNASASTD7009Audit(ctx)
	if a == nil {
		return nil
	}
	a.check(rootScope)
	return a.diags
}

// Library qualified names the pass binds to (see engine-contract.json upstream).
const (
	nasastd7009CriticalityAssessment      = "NASASTD7009::CriticalityAssessment"
	nasastd7009DecisionConsequence        = "NASASTD7009::DecisionConsequence"
	nasastd7009MSInfluence                = "NASASTD7009::MSInfluence"
	nasastd7009CriticalityRating          = "NASASTD7009::CriticalityRating"
	nasastd7009FactorLevel                = "NASASTD7009::FactorLevel"
	nasastd7009CapabilityFactor           = "NASASTD7009::CapabilityFactor"
	nasastd7009ResultsFactor              = "NASASTD7009::ResultsFactor"
	nasastd7009CapabilityFactorAssessment = "NASASTD7009::CapabilityFactorAssessment"
	nasastd7009ResultsFactorAssessment    = "NASASTD7009::ResultsFactorAssessment"
	nasastd7009CapabilityAssessment       = "NASASTD7009::MSCapabilityAssessment"
	nasastd7009ResultsAssessment          = "NASASTD7009::MSResultsAssessment"
	nasastd7009ResultsWarning             = "NASASTD7009::ResultsWarning"
	nasastd7009WarningOccurrence          = "NASASTD7009::WarningOccurrence"
	nasastd7009ResultsReport              = "NASASTD7009::MSResultsReport"
	nasastd7009UncertaintyEstimateKind    = "NASASTD7009::UncertaintyEstimateKind"
	nasastd7009Applicability              = "NASASTD7009::Applicability"
	nasastd7009Applicable                 = "NASASTD7009::Applicable"
	nasastd7009MSRequirement              = "NASASTD7009::MSRequirement"
	nasastd7009MSRequirements             = "NASASTD7009::MSRequirements"
	nasastd7009IdentificationMatrix       = "NASASTD7009::RequirementsIdentificationMatrix"
)

// nasastd7009Assessment is one of the two Appendix E assessments: its record, the
// assessment of one of its factors, the factors, and the requirement asking for it.
type nasastd7009Assessment struct {
	record, factorAssessment, factors string
	noun, requirementID               string
}

var (
	nasastd7009Capability = nasastd7009Assessment{
		record: nasastd7009CapabilityAssessment, factorAssessment: nasastd7009CapabilityFactorAssessment,
		factors: nasastd7009CapabilityFactor, noun: "capability", requirementID: "M&S 48",
	}
	nasastd7009Results = nasastd7009Assessment{
		record: nasastd7009ResultsAssessment, factorAssessment: nasastd7009ResultsFactorAssessment,
		factors: nasastd7009ResultsFactor, noun: "results", requirementID: "M&S 31",
	}
)

type nasastd7009Audit struct {
	*libraryAudit
	// requirements are the [M&S n] requirement definitions, in library order.
	requirements []*symbols.Symbol
	// requirementsLoaded marks requirements as read.
	requirementsLoaded bool
}

// newNASASTD7009Audit returns nil when the bundled NASASTD7009 library is not loaded.
func newNASASTD7009Audit(ctx *Context) *nasastd7009Audit {
	base := newLibraryAudit(ctx, nasastd7009Source,
		nasastd7009CriticalityAssessment, nasastd7009DecisionConsequence, nasastd7009MSInfluence,
		nasastd7009CriticalityRating, nasastd7009FactorLevel, nasastd7009CapabilityFactor, nasastd7009ResultsFactor,
		nasastd7009CapabilityFactorAssessment, nasastd7009ResultsFactorAssessment, nasastd7009CapabilityAssessment,
		nasastd7009ResultsAssessment, nasastd7009ResultsWarning, nasastd7009WarningOccurrence, nasastd7009ResultsReport,
		nasastd7009UncertaintyEstimateKind, nasastd7009Applicability, nasastd7009Applicable, nasastd7009MSRequirement,
		nasastd7009MSRequirements, nasastd7009IdentificationMatrix)
	if base == nil {
		return nil
	}
	return &nasastd7009Audit{libraryAudit: base}
}

func (a *nasastd7009Audit) check(root *symbols.Scope) {
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		a.checkAnnotations(sym)
		if _, ok := sym.Decl.(*ast.Usage); !ok {
			return
		}
		types := a.model.FeatureTypeSet(sym)
		switch {
		case a.anyConformsTo(types, nasastd7009CapabilityAssessment):
			a.checkAssessment(sym, nasastd7009Capability)
		case a.anyConformsTo(types, nasastd7009ResultsAssessment):
			a.checkAssessment(sym, nasastd7009Results)
		case a.anyConformsTo(types, nasastd7009ResultsWarning):
			a.checkWarning(sym)
		case a.anyConformsTo(types, nasastd7009ResultsReport):
			a.checkReport(sym)
		case a.anyConformsTo(types, nasastd7009IdentificationMatrix):
			a.checkMatrix(sym, root)
		}
	})
}

// nested returns the features sym states at any depth, each once.
func (a *nasastd7009Audit) nested(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	visited := map[*symbols.Symbol]bool{sym: true}
	var walk func(owner *symbols.Symbol)
	walk = func(owner *symbols.Symbol) {
		for _, f := range a.features(owner) {
			if visited[f] {
				continue
			}
			visited[f] = true
			out = append(out, f)
			walk(f)
		}
	}
	walk(sym)
	return out
}

// literals returns the literal names of a library enumeration, in declaration order.
func (a *nasastd7009Audit) literals(enumFQN string) []string {
	scope := a.defs[enumFQN].Scope
	if scope == nil {
		return nil
	}
	var out []string
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind != symbols.SymbolAlias && member.IsFeature() && member.Name != "" {
			out = append(out, member.Name)
		}
		return true
	})
	return out
}

// literalFeature reads a constant feature of a library enumeration literal.
func (a *nasastd7009Audit) literalFeature(enumFQN, literal, feature string) (symbols.FilterValue, bool) {
	sym := libraryElement(a.ctx, enumFQN+"::"+literal)
	if sym == nil {
		return symbols.FilterValue{}, false
	}
	values, present, err := a.model.DeclaredFeatureValues(sym, feature)
	if err != nil || !present || len(values) != 1 {
		return symbols.FilterValue{}, false
	}
	return values[0], true
}

// literalInt reads an Integer feature of a library enumeration literal.
func (a *nasastd7009Audit) literalInt(enumFQN, literal, feature string) (int64, bool) {
	v, ok := a.literalFeature(enumFQN, literal, feature)
	if !ok || v.Kind != symbols.FilterValueInt {
		return 0, false
	}
	return v.Int, true
}

// factorTitle names a factor as the Standard does, falling back on the literal name.
func (a *nasastd7009Audit) factorTitle(enumFQN, literal string) string {
	if v, ok := a.literalFeature(enumFQN, literal, "title"); ok && v.Kind == symbols.FilterValueString && v.Str != "" {
		return v.Str
	}
	return literal
}

// stringOf reads a String feature of sym, or "".
func (a *nasastd7009Audit) stringOf(sym *symbols.Symbol, feature string) string {
	values, present, err := a.model.DeclaredFeatureValues(sym, feature)
	if err != nil || !present {
		return ""
	}
	for _, v := range values {
		if v.Kind == symbols.FilterValueString && v.Str != "" {
			return v.Str
		}
	}
	return ""
}

// checkAssessment expects an assessment record to state a level for each of its factors
// ([M&S 48] for the capability assessment, [M&S 31] for the results assessment).
func (a *nasastd7009Audit) checkAssessment(record *symbols.Symbol, kind nasastd7009Assessment) {
	assessed := map[string]bool{}
	for _, f := range a.nested(record) {
		if !a.isA(f, kind.factorAssessment) {
			continue
		}
		if a.literalOf(f, "level", nasastd7009FactorLevel) == "" {
			continue
		}
		if factor := a.literalOf(f, "factor", kind.factors); factor != "" {
			assessed[factor] = true
		}
	}
	factors := a.literals(kind.factors)
	var missing []string
	for _, factor := range factors {
		if !assessed[factor] {
			missing = append(missing, a.factorTitle(kind.factors, factor))
		}
	}
	if len(missing) == 0 {
		return
	}
	a.report(record, CodeNASASTD7009AssessmentMissingFactor, fmt.Sprintf(
		"This M&S %s assessment records no assessed level for %d of its %d factors: %s. NASA-STD-7009B [%s] has the assessment recorded according to each factor, so nest one `%s` per factor with its `factor` and `level`.",
		kind.noun, len(missing), len(factors), strings.Join(missing, ", "), kind.requirementID,
		strings.TrimPrefix(kind.factorAssessment, "NASASTD7009::")))
}

// checkWarning expects a warning of a results report to estimate its impact [M&S 32].
func (a *nasastd7009Audit) checkWarning(warning *symbols.Symbol) {
	if a.stringOf(warning, "impact") != "" {
		return
	}
	a.report(warning, CodeNASASTD7009WarningMissingImpact,
		"This warning states no impact: NASA-STD-7009B [M&S 32] has each warning reported to decision makers accompanied by at least a qualitative estimate of the impact of the occurrence, so give it an `impact`.")
}

// checkReport expects a report of M&S results to hold what section 4.3.8 has it include and
// to warn of every factor assessed below its threshold [M&S 32] a.
func (a *nasastd7009Audit) checkReport(report *symbols.Symbol) {
	nested := a.nested(report)
	capability, results, warned := false, false, false
	var below []string
	for _, f := range nested {
		switch {
		case a.isA(f, nasastd7009CapabilityAssessment):
			capability = true
		case a.isA(f, nasastd7009ResultsAssessment):
			results = true
		case a.isA(f, nasastd7009ResultsWarning):
			warned = warned || a.literalOf(f, "kind", nasastd7009WarningOccurrence) == "unachievedAcceptanceCriteria"
		case a.isA(f, nasastd7009CapabilityFactorAssessment):
			below = a.noteBelowThreshold(below, f, nasastd7009CapabilityFactor)
		case a.isA(f, nasastd7009ResultsFactorAssessment):
			below = a.noteBelowThreshold(below, f, nasastd7009ResultsFactor)
		}
	}
	var missing []string
	if !capability {
		missing = append(missing, "the outcome of the M&S capability assessment [M&S 50]")
	}
	if !results {
		missing = append(missing, "the outcome of the M&S results assessment [M&S 35]")
	}
	estimate := a.literalOf(report, "uncertaintyEstimate", nasastd7009UncertaintyEstimateKind)
	switch {
	case estimate == "":
		missing = append(missing, "an estimate of results uncertainty, or the statement that none is available (`uncertaintyEstimate`) [M&S 33]")
	case estimate != "notAvailable" && a.stringOf(report, "uncertaintyProcess") == "":
		missing = append(missing, "the processes used to obtain the estimate of uncertainty (`uncertaintyProcess`) [M&S 34]")
	}
	if a.stringOf(report, "qualifications") == "" {
		missing = append(missing, "the qualifications of the developers, users, operators or analysts (`qualifications`) [M&S 37]")
	}
	if a.stringOf(report, "riskAcceptance") == "" {
		missing = append(missing, "the assessment of, and rationale for, acceptance of the risks (`riskAcceptance`) [M&S 39]")
	}
	if len(missing) > 0 {
		a.report(report, CodeNASASTD7009ReportMissingContent, fmt.Sprintf(
			"This report of M&S results does not include %s. NASA-STD-7009B section 4.3.8 has each included when reporting M&S results to decision makers.",
			strings.Join(missing, "; ")))
	}
	if len(below) > 0 && !warned {
		a.report(report, CodeNASASTD7009ReportMissingWarning, fmt.Sprintf(
			"This report of M&S results assesses %s below the threshold set for it and carries no warning of it: NASA-STD-7009B [M&S 32] a has unachieved acceptance criteria, which [M&S 43] e makes the assessment level thresholds part of, reported with an explicit warning, so nest a `ResultsWarning` of kind `unachievedAcceptanceCriteria` with its `impact`.",
			strings.Join(below, ", ")))
	}
}

// noteBelowThreshold appends the factor of an assessment whose level is below its threshold.
func (a *nasastd7009Audit) noteBelowThreshold(below []string, f *symbols.Symbol, factors string) []string {
	level, okLevel := a.literalInt(nasastd7009FactorLevel, a.literalOf(f, "level", nasastd7009FactorLevel), "level")
	threshold, okThreshold := a.literalInt(nasastd7009FactorLevel, a.literalOf(f, "threshold", nasastd7009FactorLevel), "level")
	if !okLevel || !okThreshold || level >= threshold {
		return below
	}
	name := a.model.EffectiveNameOf(f)
	if factor := a.literalOf(f, "factor", factors); factor != "" {
		name = a.factorTitle(factors, factor)
	}
	return append(below, fmt.Sprintf("%s (level %d, threshold %d)", name, level, threshold))
}

// checkAnnotations audits the criticality assessments and matrix entries sym carries.
func (a *nasastd7009Audit) checkAnnotations(sym *symbols.Symbol) {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		switch {
		case a.annotationIs(facts, nasastd7009CriticalityAssessment):
			a.checkCriticality(sym, facts)
		case a.annotationIs(facts, nasastd7009Applicability):
			a.checkApplicability(sym, facts)
		}
	}
}

// checkCriticality expects a rating that differs from Figure 1 to record the method used (section 4.1.1.2).
func (a *nasastd7009Audit) checkCriticality(sym *symbols.Symbol, facts symbols.AnnotationFacts) {
	var consequence, influence, rating string
	method := false
	for _, v := range facts.Values {
		switch v.Feature {
		case "consequence":
			consequence = enumLiteral(v.Value, nasastd7009DecisionConsequence)
		case "influence":
			influence = enumLiteral(v.Value, nasastd7009MSInfluence)
		case "rating":
			rating = enumLiteral(v.Value, nasastd7009CriticalityRating)
		case "method":
			method = v.Value.Kind != symbols.FilterValueString || v.Value.Str != ""
		}
	}
	if rating == "" || method {
		return
	}
	matrix := a.matrixRating(consequence, influence)
	if matrix == "" || matrix == rating {
		return
	}
	a.report(sym, CodeNASASTD7009CriticalityRating, fmt.Sprintf(
		"This criticality assessment rates decision consequence %s with M&S influence %s as %s where Figure 1 of NASA-STD-7009B Appendix D rates it %s, and records no method: section 4.1.1.2 has the method used, Appendix D as is or modified or another method, recorded, so correct the `rating` or state the `method`.",
		consequence, influence, rating, matrix))
}

// matrixRating reads the cell of Figure 1 off the library's influence rows, or "".
func (a *nasastd7009Audit) matrixRating(consequence, influence string) string {
	level, ok := a.literalInt(nasastd7009DecisionConsequence, consequence, "level")
	yellowFrom, okYellow := a.literalInt(nasastd7009MSInfluence, influence, "yellowFrom")
	if !ok || !okYellow {
		return ""
	}
	if redFrom, hasRed := a.literalInt(nasastd7009MSInfluence, influence, "redFrom"); hasRed && level >= redFrom {
		return "red"
	}
	if level >= yellowFrom {
		return "yellow"
	}
	return "green"
}

// checkApplicability expects a matrix entry to say why a requirement is not applicable
// (Appendix A.1) and where the record of an applicable requirement is kept (section 4.3.8.8).
func (a *nasastd7009Audit) checkApplicability(sym *symbols.Symbol, facts symbols.AnnotationFacts) {
	applicable := ""
	comments, location := false, false
	for _, v := range facts.Values {
		switch v.Feature {
		case "applicable":
			applicable = enumLiteral(v.Value, nasastd7009Applicable)
		case "comments":
			comments = v.Value.Kind != symbols.FilterValueString || v.Value.Str != ""
		case "recordLocation":
			location = v.Value.Kind != symbols.FilterValueString || v.Value.Str != ""
		}
	}
	switch {
	case applicable == "no" && !comments:
		a.report(sym, CodeNASASTD7009EntryIncomplete,
			"This entry is marked not applicable with no comments: NASA-STD-7009B Appendix A has the Comments column give the explanation or justification when a requirement is not applicable, and section 4.3.8.8 reads the matrix as carrying the rationale if not applicable, so give the `@Applicability` annotation its `comments`.")
	case applicable == "yes" && !location && a.isRequirement(sym):
		a.report(sym, CodeNASASTD7009EntryIncomplete,
			"This entry is marked applicable with no record location: NASA-STD-7009B section 4.3.8.8 says the location of applicable/required records is also necessary for compliance, so give the `@Applicability` annotation a `recordLocation`.")
	}
}

// isRequirement reports whether sym, a usage or a definition, is an [M&S n] requirement.
func (a *nasastd7009Audit) isRequirement(sym *symbols.Symbol) bool {
	if sym.IsFeature() {
		return a.isA(sym, nasastd7009MSRequirement)
	}
	return a.conformsTo(sym, nasastd7009MSRequirement)
}

// loadRequirements reads the [M&S n] requirement definitions off the library, once.
func (a *nasastd7009Audit) loadRequirements() {
	if a.requirementsLoaded {
		return
	}
	a.requirementsLoaded = true
	scope := a.defs[nasastd7009MSRequirements].Scope
	if scope == nil {
		return
	}
	seen := map[*symbols.Symbol]bool{}
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind != symbols.SymbolAlias && member.DeclaresDefinition() && !seen[member] && a.conformsTo(member, nasastd7009MSRequirement) {
			seen[member] = true
			a.requirements = append(a.requirements, member)
		}
		return true
	})
}

// checkMatrix expects the document holding a Requirements Identification Matrix to state
// an entry for every requirement of the Standard (Appendix A, Table 1).
func (a *nasastd7009Audit) checkMatrix(matrix *symbols.Symbol, root *symbols.Scope) {
	a.loadRequirements()
	covered := map[*symbols.Symbol]bool{}
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		if !sym.IsFeature() || !a.hasApplicability(sym) {
			return
		}
		for _, t := range a.model.FeatureTypeSet(sym) {
			for _, req := range a.requirements {
				if t == req || a.model.Conforms(t, req) {
					covered[req] = true
				}
			}
		}
	})
	var missing []string
	for _, req := range a.requirements {
		if covered[req] {
			continue
		}
		id := a.stringOf(req, "id")
		if id == "" {
			id = a.model.EffectiveNameOf(req)
		}
		missing = append(missing, "["+id+"]")
	}
	if len(missing) == 0 {
		return
	}
	a.report(matrix, CodeNASASTD7009MatrixMissingReq, fmt.Sprintf(
		"This Requirements Identification Matrix has no entry for %d of the %d requirements of NASA-STD-7009B: %s. Table 1 lists every requirement, so add an `@Applicability` usage of each from `NASASTD7009::MSRequirements`.",
		len(missing), len(a.requirements), strings.Join(missing, ", ")))
}

// hasApplicability reports whether sym carries an @Applicability annotation.
func (a *nasastd7009Audit) hasApplicability(sym *symbols.Symbol) bool {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if a.annotationIs(facts, nasastd7009Applicability) {
			return true
		}
	}
	return false
}
