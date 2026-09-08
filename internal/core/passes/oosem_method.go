package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// OOSEMMethodPass audits the artefacts a model classifies with the OOSEM
// library against the method's traceability and allocation rules. Each rule
// fires only once the model declares the artefacts the rule relates, so a model
// still being built is not told what it has not reached yet.
type OOSEMMethodPass struct{}

// Diagnostic codes of the OOSEM method rules.
const (
	CodeOOSEMRequirementNotDerived   = "oosem-requirement-not-derived"
	CodeOOSEMRequirementNotSatisfied = "oosem-requirement-not-satisfied"
	CodeOOSEMLogicalNotAllocated     = "oosem-logical-component-not-allocated"
	CodeOOSEMUseCaseSubject          = "oosem-use-case-subject"
)

const oosemSource = "oosem"

func (OOSEMMethodPass) Level() PassLevel { return LevelConstraint }

func (OOSEMMethodPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
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
	a := newOOSEMAudit(ctx)
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

// oosemKind is an OOSEM artefact class, told by conformance to its library definition.
type oosemKind int

const (
	oosemNone oosemKind = iota
	oosemStakeholderNeed
	oosemMissionRequirement
	oosemSystemRequirement
	oosemComponentRequirement
	oosemLogicalComponent
	oosemPhysicalComponent
	oosemNode
	oosemSystemContext
	oosemEnterprise
	oosemEnterpriseUseCase
	oosemSystemUseCase
)

// oosemKindDefinitions lists the library definitions each kind conforms to.
// Order matters where kinds specialize one another: a data component is a
// physical component, so DataComponent is listed with PhysicalComponent.
var oosemKindDefinitions = []struct {
	kind oosemKind
	fqns []string
}{
	{oosemStakeholderNeed, []string{"OOSEM::StakeholderNeed"}},
	{oosemMissionRequirement, []string{"OOSEM::MissionRequirement"}},
	{oosemSystemRequirement, []string{"OOSEM::SystemRequirement"}},
	{oosemComponentRequirement, []string{"OOSEM::ComponentRequirement"}},
	{oosemLogicalComponent, []string{"OOSEM::LogicalComponent"}},
	{oosemPhysicalComponent, []string{"OOSEM::PhysicalComponent", "OOSEM::DataComponent"}},
	{oosemNode, []string{"OOSEM::Node"}},
	{oosemSystemContext, []string{"OOSEM::SystemContext"}},
	{oosemEnterprise, []string{"OOSEM::Enterprise"}},
	{oosemEnterpriseUseCase, []string{"OOSEM::EnterpriseUseCase"}},
	{oosemSystemUseCase, []string{"OOSEM::SystemUseCase"}},
}

var oosemKindNames = map[oosemKind]string{
	oosemStakeholderNeed:      "stakeholder need",
	oosemMissionRequirement:   "mission requirement",
	oosemSystemRequirement:    "system requirement",
	oosemComponentRequirement: "component requirement",
	oosemLogicalComponent:     "logical component",
	oosemPhysicalComponent:    "physical component",
	oosemNode:                 "node",
	oosemSystemContext:        "system context",
	oosemEnterprise:           "enterprise",
	oosemEnterpriseUseCase:    "enterprise use case",
	oosemSystemUseCase:        "system use case",
}

// oosemDerivesFrom is the requirement level each level derives from.
var oosemDerivesFrom = map[oosemKind]oosemKind{
	oosemMissionRequirement:   oosemStakeholderNeed,
	oosemSystemRequirement:    oosemMissionRequirement,
	oosemComponentRequirement: oosemSystemRequirement,
}

const (
	derivationMetadataFQN          = "RequirementDerivation::DerivationMetadata"
	originalRequirementMetadataFQN = "RequirementDerivation::OriginalRequirementMetadata"
	derivedRequirementMetadataFQN  = "RequirementDerivation::DerivedRequirementMetadata"
)

type oosemAudit struct {
	ctx   *Context
	model *semantics.Model
	// definitions holds the library definition of each OOSEM kind.
	definitions map[oosemKind][]*symbols.Symbol
	// present records the kinds the workspace declares at all.
	present map[oosemKind]bool
	// derivedFrom holds, per requirement at a `#derive` end, the kinds of the
	// `#original` ends of the same `#derivation`; satisfied holds the requirements
	// a `satisfy` names; allocated the sources allocated to a node or physical component.
	derivedFrom map[symbols.ElementKey]map[oosemKind]bool
	satisfied   map[symbols.ElementKey]bool
	allocated   map[symbols.ElementKey]bool
	// statesSatisfaction reports whether the workspace states any satisfy.
	statesSatisfaction bool
	kinds              map[*symbols.Symbol]oosemKind
	// typeKinds memoizes kindOfType: one type classifies every feature it types.
	typeKinds map[*symbols.Symbol]oosemKind
	diags     []Diagnostic
}

// newOOSEMAudit returns nil when the bundled OOSEM library is not loaded, since
// no element can then be an OOSEM artefact; a workspace package of the same
// name is not the method vocabulary.
func newOOSEMAudit(ctx *Context) *oosemAudit {
	a := &oosemAudit{
		ctx:         ctx,
		model:       ctx.Model(),
		definitions: map[oosemKind][]*symbols.Symbol{},
		present:     map[oosemKind]bool{},
		derivedFrom: map[symbols.ElementKey]map[oosemKind]bool{},
		satisfied:   map[symbols.ElementKey]bool{},
		allocated:   map[symbols.ElementKey]bool{},
		kinds:       map[*symbols.Symbol]oosemKind{},
		typeKinds:   map[*symbols.Symbol]oosemKind{},
	}
	found := false
	for _, entry := range oosemKindDefinitions {
		for _, fqn := range entry.fqns {
			for _, def := range ctx.Index.LookupQualified(fqn) {
				if def != nil && ctx.Index.Library(def) {
					a.definitions[entry.kind] = append(a.definitions[entry.kind], def)
					found = true
				}
			}
		}
	}
	if !found {
		return nil
	}
	return a
}

// kindOf classifies sym: a usage by the types it has, a definition by itself.
func (a *oosemAudit) kindOf(sym *symbols.Symbol) oosemKind {
	if sym == nil {
		return oosemNone
	}
	if kind, ok := a.kinds[sym]; ok {
		return kind
	}
	a.kinds[sym] = oosemNone
	var types []*symbols.Symbol
	if sym.IsFeature() {
		types = a.model.FeatureTypeSet(sym)
	} else if _, ok := sym.Decl.(*ast.Definition); ok {
		types = []*symbols.Symbol{sym}
	}
	kind := oosemNone
	for _, t := range types {
		if kind = a.kindOfType(t); kind != oosemNone {
			break
		}
	}
	a.kinds[sym] = kind
	return kind
}

func (a *oosemAudit) kindOfType(t *symbols.Symbol) oosemKind {
	if kind, ok := a.typeKinds[t]; ok {
		return kind
	}
	kind := oosemNone
search:
	for _, entry := range oosemKindDefinitions {
		for _, def := range a.definitions[entry.kind] {
			if a.model.Conforms(t, def) {
				kind = entry.kind
				break search
			}
		}
	}
	a.typeKinds[t] = kind
	return kind
}

// gather records the kinds a document declares and the derivation,
// satisfaction and allocation relationships it states.
func (a *oosemAudit) gather(root *symbols.Scope) {
	w8dWalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		usage, isUsage := sym.Decl.(*ast.Usage)
		if kind := a.kindOf(sym); kind != oosemNone {
			a.present[kind] = true
		}
		if !isUsage {
			return
		}
		switch {
		case usage.Kind == ast.UsageConnection:
			a.gatherDerivation(sym)
		case usage.Kind == ast.UsageAllocation:
			a.gatherAllocation(sym, usage)
		case usage.Kind == ast.UsageSatisfy && usage.Keyword != "verify":
			a.gatherSatisfaction(sym, usage)
		}
	})
}

