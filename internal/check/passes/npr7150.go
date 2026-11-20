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

// NPR7150Pass audits the elements a model classifies with the NPR7150 library against what
// NPR 7150.2D states of them; a model that never touches the library gets no finding.
type NPR7150Pass struct{}

// Diagnostic codes of the NPR 7150.2D rules.
const (
	CodeNPR7150SoftwareUnclassified      = "npr7150-software-missing-classification"
	CodeNPR7150ClassESafetyCritical      = "npr7150-class-e-safety-critical"
	CodeNPR7150MappingNoJustification    = "npr7150-mapping-missing-justification"
	CodeNPR7150MatrixMissingRequirement  = "npr7150-mapping-matrix-missing-requirement"
	CodeNPR7150SafetyCriticalNotApplied  = "npr7150-safety-critical-requirement-not-applicable"
	npr7150Source                        = "npr7150"
	npr7150SoftwareSystem                = "NPR7150::SoftwareSystem"
	npr7150SoftwareClass                 = "NPR7150::SoftwareClass"
	npr7150ClassApplicability            = "NPR7150::ClassApplicability"
	npr7150SafetyCriticalCondition       = "NPR7150::SafetyCriticalSoftwareCondition"
	npr7150SWERequirement                = "NPR7150::SWERequirement"
	npr7150SWERequirements               = "NPR7150::SWERequirements"
	npr7150RequirementMapping            = "NPR7150::RequirementMapping"
	npr7150MappingKind                   = "NPR7150::MappingKind"
	npr7150RequirementsMappingMatrix     = "NPR7150::RequirementsMappingMatrix"
	npr7150SoftwareClassFeature          = "softwareClass"
	npr7150SafetyCriticalFeature         = "safetyCritical"
	npr7150MappingNotApplicable          = "notApplicable"
	npr7150MappingTailored               = "tailored"
	npr7150ClassWithoutSafetyCriticality = "E"
)

// Level reports the tier the pass runs at.
func (NPR7150Pass) Level() PassLevel { return LevelConstraint }

// Run audits one document against the NPR7150 library it uses.
func (NPR7150Pass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
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
	a := newNPR7150Audit(ctx)
	if a == nil {
		return nil
	}
	a.check(rootScope)
	return a.diags
}

// npr7150Requirement is one row of Table 2: a requirement and the classes it is invoked for.
type npr7150Requirement struct {
	def            *symbols.Symbol
	classes        map[string]bool
	safetyCritical bool
}

// npr7150Mapping is one @RequirementMapping entry of the document.
type npr7150Mapping struct {
	usage  *symbols.Symbol
	comply string
}

type npr7150Audit struct {
	ctx   *Context
	model *semantics.Model
	// defs holds the library definitions by qualified name.
	defs map[string]*symbols.Symbol
	// requirements are the rows of Table 2 in library order, filled by loadMatrix.
	requirements []npr7150Requirement
	matrixLoaded bool
	diags        []diag.Diagnostic
}

// newNPR7150Audit returns nil when the bundled NPR7150 library is not loaded:
// a workspace package of the same name is not the NPR's vocabulary.
func newNPR7150Audit(ctx *Context) *npr7150Audit {
	a := &npr7150Audit{ctx: ctx, model: ctx.Model(), defs: map[string]*symbols.Symbol{}}
	for _, fqn := range []string{
		npr7150SoftwareSystem, npr7150SoftwareClass, npr7150ClassApplicability, npr7150SafetyCriticalCondition,
		npr7150SWERequirement, npr7150SWERequirements, npr7150RequirementMapping, npr7150MappingKind,
		npr7150RequirementsMappingMatrix,
	} {
		def := npr7123LibraryElement(ctx, fqn)
		if def == nil {
			return nil
		}
		a.defs[fqn] = def
	}
	return a
}

