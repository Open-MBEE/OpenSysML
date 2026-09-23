package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// typeFilter is how one element type a table names selects migrated elements:
// by a user classifier, by v2 metaclass names, or not at all.
type typeFilter struct {
	// classifiers are the written user classifiers rows are typed by; none
	// when the type is a metaclass or stereotype.
	classifiers []*sysmlv1.Element
	// types are the v2 metaclass names a row must conform to one of.
	types []string
	// metadata is the written metadata def of a user stereotype rows carry.
	metadata string
	// all is set when the type admits every migrated element.
	all bool
	// note says why the filter is only an approximation; refused why it has
	// no v2 form. label names the v1 type for both.
	note, refused, label string
}

// v2Types lists the v2 metaclasses a family of v1 elements migrates to, and
// what makes listing by them approximate.
type v2Types struct {
	types []string
	note  string
}

// The v2 metaclass names the migrator's declarations report as @type.
const (
	typePartDef         = "PartDefinition"
	typeRequirementDef  = "RequirementDefinition"
	typeConstraintDef   = "ConstraintDefinition"
	typePortDef         = "PortDefinition"
	typeAttributeDef    = "AttributeDefinition"
	typeEnumDef         = "EnumerationDefinition"
	typeItemDef         = "ItemDefinition"
	typeConnectionDef   = "ConnectionDefinition"
	typeActionDef       = "ActionDefinition"
	typeCalcDef         = "CalculationDefinition"
	typeStateDef        = "StateDefinition"
	typeUseCaseDef      = "UseCaseDefinition"
	typeVerificationDef = "VerificationCaseDefinition"
	typeOccurrenceDef   = "OccurrenceDefinition"
	typePartUsage       = "PartUsage"
	typeAttributeUsage  = "AttributeUsage"
	typeItemUsage       = "ItemUsage"
	typeReferenceUsage  = "ReferenceUsage"
	typePortUsage       = "PortUsage"
	typeConstraintUsage = "ConstraintUsage"
	typeRequirementUse  = "RequirementUsage"
	typeActionUsage     = "ActionUsage"
	typeStateUsage      = "StateUsage"
	typeCalcUsage       = "CalculationUsage"
	typeUseCaseUsage    = "UseCaseUsage"
	typeViewUsage       = "ViewUsage"
	typeViewpointUsage  = "ViewpointUsage"
	typeEnumUsage       = "EnumerationUsage"
	typeConnectionUsage = "ConnectionUsage"
	typeBindingUsage    = "BindingConnectorAsUsage"
	typeInterfaceUsage  = "InterfaceUsage"
	typeFlowUsage       = "FlowUsage"
	typeSatisfyUsage    = "SatisfyRequirementUsage"
	typeAllocationUsage = "AllocationUsage"
	typeDependency      = "Dependency"
	typeComment         = "Comment"
	typePackage         = "Package"
	typeDefinition      = "Definition"
)

// classTypes are the definitions a UML class of any stereotype migrates to.
var classTypes = []string{typePartDef, typeRequirementDef, typeConstraintDef, typePortDef,
	typeVerificationDef, typeActionDef, typeStateDef, typeCalcDef, typeViewUsage, typeViewpointUsage}

// propertyTypes are the usages a UML property of any type migrates to.
var propertyTypes = []string{typeAttributeUsage, typePartUsage, typeItemUsage, typeReferenceUsage,
	typePortUsage, typeConstraintUsage, typeRequirementUse, typeActionUsage, typeStateUsage,
	typeCalcUsage, typeUseCaseUsage}

// behaviorTypes are the definitions a UML behavior migrates to.
var behaviorTypes = []string{typeActionDef, typeCalcDef, typeStateDef, typeVerificationDef}

// classifierTypes are what a UML type, which is always a classifier, migrates
// to: a definition, or the view or viewpoint usage a «View» or «Viewpoint»
// class becomes.
var classifierTypes = []string{typeDefinition, typeViewUsage, typeViewpointUsage}

// namespaceTypes add to the classifiers the packages and the states, which are
// namespaces in UML; a «View» package is a view usage too.
var namespaceTypes = []string{typePackage, typeDefinition, typeViewUsage, typeViewpointUsage, typeStateUsage}

