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
}

var standardProvenance = provenance{isStandardNamespace, isStandardDefinition}

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

// toolContent says why e is the modeling tool's own content rather than the
// model's: an element of the tool's specification-dialog customization or UI
// prototyping profile, or a classifier its simulation profile configures other
// than a run configuration. "" for model content, which the tool's markers
// annotate: a classifier with a standard stereotype or a behavior of its own is
// the model's however the tool also draws it.
func toolContent(e *sysmlv1.Element) string {
	if isClassifier(e) && (behaves(e) || slices.ContainsFunc(e.Stereotypes, hasStandardName)) {
		return ""
	}
	for _, s := range e.Stereotypes {
		purpose := toolProfile(s.Namespace)
		switch {
		case purpose == "" || isMagicDrawCustomization(s.Namespace):
			continue
		case isSimulationProfile(s.Namespace):
			if isSimulationConfig(s) || !isClassifier(e) {
				continue
			}
		}
		return "«" + s.Name + "» marks " + purpose + ", which has no model meaning"
	}
	return ""
}

// isClassifier reports whether e is a UML class or component.
func isClassifier(e *sysmlv1.Element) bool {
	return e.Type == "Class" || e.Type == "Component"
}

// behaves reports whether classifier e runs: it is active, names a classifier
// behavior, or owns a behavior.
func behaves(e *sysmlv1.Element) bool {
	if e.Attrs["isActive"] == "true" || e.Attrs["classifierBehavior"] != "" {
		return true
	}
	for _, c := range e.Children {
		if isBehavior(c) {
			return true
		}
	}
	return false
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