// loadMatrix reads Table 2 off the library, once, when the document first
// states a mapping matrix or a mapping entry.
func (a *npr7150Audit) loadMatrix() {
	if a.matrixLoaded {
		return
	}
	a.matrixLoaded = true
	scope := a.defs[npr7150SWERequirements].Scope
	if scope == nil {
		return
	}
	seen := map[*symbols.Symbol]bool{}
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolAlias || !member.DeclaresDefinition() || seen[member] || !a.conformsTo(member, npr7150SWERequirement) {
			return true
		}
		seen[member] = true
		row := npr7150Requirement{def: member, classes: map[string]bool{}}
		for _, facts := range a.model.AnnotationFactsOf(member) {
			switch {
			case a.annotationIs(facts, npr7150SafetyCriticalCondition):
				row.safetyCritical = true
			case a.annotationIs(facts, npr7150ClassApplicability):
				for _, v := range facts.Values {
					if class := npr7123Literal(v.Value, npr7150SoftwareClass); v.Feature == npr7150SoftwareClassFeature && class != "" {
						row.classes[class] = true
					}
				}
			}
		}
		a.requirements = append(a.requirements, row)
		return true
	})
}

// annotationIs reports whether an annotation's type is, or conforms to, the library metadata definition.
func (a *npr7150Audit) annotationIs(facts symbols.AnnotationFacts, fqn string) bool {
	def := a.defs[fqn]
	for _, t := range a.ctx.Index.LookupQualified(facts.TypeFQN) {
		if t == def || a.model.Conforms(t, def) {
			return true
		}
	}
	return false
}

// conformsTo reports whether a type is, or specializes, the library definition fqn.
func (a *npr7150Audit) conformsTo(t *symbols.Symbol, fqn string) bool {
	def := a.defs[fqn]
	return t == def || a.model.Conforms(t, def)
}

// anyConformsTo reports whether one of types conforms to the library definition fqn.
func (a *npr7150Audit) anyConformsTo(types []*symbols.Symbol, fqn string) bool {
	for _, t := range types {
		if a.conformsTo(t, fqn) {
			return true
		}
	}
	return false
}

// softwareClasses reads the software class literals sym states, in class order.
func (a *npr7150Audit) softwareClasses(sym *symbols.Symbol) []string {
	values, present, err := a.model.DeclaredFeatureValues(sym, npr7150SoftwareClassFeature)
	if err != nil || !present {
		return nil
	}
	var classes []string
	for _, v := range values {
		if class := npr7123Literal(v, npr7150SoftwareClass); class != "" {
			classes = append(classes, class)
		}
	}
	sort.Strings(classes)
	return classes
}

// safetyCritical reports whether sym states `safetyCritical = true`.
func (a *npr7150Audit) safetyCritical(sym *symbols.Symbol) bool {
	values, present, err := a.model.DeclaredFeatureValues(sym, npr7150SafetyCriticalFeature)
	if err != nil || !present {
		return false
	}
	for _, v := range values {
		if v.Kind == symbols.FilterValueBool && v.Bool {
			return true
		}
	}
	return false
}

// mapping reads the @RequirementMapping annotation of sym; ok is false when it carries none.
func (a *npr7150Audit) mapping(sym *symbols.Symbol) (comply string, justified, ok bool) {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if !a.annotationIs(facts, npr7150RequirementMapping) {
			continue
		}
		ok = true
		for _, v := range facts.Values {
			switch v.Feature {
			case "comply":
				comply = npr7123Literal(v.Value, npr7150MappingKind)
			case "justification":
				justified = v.Value.Kind == symbols.FilterValueString && v.Value.Str != ""
			}
		}
	}
	return comply, justified, ok
}

func (a *npr7150Audit) check(root *symbols.Scope) {
	var matrices, safetyCriticalSystems []*symbols.Symbol
	var mappings []npr7150Mapping
	kit.WalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		comply, justified, mapped := a.mapping(sym)
		if mapped {
			if (comply == npr7150MappingTailored || comply == npr7150MappingNotApplicable) && !justified {
				a.report(sym, CodeNPR7150MappingNoJustification, fmt.Sprintf(
					"This mapping entry is marked %s with no justification: NPR 7150.2D Appendix C has a requirement tailored out of the Requirements Mapping Matrix carry its justification, and section 2.2.5 asks a request for relief for its rationale, so give the `@RequirementMapping` annotation a `justification`.", comply))
			}
		}
		if _, ok := sym.Decl.(*ast.Usage); !ok {
			return
		}
		if mapped {
			mappings = append(mappings, npr7150Mapping{usage: sym, comply: comply})
		}
		types := a.model.FeatureTypeSet(sym)
		switch {
		case a.anyConformsTo(types, npr7150SoftwareSystem):
			if a.checkSoftwareSystem(sym) {
				safetyCriticalSystems = append(safetyCriticalSystems, sym)
			}
		case a.anyConformsTo(types, npr7150RequirementsMappingMatrix):
			matrices = append(matrices, sym)
		}
	})
	if len(matrices) > 0 || (len(safetyCriticalSystems) > 0 && len(mappings) > 0) {
		a.loadMatrix()
	}
	for _, matrix := range matrices {
		a.checkMatrix(matrix, mappings)
	}
	if len(safetyCriticalSystems) > 0 {
		a.checkSafetyCritical(safetyCriticalSystems[0], mappings)
	}
}

