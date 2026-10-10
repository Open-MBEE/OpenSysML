package migrate

import (
	"slices"
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
	// excluding names a set of newly conforming elements that are not in the
	// original UML metaclass's result set.
	excluding *typeFilterExclusion
	// usageKinds are the categories whose classifiers migrate to usages a
	// table lists by name; usages are the plain names of those written.
	usageKinds []category
	usages     []string
	// instances admits the usages a package owns, which instance
	// specifications are written as.
	instances bool
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
	types     []string
	excluding *typeFilterExclusion
	usages    []category
	// instances admits the usages a package owns: the instance specifications.
	instances bool
	note      string
}

type typeFilterExclusion struct {
	source []string
	keep   []string
}

// The v2 metaclass names the migrator's declarations report as @type.
const (
	typePartDef              = "PartDefinition"
	typeConstraintDef        = "ConstraintDefinition"
	typePortDef              = "PortDefinition"
	typeAttributeDef         = "AttributeDefinition"
	typeEnumDef              = "EnumerationDefinition"
	typeItemDef              = "ItemDefinition"
	typeConnectionDef        = "ConnectionDefinition"
	typeActionDef            = "ActionDefinition"
	typeCalcDef              = "CalculationDefinition"
	typeStateDef             = "StateDefinition"
	typeUseCaseDef           = "UseCaseDefinition"
	typeVerificationDef      = "VerificationCaseDefinition"
	typeOccurrenceDef        = "OccurrenceDefinition"
	typeOccurrenceUsage      = "OccurrenceUsage"
	typeEventOccurrenceUsage = "EventOccurrenceUsage"
	typePartUsage            = "PartUsage"
	typeAttributeUsage       = "AttributeUsage"
	typeItemUsage            = "ItemUsage"
	typeReferenceUsage       = "ReferenceUsage"
	typePortUsage            = "PortUsage"
	typeConstraintUsage      = "ConstraintUsage"
	typeRequirementUse       = "RequirementUsage"
	typeActionUsage          = "ActionUsage"
	typeStateUsage           = "StateUsage"
	typeCalcUsage            = "CalculationUsage"
	typeUseCaseUsage         = "UseCaseUsage"
	typeViewUsage            = "ViewUsage"
	typeViewpointUsage       = "ViewpointUsage"
	typeEnumUsage            = "EnumerationUsage"
	typeConnectionUsage      = "ConnectionUsage"
	typeBindingUsage         = "BindingConnectorAsUsage"
	typeInterfaceUsage       = "InterfaceUsage"
	typeFlowUsage            = "FlowUsage"
	typeSatisfyUsage         = "SatisfyRequirementUsage"
	typeAllocationUsage      = "AllocationUsage"
	typeAllocationDef        = "AllocationDefinition"
	typeDependency           = "Dependency"
	typeComment              = "Comment"
	typePackage              = "Package"
	typeDefinition           = "Definition"
)

// classTypes are the definitions a UML class of any stereotype migrates to.
var classTypes = []string{typePartDef, typeConstraintDef, typePortDef,
	typeVerificationDef, typeActionDef, typeStateDef, typeCalcDef, typeViewUsage, typeViewpointUsage}

// propertyTypes are the usages a UML property of any type migrates to.
var propertyTypes = []string{typeAttributeUsage, typePartUsage, typeItemUsage, typeReferenceUsage,
	typePortUsage, typeConstraintUsage, typeRequirementUse, typeActionUsage, typeStateUsage,
	typeCalcUsage, typeUseCaseUsage}

// PartDefinition filters admit plain occurrence defs because a plain class used to be a part def.
func occurrenceAwareTypes(types []string, include, exclude, note string) v2Types {
	keep := append([]string(nil), types...)
	all := append(append([]string(nil), keep...), include)
	return v2Types{
		types: all,
		excluding: &typeFilterExclusion{
			source: []string{exclude},
			keep:   keep,
		},
		note: note,
	}
}

// withUsages adds to t the categories whose classifiers are written as usages.
func withUsages(t v2Types, usages ...category) v2Types {
	t.usages = append(t.usages, usages...)
	return t
}

// behaviorTypes are the definitions a UML behavior migrates to.
var behaviorTypes = []string{typeActionDef, typeCalcDef, typeStateDef, typeVerificationDef}

// classifierTypes are what a UML type, which is always a classifier, migrates
// to by metaclass: a definition, or the view or viewpoint usage a «View» or
// «Viewpoint» class becomes. Actors and use cases become part and use case
// usages, which by type a query cannot tell from the usage a property becomes,
// so classifierUsages lists them by name instead.
var classifierTypes = []string{typeDefinition, typeViewUsage, typeViewpointUsage}

var classifierUsages = []category{catActor, catUseCase, catRequirement}

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
	elementTypeSubject  = "the element type "
)

