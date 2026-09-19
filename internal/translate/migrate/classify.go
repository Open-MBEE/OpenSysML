package migrate

import (
	"net/url"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// category is the SysML v2 declaration a v1 classifier or package becomes.
type category int

const (
	catNone category = iota
	catPackage
	catPartDef
	catPortDef
	catAttributeDef
	catEnumDef
	catConstraintDef
	catRequirementDef
	catConnectionDef
	catIndividualDef
	catVerificationDef
	catItemDef
	// catActionDef is a behavior with a v2 action form: an activity, an
	// operation, or an interaction whose messages are signal sends.
	catActionDef
	// catCalcDef is an opaque or function behavior computing a result.
	catCalcDef
	catStateDef
	// catSimConfig is a simulation tool's run configuration: an action def
	// that instantiates its execution target and performs its behavior.
	catSimConfig
	// catValue is an instance of a value type: an attribute usage holding its
	// slot values, since an individual cannot specialize an attribute def.
	catValue
	// catLibrary marks profile and bundled-library content that is not migrated.
	catLibrary
	// catUnmapped marks a classifier this migration has no v2 form for.
	catUnmapped
)

// keyword is the v2 declaration keyword of a category; "" for none.
func (c category) keyword() string {
	switch c {
	case catPackage:
		return "package"
	case catPartDef:
		return "part def"
	case catPortDef:
		return "port def"
	case catAttributeDef:
		return "attribute def"
	case catEnumDef:
		return "enum def"
	case catConstraintDef:
		return "constraint def"
	case catRequirementDef:
		return "requirement def"
	case catConnectionDef:
		return "connection def"
	case catIndividualDef:
		return "individual def"
	case catVerificationDef:
		return "verification def"
	case catItemDef:
		return "item def"
	case catActionDef:
		return "action def"
	case catCalcDef:
		return "calc def"
	case catStateDef:
		return "state def"
	case catSimConfig:
		return "action def"
	case catValue:
		return "attribute"
	}
	return ""
}

// requirementStereotypes are the SysML profile's requirement stereotypes.
var requirementStereotypes = []string{"Requirement", "AbstractRequirement"}

// scalarValues maps the v1 primitive value type names to the ScalarValues
// library types they correspond to.
var scalarValues = map[string]string{
	"Real":             "Real",
	"Integer":          "Integer",
	"Boolean":          "Boolean",
	"String":           "String",
	"UnlimitedNatural": "Natural",
	"Natural":          "Natural",
	"Complex":          "Complex",
	"Number":           "Number",
	"Rational":         "Rational",
}

// hostScalars maps the machine-level datatypes a modeling tool's own library
// offers, named as a programming language names them, to ScalarValues types.
var hostScalars = map[string]string{
	"float":   "Real",
	"double":  "Real",
	"int":     "Integer",
	"long":    "Integer",
	"short":   "Integer",
	"byte":    "Integer",
	"boolean": "Boolean",
}

// libraryRoots are the names of the SysML and UML profile and model library
// packages an export carries alongside the user's model.
var libraryRoots = map[string]bool{
	"SysML":                true,
	"StandardProfile":      true,
	"UML Standard Profile": true,
	"QUDV":                 true,
	"ISO-80000":            true,
	"SI Definitions":       true,
	"SIDefinitions":        true,
	"PrimitiveTypes":       true,
	"PrimitiveValueTypes":  true,
	"Libraries":            true,
}

// isStandard reports whether s comes from a standard profile rather than a
// user's own, whose same-named stereotypes carry no SysML meaning.
func isStandard(s *sysmlv1.Stereotype) bool {
	return isStandardNamespace(s.Namespace)
}

// isStandardNamespace matches, by host and path, the OMG SysML and UML profiles,
// Eclipse UML2's UML standard profile and Papyrus' SysML profile; nothing else.
func isStandardNamespace(ns string) bool {
	u, err := url.Parse(ns)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path := u.Path
	switch {
	case host == "omg.org" || strings.HasSuffix(host, ".omg.org"):
		return strings.HasPrefix(path, "/spec/SysML/") || strings.HasPrefix(path, "/spec/UML/")
	case host == "eclipse.org" || strings.HasSuffix(host, ".eclipse.org"):
		return (strings.HasPrefix(path, "/uml2/") && strings.Contains(path, "/UML/Profile/Standard")) ||
			strings.HasPrefix(strings.ToLower(path), "/papyrus/sysml/")
	}
	return false
}

// isMagicDrawCustomization matches the namespace MagicDraw and Cameo give
// their SysML customization profile (…magicdraw.com/spec/Customization/…).
func isMagicDrawCustomization(ns string) bool {
	u, err := url.Parse(ns)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	return (host == "magicdraw.com" || host == "nomagic.com") &&
		strings.HasPrefix(strings.ToLower(u.Path), "/spec/customization/")
}

// stereo returns e's application of the named standard-profile stereotype, or nil.
func stereo(e *sysmlv1.Element, name string) *sysmlv1.Stereotype {
	for _, s := range e.Stereotypes {
		if s.Name == name && isStandard(s) {
			return s
		}
	}
	return nil
}

// has reports whether any of the named standard-profile stereotypes applies to e.
func has(e *sysmlv1.Element, names ...string) bool {
	for _, n := range names {
		if stereo(e, n) != nil {
			return true
		}
	}
	return false
}

// isLibrary reports whether e sits in profile or bundled-library content: a
// profile, a package the model marks as a library or auxiliary resource, or a
// document root with a library name that sits beside the user's Model.
func (m *migration) isLibrary(e *sysmlv1.Element) bool {
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Type == "Profile" || has(cur, "ModelLibrary", "modelLibrary", "auxiliaryResource") {
			return true
		}
		if cur.Parent == nil && cur.Type != "Model" && libraryRoots[cur.Name] && m.besideUserModel(cur) {
			return true
		}
	}
	return false
}

