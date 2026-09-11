package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MOSAPass audits the elements a model classifies with the MOSA library for the
// openness MOSA asks of them; each rule fires only once the model states that kind of fact.
type MOSAPass struct{}

// Diagnostic codes of the MOSA rules.
const (
	CodeMOSAInterfaceNoStandard    = "mosa-interface-no-standard"
	CodeMOSAInterfaceNoControl     = "mosa-interface-no-control"
	CodeMOSAInterfaceNotTraced     = "mosa-interface-not-traced"
	CodeMOSAComponentNoDataRights  = "mosa-component-no-data-rights"
	CodeMOSAProprietaryNoRationale = "mosa-proprietary-no-rationale"
	CodeMOSABoundaryNotDesignated  = "mosa-boundary-not-designated"
)

const mosaSource = "mosa"

// Level reports the tier the pass runs at.
func (MOSAPass) Level() PassLevel { return LevelConstraint }

// Run audits one workspace document against the MOSA rules.
func (MOSAPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
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
	a := newMOSAAudit(ctx)
	if a == nil {
		return nil
	}
	gathered := map[*symbols.Scope]bool{}
	for _, doc := range ctx.Index.WorkspaceDocuments() {
		if r := ctx.Index.DocumentRoot(doc); r != nil && !gathered[r] {
			gathered[r] = true
			a.gather(r)
		}
	}
	if !gathered[rootScope] {
		a.gather(rootScope)
	}
	a.check(rootScope)
	return a.diags
}

// mosaKind is a MOSA element class, told by conformance to its library definition.
type mosaKind int

const (
	mosaNone mosaKind = iota
	mosaMajorSystemPlatform
	mosaMajorSystemComponent
	mosaModularSystem
	mosaModularSystemInterface
	mosaStandard
	mosaInterfaceRequirement
)

// mosaKindDefinitions lists the library definition each kind conforms to.
var mosaKindDefinitions = []struct {
	kind mosaKind
	fqn  string
}{
	{mosaMajorSystemPlatform, "MOSA::MajorSystemPlatform"},
	{mosaMajorSystemComponent, "MOSA::MajorSystemComponent"},
	{mosaModularSystem, "MOSA::ModularSystem"},
	{mosaModularSystemInterface, "MOSA::ModularSystemInterface"},
	{mosaStandard, "MOSA::Standard"},
	{mosaInterfaceRequirement, "MOSA::InterfaceRequirement"},
}

var mosaKindNames = map[mosaKind]string{
	mosaMajorSystemPlatform:    "major system platform",
	mosaMajorSystemComponent:   "major system component",
	mosaModularSystem:          "modular system",
	mosaModularSystemInterface: "modular system interface",
	mosaStandard:               "standard",
	mosaInterfaceRequirement:   "interface requirement",
}

// mosaMetadataKind is a MOSA metadata family, told by conformance to its library definition.
type mosaMetadataKind int

const (
	mosaNoMetadata mosaMetadataKind = iota
	mosaDataRights
	mosaProprietary
	mosaInterfaceControl
	mosaConformanceMetadata
	mosaConformantMetadata
	mosaConformsToMetadata
)

// mosaMetadataDefinitions lists the library metadata definition each family conforms to.
var mosaMetadataDefinitions = []struct {
	kind mosaMetadataKind
	fqn  string
}{
	{mosaDataRights, "MOSA::DataRights"},
	{mosaProprietary, "MOSA::Proprietary"},
	{mosaInterfaceControl, "MOSA::InterfaceControl"},
	{mosaConformanceMetadata, "MOSA::StandardConformanceMetadata"},
	{mosaConformantMetadata, "MOSA::ConformantMetadata"},
	{mosaConformsToMetadata, "MOSA::ConformedStandardMetadata"},
}

// mosaMarks are the MOSA annotations an element carries; interfaceControl
// holds only for an @InterfaceControl that names a non-empty authority.
type mosaMarks struct {
	dataRights, proprietary, interfaceControl bool
}

