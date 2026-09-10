package repl

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// searchLimit bounds a %search listing: the library declares thousands of
// names, and a listing the user must scroll past discovers nothing.
const searchLimit = 40

// browseIndex is the session's symbol index, or one holding the standard
// library alone, so library symbols are reachable from an empty prompt.
func (s *Session) browseIndex() *symbols.Index {
	if idx := s.symbolIndex(); idx != nil {
		return idx
	}
	if s.idx == nil {
		s.idx, s.libSource = model.NewIndexWithStdlib()
	}
	return s.idx
}

// doSearch lists the indexed symbols whose fully-qualified name contains
// substr, case-insensitively, with the kind of each.
func (s *Session) doSearch(substr string) ([]string, bool, error) {
	idx := s.browseIndex()
	if idx == nil {
		return []string{"no symbols to search"}, false, nil
	}
	want := strings.ToLower(substr)
	type match struct {
		fqn    string
		kind   string
		onName bool // the substring matched the symbol's own name, not an ancestor's
	}
	var matches []match
	for _, fqn := range idx.FQNs() {
		lower := strings.ToLower(fqn)
		if !strings.Contains(lower, want) {
			continue
		}
		// Only where a name is declared: the library re-exports most names
		// through wildcard imports, which would bury the declaration.
		sym := idx.Declaring(fqn)
		if sym == nil {
			continue
		}
		matches = append(matches, match{
			fqn:    fqn,
			kind:   sym.Notation(),
			onName: strings.Contains(suggest.LastSegment(lower), want),
		})
	}
	if len(matches) == 0 {
		return []string{fmt.Sprintf("no symbol matches %q", substr)}, false, nil
	}
	// A name that matched itself comes before one that matched through an
	// ancestor, and a shallower name before a deeper one.
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.onName != b.onName {
			return a.onName
		}
		if da, db := strings.Count(a.fqn, "::"), strings.Count(b.fqn, "::"); da != db {
			return da < db
		}
		if len(a.fqn) != len(b.fqn) {
			return len(a.fqn) < len(b.fqn)
		}
		return a.fqn < b.fqn
	})
	shown := matches
	if len(shown) > searchLimit {
		shown = shown[:searchLimit]
	}
	out := make([]string, 0, len(shown)+1)
	for _, m := range shown {
		// Spelled as the notation writes it, so a hit can be typed back into a
		// command that takes a name.
		out = append(out, fmt.Sprintf("%s  %s", notationName(m.fqn), m.kind))
	}
	if len(matches) > len(shown) {
		out = append(out, fmt.Sprintf("(%d more; narrow the search)", len(matches)-len(shown)))
	}
	return out, false, nil
}

// doBuiltins lists the library functions this build implements directly, each
// beside the declaration it is and the import its unqualified name needs.
func (s *Session) doBuiltins() ([]string, bool, error) {
	all := runtime.Builtins()
	scalar := make([]string, 0, len(all))
	collection := make([]string, 0, len(all))
	for _, b := range all {
		if b.Collection {
			collection = append(collection, fmt.Sprintf("x->%s()  %s  (import %s::*;)", b.Name, b.FQN, b.Package))
			continue
		}
		scalar = append(scalar, fmt.Sprintf("%s(%s)  %s  (import %s::*;)", b.Name, strings.Join(b.Params, ", "), b.FQN, b.Package))
	}
	out := []string{
		"Each function is called by its unqualified name only where the model imports",
		"its package, as the checker resolves it; the qualified name works anywhere,",
		"e.g. RealFunctions::sqrt(2.0). Where several packages declare the name, each",
		"import makes its own declaration the one a bare call denotes.",
		"",
		"Scalar functions:",
	}
	out = append(out, scalar...)
	out = append(out, "", "Collection and control functions (also callable as name(x, ...)):")
	out = append(out, collection...)
	return out, false, nil
}

// suggestCommand offers the meta commands closest to an unknown one.
func suggestCommand(cmd string) []string {
	return suggest.Nearest(cmd, metaCommands())
}