// gatherDerivation records, for each requirement at a `#derive` end of a
// `#derivation` connection, the kinds of the requirements at its `#original` ends.
func (a *oosemAudit) gatherDerivation(sym *symbols.Symbol) {
	if !a.annotatedWith(sym, derivationMetadataFQN) {
		return
	}
	var originals, derived []*symbols.Symbol
	for _, end := range a.bodyEnds(sym) {
		u, ok := end.Decl.(*ast.Usage)
		if !ok {
			continue
		}
		switch {
		case a.annotatedWith(end, originalRequirementMetadataFQN):
			originals = append(originals, a.referents(end, u)...)
		case a.annotatedWith(end, derivedRequirementMetadataFQN):
			derived = append(derived, a.referents(end, u)...)
		}
	}
	for _, d := range derived {
		key := symbols.KeyOf(d)
		if a.derivedFrom[key] == nil {
			a.derivedFrom[key] = map[oosemKind]bool{}
		}
		for _, o := range originals {
			a.derivedFrom[key][a.kindOf(o)] = true
		}
	}
}

// gatherAllocation records the source of an allocation whose destination is a
// node or physical component: the ends of its `allocate x to y` clause, else
// the first two `end`s its body declares.
func (a *oosemAudit) gatherAllocation(sym *symbols.Symbol, usage *ast.Usage) {
	var source, destination []*symbols.Symbol
	if len(usage.ConnectorEnds) > 0 {
		attachments := a.model.ConnectorEndAttachments(sym)
		if len(attachments) < 2 || attachments[0].Attachment == nil || attachments[1].Attachment == nil {
			return
		}
		if s, ok := a.ctx.Resolver().ResolveTarget(sym.OwnerScope, attachments[0].Attachment); ok && s != nil {
			source = append(source, s)
		}
		if d, ok := a.ctx.Resolver().ResolveTarget(sym.OwnerScope, attachments[1].Attachment); ok && d != nil {
			destination = append(destination, d)
		}
	} else {
		ends := a.bodyEnds(sym)
		if len(ends) < 2 {
			return
		}
		if u, ok := ends[0].Decl.(*ast.Usage); ok {
			source = a.referents(ends[0], u)
		}
		if u, ok := ends[1].Decl.(*ast.Usage); ok {
			destination = a.referents(ends[1], u)
		}
	}
	if !a.realises(destination) {
		return
	}
	for _, s := range source {
		a.allocated[symbols.KeyOf(s)] = true
	}
}