type mosaAudit struct {
	ctx   *Context
	model *semantics.Model
	// definitions and metadata hold the library definition of each MOSA kind and metadata family.
	definitions map[mosaKind]*symbols.Symbol
	metadata    map[mosaMetadataKind]*symbols.Symbol
	// metadataKinds memoizes metadataKindOf by the annotation type's qualified name.
	metadataKinds map[string]mosaMetadataKind
	// present records the kinds the workspace declares at all.
	present map[mosaKind]bool
	// marks caches each element's annotations; anyMarks records those stated anywhere.
	marks    map[*symbols.Symbol]mosaMarks
	anyMarks mosaMarks
	// conformant: elements at a #conformant end; satisfiers: elements a satisfy traces.
	conformant map[symbols.ElementKey]bool
	satisfiers map[symbols.ElementKey]bool
	kinds      map[*symbols.Symbol]mosaKind
	// typeKinds memoizes kindOfType: one type classifies every feature it types.
	typeKinds map[*symbols.Symbol]mosaKind
	diags     []Diagnostic
}

// newMOSAAudit returns nil when the bundled MOSA library is not loaded: a
// workspace package of the same name is not the approach's vocabulary.
func newMOSAAudit(ctx *Context) *mosaAudit {
	a := &mosaAudit{
		ctx:           ctx,
		model:         ctx.Model(),
		definitions:   map[mosaKind]*symbols.Symbol{},
		metadata:      map[mosaMetadataKind]*symbols.Symbol{},
		metadataKinds: map[string]mosaMetadataKind{},
		present:       map[mosaKind]bool{},
		marks:         map[*symbols.Symbol]mosaMarks{},
		conformant:    map[symbols.ElementKey]bool{},
		satisfiers:    map[symbols.ElementKey]bool{},
		kinds:         map[*symbols.Symbol]mosaKind{},
		typeKinds:     map[*symbols.Symbol]mosaKind{},
	}
	for _, entry := range mosaKindDefinitions {
		if def := mosaLibraryDefinition(ctx, entry.fqn); def != nil {
			a.definitions[entry.kind] = def
		}
	}
	for _, entry := range mosaMetadataDefinitions {
		if def := mosaLibraryDefinition(ctx, entry.fqn); def != nil {
			a.metadata[entry.kind] = def
		}
	}
	if len(a.definitions) == 0 && len(a.metadata) == 0 {
		return nil
	}
	return a
}

// mosaLibraryDefinition returns the bundled library element registered under fqn, or nil.
func mosaLibraryDefinition(ctx *Context, fqn string) *symbols.Symbol {
	for _, def := range ctx.Index.LookupQualified(fqn) {
		if def != nil && ctx.Index.Library(def) {
			return def
		}
	}
	return nil
}

// metadataKindOf classifies an annotation by its type's qualified name: the
// family whose library definition the type is or conforms to.
func (a *mosaAudit) metadataKindOf(typeFQN string) mosaMetadataKind {
	if kind, ok := a.metadataKinds[typeFQN]; ok {
		return kind
	}
	kind := mosaNoMetadata
	types := a.ctx.Index.LookupQualified(typeFQN)
families:
	for _, entry := range mosaMetadataDefinitions {
		def := a.metadata[entry.kind]
		if def == nil {
			continue
		}
		for _, t := range types {
			if t == def || a.model.Conforms(t, def) {
				kind = entry.kind
				break families
			}
		}
	}
	a.metadataKinds[typeFQN] = kind
	return kind
}

// kindOf classifies sym: a usage by the types it has, a definition by itself.
func (a *mosaAudit) kindOf(sym *symbols.Symbol) mosaKind {
	if sym == nil {
		return mosaNone
	}
	if kind, ok := a.kinds[sym]; ok {
		return kind
	}
	a.kinds[sym] = mosaNone
	var types []*symbols.Symbol
	if sym.IsFeature() {
		types = a.model.FeatureTypeSet(sym)
	} else if _, ok := sym.Decl.(*ast.Definition); ok {
		types = []*symbols.Symbol{sym}
	}
	kind := mosaNone
	for _, t := range types {
		if kind = a.kindOfType(t); kind != mosaNone {
			break
		}
	}
	a.kinds[sym] = kind
	return kind
}

