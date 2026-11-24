package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// NPR7123Pass audits the elements a model classifies with the NPR7123 library against what
// NPR 7123.1D states of them; a model that never touches the library gets no finding.
type NPR7123Pass struct{}

// Diagnostic codes of the NPR 7123.1D rules.
const (
	CodeNPR7123ReviewMissingCriteria     = "npr7123-review-missing-entrance-or-success-criteria"
	CodeNPR7123ReviewMissingProduct      = "npr7123-review-missing-minimum-product"
	CodeNPR7123ComplianceNoJustification = "npr7123-compliance-missing-justification"
	CodeNPR7123MatrixMissingRequirement  = "npr7123-compliance-matrix-missing-requirement"
	CodeNPR7123SEMPMissingProcess        = "npr7123-semp-missing-process"
)

const npr7123Source = "npr7123"

// Level reports the tier the pass runs at.
func (NPR7123Pass) Level() PassLevel { return LevelConstraint }

// Run audits one document against the NPR7123 library it uses.
func (NPR7123Pass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
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
	a := newNPR7123Audit(ctx)
	if a == nil {
		return nil
	}
	a.check(rootScope)
	return a.diags
}

// Library qualified names the pass binds to (see engine-contract.json upstream).
const (
	npr7123LifeCycleReview     = "NPR7123::LifeCycleReview"
	npr7123EntranceCriterion   = "NPR7123::EntranceCriterion"
	npr7123SuccessCriterion    = "NPR7123::SuccessCriterion"
	npr7123WorkProduct         = "NPR7123::WorkProduct"
	npr7123MaturityExpectation = "NPR7123::MaturityExpectation"
	npr7123ReviewKind          = "NPR7123::ReviewKind"
	npr7123WorkProductMaturity = "NPR7123::WorkProductMaturity"
	npr7123Compliance          = "NPR7123::Compliance"
	npr7123ComplyKind          = "NPR7123::ComplyKind"
	npr7123ComplianceMatrix    = "NPR7123::ComplianceMatrix"
	npr7123SERequirement       = "NPR7123::SERequirement"
	npr7123SERequirements      = "NPR7123::SERequirements"
	npr7123SEMP                = "NPR7123::SEMP"
	npr7123TechnicalProcess    = "NPR7123::TechnicalProcess"
)

// npr7123MinimumProduct is one required cell of Table 5-1: the product a
// review kind must present and the maturity section 5.2.2.2 asks for.
type npr7123MinimumProduct struct {
	product       *symbols.Symbol
	maturity      string
	requirementID string
}

type npr7123Audit struct {
	*libraryAudit
	// minimumProducts maps a review kind literal name to its required products;
	// it, requirements and processes are filled by loadTables.
	minimumProducts map[string][]npr7123MinimumProduct
	// requirements are the active SE requirement definitions, in library order.
	requirements []*symbols.Symbol
	// processes are the 17 common technical process definitions, in library order.
	processes []*symbols.Symbol
	// tablesLoaded marks minimumProducts, requirements and processes as read.
	tablesLoaded bool
}

// newNPR7123Audit returns nil when the bundled NPR7123 library is not loaded.
func newNPR7123Audit(ctx *Context) *npr7123Audit {
	base := newLibraryAudit(ctx, npr7123Source,
		npr7123LifeCycleReview, npr7123EntranceCriterion, npr7123SuccessCriterion, npr7123WorkProduct,
		npr7123MaturityExpectation, npr7123ReviewKind, npr7123WorkProductMaturity, npr7123Compliance,
		npr7123ComplyKind, npr7123ComplianceMatrix, npr7123SERequirement, npr7123SERequirements,
		npr7123SEMP, npr7123TechnicalProcess)
	if base == nil || base.defs[npr7123WorkProduct].OwnerScope == nil {
		return nil
	}
	return &npr7123Audit{libraryAudit: base, minimumProducts: map[string][]npr7123MinimumProduct{}}
}

