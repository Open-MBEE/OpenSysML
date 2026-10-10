package sysmlv1

import (
	"bytes"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// moduleStereotype is a stereotype a used module's snapshot declares: its
// name, the profile element owning it, and the ids of the stereotypes its
// generalizations name. The snapshot is not part of the model.
type moduleStereotype struct {
	id       string
	name     string
	profile  *xmi.Element
	generals []string
}

// moduleEntry reports whether an archive entry is a MagicDraw snapshot of a
// used module's shared model, which declares the module's stereotypes by id.
func moduleEntry(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, "shared_umodel$dsnapshot") || strings.HasSuffix(lower, "shared_umodel.snapshot")
}

// indexModule reads one module snapshot: every stereotype it declares, keyed
// by id, with the profile owning it and the generals it specializes. A
// snapshot that is not XML is ignored.
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
			m.moduleStereotypes[e.ID] = moduleStereotype{id: e.ID, name: e.Name(), profile: profile, generals: moduleGenerals(e)}
		}
		for _, c := range e.Children {
			walk(c, profile)
		}
	}
	walk(doc.Root, nil)
}

// moduleGenerals lists the ids a snapshot stereotype's generalizations name,
// whether written as a general attribute, an idref or an href's fragment.
func moduleGenerals(e *xmi.Element) []string {
	var ids []string
	for _, g := range e.Children {
		if local(g.Tag) != "generalization" {
			continue
		}
		ids = append(ids, strings.Fields(g.Attr("general"))...)
		for _, target := range g.Children {
			if local(target.Tag) != "general" {
				continue
			}
			for _, raw := range []string{target.Attr("href"), target.Attr("idref")} {
				if raw == "" {
					continue
				}
				if i := strings.LastIndexByte(raw, '#'); i >= 0 {
					raw = raw[i+1:]
				}
				ids = append(ids, raw)
			}
		}
	}
	return ids
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
// stereotype table binds one of its stereotypes to, or, failing a table row,
// the namespace an application is written under that denotes the profile by
// URI or by name, so ids of the profile's other stereotypes resolve to a name
// and namespace too.
func (m *Model) bindModuleProfiles() {
	profiles := map[*xmi.Element]string{}
	for id, known := range m.stereotypeNames {
		if s, ok := m.moduleStereotypes[id]; ok {
			profiles[s.profile] = known.namespace
		}
	}
	for _, s := range m.moduleStereotypes {
		if _, bound := profiles[s.profile]; bound {
			continue
		}
		if ns := m.appliedNamespaceDenoting(s.profile); ns != "" {
			profiles[s.profile] = ns
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

// appliedNamespaceDenoting is the one namespace the document's applications
// are written under that denotes module profile p; "" when none or several do.
func (m *Model) appliedNamespaceDenoting(p *xmi.Element) string {
	found := ""
	for _, s := range m.Stereotypes {
		if s.Namespace == found || !moduleProfileDenoted(s.Namespace, p) {
			continue
		}
		if found != "" {
			return ""
		}
		found = s.Namespace
	}
	return found
}

// moduleProfileDenoted reports whether application namespace ns names module
// profile p: by the profile's URI, or by the namespace's document spelling
// the profile's name, as denotes does for a profile the document defines.
func moduleProfileDenoted(ns string, p *xmi.Element) bool {
	if uri := p.Attr("URI"); uri != "" && uri == ns {
		return true
	}
	doc := namespaceDocument(ns)
	return doc != "" && SameName(p.Name(), doc)
}

// moduleDefinition finds the snapshot declaration of the stereotype an
// application instantiates when no document read defines it: the one the
// tool's stereotypesHREFS table names, else the single declaration of the
// same name in a module profile the application's namespace denotes.
func (m *Model) moduleDefinition(s *Stereotype) (moduleStereotype, bool) {
	if href, ok := m.stereotypeHrefs[stereotypeKey{s.Namespace, s.Name}]; ok {
		id := href
		if i := strings.LastIndexByte(href, '#'); i >= 0 {
			id = href[i+1:]
		}
		decl, ok := m.moduleStereotypes[id]
		return decl, ok
	}
	var found moduleStereotype
	var any bool
	for _, decl := range m.moduleStereotypes {
		if !SameName(decl.name, s.Name) || !moduleProfileDenoted(s.Namespace, decl.profile) {
			continue
		}
		if any && found.id != decl.id {
			return moduleStereotype{}, false
		}
		found, any = decl, true
	}
	return found, any
}

// moduleAncestors lists the stereotypes a snapshot declaration specializes,
// transitively, nearest first and each once: elements of the documents read
// when they define one, else elements standing for the snapshot's own.
func (m *Model) moduleAncestors(decl moduleStereotype) []*Element {
	seen := map[string]bool{decl.id: true}
	var out []*Element
	queue := append([]string(nil), decl.generals...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		if e, ok := m.byID[id]; ok {
			out = append(out, e)
			for _, a := range m.Ancestors(e) {
				if !seen[a.ID] {
					seen[a.ID] = true
					out = append(out, a)
				}
			}
			continue
		}
		g, ok := m.moduleStereotypes[id]
		if !ok {
			continue
		}
		out = append(out, m.moduleElement(g))
		queue = append(queue, g.generals...)
	}
	return out
}

// moduleElement is the element standing for a snapshot stereotype: named and
// typed as the snapshot declares it, owned by an element standing for its
// profile, and in no document read.
func (m *Model) moduleElement(decl moduleStereotype) *Element {
	if e, ok := m.moduleElements[decl.id]; ok {
		return e
	}
	if m.moduleElements == nil {
		m.moduleElements = map[string]*Element{}
	}
	profile, ok := m.moduleElements[decl.profile.ID]
	if !ok {
		profile = &Element{ID: decl.profile.ID, Type: "Profile", Name: decl.profile.Name(),
			Attrs: map[string]string{}, refs: map[string][]string{}}
		if uri := decl.profile.Attr("URI"); uri != "" {
			profile.Attrs["URI"] = uri
		}
		m.moduleElements[decl.profile.ID] = profile
	}
	e := &Element{ID: decl.id, Type: "Stereotype", Role: "packagedElement", Name: decl.name, Parent: profile,
		Attrs: map[string]string{}, refs: map[string][]string{}}
	profile.Children = append(profile.Children, e)
	m.moduleElements[decl.id] = e
	return e
}

// StereotypeAncestors lists the stereotypes the one an id or href names
// specializes, transitively, as the archive's module snapshots declare them;
// none for an id no snapshot declares.
func (m *Model) StereotypeAncestors(id string) []StereotypeRef {
	frag := id
	if i := strings.LastIndexByte(id, '#'); i >= 0 {
		frag = id[i+1:]
	}
	decl, ok := m.moduleStereotypes[frag]
	if !ok {
		return nil
	}
	var refs []StereotypeRef
	for _, a := range m.moduleAncestors(decl) {
		refs = append(refs, m.StereotypeRef(a.ID))
	}
	return refs
}
