package sysmlv1

import (
	"net/url"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// IsSysMLNamespace reports whether ns is an OMG SysML v1 profile namespace,
// of any version, or Papyrus' copy of it.
func IsSysMLNamespace(ns string) bool {
	u, err := url.Parse(ns)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch {
	case host == "omg.org" || strings.HasSuffix(host, ".omg.org"):
		return strings.HasPrefix(u.Path, "/spec/SysML/")
	case host == "eclipse.org" || strings.HasSuffix(host, ".eclipse.org"):
		return strings.HasPrefix(strings.ToLower(u.Path), "/papyrus/sysml/")
	}
	return false
}

// Exact namespaces of the tool profiles the reader gives a typed form. Cameo
// serializes user profiles under the same host as its own, so only an exact
// namespace, never its host, identifies tool content.
const (
	// MagicDrawProfileNS is MagicDraw's own profile: tables and relation maps.
	MagicDrawProfileNS = "http://www.omg.org/spec/UML/20131001/MagicDrawProfile"
	// DependencyMatrixNS is MagicDraw's dependency matrix profile.
	DependencyMatrixNS = "http://www.magicdraw.com/schemas/Dependency_Matrix_Profile.xmi"
	// DocGenNS is the Open-MBEE MDK document generation profile.
	DocGenNS = "http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
	// DocGenCollaboratorNS is the MDK view collaborator profile.
	DocGenCollaboratorNS = "http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi"
)

// StereotypeRef names a stereotype an id refers to: by the namespace and name
// its profile declared it under when the document tells, by its element when
// the profile is bundled, and by the raw id always.
type StereotypeRef struct {
	// ID is the id or href as written.
	ID string
	// Name is the stereotype's name; "" when the document does not tell.
	Name string
	// Namespace is the profile namespace the name belongs to; "" when unknown.
	Namespace string
	// Element is the bundled stereotype element; nil when the profile is not
	// in the read documents.
	Element *Element
}

// Known reports whether the document told what stereotype the id names.
func (r StereotypeRef) Known() bool {
	return r.Name != ""
}

// stereotypeNames indexes MagicDraw's stereotypesHREFS table: what a tool
// stereotype id or href is called and under which namespace.
type stereotypeName struct {
	name, namespace string
}

// indexStereotypes reads one stereotypesHREFS table: each entry names a
// stereotype "prefix:Name" and its href, the prefix bound by an xmlns
// declaration in scope. Entries whose prefix is unbound are skipped. The
// first href recorded for a namespace and name is the one applications
// resolve their definition through.
func (m *Model) indexStereotypes(raw *xmi.Element) {
	for _, entry := range raw.Children {
		if entry.Tag != "stereotype" {
			continue
		}
		prefix, name, ok := strings.Cut(entry.Attr("name"), ":")
		href := entry.Attr("stereotypeHREF")
		if !ok || name == "" || href == "" {
			continue
		}
		namespace := entry.Namespace(prefix)
		if namespace == "" {
			continue
		}
		if m.stereotypeNames == nil {
			m.stereotypeNames = map[string]stereotypeName{}
		}
		key := stereotypeKey{namespace, name}
		if _, dup := m.stereotypeHrefs[key]; !dup {
			m.stereotypeHrefs[key] = href
		}
		known := stereotypeName{name: name, namespace: namespace}
		m.stereotypeNames[href] = known
		if i := strings.LastIndexByte(href, '#'); i >= 0 {
			m.stereotypeNames[href[i+1:]] = known
		}
	}
}

// StereotypeRef resolves an id or href to the stereotype it names: through
// the tool's stereotype table, else through a bundled Stereotype element,
// whose profile namespace is the owning Profile's URI when it declares one.
func (m *Model) StereotypeRef(id string) StereotypeRef {
	ref := StereotypeRef{ID: id, Element: m.byID[id]}
	frag := id
	if i := strings.LastIndexByte(id, '#'); i >= 0 {
		frag = id[i+1:]
		if ref.Element == nil {
			ref.Element = m.byID[frag]
		}
	}
	for _, key := range []string{id, frag} {
		if known, ok := m.stereotypeNames[key]; ok {
			ref.Name, ref.Namespace = known.name, known.namespace
			return ref
		}
	}
	if e := ref.Element; e != nil && e.Type == "Stereotype" {
		ref.Name = e.Name
		for p := e.Parent; p != nil; p = p.Parent {
			if p.Type == "Profile" {
				ref.Namespace = p.Attrs["URI"]
				break
			}
		}
	}
	return ref
}

// TagRefs resolves a reference-valued tag of a stereotype application to the
// elements it names, one ElementRef per id in serialized order, an href of
// another document yielding its proxy and a dangling id a nil Element.
func (m *Model) TagRefs(s *Stereotype, tag string) []ElementRef {
	ids := s.IDs(tag)
	if len(ids) == 0 {
		return nil
	}
	refs := make([]ElementRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, m.elementRef(id))
	}
	return refs
}

// elementRef resolves one id or href as a table definition wrote it.
func (m *Model) elementRef(id string) ElementRef {
	ref := ElementRef{ID: id, Element: m.shown(id)}
	if ref.Element == nil && strings.Contains(id, "#") {
		ref.Element = m.proxy(id)
	}
	return ref
}

// Applications lists the stereotype applications of one exact profile
// namespace, in document order.
func (m *Model) Applications(namespace string) []*Stereotype {
	var out []*Stereotype
	for _, s := range m.Stereotypes {
		if s.Namespace == namespace {
			out = append(out, s)
		}
	}
	return out
}

// Applied returns e's application of the named stereotype from one exact
// profile namespace, or nil; a homonym from another profile does not count.
func (e *Element) Applied(namespace, name string) *Stereotype {
	if e == nil {
		return nil
	}
	for _, s := range e.Stereotypes {
		if s.Name == name && s.Namespace == namespace {
			return s
		}
	}
	return nil
}

// Bool reads a boolean tag; absent or unparsable is false.
func (s *Stereotype) Bool(name string) bool {
	return s.Tag(name) == "true"
}