// suggestSymbol offers the names closest to one the session could not find,
// its own declarations first as the likelier target of a typo.
func (s *Session) suggestSymbol(name string) []string {
	idx := s.browseIndex()
	if idx == nil {
		return nil
	}
	simple := suggest.LastSegment(name)
	if hits := suggest.Nearest(simple, s.declaredSymbolNames()); len(hits) > 0 {
		return hits
	}
	var out []string
	for _, last := range suggest.Nearest(simple, suggest.SimpleNames(idx)) {
		if cands := s.qualifiedSuggestions(idx, last); len(cands) > 0 {
			out = append(out, cands[0])
		}
	}
	return out
}

// declaredSymbolNames returns the names the session documents declare, at every
// nesting level, sorted.
func (s *Session) declaredSymbolNames() []string {
	return s.nameTable().sorted()
}

// notFoundError reports a name no declaration answers to, offering the
// qualified name the index does know it under, or the nearest spellings. want
// narrows what is offered to the kinds the command can act on.
func (s *Session) notFoundError(name string, want ...symbols.SymbolKind) error {
	// Spelled as the notation writes it, so the name in the failure is the name
	// that was typed — text still carrying quotes is one the notation could not
	// read as a name, and is reported as typed rather than quoted again.
	shown := name
	if !strings.Contains(name, "'") {
		shown = notationName(name)
	}
	err := unresolvedError(shown)
	msg := err.Error()
	unquoted := s.unquotedNames(name, want)
	if strings.Contains(name, "::") {
		return suggestionError(err, msg, suggest.Hint(msg, name, s.qualifiedMissSuggestions(name, want), unquoted))
	}
	if idx := s.browseIndex(); idx != nil {
		if qualified := suggest.Hint(msg, name, s.matchingKinds(s.qualifiedSuggestions(idx, name), want), unquoted); qualified != msg {
			return suggestionError(err, msg, qualified)
		}
	}
	// Quoted last, so kind lookups see the raw indexed names.
	near := notationNames(s.matchingKinds(s.suggestSymbol(name), want))
	return suggestionError(err, msg, suggest.Hint(msg, name, near, unquoted))
}

// unquotedNames returns the registered names of the wanted kinds that the typed
// name is the unquoted start of: `T::SA` for a 'SA-506' declared under T.
func (s *Session) unquotedNames(name string, want []symbols.SymbolKind) []string {
	idx := s.browseIndex()
	if idx == nil {
		return nil
	}
	var out []string
	if cut := strings.LastIndex(name, "::"); cut >= 0 {
		prefix, ok := s.qualifierFQN(idx, name[:cut])
		if !ok {
			return nil
		}
		// Every known name, so a member the qualifier re-exports is a candidate;
		// offered only when the spelling resolves as a command would look it up.
		for _, member := range suggest.Unquoted(name[cut+2:], s.knownSimpleNames(idx)) {
			if fqn := prefix + "::" + member; len(idx.LookupQualified(fqn)) == 1 {
				out = append(out, fqn)
			}
		}
		return s.matchingKinds(out, want)
	}
	for _, full := range suggest.Unquoted(name, s.knownSimpleNames(idx)) {
		for _, sym := range s.nameTable().lookup(full) {
			if fqn := knownAs(idx.GetFQN(sym), full); fqn != "" && !slices.Contains(out, fqn) {
				out = append(out, fqn)
			}
		}
		if cands := s.qualifiedNamesOf(idx, full, func(string) bool { return true }); len(cands) > 0 {
			if !slices.Contains(out, cands[0]) {
				out = append(out, cands[0])
			}
		}
	}
	return s.matchingKinds(out, want)
}

// knownAs respells fqn to end in the name it was found under: its short name,
// where that is what was matched, rather than the declared one.
func knownAs(fqn, name string) string {
	if fqn == "" || suggest.LastSegment(fqn) == name {
		return fqn
	}
	if cut := strings.LastIndex(fqn, "::"); cut >= 0 {
		return fqn[:cut+2] + name
	}
	return name
}