// realises reports whether any of the symbols is a node or physical component.
func (a *oosemAudit) realises(syms []*symbols.Symbol) bool {
	for _, s := range syms {
		if k := a.kindOf(s); k == oosemNode || k == oosemPhysicalComponent {
			return true
		}
	}
	return false
}

// bodyEnds lists the `end` members a connector declares, in declaration order.
func (a *oosemAudit) bodyEnds(sym *symbols.Symbol) []*symbols.Symbol {
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

// gatherSatisfaction records the requirement a `satisfy` names, or the
// satisfy itself when it declares its requirement. A negated satisfy and a
// satisfy naming a viewpoint state no requirement satisfied.
func (a *oosemAudit) gatherSatisfaction(sym *symbols.Symbol, usage *ast.Usage) {
	if usage.IsNegated {
		return
	}
	if usage.DeclaresRequirement {
		a.statesSatisfaction = true
		a.satisfied[symbols.KeyOf(sym)] = true
		return
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubsets {
			continue
		}
		target, ok := a.ctx.Resolver().ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil || isViewpoint(target) {
			continue
		}
		a.statesSatisfaction = true
		a.satisfied[symbols.KeyOf(target)] = true
	}
}

func isViewpoint(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefViewpoint
	case *ast.Usage:
		return d.Kind == ast.UsageViewpoint
	}
	return false
}

