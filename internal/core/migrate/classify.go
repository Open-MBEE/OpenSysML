package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
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

// standardStereotypeNamespaces are the XML namespaces of the profiles whose
// stereotypes the mapping reads: the OMG SysML and UML standard profiles and
// Papyrus' serialization of them.
var standardStereotypeNamespaces = []string{
	"omg.org/spec/SysML", "omg.org/spec/UML/", "eclipse.org/papyrus/", "/UML/Profile/Standard",
}

// isStandard reports whether s comes from a standard profile rather than a
// user's own, whose same-named stereotypes carry no SysML meaning.
func isStandard(s *xmi.Stereotype) bool {
	for _, ns := range standardStereotypeNamespaces {
		if strings.Contains(s.Namespace, ns) {
			return true
		}
	}
	return false
}

// stereo returns e's application of the named standard-profile stereotype, or nil.
func stereo(e *xmi.Element, name string) *xmi.Stereotype {
	for _, s := range e.Stereotypes {
		if s.Name == name && isStandard(s) {
			return s
		}
	}
	return nil
}

// has reports whether any of the named standard-profile stereotypes applies to e.
func has(e *xmi.Element, names ...string) bool {
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
func (m *migration) isLibrary(e *xmi.Element) bool {
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
func (m *migration) besideUserModel(root *xmi.Element) bool {
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

// scalarValue returns the ScalarValues type a v1 type maps to, or "" when the
// type is the user's own: a primitive the UML or SysML libraries define, whether
// referenced by href or bundled in the document.
func (m *migration) scalarValue(t *xmi.Element) string {
	if t == nil {
		return ""
	}
	name, ok := scalarValues[t.Name]
	if !ok {
		return ""
	}
	if t.IsProxy() {
		if primitiveLibraryHref(t.Href) {
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
func (m *migration) classify(e *xmi.Element) (category, string) {
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
		return catAttributeDef, "a signal is written as an attribute def; v2 has no signal classifier"
	case "Interface":
		return catPortDef, "a UML interface is written as a port def"
	case "InstanceSpecification":
		if has(e, "Unit", "QuantityKind") {
			return catUnmapped, "units and quantity kinds are not migrated; use the SI and ISQ libraries"
		}
		if len(m.model.Refs(e, "classifier")) == 0 {
			return catUnmapped, "an instance specification without a classifier has no v2 form"
		}
		written, note := m.instanceClassifiers(e)
		if len(written) == 0 {
			return catUnmapped, note
		}
		return catIndividualDef, note
	case "Activity", "OpaqueBehavior", "Interaction", "StateMachine", "FunctionBehavior":
		if has(e, "TestCase") {
			return catVerificationDef, "the test case's behavior is not migrated; only its verified requirements are"
		}
		return catUnmapped, "behaviors are not migrated yet"
	case "UseCase":
		return catUnmapped, "use cases are not migrated yet"
	case "Collaboration", "Node", "Device", "ExecutionEnvironment", "Artifact":
		return catUnmapped, "no v2 form for a UML " + e.Type
	}
	return catUnmapped, "no v2 form for a UML " + e.Type
}

// kindOf names the v1 element as its author saw it: its classifying
// stereotype in guillemets, else its UML metaclass.
func kindOf(e *xmi.Element) string {
	if len(e.Stereotypes) > 0 {
		return "«" + e.Stereotypes[0].Name + "»" + " " + e.Type
	}
	return e.Type
}

// qualifiedName joins the v1 names from below the root model to e.
func qualifiedName(e *xmi.Element) string {
	path := e.Path()
	if len(path) > 1 && rootOf(e).Type == "Model" {
		path = path[1:]
	}
	return strings.Join(path, "::")
}

func rootOf(e *xmi.Element) *xmi.Element {
	for e.Parent != nil {
		e = e.Parent
	}
	return e
}

// instanceClassifiers splits an instance's classifiers into those it can
// specialize in v2 and a note over those it cannot.
func (m *migration) instanceClassifiers(e *xmi.Element) ([]*xmi.Element, string) {
	var written []*xmi.Element
	var notes []string
	for _, c := range m.model.Refs(e, "classifier") {
		if c.IsProxy() || m.isLibrary(c) {
			notes = append(notes, "the instance's classifier "+c.Name+" is outside the document or in a library, so it has no v2 definition to specialize")
			continue
		}
		if cc, _ := m.classify(c); cc.keyword() == "" {
			notes = append(notes, "the instance's classifier "+qualifiedName(c)+" is not migrated")
			continue
		}
		written = append(written, c)
	}
	return written, strings.Join(notes, "; ")
}