// qualifierFQN is the registered name of the one declaration a qualifier
// denotes; a qualifier that resolves nowhere, or to several, qualifies nothing.
func (s *Session) qualifierFQN(idx *symbols.Index, qualifier string) (string, bool) {
	matches := idx.LookupQualified(qualifier)
	if len(matches) != 1 {
		return "", false
	}
	if fqn := idx.GetFQN(matches[0]); fqn != "" {
		return fqn, true
	}
	return qualifier, true
}

// knownSimpleNames returns every simple name the session or the index declares,
// sorted.
func (s *Session) knownSimpleNames(idx *symbols.Index) []string {
	names := append(s.declaredSymbolNames(), suggest.SimpleNames(idx)...)
	sort.Strings(names)
	return slices.Compact(names)
}

// qualifiedMissSuggestions offers the members of a qualified name's own
// qualifier nearest its last segment. A near name under another qualifier is
// not what was typed, so a qualifier that resolves nowhere offers nothing.
func (s *Session) qualifiedMissSuggestions(name string, want []symbols.SymbolKind) []string {
	idx := s.browseIndex()
	if idx == nil {
		return nil
	}
	cut := strings.LastIndex(name, "::")
	qualifier, last := name[:cut], name[cut+2:]
	prefix, ok := s.qualifierFQN(idx, qualifier)
	if !ok {
		return nil
	}
	var out []string
	for _, member := range suggest.Nearest(last, s.memberNames(idx, prefix)) {
		out = append(out, prefix+"::"+member)
	}
	// Quoted last, so kind lookups see the raw indexed names and the hint can
	// be typed back into a command.
	return notationNames(s.matchingKinds(out, want))
}

// memberNames lists the simple names the index registers directly under prefix.
func (s *Session) memberNames(idx *symbols.Index, prefix string) []string {
	seen := map[string]bool{}
	var out []string
	for _, fqn := range idx.FQNs() {
		rest, ok := strings.CutPrefix(fqn, prefix+"::")
		if !ok || strings.Contains(rest, "::") || seen[rest] {
			continue
		}
		seen[rest] = true
		out = append(out, rest)
	}
	sort.Strings(out)
	return out
}

// matchingKinds keeps the candidates a command of the wanted kinds can act on,
// so `-action` is not offered a function. Nothing wanted keeps every candidate.
func (s *Session) matchingKinds(cands []string, want []symbols.SymbolKind) []string {
	if len(want) == 0 || len(cands) == 0 {
		return cands
	}
	out := make([]string, 0, len(cands))
	for _, cand := range cands {
		if s.candidateHasKind(cand, want) {
			out = append(out, cand)
		}
	}
	return out
}

// candidateHasKind reports whether any declaration cand denotes is of a wanted
// kind, read from the index or from the session's own scope trees.
func (s *Session) candidateHasKind(name string, want []symbols.SymbolKind) bool {
	var matches []*symbols.Symbol
	if idx := s.browseIndex(); idx != nil {
		matches = idx.LookupQualified(name)
	}
	if len(matches) == 0 {
		matches = s.nameTable().lookup(name)
	}
	for _, sym := range matches {
		if slices.Contains(want, sym.Kind) {
			return true
		}
	}
	return false
}

// suggestionError offers what suggested added to msg while keeping err's
// sentinel, which callers match on to tell a missing name from other failures.
func suggestionError(err error, msg, suggested string) error {
	if suggested == msg {
		return err
	}
	return fmt.Errorf("%w%s", err, strings.TrimPrefix(suggested, msg))
}

// unknownCommandLine reports an unrecognised meta command, naming the closest
// spellings when the input is near one.
func unknownCommandLine(cmd string) string {
	if cands := suggestCommand(cmd); len(cands) > 0 {
		return fmt.Sprintf("unknown command %q — did you mean %s?", cmd, suggest.OrList(cands))
	}
	return fmt.Sprintf("unknown command %q (try %%help)", cmd)
}
