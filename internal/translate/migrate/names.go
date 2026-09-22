package migrate

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// writeName writes a v1 name as a v2 name: bare when it is a basic identifier
// no keyword reserves, quoted as an unrestricted name otherwise.
func writeName(name string) string {
	if source.IsIdentifier(name) && !source.IsKeyword(name) {
		return name
	}
	return source.UnrestrictedNameText(name)
}

// nameOf returns the v2 name of an element: its own, or the one synthesized
// for an anonymous element that something refers to; "" when it is anonymous
// and stays so.
func (m *migration) nameOf(e *sysmlv1.Element) string {
	if n, ok := m.names[e]; ok {
		return n
	}
	return e.Name
}

// nameFor returns the v2 name of an element, synthesizing one for an anonymous
// element the first time it is asked for, so it can be referred to.
func (m *migration) nameFor(e *sysmlv1.Element) string {
	// An action node is named as its graph's writer names it.
	if n := m.nameNode(e); n != "" {
		return n
	}
	if n := m.nameOf(e); n != "" {
		return n
	}
	// Distinct within the owner: the id is unique, so its tail is too once the
	// whole is used on a clash.
	base := "unnamed"
	if t := m.model.Ref(e, "type"); t != nil && t.Name != "" {
		base = lowerFirst(t.Name)
	}
	name := base
	for i := 2; m.nameTaken(e.Parent, name); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	m.names[e] = name
	return name
}

func (m *migration) nameTaken(owner *sysmlv1.Element, name string) bool {
	if m.taken[owner][name] {
		return true
	}
	if owner == nil {
		return false
	}
	for _, c := range owner.Children {
		if m.nameOf(c) == name {
			return true
		}
	}
	return false
}

// take reserves a synthesized member name in owner's body.
func (m *migration) take(owner *sysmlv1.Element, name string) {
	if m.taken[owner] == nil {
		m.taken[owner] = map[string]bool{}
	}
	m.taken[owner][name] = true
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// segments returns the v2 qualified-name segments of an element: the names
// from the top-level declaration down, the root Model not being written. A
// lone region is its owner's body; one of several is a sub-state of a parallel state.
// A connection point is a member of its owner, whichever region a tool listed it in;
// a method is the body of its operation.
func (m *migration) segments(e *sysmlv1.Element) []string {
	path := m.path(e)
	segs := make([]string, len(path))
	for i, s := range path {
		segs[i] = s.name
	}
	return segs
}

// segment is one step of a qualified name; feature marks a step that is a usage.
type segment struct {
	name    string
	feature bool
}

// path returns the segments of e's qualified name, see segments.
func (m *migration) path(e *sysmlv1.Element) []segment {
	var segs []segment
	for cur := e; cur != nil; cur = memberOwner(cur) {
		if cur.Parent == nil && cur.Type == "Model" {
			break
		}
		if cur.Type == "Region" && cur.Role == "region" {
			if p, ok := m.parallel[cur]; ok {
				segs = append([]segment{{name: p}, {name: m.nameFor(cur)}}, segs...)
			}
			continue
		}
		if op := m.methodOf[cur]; op != nil {
			cur = op
		}
		segs = append([]segment{{name: m.nameFor(cur), feature: m.isUsage(cur)}}, segs...)
	}
	return segs
}

// isUsage says whether e is written as a usage whose members are features of
// it: a view or viewpoint, or a property. A feature owned by one is reached by
// a feature chain, not a qualified name.
func (m *migration) isUsage(e *sysmlv1.Element) bool {
	switch e.Type {
	case "Property", "Port":
		return true
	}
	cat, _ := m.classify(e)
	return cat == catView || cat == catViewpoint
}

// scopeChain lists scope and its ancestors, innermost first, stopping at the
// root Model, which is no scope of the output.
func scopeChain(scope *sysmlv1.Element) []*sysmlv1.Element {
	var chain []*sysmlv1.Element
	for cur := scope; cur != nil; cur = cur.Parent {
		if cur.Parent == nil && cur.Type == "Model" {
			break
		}
		chain = append(chain, cur)
	}
	return chain
}

// ref writes a reference to target from inside scope's body (nil for the top
// level): the shortest qualified name that resolves there, which is the simple
// name when target is a member of an enclosing scope no nearer scope shadows,
// and the full qualified name otherwise.
func (m *migration) ref(target, scope *sysmlv1.Element) string {
	return m.refMember(target.Parent, m.nameOf(target), m.path(target), scope)
}

// refMember writes a reference from inside scope's body to the member of owner
// named name, whose qualified name is path: a synthesized declaration written
// beside owner's members refers like one of them.
func (m *migration) refMember(owner *sysmlv1.Element, name string, path []segment, scope *sysmlv1.Element) string {
	if owner != nil && owner.Type == "Model" && owner.Parent == nil {
		owner = nil
	}
	chain := scopeChain(scope)
	for i, s := range chain {
		if s != owner {
			continue
		}
		shadowed := false
		for _, inner := range chain[:i] {
			if name != "" && m.nameTaken(inner, name) {
				shadowed = true
				break
			}
		}
		if !shadowed {
			return writeName(path[len(path)-1].name)
		}
	}
	if owner == nil && len(chain) > 0 {
		// A top-level declaration: visible everywhere unless shadowed.
		for _, inner := range chain {
			if name != "" && m.nameTaken(inner, name) {
				return m.qualifiedFrom(path, chain)
			}
		}
		return writeName(path[len(path)-1].name)
	}
	return m.qualifiedFrom(path, chain)
}

// namespaces is the path of a qualified name whose every segment is a namespace.
func namespaces(segs []string) []segment {
	path := make([]segment, len(segs))
	for i, s := range segs {
		path[i] = segment{name: s}
	}
	return path
}

// qualifiedFrom writes path, a qualified name, so it resolves from inside the
// scopes of chain: from the global namespace ($::) when one of them declares a
// member named like its first segment, which would shadow the relative path.
// A feature owned by a feature is reached by a chain: `Outer.inner`, since a
// usage's members are not accessible by qualified name.
func (m *migration) qualifiedFrom(path []segment, chain []*sysmlv1.Element) string {
	var b strings.Builder
	for _, s := range chain {
		if m.nameTaken(s, path[0].name) {
			b.WriteString("$::")
			break
		}
	}
	for i, s := range path {
		switch {
		case i == 0:
		case s.feature && path[i-1].feature:
			b.WriteString(".")
		default:
			b.WriteString("::")
		}
		b.WriteString(writeName(s.name))
	}
	return b.String()
}

func (m *migration) qualified(segs []string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = writeName(s)
	}
	return strings.Join(parts, "::")
}

// v2Name is the qualified name a report entry records for a written element.
func (m *migration) v2Name(e *sysmlv1.Element) string {
	return m.qualified(m.segments(e))
}