// metaclassTypes maps a UML metaclass to the v2 metaclasses its elements
// migrate to; a nil entry admits every element.
var metaclassTypes = map[string]v2Types{
	"Element":      {},
	"NamedElement": {note: "every migrated element is named, so a NamedElement filter admits all of them"},
	"PackageableElement": {types: packageableTypes, usages: classifierUsages, instances: true,
		note: noteClassifierExtra + "; instance specifications are listed, but not links, which a query cannot tell from the connections of a use case diagram"},
	"Namespace": {types: namespaceTypes, usages: classifierUsages, note: noteDiagramViews +
		"; transitions and structured activity nodes are not"},
	"Package":            {types: []string{typePackage}},
	"Model":              {types: []string{typePackage}, note: "a model is a package once migrated"},
	"Type":               {types: classifierTypes, usages: classifierUsages, note: noteClassifierExtra},
	"Classifier":         {types: classifierTypes, usages: classifierUsages, note: noteClassifierExtra},
	"Class":              withUsages(occurrenceAwareTypes(classTypes, typeOccurrenceDef, typeItemDef, ""), catRequirement),
	"Component":          occurrenceAwareTypes([]string{typePartDef}, typeOccurrenceDef, typeItemDef, "a component is a part def once migrated, as a block is"),
	"Actor":              {usages: []category{catActor}},
	"Behavior":           {types: behaviorTypes},
	"Activity":           {types: []string{typeActionDef, typeCalcDef}},
	"OpaqueBehavior":     {types: []string{typeActionDef, typeCalcDef}},
	"FunctionBehavior":   {types: []string{typeCalcDef}},
	"Interaction":        {types: []string{typeActionDef}},
	"StateMachine":       {types: []string{typeStateDef}},
	"Operation":          {types: []string{typeActionDef}, note: "an operation is an action def once migrated, as an activity is"},
	"UseCase":            {usages: []category{catUseCase}},
	"DataType":           {types: []string{typeAttributeDef, typeEnumDef}},
	"PrimitiveType":      {types: []string{typeAttributeDef}},
	"Enumeration":        {types: []string{typeEnumDef}},
	"EnumerationLiteral": {types: []string{typeEnumUsage}},
	"Signal":             {types: []string{typeItemDef}},
	"Interface":          {types: []string{typePortDef}, note: "an interface is a port def once migrated, as an interface block is"},
	"Association":        {types: []string{typeConnectionDef}},
	"AssociationClass":   {types: []string{typeConnectionDef}},
	"InstanceSpecification": {instances: true,
		note: "instance specifications are the usages a package owns; links are not listed, which a query cannot tell from the connections of a use case diagram"},
	"Property":  occurrenceAwareTypes(propertyTypes, typeOccurrenceUsage, typeEventOccurrenceUsage, "properties typed by a view or viewpoint are not listed"),
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
	"Block":               occurrenceAwareTypes([]string{typePartDef}, typeOccurrenceDef, typeItemDef, ""),
	"Requirement":         {usages: []category{catRequirement}},
	"AbstractRequirement": {usages: []category{catRequirement}},
	"ConstraintBlock":     {types: []string{typeConstraintDef}},
	"InterfaceBlock":      {types: []string{typePortDef}},
	"ValueType":           {types: []string{typeAttributeDef, typeEnumDef}},
	"Stakeholder":         occurrenceAwareTypes([]string{typePartDef}, typeOccurrenceDef, typeItemDef, "a stakeholder is a part def once migrated, as a block is"),
	"View":                {types: []string{typeViewUsage}},
	"Viewpoint":           {types: []string{typeViewpointUsage}},
	"TestCase":            {types: []string{typeVerificationDef}},
	"Satisfy":             {types: []string{typeSatisfyUsage}},
	"Allocate":            {types: []string{typeAllocationUsage, typeAllocationDef}},
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
		return typeFilter{label: ref.ID, refused: elementTypeSubject + ref.ID + " " + m.unresolvedRef(ref)}
	}
	if doc, name, ok := standardHref(e.Href); ok {
		switch {
		case doc == "UML":
			return m.nameUsages(metaclassFilter(name))
		case stereotypeTypes[name].types != nil || stereotypeTypes[name].usages != nil:
			return m.nameUsages(fromTypes("«"+name+"»", stereotypeTypes[name]))
		default:
			return typeFilter{label: "«" + name + "»", refused: "no v2 metaclass stands for the elements of «" + name + "»"}
		}
	}
	if e.IsProxy() {
		return m.proxyTypeFilter(e)
	}
	if e.Type == "Stereotype" {
		return m.stereotypeTypeFilter(e)
	}
	if !m.written(e) {
		return typeFilter{label: qualifiedName(e), refused: elementTypeSubject + kindOf(e) + " " + qualifiedName(e) + " is not migrated"}
	}
	cat, _ := m.classify(e)
	if cat.keyword() == "" || cat == catPackage {
		return typeFilter{label: qualifiedName(e), refused: elementTypeSubject + kindOf(e) + " " + qualifiedName(e) + " is not a classifier rows can be typed by"}
	}
	return typeFilter{classifiers: []*sysmlv1.Element{e}, label: qualifiedName(e)}
}