func (a *mosaAudit) kindOfType(t *symbols.Symbol) mosaKind {
	if kind, ok := a.typeKinds[t]; ok {
		return kind
	}
	kind := mosaNone
	for _, entry := range mosaKindDefinitions {
		if def := a.definitions[entry.kind]; def != nil && a.model.Conforms(t, def) {
			kind = entry.kind
			break
		}
	}
	a.typeKinds[t] = kind
	return kind
}

// marksOf returns the MOSA annotations sym itself carries.
func (a *mosaAudit) marksOf(sym *symbols.Symbol) mosaMarks {
	if m, ok := a.marks[sym]; ok {
		return m
	}
	var m mosaMarks
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		switch a.metadataKindOf(facts.TypeFQN) {
		case mosaDataRights:
			m.dataRights = true
		case mosaProprietary:
			m.proprietary = true
		case mosaInterfaceControl:
			m.interfaceControl = m.interfaceControl || mosaStatesString(facts, "authority")
		}
	}
	a.marks[sym] = m
	return m
}

// effectiveMarks returns the annotations sym or anything it specializes carry:
// its types, the features it subsets and their supertypes in turn.
func (a *mosaAudit) effectiveMarks(sym *symbols.Symbol) mosaMarks {
	m := a.marksOf(sym)
	for _, t := range a.model.AllSupertypes(sym) {
		tm := a.marksOf(t)
		m.dataRights = m.dataRights || tm.dataRights
		m.proprietary = m.proprietary || tm.proprietary
		m.interfaceControl = m.interfaceControl || tm.interfaceControl
	}
	return m
}

// gather records the kinds and annotations a document declares and the
// conformances and satisfactions it states.
func (a *mosaAudit) gather(root *symbols.Scope) {
	w8dWalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		if kind := a.kindOf(sym); kind != mosaNone {
			a.present[kind] = true
		}
		m := a.marksOf(sym)
		a.anyMarks.dataRights = a.anyMarks.dataRights || m.dataRights
		a.anyMarks.proprietary = a.anyMarks.proprietary || m.proprietary
		a.anyMarks.interfaceControl = a.anyMarks.interfaceControl || m.interfaceControl
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok {
			return
		}
		switch {
		case usage.Kind == ast.UsageConnection:
			a.gatherConformance(sym)
		case usage.Kind == ast.UsageSatisfy && usage.Keyword != "verify":
			a.gatherSatisfaction(sym, usage)
		}
	})
}

// gatherConformance records the elements at the `#conformant` ends of a
// `#conformance` connection whose `#conformsTo` ends name at least one standard.
func (a *mosaAudit) gatherConformance(sym *symbols.Symbol) {
	if !a.annotatedWith(sym, mosaConformanceMetadata) {
		return
	}
	var conformant []*symbols.Symbol
	namesStandard := false
	for _, end := range a.bodyEnds(sym) {
		u, ok := end.Decl.(*ast.Usage)
		if !ok {
			continue
		}
		switch {
		case a.annotatedWith(end, mosaConformsToMetadata):
			for _, target := range a.referents(end, u) {
				namesStandard = namesStandard || a.kindOf(target) == mosaStandard
			}
		case a.annotatedWith(end, mosaConformantMetadata):
			conformant = append(conformant, a.referents(end, u)...)
		}
	}
	if !namesStandard {
		return
	}
	for _, target := range conformant {
		a.conformant[symbols.KeyOf(target)] = true
	}
}