// loadTables reads Table 5-1, the SE requirements and the technical processes off the
// library, once, when the document first states a review, a SEMP or a compliance matrix.
func (a *npr7123Audit) loadTables() {
	if a.tablesLoaded {
		return
	}
	a.tablesLoaded = true
	library := a.defs[npr7123WorkProduct].OwnerScope
	seen := map[*symbols.Symbol]bool{}
	library.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolAlias || !member.DeclaresDefinition() || seen[member] {
			return true
		}
		seen[member] = true
		switch {
		case member != a.defs[npr7123TechnicalProcess] && a.conformsTo(member, npr7123TechnicalProcess):
			a.processes = append(a.processes, member)
		case member != a.defs[npr7123WorkProduct] && a.conformsTo(member, npr7123WorkProduct):
			a.noteMinimumProducts(member)
		}
		return true
	})
	if scope := a.defs[npr7123SERequirements].Scope; scope != nil {
		scope.ForEachMember(func(member *symbols.Symbol) bool {
			if member.Kind != symbols.SymbolAlias && member.DeclaresDefinition() && !seen[member] && a.conformsTo(member, npr7123SERequirement) {
				seen[member] = true
				a.requirements = append(a.requirements, member)
			}
			return true
		})
	}
}

// noteMinimumProducts records the required cells of Table 5-1 a work product
// definition carries as @MaturityExpectation annotations.
func (a *npr7123Audit) noteMinimumProducts(product *symbols.Symbol) {
	for _, facts := range a.model.AnnotationFactsOf(product) {
		if !a.annotationIs(facts, npr7123MaturityExpectation) {
			continue
		}
		required := false
		var review, maturity, requirementID string
		for _, v := range facts.Values {
			switch v.Feature {
			case "required":
				required = v.Value.Kind == symbols.FilterValueBool && v.Value.Bool
			case "review":
				review = enumLiteral(v.Value, npr7123ReviewKind)
			case "maturity":
				maturity = enumLiteral(v.Value, npr7123WorkProductMaturity)
			case "requirementId":
				if v.Value.Kind == symbols.FilterValueString {
					requirementID = v.Value.Str
				}
			}
		}
		if required && review != "" && maturity != "" {
			a.minimumProducts[review] = append(a.minimumProducts[review],
				npr7123MinimumProduct{product: product, maturity: maturity, requirementID: requirementID})
		}
	}
}

// statesAs reports whether f subsets, redefines or references, directly or through
// another feature, the library's LifeCycleReview feature named name.
func (a *npr7123Audit) statesAs(f *symbols.Symbol, name string) bool {
	target, ok := a.model.LookupMember(a.defs[npr7123LifeCycleReview], name)
	return ok && target != nil && a.specializesFeature(f, target, map[*symbols.Symbol]bool{})
}

func (a *npr7123Audit) specializesFeature(f, target *symbols.Symbol, seen map[*symbols.Symbol]bool) bool {
	if f == nil || seen[f] {
		return false
	}
	seen[f] = true
	if f == target {
		return true
	}
	usage, ok := f.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines && rel.Kind != ast.RelReferences) {
			continue
		}
		if t, ok := a.ctx.Resolver().ResolveTarget(f.OwnerScope, rel.Target); ok && a.specializesFeature(t, target, seen) {
			return true
		}
	}
	return false
}

func (a *npr7123Audit) check(root *symbols.Scope) {
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		a.checkCompliance(sym)
		if _, ok := sym.Decl.(*ast.Usage); !ok {
			return
		}
		// Every usage of every document comes through here, so its types are derived once.
		types := a.model.FeatureTypeSet(sym)
		switch {
		case a.anyConformsTo(types, npr7123LifeCycleReview):
			a.checkReview(sym)
		case a.anyConformsTo(types, npr7123SEMP):
			if !a.reviewedProduct(sym) {
				a.checkSEMP(sym)
			}
		case a.anyConformsTo(types, npr7123ComplianceMatrix):
			a.checkMatrix(sym, root)
		}
	})
}