// proxyTypeFilter decides how a type outside the document filters rows: by
// the stereotype or metaclass it stands for, or by its specializers inside.
func (m *migration) proxyTypeFilter(e *sysmlv1.Element) typeFilter {
	s := m.model.StereotypeRef(e.ID)
	if s.Name != "" && isStandardNamespace(s.Namespace) {
		if t, ok := stereotypeTypes[s.Name]; ok {
			return m.nameUsages(fromTypes("«"+s.Name+"»", t))
		}
		return typeFilter{label: "«" + s.Name + "»", refused: "no v2 metaclass stands for the elements of «" + s.Name + "»"}
	}
	customization := isCustomizationHref(e.Href) || isMagicDrawCustomization(s.Namespace)
	name := e.Name
	if name == "" && customization {
		// The tool's stereotype table names what the href alone does not.
		name = s.Name
	}
	if name == "" {
		return typeFilter{label: e.Href, refused: elementTypeSubject + e.Href + " is in a module the archive does not describe"}
	}
	if t, ok := stereotypeTypes[name]; ok && customization {
		return m.nameUsages(fromTypes("«"+name+"»", t))
	}
	if subs := m.specializers(e); len(subs) > 0 {
		return typeFilter{classifiers: subs, label: qualifiedName(e),
			note: elementTypeSubject + qualifiedName(e) + " is outside the document; rows are filtered by the document's classifiers specializing it"}
	}
	return typeFilter{label: qualifiedName(e), refused: elementTypeSubject + qualifiedName(e) + " is outside the document, and not a UML metaclass or a SysML stereotype"}
}

// stereotypeTypeFilter decides how a stereotype of the document filters rows.
func (m *migration) stereotypeTypeFilter(e *sysmlv1.Element) typeFilter {
	if t, ok := stereotypeTypes[e.Name]; ok && m.isLibrary(e) && libraryRoots[pathRoot(qualifiedName(e))] {
		return m.nameUsages(fromTypes("«"+e.Name+"»", t))
	}
	if m.userStereotype(e) && m.written(e) {
		return typeFilter{label: "«" + e.Name + "»", metadata: m.plainName(e)}
	}
	return typeFilter{label: "«" + e.Name + "»", refused: "«" + e.Name + "» is not written as a metadata def rows could be filtered by"}
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
	return fold(hrefDocument(href)) == fold(hrefDocument(magicDrawCustomizationModule))
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
	if t.types == nil && t.usages == nil && !t.instances {
		return typeFilter{label: name, all: true, note: t.note}
	}
	return fromTypes(name, t)
}

// nameUsages fills in the written classifiers of the filter's usage kinds,
// which the query lists by name since their type is a property's too.
func (m *migration) nameUsages(f typeFilter) typeFilter {
	if len(f.usageKinds) == 0 {
		return f
	}
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		if cat, _ := m.classify(e); slices.Contains(f.usageKinds, cat) && m.written(e) {
			f.usages = append(f.usages, m.plainName(e))
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, r := range m.model.Roots {
		if !m.isLibrary(r) {
			walk(r)
		}
	}
	return f
}

func fromTypes(label string, t v2Types) typeFilter {
	return typeFilter{label: label, types: t.types, excluding: t.excluding, usageKinds: t.usages, instances: t.instances, note: t.note}
}

// query selects from src the rows of the filter's types, less the excluded
// ones, and the rows among its named usages.
func (f typeFilter) query(src qx) qx {
	var rows qx
	typed := len(f.types) > 0
	if typed {
		rows = whereType(src, f.types...)
		if f.excluding != nil {
			outside := qcall("Except",
				qarg1("source", whereType(src, f.excluding.source...)),
				qarg1("exclude", whereType(src, f.excluding.keep...)))
			rows = qcall("Except", qarg1("source", rows), qarg1("exclude", outside))
		}
	}
	if len(f.usages) == 0 {
		if !typed {
			// Named takes at least one name; no usage written means no row.
			return qempty()
		}
		return rows
	}
	named := qcall("Named", qstrs("qualifiedName", f.usages...))
	// Named ∩ src, as the difference of named and what of it is outside src.
	usages := qcall("Except",
		qarg1("source", named),
		qarg1("exclude", qcall("Except", qarg1("source", named), qarg1("exclude", src))))
	if !typed {
		return usages
	}
	return qcall("Union", qarg1("source", rows), qarg1("other", usages))
}

func mergeTypeFilters(filters []typeFilter) typeFilter {
	var types, source, keep, usages uniqueNames
	var excluding, instances bool
	for _, f := range filters {
		types.add(f.types...)
		usages.add(f.usages...)
		instances = instances || f.instances
		if f.excluding != nil {
			excluding = true
			source.add(f.excluding.source...)
			keep.add(f.excluding.keep...)
		} else {
			keep.add(f.types...)
		}
	}
	merged := typeFilter{types: types, usages: usages, instances: instances}
	if excluding {
		merged.excluding = &typeFilterExclusion{source: source, keep: keep}
	}
	return merged
}