// packageableTypes are what the elements a package can own migrate to: the
// classifiers, packages, and the dependencies of every stereotype.
var packageableTypes = []string{typePackage, typeDefinition, typeViewUsage, typeViewpointUsage,
	typeDependency, typeSatisfyUsage, typeAllocationUsage}

// Notes on what the broad UML metaclasses list once migrated.
const (
	noteDiagramViews    = "the views diagrams became are listed too"
	noteClassifierExtra = "the views diagrams became and the action defs operations became are listed too"
)

// metaclassTypes maps a UML metaclass to the v2 metaclasses its elements
// migrate to; a nil entry admits every element.
var metaclassTypes = map[string]v2Types{
	"Element":      {},
	"NamedElement": {note: "every migrated element is named, so a NamedElement filter admits all of them"},
	"PackageableElement": {types: packageableTypes,
		note: noteClassifierExtra + "; instances of value types, written as attributes, are not"},
	"Namespace": {types: namespaceTypes, note: noteDiagramViews +
		"; transitions and structured activity nodes are not"},
	"Package":            {types: []string{typePackage}},
	"Model":              {types: []string{typePackage}, note: "a model is a package once migrated"},
	"Type":               {types: classifierTypes, note: noteClassifierExtra},
	"Classifier":         {types: classifierTypes, note: noteClassifierExtra},
	"Class":              {types: classTypes},
	"Component":          {types: []string{typePartDef}, note: "a component is a part def once migrated, as a block is"},
	"Actor":              {types: []string{typePartDef}, note: "an actor is a part def once migrated, as a block is"},
	"Behavior":           {types: behaviorTypes},
	"Activity":           {types: []string{typeActionDef, typeCalcDef}},
	"OpaqueBehavior":     {types: []string{typeActionDef, typeCalcDef}},
	"FunctionBehavior":   {types: []string{typeCalcDef}},
	"Interaction":        {types: []string{typeActionDef}},
	"StateMachine":       {types: []string{typeStateDef}},
	"Operation":          {types: []string{typeActionDef}, note: "an operation is an action def once migrated, as an activity is"},
	"UseCase":            {types: []string{typeUseCaseDef}},
	"DataType":           {types: []string{typeAttributeDef, typeEnumDef}},
	"PrimitiveType":      {types: []string{typeAttributeDef}},
	"Enumeration":        {types: []string{typeEnumDef}},
	"EnumerationLiteral": {types: []string{typeEnumUsage}},
	"Signal":             {types: []string{typeItemDef}},
	"Interface":          {types: []string{typePortDef}, note: "an interface is a port def once migrated, as an interface block is"},
	"Association":        {types: []string{typeConnectionDef}},
	"AssociationClass":   {types: []string{typeConnectionDef}},
	"InstanceSpecification": {types: []string{typeOccurrenceDef},
		note: "instances of value types are written as attributes, which an OccurrenceDefinition filter leaves out"},
	"Property":  {types: propertyTypes, note: "properties typed by a view or viewpoint are not listed"},
	"Port":      {types: []string{typePortUsage}},
	"Connector": {types: []string{typeConnectionUsage, typeBindingUsage, typeInterfaceUsage, typeFlowUsage}},
	"Constraint": {types: []string{typeConstraintUsage},
		note: "a constraint is a constraint usage once migrated, as a constraint property is"},
	"Comment":     {types: []string{typeComment}},
	"Dependency":  {types: []string{typeDependency}},
	"Abstraction": {types: []string{typeDependency}, note: "every dependency is listed, not only abstractions"},
	"Realization": {types: []string{typeDependency}, note: "every dependency is listed, not only realizations"},
	"Usage":       {types: []string{typeDependency}, note: "every dependency is listed, not only usages"},
	"Diagram":     {types: []string{typeViewUsage}, note: "views a «View» class became are listed with the diagrams' views"},
	"State":       {types: []string{typeStateUsage}},
	"Action":      {types: []string{typeActionUsage}},
	"Region":      {types: []string{typeStateUsage}, note: "a region is written into its state, so states are listed for regions"},
}