// checkReview expects a life-cycle review to state entrance and success criteria (section
// 5.2.2.1, SE-34) and the section 5.2.2.2 minimum products of its kind at the maturity asked for.
func (a *npr7123Audit) checkReview(review *symbols.Symbol) {
	a.loadTables()
	nested := a.features(review)
	entrance, success := false, false
	for _, f := range nested {
		entrance = entrance || (a.isA(f, npr7123EntranceCriterion) && a.statesAs(f, "entranceCriteria"))
		success = success || (a.isA(f, npr7123SuccessCriterion) && a.statesAs(f, "successCriteria"))
	}
	switch {
	case !entrance && !success:
		a.report(review, CodeNPR7123ReviewMissingCriteria,
			"This life-cycle review states no entrance criteria and no success criteria: NPR 7123.1D section 5.2.2.1 (SE-34) has the technical team define both for each review, so nest `EntranceCriterion` and `SuccessCriterion` requirement usages, subsetting `entranceCriteria` and `successCriteria`, from the library's `ReviewCriteria` package.")
	case !entrance:
		a.report(review, CodeNPR7123ReviewMissingCriteria,
			"This life-cycle review states no entrance criteria: NPR 7123.1D section 5.2.2.1 (SE-34) has the technical team define entrance criteria for each review, so nest `EntranceCriterion` requirement usages subsetting `entranceCriteria`.")
	case !success:
		a.report(review, CodeNPR7123ReviewMissingCriteria,
			"This life-cycle review states no success criteria: NPR 7123.1D section 5.2.2.1 (SE-34) has the technical team define success criteria for each review, so nest `SuccessCriterion` requirement usages subsetting `successCriteria`.")
	}
	kind := a.literalOf(review, "kind", npr7123ReviewKind)
	for _, minimum := range a.minimumProducts[kind] {
		var stated *symbols.Symbol
		maturity := ""
		for _, f := range nested {
			if !a.isA(f, npr7123WorkProduct) || !a.isAProduct(f, minimum.product) || !a.statesAs(f, "reviewedProducts") {
				continue
			}
			m := a.literalOf(f, "maturity", npr7123WorkProductMaturity)
			if stated == nil || npr7123StatedRank(m) > npr7123StatedRank(maturity) {
				stated, maturity = f, m
			}
		}
		name := a.model.EffectiveNameOf(minimum.product)
		if stated == nil {
			a.report(review, CodeNPR7123ReviewMissingProduct, fmt.Sprintf(
				"This %s states no %s: NPR 7123.1D section 5.2.2.2 (%s) has the technical team provide it at this review at maturity %s, so nest a `%s` item subsetting `reviewedProducts` with its `maturity`.",
				kind, name, minimum.requirementID, minimum.maturity, name))
			continue
		}
		if maturity != "" && npr7123MaturityRank(maturity) < npr7123MaturityRank(minimum.maturity) {
			target, inherited := stated, ""
			if stated.OwnerScope == nil || stated.OwnerScope.Owner() != review {
				target, inherited = review, a.inheritedFrom(stated)
			}
			a.report(target, CodeNPR7123ReviewMissingProduct, fmt.Sprintf(
				"This %s%s is stated at maturity %s, below the %s NPR 7123.1D section 5.2.2.2 (%s) asks for at %s; mature the product or document the tailoring in the SEMP.",
				name, inherited, maturity, minimum.maturity, minimum.requirementID, kind))
		}
	}
}

// inheritedFrom names the owner a feature is inherited from, as ", inherited from <owner>".
func (a *npr7123Audit) inheritedFrom(f *symbols.Symbol) string {
	if f.OwnerScope == nil || f.OwnerScope.Owner() == nil {
		return ""
	}
	return ", inherited from " + a.model.EffectiveNameOf(f.OwnerScope.Owner())
}

// isAProduct reports whether a feature is typed by the given work product definition or a specialization of it.
func (a *npr7123Audit) isAProduct(f, product *symbols.Symbol) bool {
	for _, t := range a.model.FeatureTypeSet(f) {
		if t == product || a.model.Conforms(t, product) {
			return true
		}
	}
	return false
}

// npr7123StatedRank ranks a stated product for the minimum-product check: one
// stating no maturity is not below any, so it outranks every stated maturity.
func npr7123StatedRank(maturity string) int {
	if maturity == "" {
		return 4
	}
	return npr7123MaturityRank(maturity)
}

// npr7123MaturityRank orders the Appendix F vocabulary: preliminary or initial, then baselined
// or approved, then final or an update of it; "approach" states no product yet.
func npr7123MaturityRank(maturity string) int {
	switch maturity {
	case "initial", "preliminary":
		return 1
	case "approve", "baseline":
		return 2
	case "final", "update":
		return 3
	}
	return 0
}