// besideUserModel reports whether another root of root's document is a Model
// or a package not named like a library, which the user's content then is.
func (m *migration) besideUserModel(root *sysmlv1.Element) bool {
	for _, r := range m.model.Roots {
		if r == root || r.IsProxy() {
			continue
		}
		if r.Type == "Model" || (r.Type == "Package" && !libraryRoots[r.Name]) {
			return true
		}
	}
	return false
}

// primitiveLibraryHref reports whether an href points into the UML or SysML
// primitive type libraries rather than into another user project.
func primitiveLibraryHref(href string) bool {
	path := href
	if i := strings.IndexByte(href, '#'); i >= 0 {
		path = href[:i]
	}
	for _, lib := range []string{"PrimitiveTypes", "PrimitiveValueTypes", "/spec/UML/", "/spec/SysML/"} {
		if strings.Contains(path, lib) {
			return true
		}
	}
	return false
}

// libraryReference reports whether a proxy points into a standard library or
// profile module: the qualified name the tool records starts in a library
// root, and the document the href names is that library's own module, not a
// used project whose top package happens to share the name.
func libraryReference(t *sysmlv1.Element) bool {
	root := pathRoot(t.QualifiedName)
	return root != "" && libraryRoots[root] && fold(hrefDocument(t.Href)) == fold(root)
}

// modelExtensions are the file extensions a model or module document carries.
var modelExtensions = []string{".mdzip", ".mdxml", ".xmi", ".xml", ".uml", ".zip"}

// hrefDocument is the module an href names: the document without its
// directory, query, fragment or model extension.
func hrefDocument(href string) string {
	doc := href
	if i := strings.IndexAny(doc, "#?"); i >= 0 {
		doc = doc[:i]
	}
	if i := strings.LastIndexAny(doc, "/\\"); i >= 0 {
		doc = doc[i+1:]
	}
	if u, err := url.PathUnescape(doc); err == nil {
		doc = u
	}
	for _, ext := range modelExtensions {
		if len(doc) > len(ext) && strings.EqualFold(doc[len(doc)-len(ext):], ext) {
			return doc[:len(doc)-len(ext)]
		}
	}
	return doc
}

// fold lowers a name and drops the separators tools vary in.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '.':
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// pathRoot is the first segment of a qualified name.
func pathRoot(qualified string) string {
	if i := strings.Index(qualified, "::"); i >= 0 {
		return qualified[:i]
	}
	return qualified
}

// quantityLibraries are the model libraries whose every value type is a
// quantity value: a number stated in a unit.
var quantityLibraries = map[string]bool{"ISO-80000": true}

// quantityValueType reports whether t is a value type of a quantity library.
func (m *migration) quantityValueType(t *sysmlv1.Element) bool {
	if t.IsProxy() {
		return quantityLibraries[pathRoot(t.QualifiedName)] && libraryReference(t)
	}
	return t.Type == "DataType" && m.isLibrary(t) && quantityLibraries[rootOf(t).Name]
}

// quantity reports whether value type e is a magnitude: its «ValueType» names
// a unit or quantity kind and it has no fields of its own.
func quantity(e *sysmlv1.Element) bool {
	vt := stereo(e, "ValueType")
	if vt == nil || len(e.Owned("ownedAttribute")) > 0 {
		return false
	}
	return vt.Tag("unit") != "" || vt.Tag("quantityKind") != ""
}