// gatherSatisfaction records what a `satisfy` says satisfies its requirement:
// the element after `by`, else the element whose body declares the satisfy.
func (a *mosaAudit) gatherSatisfaction(sym *symbols.Symbol, usage *ast.Usage) {
	if usage.IsNegated {
		return
	}
	named := false
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubject {
			continue
		}
		named = true
		if target, ok := a.ctx.Resolver().ResolveTarget(sym.OwnerScope, rel.Target); ok && target != nil {
			a.satisfiers[symbols.KeyOf(target)] = true
		}
	}
	if named || sym.OwnerScope == nil {
		return
	}
	if owner := sym.OwnerScope.Owner(); owner != nil && owner.IsFeature() {
		a.satisfiers[symbols.KeyOf(owner)] = true
	}
}

// bodyEnds lists the `end` members a connector declares, in declaration order.
func (a *mosaAudit) bodyEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var ends []*symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if u, ok := member.Decl.(*ast.Usage); ok && u.IsEnd {
			ends = append(ends, member)
		}
		return true
	})
	return ends
}

// referents resolves the features an end refers to or subsets.
func (a *mosaAudit) referents(sym *symbols.Symbol, usage *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelReferences && rel.Kind != ast.RelSubsets {
			continue
		}
		if target, ok := a.ctx.Resolver().ResolveTarget(sym.OwnerScope, rel.Target); ok && target != nil {
			out = append(out, target)
		}
	}
	return out
}

// annotatedWith reports whether sym carries an annotation of the metadata family.
func (a *mosaAudit) annotatedWith(sym *symbols.Symbol, kind mosaMetadataKind) bool {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if a.metadataKindOf(facts.TypeFQN) == kind {
			return true
		}
	}
	return false
}

// check judges the document's own elements.
func (a *mosaAudit) check(root *symbols.Scope) {
	w8dWalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		a.checkProprietary(sym)
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok {
			return
		}
		switch usage.Kind {
		case ast.UsagePart, ast.UsageItem:
			a.checkComponent(sym)
		case ast.UsageInterface, ast.UsageConnection, ast.UsageConnector:
			if a.kindOf(sym) == mosaModularSystemInterface {
				a.checkInterface(sym)
			} else {
				a.checkBoundary(sym, usage)
			}
		}
	})
}

// checkProprietary expects a @Proprietary annotation to state its rationale.
func (a *mosaAudit) checkProprietary(sym *symbols.Symbol) {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if a.metadataKindOf(facts.TypeFQN) != mosaProprietary {
			continue
		}
		if mosaStatesString(facts, "rationale") {
			return
		}
		a.report(sym, CodeMOSAProprietaryNoRationale,
			"This element is marked @Proprietary with no rationale: MOSA expects the justification for each proprietary element to be recorded, so give the annotation a `rationale`.")
		return
	}
}

// mosaStatesString reports whether an annotation binds feature to a non-empty string.
func mosaStatesString(facts symbols.AnnotationFacts, feature string) bool {
	for _, v := range facts.Values {
		if v.Feature == feature && v.Value.Kind == symbols.FilterValueString && v.Value.Str != "" {
			return true
		}
	}
	return false
}

// checkComponent expects a major system component or modular system to record
// its data rights once the model records any.
func (a *mosaAudit) checkComponent(sym *symbols.Symbol) {
	kind := a.kindOf(sym)
	if kind != mosaMajorSystemComponent && kind != mosaModularSystem {
		return
	}
	if !a.anyMarks.dataRights || a.effectiveMarks(sym).dataRights {
		return
	}
	a.report(sym, CodeMOSAComponentNoDataRights, fmt.Sprintf(
		"This %s records no data rights: the model records them elsewhere, so MOSA expects a @DataRights annotation on it or its definition stating the rights the program holds in its technical data.",
		mosaKindNames[kind]))
}

