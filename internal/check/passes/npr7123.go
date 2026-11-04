package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// NPR7123Pass audits the elements a model classifies with the NPR7123 library
// against what NPR 7123.1D states of them; each rule fires only on an element
// of the kind the NPR speaks about, and a model that never touches the library gets no finding.
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
	ctx   *Context
	model *semantics.Model
	// defs holds the library definitions by qualified name.
	defs map[string]*symbols.Symbol
	// minimumProducts maps a review kind literal name to its required products.
	minimumProducts map[string][]npr7123MinimumProduct
	// requirements are the active SE requirement definitions, in library order.
	requirements []*symbols.Symbol
	// processes are the 17 common technical process definitions, in library order.
	processes []*symbols.Symbol
	// conforms memoizes conformance of a type to a library definition.
	conforms map[[2]*symbols.Symbol]bool
	diags    []diag.Diagnostic
}

// newNPR7123Audit returns nil when the bundled NPR7123 library is not loaded:
// a workspace package of the same name is not the NPR's vocabulary.
func newNPR7123Audit(ctx *Context) *npr7123Audit {
	a := &npr7123Audit{
		ctx:             ctx,
		model:           ctx.Model(),
		defs:            map[string]*symbols.Symbol{},
		minimumProducts: map[string][]npr7123MinimumProduct{},
		conforms:        map[[2]*symbols.Symbol]bool{},
	}
	for _, fqn := range []string{
		npr7123LifeCycleReview, npr7123EntranceCriterion, npr7123SuccessCriterion, npr7123WorkProduct,
		npr7123MaturityExpectation, npr7123ReviewKind, npr7123WorkProductMaturity, npr7123Compliance,
		npr7123ComplyKind, npr7123ComplianceMatrix, npr7123SERequirement, npr7123SERequirements,
		npr7123SEMP, npr7123TechnicalProcess,
	} {
		def := npr7123LibraryElement(ctx, fqn)
		if def == nil {
			return nil
		}
		a.defs[fqn] = def
	}
	library := a.defs[npr7123WorkProduct].OwnerScope
	if library == nil {
		return nil
	}
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
	return a
}

// npr7123LibraryElement returns the bundled library element registered under fqn, or nil.
func npr7123LibraryElement(ctx *Context, fqn string) *symbols.Symbol {
	for _, def := range ctx.Index.LookupQualified(fqn) {
		if def != nil && ctx.Index.Library(def) {
			return def
		}
	}
	return nil
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
				review = npr7123Literal(v.Value, npr7123ReviewKind)
			case "maturity":
				maturity = npr7123Literal(v.Value, npr7123WorkProductMaturity)
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

// npr7123Literal returns the literal name of an enumeration value of enumFQN, or "".
func npr7123Literal(v symbols.FilterValue, enumFQN string) string {
	if v.Kind != symbols.FilterValueRef || !strings.HasPrefix(v.RefFQN, enumFQN+"::") {
		return ""
	}
	return strings.TrimPrefix(v.RefFQN, enumFQN+"::")
}

// annotationIs reports whether an annotation's type is, or conforms to, the library metadata definition.
func (a *npr7123Audit) annotationIs(facts symbols.AnnotationFacts, fqn string) bool {
	def := a.defs[fqn]
	for _, t := range a.ctx.Index.LookupQualified(facts.TypeFQN) {
		if t == def || a.model.Conforms(t, def) {
			return true
		}
	}
	return false
}

// conformsTo reports whether a type is, or specializes, the library definition fqn.
func (a *npr7123Audit) conformsTo(t *symbols.Symbol, fqn string) bool {
	def := a.defs[fqn]
	key := [2]*symbols.Symbol{t, def}
	if ok, seen := a.conforms[key]; seen {
		return ok
	}
	ok := t == def || a.model.Conforms(t, def)
	a.conforms[key] = ok
	return ok
}

// isA reports whether a feature is typed, directly or through a keyword, by a
// type conforming to the library definition fqn.
func (a *npr7123Audit) isA(sym *symbols.Symbol, fqn string) bool {
	if sym == nil || !sym.IsFeature() {
		return false
	}
	for _, t := range a.model.FeatureTypeSet(sym) {
		if a.conformsTo(t, fqn) {
			return true
		}
	}
	return false
}

// nestedFeatures returns the features declared in sym's own body.
func (a *npr7123Audit) nestedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind != symbols.SymbolAlias && member.IsFeature() && member.OwnerScope == sym.Scope && !seen[member] {
			seen[member] = true
			out = append(out, member)
		}
		return true
	})
	return out
}

