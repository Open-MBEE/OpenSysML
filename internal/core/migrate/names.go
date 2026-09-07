package migrate

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// writeName writes a v1 name as a v2 name: bare when it is a basic identifier
// no keyword reserves, quoted as an unrestricted name otherwise.
func writeName(name string) string {
	if lexer.IsIdentifier(name) && !lexer.IsKeyword(name) {
		return name
	}
	return lexer.UnrestrictedNameText(name)
}

// nameOf returns the v2 name of an element: its own, or the one synthesized
// for an anonymous element that something refers to; "" when it is anonymous
// and stays so.
func (m *migration) nameOf(e *xmi.Element) string {
	if n, ok := m.names[e]; ok {
		return n
	}
	return e.Name
}

// nameFor returns the v2 name of an element, synthesizing one for an anonymous
// element the first time it is asked for, so it can be referred to.
func (m *migration) nameFor(e *xmi.Element) string {
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

func (m *migration) nameTaken(owner *xmi.Element, name string) bool {
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
func (m *migration) take(owner *xmi.Element, name string) {
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
// from the top-level declaration down, the root Model not being written.
func (m *migration) segments(e *xmi.Element) []string {
	var segs []string
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Parent == nil && cur.Type == "Model" {
			break
		}
		segs = append([]string{m.nameFor(cur)}, segs...)
	}
	return segs
}

// scopeChain lists scope and its ancestors, innermost first, stopping at the
// root Model, which is no scope of the output.
func scopeChain(scope *xmi.Element) []*xmi.Element {
	var chain []*xmi.Element
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
func (m *migration) ref(target, scope *xmi.Element) string {
	segs := m.segments(target)
	owner := target.Parent
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
			if n := m.nameOf(target); n != "" && m.nameTaken(inner, n) {
				shadowed = true
				break
			}
		}
		if !shadowed {
			return writeName(segs[len(segs)-1])
		}
	}
	if owner == nil && len(chain) > 0 {
		// A top-level declaration: visible everywhere unless shadowed.
		for _, inner := range chain {
			if n := m.nameOf(target); n != "" && m.nameTaken(inner, n) {
				return m.qualifiedFrom(segs, chain)
			}
		}
		return writeName(segs[len(segs)-1])
	}
	return m.qualifiedFrom(segs, chain)
}

// qualifiedFrom writes segs as a qualified name that resolves from inside the
// scopes of chain: from the global namespace ($::) when one of them declares a
// member named like its first segment, which would shadow the relative path.
func (m *migration) qualifiedFrom(segs []string, chain []*xmi.Element) string {
	q := m.qualified(segs)
	for _, s := range chain {
		if m.nameTaken(s, segs[0]) {
			return "$::" + q
		}
	}
	return q
}

func (m *migration) qualified(segs []string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = writeName(s)
	}
	return strings.Join(parts, "::")
}

// v2Name is the qualified name a report entry records for a written element.
func (m *migration) v2Name(e *xmi.Element) string {
	return m.qualified(m.segments(e))
}