// scalarBase returns the ScalarValues type the values of type t are, through
// its generalizations; "" when t is structured or its base is unknown.
func (m *migration) scalarBase(t *sysmlv1.Element) string {
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element) string
	walk = func(t *sysmlv1.Element) string {
		if t == nil || seen[t] {
			return ""
		}
		seen[t] = true
		if sv := m.scalarValue(t); sv != "" {
			return sv
		}
		if m.quantityValueType(t) {
			return "Real"
		}
		if t.IsProxy() {
			return ""
		}
		if cat, _ := m.classify(t); cat != catAttributeDef {
			return ""
		}
		structured := false
		for _, g := range t.Owned("generalization") {
			general := m.model.Ref(g, "general")
			if sv := walk(general); sv != "" {
				return sv
			}
			if general != nil && !general.IsProxy() && !m.isLibrary(general) {
				if gc, _ := m.classify(general); gc == catAttributeDef {
					structured = true
				}
			}
		}
		if !structured && quantity(t) {
			return "Real"
		}
		return ""
	}
	return walk(t)
}

// structuredValueType reports whether t is written as a value type with no
// ScalarValues base, one no literal can be a value of.
func (m *migration) structuredValueType(t *sysmlv1.Element) bool {
	if t == nil || t.IsProxy() || m.isLibrary(t) || m.scalarBase(t) != "" {
		return false
	}
	cat, _ := m.classify(t)
	return cat == catAttributeDef || cat == catEnumDef
}

// scalarValue returns the ScalarValues type a v1 type maps to, or "" when the
// type is the user's own: a primitive the UML or SysML libraries define, whether
// referenced by href or bundled in the document, or a tool library's own
// machine-level datatype.
func (m *migration) scalarValue(t *sysmlv1.Element) string {
	if t == nil {
		return ""
	}
	name, ok := scalarValues[t.Name]
	if !ok {
		if name, ok = hostScalars[t.Name]; !ok {
			return ""
		}
	}
	if t.IsProxy() {
		if primitiveLibraryHref(t.Href) || libraryReference(t) {
			return name
		}
		return ""
	}
	if (t.Type == "PrimitiveType" || t.Type == "DataType") && m.isLibrary(t) {
		return name
	}
	return ""
}

// classify decides which v2 declaration a classifier or package becomes, and
// why the choice is only an approximation when it is.
func (m *migration) classify(e *sysmlv1.Element) (category, string) {
	if e.IsProxy() {
		return catNone, ""
	}
	if m.isLibrary(e) {
		return catLibrary, ""
	}
	switch e.Type {
	case "Model", "Package":
		return catPackage, ""
	case "Profile":
		return catLibrary, ""
	case "Class", "Component":
		switch {
		case simulationConfig(e) != nil:
			return catSimConfig, ""
		case has(e, requirementStereotypes...):
			return catRequirementDef, ""
		case has(e, "ConstraintBlock"):
			return catConstraintDef, ""
		case has(e, "InterfaceBlock"):
			return catPortDef, ""
		case has(e, "Block"):
			return catPartDef, ""
		case has(e, "Stakeholder"):
			return catPartDef, "a v1 «Stakeholder» is written as a part def"
		case has(e, "View"):
			return catUnmapped, "views are not migrated yet"
		case has(e, "Viewpoint"):
			return catUnmapped, "viewpoints are not migrated yet"
		}
		return catPartDef, "a plain UML class without «Block» is written as a part def"
	case "Actor":
		return catPartDef, "a UML actor is written as a part def"
	case "AssociationClass":
		return catConnectionDef, ""
	case "Association":
		return catConnectionDef, ""
	case "DataType":
		if has(e, "ValueType") {
			return catAttributeDef, ""
		}
		return catAttributeDef, "a UML data type without «ValueType» is written as an attribute def"
	case "PrimitiveType":
		return catAttributeDef, ""
	case "Enumeration":
		return catEnumDef, ""
	case "Signal":
		return catItemDef, ""
	case "Interface":
		return catPortDef, "a UML interface is written as a port def"
	case "InstanceSpecification":
		if has(e, "Unit", "QuantityKind") {
			return catUnmapped, "units and quantity kinds are not migrated; use the SI and ISQ libraries"
		}
		if len(m.classifiersOf(e)) == 0 {
			return catUnmapped, joinNotes("an instance specification without a classifier has no v2 form", m.snapshots[e].note)
		}
		occurrences, values, note := m.instanceClassifiers(e)
		switch {
		case len(occurrences) == 0 && len(values) == 0:
			return catUnmapped, note
		case len(occurrences) == 0:
			return catValue, note
		}
		for _, v := range values {
			note = joinNotes(note, "the instance's classifier "+qualifiedName(v)+" is not written: an individual cannot specialize a value type")
		}
		return catIndividualDef, note
	case "Activity", "OpaqueBehavior", "Interaction", "StateMachine", "FunctionBehavior":
		if has(e, "TestCase") {
			if e.Type == "Interaction" {
				if _, note := m.scenario(e, m.subjectName(e)); note != "" {
					return catVerificationDef, "the test case's scenario is not migrated: " + note + "; only its verified requirements are"
				}
				return catVerificationDef, ""
			}
			return catVerificationDef, "the test case's behavior is not migrated; only its verified requirements are"
		}
		return m.classifyBehavior(e)
	case "Operation":
		return catActionDef, ""
	case "Reception":
		return catUnmapped, "a reception names the signal its owner accepts, which the owner's behaviors carry as accept"
	case "UseCase":
		return catUnmapped, "use cases are not migrated yet"
	case "Collaboration", "Node", "Device", "ExecutionEnvironment", "Artifact":
		return catUnmapped, "no v2 form for a UML " + e.Type
	}
	return catUnmapped, "no v2 form for a UML " + e.Type
}