// literalOf reads an enumeration-valued feature of sym as its literal name, or "".
func (a *npr7123Audit) literalOf(sym *symbols.Symbol, feature, enumFQN string) string {
	values, present, err := a.model.DeclaredFeatureValues(sym, feature)
	if err != nil || !present {
		return ""
	}
	for _, v := range values {
		if lit := npr7123Literal(v, enumFQN); lit != "" {
			return lit
		}
	}
	return ""
}

func (a *npr7123Audit) check(root *symbols.Scope) {
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		a.checkCompliance(sym)
		if _, ok := sym.Decl.(*ast.Usage); !ok {
			return
		}
		switch {
		case a.isA(sym, npr7123LifeCycleReview):
			a.checkReview(sym)
		case a.isA(sym, npr7123SEMP):
			if !a.reviewedProduct(sym) {
				a.checkSEMP(sym)
			}
		case a.isA(sym, npr7123ComplianceMatrix):
			a.checkMatrix(sym, root)
		}
	})
}

// checkReview expects a life-cycle review to state entrance and success
// criteria (section 5.2.2.1, SE-34) and the minimum products of section
// 5.2.2.2 for its kind at the maturity the section asks for.
func (a *npr7123Audit) checkReview(review *symbols.Symbol) {
	nested := a.nestedFeatures(review)
	entrance, success := false, false
	for _, f := range nested {
		entrance = entrance || a.isA(f, npr7123EntranceCriterion)
		success = success || a.isA(f, npr7123SuccessCriterion)
	}
	switch {
	case !entrance && !success:
		a.report(review, CodeNPR7123ReviewMissingCriteria,
			"This life-cycle review states no entrance criteria and no success criteria: NPR 7123.1D section 5.2.2.1 (SE-34) has the technical team define both for each review, so nest `EntranceCriterion` and `SuccessCriterion` requirement usages, subsetting `entranceCriteria` and `successCriteria`, from `NPR7123::ReviewCriteria`.")
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
		for _, f := range nested {
			if a.isA(f, npr7123WorkProduct) && a.isAProduct(f, minimum.product) {
				stated = f
				break
			}
		}
		name := a.model.EffectiveNameOf(minimum.product)
		if stated == nil {
			a.report(review, CodeNPR7123ReviewMissingProduct, fmt.Sprintf(
				"This %s states no %s: NPR 7123.1D section 5.2.2.2 (%s) has the technical team provide it at this review at maturity %s, so nest a `%s` item subsetting `reviewedProducts` with its `maturity`.",
				kind, name, minimum.requirementID, minimum.maturity, name))
			continue
		}
		maturity := a.literalOf(stated, "maturity", npr7123WorkProductMaturity)
		if maturity != "" && npr7123MaturityRank(maturity) < npr7123MaturityRank(minimum.maturity) {
			a.report(stated, CodeNPR7123ReviewMissingProduct, fmt.Sprintf(
				"This %s is stated at maturity %s, below the %s NPR 7123.1D section 5.2.2.2 (%s) asks for at %s; mature the product or document the tailoring in the SEMP.",
				name, maturity, minimum.maturity, minimum.requirementID, kind))
		}
	}
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

// npr7123MaturityRank orders the Appendix F vocabulary: a preliminary or
// initial product precedes a baselined or approved one, which precedes its
// final form or an update of it; "approach" states no product yet.
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
				comply = npr7123Literal(v.Value, npr7123ComplyKind)
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
	return owner != nil && a.isA(owner.Owner(), npr7123LifeCycleReview)
}

// checkSEMP expects a SEMP to address each of the 17 common technical processes (SE-58).
func (a *npr7123Audit) checkSEMP(semp *symbols.Symbol) {
	addressed := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		for _, f := range a.nestedFeatures(sym) {
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

func (a *npr7123Audit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   npr7123Source,
	})
}
