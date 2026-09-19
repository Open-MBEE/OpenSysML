package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Messages of the KerML classifier specialization rules (KerML 1.0 §8.4.4:
// validateDataTypeSpecialization, validateClassSpecialization,
// validateStructureSpecialization, validateBehaviorSpecialization).
const (
	msgW11ASpecializeClassOrAssoc    = "Cannot specialize class or association"
	msgW11ASpecializeDataTypeOrAssoc = "Cannot specialize data type or association"
	msgW11ASpecializeBehavior        = "Cannot specialize behavior"
	msgW11ASpecializeStructure       = "Cannot specialize structure"
)

// SysML wordings of the data type and class rules: an attribute definition is
// the DataType and an item definition the Structure of SysML (SysML v2 §8.3.7, §8.3.10).
const (
	msgW11ASpecializeItemDef      = "Cannot specialize item definition"
	msgW11ASpecializeAttributeDef = "Cannot specialize attribute definition"
)

// w11aFamily is the KerML metaclass family a type declaration belongs to. A
// declaration may belong to several: an `assoc struct` is both.
type w11aFamily struct {
	dataType    bool
	class       bool
	association bool
	structure   bool
	behavior    bool
}

func (f w11aFamily) any() bool {
	return f.dataType || f.class || f.association || f.structure || f.behavior
}

// w11aFamilies maps a KerML type keyword to its metaclass family: a Structure
// and a Behavior are Classes, an Interaction an Association and a Behavior, and
// an AssociationStructure an Association and a Structure (KerML 1.0 §8.4, §9.2).
var w11aFamilies = map[string]w11aFamily{
	"datatype":           {dataType: true},
	"class":              {class: true},
	"struct":             {class: true, structure: true},
	"assoc":              {association: true},
	"association":        {association: true},
	"assoc struct":       {association: true, class: true, structure: true},
	"association struct": {association: true, class: true, structure: true},
	"behavior":           {class: true, behavior: true},
	"function":           {class: true, behavior: true},
	"interaction":        {association: true, class: true, behavior: true},
	"predicate":          {class: true, behavior: true},
	"metaclass":          {class: true, structure: true},
}

// w11aDefFamilies maps a SysML definition kind to the KerML metaclass family
// its metaclass specializes (SysML v2 §8.3): attribute and enumeration
// definitions are data types, every other definition a class — structures,
// behaviors, and association structures among the connection kinds.
var w11aDefFamilies = map[ast.DefinitionKind]w11aFamily{
	ast.DefAttribute:        {dataType: true},
	ast.DefEnumeration:      {dataType: true},
	ast.DefOccurrence:       {class: true},
	ast.DefIndividual:       {class: true},
	ast.DefItem:             {class: true, structure: true},
	ast.DefPart:             {class: true, structure: true},
	ast.DefPort:             {class: true, structure: true},
	ast.DefView:             {class: true, structure: true},
	ast.DefRendering:        {class: true, structure: true},
	ast.DefMetadata:         {class: true, structure: true},
	ast.DefMetaclass:        {class: true, structure: true},
	ast.DefConnection:       {association: true, class: true, structure: true},
	ast.DefInterface:        {association: true, class: true, structure: true},
	ast.DefAllocation:       {association: true, class: true, structure: true},
	ast.DefFlow:             {association: true, class: true, behavior: true},
	ast.DefAction:           {class: true, behavior: true},
	ast.DefState:            {class: true, behavior: true},
	ast.DefCalc:             {class: true, behavior: true},
	ast.DefConstraint:       {class: true, behavior: true},
	ast.DefBool:             {class: true, behavior: true},
	ast.DefRequirement:      {class: true, behavior: true},
	ast.DefConcern:          {class: true, behavior: true},
	ast.DefViewpoint:        {class: true, behavior: true},
	ast.DefCase:             {class: true, behavior: true},
	ast.DefAnalysisCase:     {class: true, behavior: true},
	ast.DefVerificationCase: {class: true, behavior: true},
	ast.DefUseCase:          {class: true, behavior: true},
}

// W11AKerMLSpecializationPass checks the kind of a classifier's supertypes: a
// data type specializes neither a class nor an association, a class neither a
// data type nor an association, and a structure and a behavior do not
// specialize each other (KerML 1.0 §8.4.4). A SysML definition is a classifier
// of the family its metaclass specializes, and its data type and class rules
// are worded in SysML terms (SysML v2 §8.3.7.2, §8.3.10.2).
type W11AKerMLSpecializationPass struct{}

func (W11AKerMLSpecializationPass) Level() PassLevel { return LevelType }

func (W11AKerMLSpecializationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w11aSpecializationChecker{resolver: ctx.Resolver(), sysml: ctx.Kind != source.KindKerML}
	kit.WalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w11aSpecializationChecker struct {
	resolver *resolve.Resolver
	sysml    bool
	diags    []diag.Diagnostic
}

func (c *w11aSpecializationChecker) check(sym *symbols.Symbol) {
	sub := w11aClassifierFamily(sym)
	if !sub.any() {
		return
	}
	for _, rel := range w11aRelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !w11aIsSpecialization(rel.Kind) {
			continue
		}
		target, ok := c.resolver.ResolveTarget(kit.ReferenceScope(sym), rel.Target)
		if !ok || target == nil {
			continue
		}
		if target.Kind == symbols.SymbolAlias {
			if resolved, ok := c.resolver.ResolveAliasTarget(target); ok && resolved != nil {
				target = resolved
			}
		}
		sup := w11aClassifierFamily(target)
		if !sup.any() {
			continue
		}
		for _, msg := range w11aSpecializationMessages(sub, sup) {
			if c.sysml {
				msg = w11aSysMLMessage(msg)
			}
			c.diags = append(c.diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     rel.Target.Span(),
				Message:  msg,
				Code:     "specialization-kind",
				Source:   "type",
			})
		}
	}
}

