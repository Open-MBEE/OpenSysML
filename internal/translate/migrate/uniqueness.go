package migrate

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// usageKinds are the kinds of the usage keywords featureKeyword writes; a
// keyword-less `ref` is a default reference usage, which the parser reads as
// an attribute.
var usageKinds = map[string]ast.UsageKind{
	"attribute":   ast.UsageAttribute,
	"ref":         ast.UsageAttribute,
	"part":        ast.UsagePart,
	"item":        ast.UsageItem,
	"port":        ast.UsagePort,
	"action":      ast.UsageAction,
	"state":       ast.UsageState,
	"calc":        ast.UsageCalc,
	"constraint":  ast.UsageConstraint,
	"requirement": ast.UsageRequirement,
	"use case":    ast.UsageUseCase,
	"actor":       ast.UsageActor,
	"view":        ast.UsageView,
	"viewpoint":   ast.UsageViewpoint,
}

// nestedOwners are the declaration kinds the categories a feature's owner can
// take are written as.
var nestedOwners = map[category]semantics.NestedOwner{
	catPartDef:         {Def: ast.DefPart, IsDef: true},
	catPortDef:         {Def: ast.DefPort, IsDef: true},
	catAttributeDef:    {Def: ast.DefAttribute, IsDef: true},
	catEnumDef:         {Def: ast.DefEnumeration, IsDef: true},
	catConstraintDef:   {Def: ast.DefConstraint, IsDef: true},
	catRequirementDef:  {Def: ast.DefRequirement, IsDef: true},
	catConnectionDef:   {Def: ast.DefConnection, IsDef: true},
	catIndividualDef:   {Def: ast.DefIndividual, IsDef: true},
	catVerificationDef: {Def: ast.DefVerificationCase, IsDef: true},
	catItemDef:         {Def: ast.DefItem, IsDef: true},
	catActionDef:       {Def: ast.DefAction, IsDef: true},
	catSimConfig:       {Def: ast.DefAction, IsDef: true},
	catCalcDef:         {Def: ast.DefCalc, IsDef: true},
	catStateDef:        {Def: ast.DefState, IsDef: true},
	catUseCaseDef:      {Def: ast.DefUseCase, IsDef: true},
	catMetadataDef:     {Def: ast.DefMetadata, IsDef: true},
	catView:            {Usage: ast.UsageView},
	catViewpoint:       {Def: ast.DefViewpoint, IsDef: true},
	catValue:           {Usage: ast.UsageAttribute},
}

// implicitlyUnique is the unique library feature a usage written as prefix+kw
// in an owner of category owner implicitly subsets, which forbids it the
// nonunique modifier (subsetting-uniqueness-conformance); "" when it may be
// nonunique. A directed usage is a parameter, which subsets nothing implicitly.
func implicitlyUnique(kw, prefix, dir string, owner category) string {
	kind, ok := usageKinds[kw]
	o, known := nestedOwners[owner]
	if !ok || !known || dir != "" {
		return ""
	}
	u := semantics.NestedUsage{Kind: kind, Composite: prefix == ""}
	for _, fqn := range semantics.ImplicitSubsettingCandidates(u, o) {
		if libraryUnique(fqn) {
			return fqn
		}
	}
	return ""
}

// library is a semantic model over the shared standard library, built once.
var library = sync.OnceValue(func() *semantics.Model {
	resolver := resolve.New(libs.SharedBase())
	m := semantics.NewModel(resolver)
	resolver.SetModel(m)
	return m
})

// libraryUnique reports whether the standard library declares the feature fqn
// unique; a name it does not declare constrains nothing.
func libraryUnique(fqn string) bool {
	syms := libs.SharedBase().LookupQualified(fqn)
	return len(syms) == 1 && library().IsUnique(syms[0])
}

// writtenUnique is why the usage written for p must be unique, "" when it may
// keep a declared nonunique: the unique library feature it implicitly subsets,
// or a written redefinition or subsetting of a feature that is itself written
// unique, whose uniqueness conformance it must then keep.
func (m *migration) writtenUnique(p *sysmlv1.Element, kw, prefix, dir string, owner category) string {
	return m.uniqueReason(p, kw, prefix, dir, owner, map[*sysmlv1.Element]bool{p: true})
}

func (m *migration) uniqueReason(p *sysmlv1.Element, kw, prefix, dir string, owner category, seen map[*sysmlv1.Element]bool) string {
	if fqn := implicitlyUnique(kw, prefix, dir, owner); fqn != "" {
		return kwArticle(prefix+kw) + " in " + kwArticle(owner.keyword()) + " implicitly subsets " + fqn + ", which is unique"
	}
	for role, verb := range map[string]string{"redefinedProperty": "redefines", "subsettedProperty": "subsets"} {
		for _, r := range m.model.Refs(p, role) {
			if m.written(r) && m.featureWrittenUnique(r, seen) {
				return "it " + verb + " " + m.featureRef(r) + ", which is written unique"
			}
		}
	}
	if r, redefinable := m.shadowed(p); redefinable && m.featureWrittenUnique(r, seen) {
		return "it redefines the inherited " + m.featureRef(r) + ", which is written unique"
	}
	return ""
}

// featureWrittenUnique reports whether the usage written for r is unique: v1
// declares it so and no array type modifier says otherwise, or writtenUnique
// forces it. A cycle of subsettings settles on what is declared.
func (m *migration) featureWrittenUnique(r *sysmlv1.Element, seen map[*sysmlv1.Element]bool) bool {
	if r.Attrs["isUnique"] != "false" && m.typeModifier(r).shape(false) == "" {
		return true
	}
	if seen[r] {
		return false
	}
	seen[r] = true
	owner := m.classifyParent(r)
	kw, prefix, _ := m.featureKeyword(r, owner)
	dir, _ := m.featureDirection(r, owner, kw)
	return m.uniqueReason(r, kw, m.referencePrefix(r, owner, kw, dir, prefix), dir, owner, seen) != ""
}

// referencePrefix is the prefix a usage ends up with once an interface block
// (featureModifiers) or a pointer type modifier (typeModifier.ref) forces a
// reference.
func (m *migration) referencePrefix(p *sysmlv1.Element, owner category, kw, dir, prefix string) string {
	if tm := m.typeModifier(p); tm != nil && tm.ref {
		return "ref "
	}
	if interfaceReference(owner, kw, dir, prefix) {
		return "ref "
	}
	return prefix
}

// interfaceReference reports whether a usage of an interface block is forced
// a reference: its usages other than ports must not be composite.
func interfaceReference(owner category, kw, dir, prefix string) bool {
	return owner == catPortDef && dir == "" && prefix == "" && (kw == "item" || kw == "part")
}
