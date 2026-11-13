package resolve

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ResolveGlobal resolves a qualified name from the root namespace (KerML 8.2.3.5), each
// segment through the visible memberships of the one before; quiet, and nothing memoized under it.
func (r *Resolver) ResolveGlobal(segments []string) []*symbols.Symbol {
	var elements []*symbols.Symbol
	r.Query("", func() {
		r.aside(func() {
			for _, sym := range r.globalMembers(segments) {
				if element := r.AliasedElement(sym); !r.AliasNamesNothing(element) &&
					!slices.Contains(elements, element) {
					elements = append(elements, element)
				}
			}
		})
	})
	return elements
}

// ResolveGlobalMembers is ResolveGlobal with an alias in the last segment
// answered as the alias membership rather than the element it names.
func (r *Resolver) ResolveGlobalMembers(segments []string) []*symbols.Symbol {
	var found []*symbols.Symbol
	r.Query("", func() {
		r.aside(func() { found = r.globalMembers(segments) })
	})
	return found
}

func (r *Resolver) globalMembers(segments []string) []*symbols.Symbol {
	if len(segments) == 0 || segments[0] == "" {
		return nil
	}
	all := r.globalCandidates(nil, segments[0])
	if len(segments) > 1 && len(all) > 1 {
		all = all[:1]
	}
	for _, name := range segments[1:] {
		if name == "" || len(all) != 1 {
			return nil
		}
		cur := r.AliasedElement(all[0])
		if r.AliasNamesNothing(cur) {
			return nil
		}
		all = r.membersNamed(nil, cur, name, true, nil)
	}
	return all
}
