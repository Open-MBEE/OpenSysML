package migrate

import (
	"net/url"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// isStandard reports whether s is applied from a standard profile itself,
// rather than from a user's own, whose same-named stereotypes carry no SysML meaning.
func isStandard(s *sysmlv1.Stereotype) bool {
	return isStandardNamespace(s.Namespace) || (s.Definition != nil && isStandardDefinition(s.Definition))
}

// provenance tells a profile's stereotypes apart from same-named ones elsewhere:
// by the namespace an application is written under, or by where a definition lives.
type provenance struct {
	namespace  func(ns string) bool
	definition func(d *sysmlv1.Element) bool
	// generals, when set, names the profile's stereotypes an application
	// inherits from a definition no document or module read declares.
	generals func(s *sysmlv1.Stereotype) []string
}

var standardProvenance = provenance{namespace: isStandardNamespace, definition: isStandardDefinition, generals: toolGenerals}

// toolBlockStereotypes are the stereotypes MagicDraw and Cameo define in their
// SysML profile, under the OMG SysML namespace, as specializations of «Block»:
// the SysML Plugin's element descriptions document each as a block.
var toolBlockStereotypes = map[string]bool{"System": true, "Subsystem": true, "Domain": true, "External": true}

// toolGenerals names the standard stereotype the modeling tool's own SysML
// profile derives s from when the profile is in no document or module read,
// so the generalization is invisible: «Subsystem» specializes «Block».
func toolGenerals(s *sysmlv1.Stereotype) []string {
	if !isToolBlockStereotype(s) {
		return nil
	}
	return []string{"Block"}
}

// isToolBlockStereotype reports whether s applies one of the tool's block
// stereotypes under the SysML namespace with no document or snapshot declaring it.
func isToolBlockStereotype(s *sysmlv1.Stereotype) bool {
	return s.Definition == nil && !s.Module && sysmlv1.IsSysMLNamespace(s.Namespace) && toolBlockStereotypes[s.Name]
}

// toolBlockStereotype returns e's application read as a block only through
// toolGenerals, or nil when the document or a module declares the generalization.
func toolBlockStereotype(e *sysmlv1.Element) *sysmlv1.Stereotype {
	var found *sysmlv1.Stereotype
	for _, s := range e.Stereotypes {
		switch {
		case isToolBlockStereotype(s):
			if found == nil {
				found = s
			}
		case appliesStandard(s, "Block"):
			return nil
		}
	}
	return found
}

// applies reports whether s applies the named stereotype of the profile p tells
// apart: directly, or through a definition specializing it, since a user
// stereotype inherits its general's meaning.
func (p provenance) applies(s *sysmlv1.Stereotype, name string) bool {
	return slices.Contains(p.names(s), name)
}

// names lists the stereotypes of the profile p tells apart that s applies:
// itself when the profile defines it, and every one its definition specializes.
func (p provenance) names(s *sysmlv1.Stereotype) []string {
	var names []string
	if p.namespace(s.Namespace) || s.Definition != nil && p.definition(s.Definition) {
		names = append(names, s.Name)
	}
	for _, g := range s.Generals {
		if p.definition(g) {
			names = append(names, g.Name)
		}
	}
	if p.generals != nil {
		names = append(names, p.generals(s)...)
	}
	return names
}

// standardNames lists the standard-profile stereotypes application s applies.
func standardNames(s *sysmlv1.Stereotype) []string {
	return standardProvenance.names(s)
}

// hasStandardName reports whether s applies any standard-profile stereotype.
func hasStandardName(s *sysmlv1.Stereotype) bool {
	return len(standardNames(s)) > 0
}

// appliesStandard reports whether s applies the named standard stereotype,
// directly or through its definition's generalizations.
func appliesStandard(s *sysmlv1.Stereotype, name string) bool {
	return standardProvenance.applies(s, name)
}

// appliesAny reports whether s applies any of the named standard stereotypes.
func appliesAny(s *sysmlv1.Stereotype, names ...string) bool {
	for _, n := range names {
		if appliesStandard(s, n) {
			return true
		}
	}
	return false
}

// stereo returns e's application carrying the named standard-profile
// stereotype's meaning, directly or by inheritance, or nil.
func stereo(e *sysmlv1.Element, name string) *sysmlv1.Stereotype {
	for _, s := range e.Stereotypes {
		if appliesStandard(s, name) {
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

// isStandardDefinition reports whether stereotype definition d belongs to a
// standard profile: a proxy into the OMG, Eclipse or bundled library modules,
// or a definition read from a bundled standard profile.
func isStandardDefinition(d *sysmlv1.Element) bool {
	if d.IsProxy() {
		return isStandardHref(d.Href) || libraryReference(d)
	}
	for cur := d.Parent; cur != nil; cur = cur.Parent {
		if cur.Type == "Profile" || cur.Parent == nil {
			return libraryRoots[cur.Name] || isStandardNamespace(cur.Attrs["URI"])
		}
	}
	return false
}

// isStandardHref matches an href into the OMG normative XMI of SysML or UML, or
// the Eclipse pathmaps of the UML metamodel, its libraries and the SysML profiles.
func isStandardHref(href string) bool {
	doc := href
	if i := strings.IndexByte(doc, '#'); i >= 0 {
		doc = doc[:i]
	}
	if isStandardNamespace(doc) {
		return true
	}
	u, err := url.Parse(doc)
	if err != nil || !strings.EqualFold(u.Scheme, "pathmap") {
		return false
	}
	host := strings.ToUpper(u.Host)
	switch host {
	case "UML_METAMODELS", "UML_LIBRARIES", "UML_PROFILES":
		return true
	}
	return strings.HasPrefix(host, "SYSML") && strings.HasSuffix(host, "_PROFILES")
}

// toolHost reports whether ns is served from the hosts MagicDraw and Cameo
// name their own profiles under, and returns its lower-cased path.
func toolHost(ns string) (path string, ok bool) {
	u, err := url.Parse(ns)
	if err != nil {
		return "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "magicdraw.com" && host != "nomagic.com" {
		return "", false
	}
	return strings.ToLower(u.Path), true
}

// toolProfilePaths are the exact namespace paths of the modeling tool's own
// profiles whose applications configure the tool, with what each configures.
var toolProfilePaths = map[string]string{
	"/schemas/simulationprofile.xmi":      "the simulation tool's configuration",
	"/schemas/dsl_customization.xmi":      "the modeling tool's specification-dialog customization",
	"/schemas/ui_prototyping_profile.xmi": "the modeling tool's UI prototyping mockup",
}

// toolProfile names what the modeling tool's profile at namespace ns
// configures; "" for a namespace that is not one of its known profiles, which a
// user's profile hosted under the tool's domain also is.
func toolProfile(ns string) string {
	if isMagicDrawCustomization(ns) {
		return "the modeling tool's SysML customization"
	}
	path, ok := toolHost(ns)
	if !ok {
		return ""
	}
	return toolProfilePaths[path]
}

// isSimulationProfile matches, by host and exact path, MagicDraw's own
// SimulationProfile; a profile of that name elsewhere is not it.
func isSimulationProfile(ns string) bool {
	path, ok := toolHost(ns)
	return ok && path == "/schemas/simulationprofile.xmi"
}

// toolContent says why e is the modeling tool's own content: a tool profile defines
// its kind and it holds nothing of the model. "" for model content, which the tool's markers annotate.
func toolContent(e *sysmlv1.Element) string {
	if isPackage(e) {
		return ""
	}
	s := toolMarker(e)
	if s == nil || holdsModelContent(e) {
		return ""
	}
	return "«" + s.Name + "» marks " + toolProfile(s.Namespace) + ", which has no model meaning"
}

// toolMarker returns e's application of a tool profile's stereotype extending e's kind, not every
// element's; the simulation profile's only on a classifier that is no run configuration. Nil for none.
func toolMarker(e *sysmlv1.Element) *sysmlv1.Stereotype {
	for _, s := range e.Stereotypes {
		switch {
		case toolProfile(s.Namespace) == "" || isMagicDrawCustomization(s.Namespace):
			continue
		case abstractMetaclasses[s.BaseMetaclass]:
			continue
		case isSimulationProfile(s.Namespace) && (isSimulationConfig(s) || !isClassifier(e)):
			continue
		}
		return s
	}
	return nil
}

// abstractMetaclasses are UML 2.5.1's: a stereotype extending one applies to every
// kind of element under it, so it annotates its element and defines no kind of its own.
var abstractMetaclasses = map[string]bool{
	"Element": true, "NamedElement": true, "Namespace": true, "PackageableElement": true,
	"TypedElement": true, "MultiplicityElement": true, "Relationship": true, "DirectedRelationship": true,
	"TemplateableElement": true, "ParameterableElement": true, "Type": true, "Classifier": true,
	"RedefinableElement": true, "Feature": true, "StructuralFeature": true, "BehavioralFeature": true,
	"ConnectableElement": true, "StructuredClassifier": true, "EncapsulatedClassifier": true,
	"BehavioredClassifier": true, "Behavior": true, "ValueSpecification": true, "LiteralSpecification": true,
	"Event": true, "MessageEvent": true, "Observation": true, "Vertex": true, "DeploymentTarget": true,
	"DeployedArtifact": true, "Action": true, "ActivityNode": true, "ActivityEdge": true, "ActivityGroup": true,
	"ControlNode": true, "ExecutableNode": true, "FinalNode": true, "ObjectNode": true, "InvocationAction": true,
	"CallAction": true, "StructuralFeatureAction": true, "WriteStructuralFeatureAction": true,
	"VariableAction": true, "WriteVariableAction": true, "LinkAction": true, "WriteLinkAction": true,
	"InteractionFragment": true, "MessageEnd": true, "ExecutionSpecification": true,
}

// isPackage reports whether e only groups its members, each classified on its own.
func isPackage(e *sysmlv1.Element) bool {
	return e.Type == "Model" || e.Type == "Package" || e.Type == "Profile"
}

// holdsModelContent reports whether e or an element it owns at any depth is the model's however the
// tool marks it: a package, a behavior, a classifier that runs, an element with a standard stereotype.
func holdsModelContent(e *sysmlv1.Element) bool {
	if isPackage(e) || isBehavior(e) || e.Attrs["isActive"] == "true" || e.Attrs["classifierBehavior"] != "" ||
		slices.ContainsFunc(e.Stereotypes, hasStandardName) {
		return true
	}
	return slices.ContainsFunc(e.Children, holdsModelContent)
}

// isClassifier reports whether e is a UML class or component.
func isClassifier(e *sysmlv1.Element) bool {
	return e.Type == "Class" || e.Type == "Component"
}

// libraryReason says why library content e is skipped: as one of the modeling
// tool's own profiles when its profile is one, else as profile or library content.
func (m *migration) libraryReason(e *sysmlv1.Element) string {
	if p := enclosingProfile(e); p != nil || e.Type == "Profile" {
		if p == nil {
			p = e
		}
		for _, ns := range m.profileNamespaces(p) {
			if purpose := toolProfile(ns); purpose != "" {
				return purpose + " profile, which has no model meaning"
			}
		}
	}
	return "profile or library content"
}

// userProfile reports whether profile p is a user's own, whose stereotypes are
// written as metadata defs: not a standard profile by name or namespace, and not
// one of the modeling tool's known profiles.
func (m *migration) userProfile(p *sysmlv1.Element) bool {
	if user, ok := m.userProfiles[p]; ok {
		return user
	}
	user := !libraryRoots[p.Name]
	for _, ns := range m.profileNamespaces(p) {
		if isStandardNamespace(ns) || toolProfile(ns) != "" {
			user = false
		}
	}
	m.userProfiles[p] = user
	return user
}

// profileNamespaces lists the XML namespaces profile p is applied under: its
// declared URI and the namespaces of the applications resolved to its stereotypes.
func (m *migration) profileNamespaces(p *sysmlv1.Element) []string {
	if m.namespacesOf == nil {
		m.namespacesOf = map[*sysmlv1.Element][]string{}
		for _, s := range m.model.Stereotypes {
			if s.Definition == nil {
				continue
			}
			for cur := s.Definition.Parent; cur != nil; cur = cur.Parent {
				if cur.Type == "Profile" {
					if !slices.Contains(m.namespacesOf[cur], s.Namespace) {
						m.namespacesOf[cur] = append(m.namespacesOf[cur], s.Namespace)
					}
					break
				}
			}
		}
	}
	nss := m.namespacesOf[p]
	if uri := p.Attrs["URI"]; uri != "" && !slices.Contains(nss, uri) {
		nss = append(nss, uri)
	}
	return nss
}
