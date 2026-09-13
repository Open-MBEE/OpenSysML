package repl

import (
	"sort"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// qualifiedScan bounds how many same-simple-name registrations are ranked; the
// library declares a handful of `length`s and thousands of names in total.
const qualifiedScan = 200

// qualifiedSuggestions offers the qualified names a bare name is known under,
// nearest scope first (session before library, a package's member before one
// nested in another element, shallower before deeper) and capped at suggest.Limit.
func (s *Session) qualifiedSuggestions(idx *symbols.Index, name string) []string {
	return s.qualifiedNamesOf(idx, name, writableName)
}

// qualifiedNamesOf is qualifiedSuggestions restricted to the registered names
// admit accepts.
func (s *Session) qualifiedNamesOf(idx *symbols.Index, name string, admit func(fqn string) bool) []string {
	if idx == nil || name == "" {
		return nil
	}
	type candidate struct {
		fqn     string
		library bool
		nested  bool // held by something other than a package or namespace
		depth   int
	}
	var cands []candidate
	add := func(fqn string) {
		sym := idx.Declaring(fqn)
		if sym == nil || !admit(fqn) {
			return
		}
		cands = append(cands, candidate{
			fqn:     fqn,
			library: idx.Library(sym),
			nested:  nestedInNonNamespace(idx, fqn),
			depth:   strings.Count(fqn, "::"),
		})
	}
	// A top-level name qualifies nothing, so it is not a suffix of itself.
	add(name)
	for _, fqn := range idx.FQNsEndingIn(name, qualifiedScan) {
		add(fqn)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		switch {
		case a.library != b.library:
			return !a.library
		case a.nested != b.nested:
			return !a.nested
		case a.depth != b.depth:
			return a.depth < b.depth
		case len(a.fqn) != len(b.fqn):
			return len(a.fqn) < len(b.fqn)
		}
		return a.fqn < b.fqn
	})
	out := make([]string, 0, suggest.Limit)
	for _, c := range cands {
		out = append(out, c.fqn)
		if len(out) == suggest.Limit {
			break
		}
	}
	return out
}

// nestedInNonNamespace reports whether fqn names a member of something other than
// a package or namespace — a function's parameter is no suggestion for a type.
func nestedInNonNamespace(idx *symbols.Index, fqn string) bool {
	cut := strings.LastIndex(fqn, "::")
	if cut < 0 {
		return false
	}
	owner := idx.Declaring(fqn[:cut])
	if owner == nil {
		return false
	}
	switch owner.Kind {
	case symbols.SymbolPackage, symbols.SymbolNamespace:
		return false
	}
	return true
}

// writableName reports whether every segment of fqn is an identifier, so it can be
// typed as it reads: `BaseFunctions::#::index` cannot.
func writableName(fqn string) bool {
	for _, seg := range strings.Split(fqn, "::") {
		if seg == "" {
			return false
		}
		for i, r := range seg {
			if unicode.IsLetter(r) || r == '_' || i > 0 && unicode.IsDigit(r) {
				continue
			}
			return false
		}
	}
	return true
}