// referents resolves the features an end refers to or subsets.
func (a *oosemAudit) referents(sym *symbols.Symbol, usage *ast.Usage) []*symbols.Symbol {
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

func (a *oosemAudit) annotatedWith(sym *symbols.Symbol, fqn string) bool {
	for _, facts := range a.model.AnnotationFactsOf(sym) {
		if facts.TypeFQN == fqn {
			return true
		}
	}
	return false
}

// check judges the document's own artefacts.
func (a *oosemAudit) check(root *symbols.Scope) {
	w8dWalkSymbols(a.ctx, root, func(sym *symbols.Symbol) {
		switch d := sym.Decl.(type) {
		case *ast.Usage:
			switch d.Kind {
			case ast.UsageRequirement:
				a.checkRequirement(sym)
			case ast.UsageSatisfy:
				if d.DeclaresRequirement {
					a.checkRequirement(sym)
				}
			case ast.UsagePart, ast.UsageItem:
				a.checkLogicalComponent(sym)
			case ast.UsageUseCase:
				a.checkUseCaseSubject(sym)
			}
		case *ast.Definition:
			if d.Kind == ast.DefUseCase {
				a.checkUseCaseSubject(sym)
			}
		}
	})
}

// checkRequirement expects a requirement to derive from the level above it
// once that level exists, and to be satisfied once anything is satisfied.
func (a *oosemAudit) checkRequirement(sym *symbols.Symbol) {
	kind := a.kindOf(sym)
	from, derives := oosemDerivesFrom[kind]
	if !derives {
		return
	}
	key := symbols.KeyOf(sym)
	if a.present[from] && !a.derivedFrom[key][from] {
		a.report(sym, CodeOOSEMRequirementNotDerived, fmt.Sprintf(
			"This %s derives from no %s: the model declares %ss, so OOSEM expects a #derivation connection naming it at a #derive end and a %s at an #original end.",
			oosemKindNames[kind], oosemKindNames[from], oosemKindNames[from], oosemKindNames[from]))
	}
	if kind != oosemMissionRequirement && a.statesSatisfaction && !a.satisfied[key] {
		a.report(sym, CodeOOSEMRequirementNotSatisfied, fmt.Sprintf(
			"This %s is satisfied by nothing: the model states satisfactions, so OOSEM expects a `satisfy` naming it.",
			oosemKindNames[kind]))
	}
}

// checkLogicalComponent expects a logical component usage to be allocated to
// a node or physical component once the model declares either; an allocation
// of its type or of an enclosing component covers it.
func (a *oosemAudit) checkLogicalComponent(sym *symbols.Symbol) {
	if a.kindOf(sym) != oosemLogicalComponent {
		return
	}
	if !a.present[oosemNode] && !a.present[oosemPhysicalComponent] {
		return
	}
	if a.allocated[symbols.KeyOf(sym)] {
		return
	}
	for _, t := range a.model.FeatureTypeSet(sym) {
		if a.allocated[symbols.KeyOf(t)] {
			return
		}
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil && a.allocated[symbols.KeyOf(owner)] {
			return
		}
	}
	a.report(sym, CodeOOSEMLogicalNotAllocated,
		"This logical component is allocated to nothing: the model declares nodes or physical components, so OOSEM expects an `allocate` from it (or its type) to one.")
}

// checkUseCaseSubject expects a system use case's subject to be a system
// context and an enterprise use case's subject to be an enterprise.
func (a *oosemAudit) checkUseCaseSubject(sym *symbols.Symbol) {
	kind := a.kindOf(sym)
	var want oosemKind
	switch kind {
	case oosemSystemUseCase:
		want = oosemSystemContext
	case oosemEnterpriseUseCase:
		want = oosemEnterprise
	default:
		return
	}
	// Effective members, so a subject inherited from a plain base use case is
	// judged too; one reached through a model use case of the same kind is judged there.
	judged := a.subjectsJudgedHere(sym, kind)
	for _, member := range a.model.MembersOf(sym) {
		u, ok := member.Decl.(*ast.Usage)
		if !ok || u.Kind != ast.UsageSubject || !judged[member] {
			continue
		}
		at := member
		if member.OwnerScope != sym.Scope {
			at = sym
		}
		// Types the wanted kind itself conforms to (Anything, Part) say nothing either way.
		typed, served := false, false
		for _, t := range a.model.FeatureTypeSet(member) {
			if a.kindOfType(t) == want {
				served = true
				break
			}
			if !a.generalizes(t, want) {
				typed = true
			}
		}
		if served || !typed {
			continue
		}
		a.report(at, CodeOOSEMUseCaseSubject, fmt.Sprintf(
			"The subject of a%s %s is the %s it serves, but this one is typed by none.",
			article(oosemKindNames[kind]), oosemKindNames[kind], oosemKindNames[want]))
	}
}

// subjectsJudgedHere collects the subjects sym declares or inherits along paths
// that cross no model use case of its own kind, since those judge their own.
func (a *oosemAudit) subjectsJudgedHere(sym *symbols.Symbol, kind oosemKind) map[*symbols.Symbol]bool {
	out := map[*symbols.Symbol]bool{}
	seen := map[*symbols.Symbol]bool{}
	var visit func(s *symbols.Symbol)
	visit = func(s *symbols.Symbol) {
		if seen[s] {
			return
		}
		seen[s] = true
		if s.Scope != nil {
			s.Scope.ForEachMember(func(member *symbols.Symbol) bool {
				if u, ok := member.Decl.(*ast.Usage); ok && u.Kind == ast.UsageSubject {
					out[member] = true
				}
				return true
			})
		}
		for _, src := range a.model.DirectMemberSources(s) {
			if a.kindOf(src) == kind && !a.ctx.Index.Library(src) {
				continue
			}
			visit(src)
		}
	}
	visit(sym)
	return out
}

// generalizes reports whether every library definition of kind conforms to t.
func (a *oosemAudit) generalizes(t *symbols.Symbol, kind oosemKind) bool {
	for _, def := range a.definitions[kind] {
		if !a.model.Conforms(def, t) {
			return false
		}
	}
	return true
}

func article(noun string) string {
	if len(noun) > 0 && (noun[0] == 'a' || noun[0] == 'e' || noun[0] == 'i' || noun[0] == 'o' || noun[0] == 'u') {
		return "n"
	}
	return ""
}

func (a *oosemAudit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   oosemSource,
	})
}
