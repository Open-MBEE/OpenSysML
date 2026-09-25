package sysmlv1

import (
	"bytes"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// moduleStereotype is a stereotype a used module's snapshot declares: its name
// and the profile element owning it. The snapshot is not part of the model.
type moduleStereotype struct {
	name    string
	profile *xmi.Element
}

// moduleEntry reports whether an archive entry is a MagicDraw snapshot of a
// used module's shared model, which declares the module's stereotypes by id.
func moduleEntry(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, "shared_umodel$dsnapshot") || strings.HasSuffix(lower, "shared_umodel.snapshot")
}

// indexModule reads one module snapshot: every stereotype it declares, keyed
// by id, with the profile owning it. A snapshot that is not XML is ignored.
func (m *Model) indexModule(data []byte) {
	doc, err := xmi.Parse(bytes.NewReader(data))
	if err != nil || doc.Root == nil {
		return
	}
	if m.moduleStereotypes == nil {
		m.moduleStereotypes = map[string]moduleStereotype{}
	}
	var walk func(e, profile *xmi.Element)
	walk = func(e, profile *xmi.Element) {
		if isModuleKind(e, "Profile") {
			profile = e
		}
		if isModuleKind(e, "Stereotype") && e.ID != "" && profile != nil {
			m.moduleStereotypes[e.ID] = moduleStereotype{name: e.Name(), profile: profile}
		}
		for _, c := range e.Children {
			walk(c, profile)
		}
	}
	walk(doc.Root, nil)
}

// isModuleKind reports whether a snapshot element is of the UML kind, by its
// xmi:type or xsi:type or, at the root, by its uml-namespaced tag.
func isModuleKind(e *xmi.Element, kind string) bool {
	if e.Type != "" {
		return local(e.Type) == kind
	}
	if t := e.Attr("type"); t != "" {
		return local(t) == kind
	}
	return e.Tag == kind && xmi.IsUMLNamespace(e.Space)
}

// bindModuleProfiles gives each module profile the namespace the document's
// stereotype table binds one of its stereotypes to, so ids of the profile's
// other stereotypes resolve to a name and namespace too.
func (m *Model) bindModuleProfiles() {
	profiles := map[*xmi.Element]string{}
	for id, known := range m.stereotypeNames {
		if s, ok := m.moduleStereotypes[id]; ok {
			profiles[s.profile] = known.namespace
		}
	}
	for id, s := range m.moduleStereotypes {
		if _, known := m.stereotypeNames[id]; known {
			continue
		}
		if ns, ok := profiles[s.profile]; ok {
			if m.stereotypeNames == nil {
				m.stereotypeNames = map[string]stereotypeName{}
			}
			m.stereotypeNames[id] = stereotypeName{name: s.name, namespace: ns}
		}
	}
}