// actionMetaclasses are the UML activity nodes the migrator writes as action usages.
var actionMetaclasses = map[string]bool{"ActivityNode": true, "ExecutableNode": true, "InvocationAction": true,
	"CallAction": true, "CallBehaviorAction": true, "CallOperationAction": true, "OpaqueAction": true,
	"SendSignalAction": true, "AcceptEventAction": true, "AcceptCallAction": true, "ValueSpecificationAction": true,
	"ReadStructuralFeatureAction": true, "AddStructuralFeatureValueAction": true, "StructuredActivityNode": true,
	"ConditionalNode": true, "LoopNode": true, "SequenceNode": true, "ExpansionRegion": true, "ControlNode": true,
	"ForkNode": true, "JoinNode": true, "DecisionNode": true, "MergeNode": true}

// stereotypeTypes maps a SysML profile stereotype to the v2 metaclasses of what
// it migrates to.
var stereotypeTypes = map[string]v2Types{
	"Block":               {types: []string{typePartDef}},
	"Requirement":         {types: []string{typeRequirementDef}},
	"AbstractRequirement": {types: []string{typeRequirementDef}},
	"ConstraintBlock":     {types: []string{typeConstraintDef}},
	"InterfaceBlock":      {types: []string{typePortDef}},
	"ValueType":           {types: []string{typeAttributeDef, typeEnumDef}},
	"Stakeholder":         {types: []string{typePartDef}, note: "a stakeholder is a part def once migrated, as a block is"},
	"View":                {types: []string{typeViewUsage}},
	"Viewpoint":           {types: []string{typeViewpointUsage}},
	"TestCase":            {types: []string{typeVerificationDef}},
	"Satisfy":             {types: []string{typeSatisfyUsage}},
	"Allocate":            {types: []string{typeAllocationUsage}},
	"Refine":              {types: []string{typeDependency}, note: "every dependency is listed, not only refinements"},
	"Trace":               {types: []string{typeDependency}, note: "every dependency is listed, not only traces"},
	"Copy":                {types: []string{typeDependency}, note: "every dependency is listed, not only copies"},
	"DeriveReqt":          {types: []string{typeConnectionDef}, note: "every connection def is listed, not only derivations"},
	"ProxyPort":           {types: []string{typePortUsage}},
	"FullPort":            {types: []string{typePortUsage}},
	"FlowPort":            {types: []string{typePortUsage}},
	"BindingConnector":    {types: []string{typeBindingUsage}},
	"ItemFlow":            {types: []string{typeFlowUsage}},
	"PartProperty":        {types: []string{typePartUsage}},
	"SharedProperty":      {types: []string{typePartUsage}, note: "every part usage is listed, composite ones included"},
	"ReferenceProperty":   {types: []string{typeReferenceUsage}},
	"ValueProperty":       {types: []string{typeAttributeUsage}},
	"ConstraintProperty":  {types: []string{typeConstraintUsage}},
	"ConstraintParameter": {types: []string{typeAttributeUsage}, note: "every attribute usage is listed, not only constraint parameters"},
}

// standardHref names, when href points into the OMG UML or SysML documents, the
// document ("UML", "SysML") and the fragment's local name ("Class", "Block").
func standardHref(href string) (doc, name string, ok bool) {
	path, frag, found := strings.Cut(href, "#")
	if !found || (!strings.Contains(path, "/spec/UML/") && !strings.Contains(path, "/spec/SysML/")) {
		return "", "", false
	}
	doc = hrefDocument(path)
	if i := strings.LastIndexByte(frag, '.'); i >= 0 {
		frag = frag[i+1:]
	}
	return doc, frag, frag != ""
}