// checkSoftwareSystem expects a system containing software to state its class (SWE-020) and a
// Class E system not to be safety-critical (Appendix D); it reports whether the system is safety-critical.
func (a *npr7150Audit) checkSoftwareSystem(system *symbols.Symbol) bool {
	classes := a.softwareClasses(system)
	critical := a.safetyCritical(system)
	if len(classes) == 0 {
		a.report(system, CodeNPR7150SoftwareUnclassified,
			"This system containing software states no software class: NPR 7150.2D section 3.5.1 (SWE-020) has the project manager classify each system and subsystem containing software by the Appendix D definitions, so state it (`:>> softwareClass = SoftwareClass::C;`).")
		return critical
	}
	if critical && len(classes) == 1 && classes[0] == npr7150ClassWithoutSafetyCriticality {
		a.report(system, CodeNPR7150ClassESafetyCritical,
			"This system is Class E and safety-critical: NPR 7150.2D Appendix D states that Class E software cannot be safety-critical software and that safety-critical software has to be classified as Class D or higher, so raise the `softwareClass`.")
	}
	return critical
}

// covers reports whether a mapping entry's usage is typed by the requirement definition.
func (a *npr7150Audit) covers(m npr7150Mapping, req *symbols.Symbol) bool {
	for _, t := range a.model.FeatureTypeSet(m.usage) {
		if t == req || a.model.Conforms(t, req) {
			return true
		}
	}
	return false
}

// checkMatrix expects the document holding a RequirementsMappingMatrix to state a mapping
// entry for every requirement Table 2 marks for one of the matrix's classes.
func (a *npr7150Audit) checkMatrix(matrix *symbols.Symbol, mappings []npr7150Mapping) {
	classes := a.softwareClasses(matrix)
	if len(classes) == 0 {
		a.report(matrix, CodeNPR7150SoftwareUnclassified,
			"This Requirements Mapping Matrix states no software class: NPR 7150.2D Appendix C invokes each requirement by software class, so state the class or classes the matrix is for (`:>> softwareClass = SoftwareClass::C;`).")
		return
	}
	var missing []string
	for _, req := range a.requirements {
		invoked := false
		for _, class := range classes {
			invoked = invoked || req.classes[class]
		}
		if !invoked {
			continue
		}
		covered := false
		for _, m := range mappings {
			if a.covers(m, req.def) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, a.model.EffectiveNameOf(req.def))
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	a.report(matrix, CodeNPR7150MatrixMissingRequirement, fmt.Sprintf(
		"This Requirements Mapping Matrix has no mapping entry for %d NPR 7150.2D requirement(s) Table 2 marks for Class %s: %s. Appendix C has each requirement marked 'X' for the project's software classification addressed in the matrix, so add a `@RequirementMapping` usage of each from `NPR7150::SWERequirements`.",
		len(missing), strings.Join(classes, ", "), strings.Join(missing, ", ")))
}

// checkSafetyCritical expects no safety-critical software requirement to be mapped
// not applicable in a document that states safety-critical software.
func (a *npr7150Audit) checkSafetyCritical(system *symbols.Symbol, mappings []npr7150Mapping) {
	for _, m := range mappings {
		if m.comply != npr7150MappingNotApplicable {
			continue
		}
		for _, req := range a.requirements {
			if req.safetyCritical && a.covers(m, req.def) {
				a.report(m.usage, CodeNPR7150SafetyCriticalNotApplied, fmt.Sprintf(
					"This mapping entry marks %s not applicable, but `%s` is safety-critical software: NPR 7150.2D section 3.7 applies the requirement to a project that has safety-critical software, so map it as complied with or tailored, with the Technical Authority's approval.",
					a.model.EffectiveNameOf(req.def), a.model.EffectiveNameOf(system)))
				break
			}
		}
	}
}

func (a *npr7150Audit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   npr7150Source,
	})
}