// checkCompliance expects a @Compliance marked tailored or notApplicable to
// state the justification Table H-1 asks for.
func (a *npr7123Audit) checkCompliance(sym *symbols.Symbol) {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if !a.annotationIs(facts, npr7123Compliance) {
			continue
		}
		comply, justified := "", false
		for _, v := range facts.Values {
			switch v.Feature {
			case "comply":
				comply = enumLiteral(v.Value, npr7123ComplyKind)
			case "justification":
				justified = v.Value.Kind == symbols.FilterValueString && v.Value.Str != ""
			}
		}
		if (comply == "tailored" || comply == "notApplicable") && !justified {
			a.report(sym, CodeNPR7123ComplianceNoJustification, fmt.Sprintf(
				"This compliance entry is marked %s with no justification: NPR 7123.1D Table H-1 asks for the justification of every tailored or not-applicable requirement, so give the `@Compliance` annotation a `justification`.", comply))
		}
	}
}

// checkMatrix expects the document holding a ComplianceMatrix to state a
// compliance entry for every active SE requirement.
func (a *npr7123Audit) checkMatrix(matrix *symbols.Symbol, root *symbols.Scope) {
	a.loadTables()
	covered := map[*symbols.Symbol]bool{}
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		if !sym.IsFeature() || !a.hasCompliance(sym) {
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
		if !covered[req] {
			missing = append(missing, a.model.EffectiveNameOf(req))
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	a.report(matrix, CodeNPR7123MatrixMissingRequirement, fmt.Sprintf(
		"This compliance matrix has no compliance entry for %d active NPR 7123.1D requirement(s): %s. Table H-1 covers every active requirement, so add a `@Compliance` usage of each from `NPR7123::SERequirements`.",
		len(missing), strings.Join(missing, ", ")))
}

// hasCompliance reports whether sym carries a @Compliance annotation.
func (a *npr7123Audit) hasCompliance(sym *symbols.Symbol) bool {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if a.annotationIs(facts, npr7123Compliance) {
			return true
		}
	}
	return false
}

// reviewedProduct reports whether sym is a product a life-cycle review states
// it reviews: a stand-in for the product at the review, not the product itself.
func (a *npr7123Audit) reviewedProduct(sym *symbols.Symbol) bool {
	owner := sym.OwnerScope
	return owner != nil && a.isReview(owner.Owner()) && a.statesAs(sym, "reviewedProducts")
}

// isReview reports whether sym, a usage or a definition, is a life-cycle review.
func (a *npr7123Audit) isReview(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.IsFeature() {
		return a.isA(sym, npr7123LifeCycleReview)
	}
	return a.conformsTo(sym, npr7123LifeCycleReview)
}

// checkSEMP expects a SEMP to address each of the 17 common technical processes (SE-58).
func (a *npr7123Audit) checkSEMP(semp *symbols.Symbol) {
	a.loadTables()
	addressed := map[*symbols.Symbol]bool{}
	visited := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		if visited[sym] {
			return
		}
		visited[sym] = true
		for _, f := range a.features(sym) {
			for _, t := range a.model.FeatureTypeSet(f) {
				for _, p := range a.processes {
					if t == p || a.model.Conforms(t, p) {
						addressed[p] = true
					}
				}
			}
			walk(f)
		}
	}
	walk(semp)
	var missing []string
	for _, p := range a.processes {
		if !addressed[p] {
			missing = append(missing, a.model.EffectiveNameOf(p))
		}
	}
	if len(missing) == 0 {
		return
	}
	a.report(semp, CodeNPR7123SEMPMissingProcess, fmt.Sprintf(
		"This SEMP does not address %d of the 17 common technical processes: %s. NPR 7123.1D section 6.2.3 (SE-58) has the SEMP define how each process, as tailored, is applied, so nest one usage per process (e.g. `#%s action ...`).",
		len(missing), strings.Join(missing, ", "), npr7123Keyword(missing[0])))
}

// npr7123Keyword spells the library keyword of a definition name: lowerCamel of the name.
func npr7123Keyword(name string) string {
	if name == "" {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}