// checkInterface expects an interface usage to conform to a standard, name an
// interface control authority and satisfy a requirement, each once the model states such facts.
func (a *mosaAudit) checkInterface(sym *symbols.Symbol) {
	marks := a.effectiveMarks(sym)
	if a.present[mosaStandard] && !marks.proprietary && !a.anyOf(sym, a.conformant) {
		a.report(sym, CodeMOSAInterfaceNoStandard,
			"This modular system interface conforms to no standard: the model declares standards, so MOSA expects a #conformance connection naming it at a #conformant end and a standard at a #conformsTo end, or a @Proprietary annotation with the rationale for it.")
	}
	if a.anyMarks.interfaceControl && !marks.interfaceControl {
		a.report(sym, CodeMOSAInterfaceNoControl,
			"This modular system interface names no interface control authority: the model names one for other interfaces, so MOSA expects an @InterfaceControl annotation on it or its definition.")
	}
	if a.present[mosaInterfaceRequirement] && !a.anyOf(sym, a.satisfiers) {
		a.report(sym, CodeMOSAInterfaceNotTraced,
			"This modular system interface satisfies no requirement: the model declares interface requirements, so MOSA expects a `satisfy` naming it (or its definition) after `by`.")
	}
}

// anyOf reports whether sym or anything it specializes is in set.
func (a *mosaAudit) anyOf(sym *symbols.Symbol, set map[symbols.ElementKey]bool) bool {
	if set[symbols.KeyOf(sym)] {
		return true
	}
	for _, t := range a.model.AllSupertypes(sym) {
		if set[symbols.KeyOf(t)] {
			return true
		}
	}
	return false
}

// mosaAttachment is one feature a connector end attaches to, with the scope it is named in.
type mosaAttachment struct {
	node  ast.Node
	scope *symbols.Scope
}

// checkBoundary expects a connector joining two distinct components to be a
// modular system interface, once the model designates any.
func (a *mosaAudit) checkBoundary(sym *symbols.Symbol, usage *ast.Usage) {
	if !a.present[mosaModularSystemInterface] {
		return
	}
	attachments := a.attachments(sym, usage)
	if len(attachments) == 0 {
		return
	}
	parties := map[*symbols.Symbol]bool{}
	for _, att := range attachments {
		party := a.boundaryParty(att)
		if party == nil {
			return
		}
		parties[party] = true
	}
	if len(parties) < 2 {
		return
	}
	a.report(sym, CodeMOSABoundaryNotDesignated,
		"This connection joins two major system components (or modular systems) but is not a modular system interface: MOSA expects the boundary between them to be designated one, so type it by a #modularSystemInterface definition or mark it #modularSystemInterface.")
}

// attachments lists what a connector's ends attach to: the `connect` clause's
// targets, else what its body `end` members reference or subset.
func (a *mosaAudit) attachments(sym *symbols.Symbol, usage *ast.Usage) []mosaAttachment {
	var out []mosaAttachment
	if len(usage.ConnectorEnds) > 0 {
		for _, att := range a.model.ConnectorEndAttachments(sym) {
			out = append(out, mosaAttachment{node: att.Attachment, scope: sym.OwnerScope})
		}
		return out
	}
	for _, end := range a.bodyEnds(sym) {
		u, ok := end.Decl.(*ast.Usage)
		if !ok {
			continue
		}
		for _, rel := range u.Relationships {
			if rel == nil || rel.Target == nil {
				continue
			}
			if rel.Kind == ast.RelReferences || rel.Kind == ast.RelSubsets {
				out = append(out, mosaAttachment{node: rel.Target, scope: end.OwnerScope})
			}
		}
	}
	return out
}

// boundaryParty is the component a connector end attaches to: the shortest
// prefix of the end's feature chain that names one.
func (a *mosaAudit) boundaryParty(att mosaAttachment) *symbols.Symbol {
	if att.node == nil {
		return nil
	}
	prefixes := []ast.Node{att.node}
	if chain, ok := att.node.(*ast.FeatureChainExpr); ok {
		prefixes = append(chainSteps(chain), att.node)
	}
	for _, prefix := range prefixes {
		target, ok := a.ctx.Resolver().ResolveTarget(att.scope, prefix)
		if !ok || target == nil {
			continue
		}
		if k := a.kindOf(target); k == mosaMajorSystemComponent || k == mosaModularSystem {
			return target
		}
	}
	return nil
}

func (a *mosaAudit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   mosaSource,
	})
}