// w11aSpecializationMessages reports every rule a supertype of this family
// breaks, one message each. An association may specialize an association
// whatever else it is, so that half of the class rule exempts one.
func w11aSpecializationMessages(sub, sup w11aFamily) []string {
	var msgs []string
	if sub.dataType && (sup.class || sup.association) {
		msgs = append(msgs, msgW11ASpecializeClassOrAssoc)
	}
	if sub.class && (sup.dataType || (sup.association && !sub.association)) {
		msgs = append(msgs, msgW11ASpecializeDataTypeOrAssoc)
	}
	if sub.structure && sup.behavior && !sup.structure {
		msgs = append(msgs, msgW11ASpecializeBehavior)
	}
	if sub.behavior && sup.structure && !sup.behavior {
		msgs = append(msgs, msgW11ASpecializeStructure)
	}
	return msgs
}

// w11aSysMLMessage is the SysML wording of a data type or class rule message;
// the structure and behavior rules are worded alike in both notations.
func w11aSysMLMessage(msg string) string {
	switch msg {
	case msgW11ASpecializeClassOrAssoc:
		return msgW11ASpecializeItemDef
	case msgW11ASpecializeDataTypeOrAssoc:
		return msgW11ASpecializeAttributeDef
	}
	return msg
}

// w11aIsSpecialization reports whether a relationship a classifier states is a
// Specialization: `:>` on a classifier is a Subclassification, never a
// Subsetting, which relates features only (KerML 1.0 §7.3.3.2).
func w11aIsSpecialization(k ast.RelationshipKind) bool {
	return k == ast.RelSpecializes || k == ast.RelSubsets
}

// w11aClassifierFamily is the metaclass family of a type declaration: by its KerML
// keyword, else by its SysML definition kind. A feature has none.
func w11aClassifierFamily(sym *symbols.Symbol) w11aFamily {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return w11aFamilies[d.Keyword]
	case *ast.Definition:
		return w11aDefinitionFamily(d.Keyword, d.Kind)
	}
	return w11aFamily{}
}

// w11aDefinitionFamily is the family of a definition written with keyword, of kind.
func w11aDefinitionFamily(keyword string, kind ast.DefinitionKind) w11aFamily {
	if f, ok := w11aFamilies[keyword]; ok {
		return f
	}
	return w11aDefFamilies[kind]
}

// w11aFamilyRuleFires reports whether a definition's supertype breaks one of the
// classifier family rules, which W11AKerMLSpecializationPass reports.
func w11aFamilyRuleFires(decl declKind, sup *symbols.Symbol) bool {
	if !decl.isDef {
		return false
	}
	return len(w11aSpecializationMessages(w11aDefinitionFamily(decl.keyword, decl.defKind), w11aClassifierFamily(sup))) > 0
}

// w11aRelationshipsOf are the relationships a type declaration states.
func w11aRelationshipsOf(sym *symbols.Symbol) []*ast.Relationship {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.Relationships
	case *ast.Definition:
		return d.Relationships
	}
	return nil
}