// typeFilter decides how the element type ref names filters rows.
func (m *migration) typeFilter(ref sysmlv1.ElementRef) typeFilter {
	e := ref.Element
	if e == nil {
		return typeFilter{label: ref.ID, refused: "the element type " + ref.ID + " " + m.unresolvedRef(ref)}
	}
	if doc, name, ok := standardHref(e.Href); ok {
		switch {
		case doc == "UML":
			return metaclassFilter(name)
		case stereotypeTypes[name].types != nil:
			return fromTypes("«"+name+"»", stereotypeTypes[name])
		default:
			return typeFilter{label: "«" + name + "»", refused: "no v2 metaclass stands for the elements of «" + name + "»"}
		}
	}
	if e.IsProxy() {
		if s := m.model.StereotypeRef(e.ID); s.Name != "" && isStandardNamespace(s.Namespace) {
			if t, ok := stereotypeTypes[s.Name]; ok {
				return fromTypes("«"+s.Name+"»", t)
			}
			return typeFilter{label: "«" + s.Name + "»", refused: "no v2 metaclass stands for the elements of «" + s.Name + "»"}
		}
		if e.Name == "" {
			return typeFilter{label: e.Href, refused: "the element type " + e.Href + " is in a module the archive does not describe"}
		}
		if t, ok := stereotypeTypes[e.Name]; ok && isCustomizationHref(e.Href) {
			return fromTypes("«"+e.Name+"»", t)
		}
		if subs := m.specializers(e); len(subs) > 0 {
			return typeFilter{classifiers: subs, label: qualifiedName(e),
				note: "the element type " + qualifiedName(e) + " is outside the document; rows are filtered by the document's classifiers specializing it"}
		}
		return typeFilter{label: qualifiedName(e), refused: "the element type " + qualifiedName(e) + " is outside the document, and not a UML metaclass or a SysML stereotype"}
	}
	if e.Type == "Stereotype" {
		if t, ok := stereotypeTypes[e.Name]; ok && m.isLibrary(e) && libraryRoots[pathRoot(qualifiedName(e))] {
			return fromTypes("«"+e.Name+"»", t)
		}
		if m.userStereotype(e) && m.written(e) {
			return typeFilter{label: "«" + e.Name + "»", metadata: m.plainName(e)}
		}
		return typeFilter{label: "«" + e.Name + "»", refused: "«" + e.Name + "» is not written as a metadata def rows could be filtered by"}
	}
	if !m.written(e) {
		return typeFilter{label: qualifiedName(e), refused: "the element type " + kindOf(e) + " " + qualifiedName(e) + " is not migrated"}
	}
	cat, _ := m.classify(e)
	if cat.keyword() == "" || cat == catPackage {
		return typeFilter{label: qualifiedName(e), refused: "the element type " + kindOf(e) + " " + qualifiedName(e) + " is not a classifier rows can be typed by"}
	}
	return typeFilter{classifiers: []*sysmlv1.Element{e}, label: qualifiedName(e)}
}

// specializers lists the written classifiers of the document that specialize
// general, directly or through other classifiers outside the document, so a
// type outside the document still filters rows exactly.
func (m *migration) specializers(general *sysmlv1.Element) []*sysmlv1.Element {
	var subs []*sysmlv1.Element
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, c := range e.Children {
			walk(c)
		}
		if len(e.Owned("generalization")) == 0 || !m.inherits(e, general) || !m.written(e) {
			return
		}
		if cat, _ := m.classify(e); cat.keyword() != "" && cat != catPackage {
			subs = append(subs, e)
		}
	}
	for _, r := range m.model.Roots {
		if !m.isLibrary(r) {
			walk(r)
		}
	}
	return subs
}

// isCustomizationHref reports whether an href points into MagicDraw's SysML
// customization module, whose stereotypes name property kinds.
func isCustomizationHref(href string) bool {
	return fold(hrefDocument(href)) == fold(magicDrawCustomizationModule)
}

// metaclassFilter is the filter of a UML metaclass.
func metaclassFilter(name string) typeFilter {
	t, ok := metaclassTypes[name]
	if actionMetaclasses[name] {
		t, ok = v2Types{types: []string{typeActionUsage}, note: "every action usage is listed, whatever kind of activity node it was"}, true
	}
	if !ok {
		return typeFilter{label: name, refused: "no v2 metaclass stands for the elements of a UML " + name}
	}
	if t.types == nil {
		return typeFilter{label: name, all: true, note: t.note}
	}
	return fromTypes(name, t)
}

func fromTypes(label string, t v2Types) typeFilter {
	return typeFilter{label: label, types: t.types, note: t.note}
}