// kindOf names the v1 element as its author saw it: its classifying
// stereotype in guillemets, else its UML metaclass.
func kindOf(e *sysmlv1.Element) string {
	if len(e.Stereotypes) > 0 {
		return "«" + e.Stereotypes[0].Name + "»" + " " + e.Type
	}
	return e.Type
}

// qualifiedName joins the v1 names from below the root model to e.
func qualifiedName(e *sysmlv1.Element) string {
	if e.IsProxy() {
		if e.QualifiedName != "" {
			return e.QualifiedName
		}
		if e.Name != "" {
			return e.Name
		}
		return e.Href
	}
	path := e.Path()
	if len(path) > 1 && rootOf(e).Type == "Model" {
		path = path[1:]
	}
	return strings.Join(path, "::")
}

func rootOf(e *sysmlv1.Element) *sysmlv1.Element {
	for e.Parent != nil {
		e = e.Parent
	}
	return e
}

// instanceClassifiers splits an instance's classifiers into the occurrence
// definitions an individual can specialize, the value types an attribute can
// be typed by, and a note over those it can use as neither.
func (m *migration) instanceClassifiers(e *sysmlv1.Element) (occurrences, values []*sysmlv1.Element, note string) {
	var notes []string
	if snap, ok := m.snapshots[e]; ok && len(m.model.Refs(e, "classifier")) == 0 {
		for _, c := range snap.classifiers {
			notes = append(notes, "classified by "+qualifiedName(c)+", the owner of its slots' defining features, since it names no classifier and is a result snapshot of "+"the run configuration "+describe(snap.config))
		}
	}
	for _, c := range m.classifiersOf(e) {
		if c.IsProxy() || m.isLibrary(c) {
			notes = append(notes, "the instance's classifier "+c.Name+" is outside the document or in a library, so it has no v2 definition to specialize")
			continue
		}
		switch cc, _ := m.classify(c); {
		case cc.keyword() == "":
			notes = append(notes, "the instance's classifier "+qualifiedName(c)+" is not migrated")
		case cc == catAttributeDef, cc == catEnumDef:
			values = append(values, c)
		default:
			occurrences = append(occurrences, c)
		}
	}
	return occurrences, values, strings.Join(notes, "; ")
}

// classifiersOf is the classifiers an instance names, or, when it names none
// and is a result snapshot under a run configuration's result location, the
// one its slots prove it of (indexSnapshots). Any other classifier-less instance has none.
func (m *migration) classifiersOf(e *sysmlv1.Element) []*sysmlv1.Element {
	if named := m.model.Refs(e, "classifier"); len(named) > 0 {
		return named
	}
	return m.snapshots[e].classifiers
}

// individualClassifiers returns the kind an individual takes from its first classifier
// of a kind (part def, constraint def; a port def gives none) and the classifiers of that kind.
func (m *migration) individualClassifiers(e *sysmlv1.Element) (kind category, written []*sysmlv1.Element, note string) {
	occurrences, _, _ := m.instanceClassifiers(e)
	kinds := make([]category, len(occurrences))
	for i, c := range occurrences {
		cc, _ := m.classify(c)
		if cc == catPortDef {
			cc = catNone
		}
		kinds[i] = cc
		if kind == catNone {
			kind = cc
		}
	}
	var notes []string
	for i, c := range occurrences {
		switch {
		case kinds[i] == kind:
			written = append(written, c)
		case kinds[i] == catNone:
			notes = append(notes, "the instance's classifier "+qualifiedName(c)+" is not written: an "+individualKeyword(kind)+" cannot specialize a port def")
		default:
			notes = append(notes, "the instance's classifier "+qualifiedName(c)+" is not written: an "+individualKeyword(kind)+" cannot specialize a "+kinds[i].keyword())
		}
	}
	return kind, written, strings.Join(notes, "; ")
}

// individualKeyword is the declaration keyword of an individual of the kind:
// `individual part def`, or `individual def` for an instance of an interface block.
func individualKeyword(kind category) string {
	if kind == catNone {
		return "individual def"
	}
	return "individual " + kind.keyword()
}
