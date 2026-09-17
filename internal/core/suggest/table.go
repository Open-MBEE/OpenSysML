package suggest

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Table indexes what an index registers, once, so a suggestion costs a lookup
// and a search of the plausible spellings rather than a scan of every name.
type Table struct {
	idx *symbols.Index
	// byName maps a simple name to the qualified spellings ending in it, and
	// byLength groups the simple names by rune length so a typo search visits
	// only the lengths its tolerance admits.
	byName   map[string][]string
	byLength map[int][]lowered
	// sorted holds the simple names in order, so the names a word starts are a
	// binary search away.
	sorted []string
	// simple remembers the simple name each qualified spelling was filed under,
	// so Refresh can unfile it without knowing what the index now says.
	simple map[string]string
}

// lowered is a candidate spelling beside its lowercase form, folded once.
type lowered struct {
	name, lower string
}

// NewTable sweeps the index once. It reads the index, so Refresh it with the
// names that change afterwards, or build a new one.
func NewTable(idx *symbols.Index) *Table {
	t := &Table{idx: idx, byName: map[string][]string{}, byLength: map[int][]lowered{}, simple: map[string]string{}}
	if idx == nil {
		return t
	}
	idx.Registered(func(fqn string, syms []*symbols.Symbol) {
		last := simpleName(fqn, syms)
		if _, seen := t.byName[last]; !seen {
			n := len([]rune(last))
			t.byLength[n] = append(t.byLength[n], lowered{name: last, lower: strings.ToLower(last)})
			t.sorted = append(t.sorted, last)
		}
		t.byName[last] = append(t.byName[last], fqn)
		t.simple[fqn] = last
	})
	sort.Strings(t.sorted)
	return t
}

// Refresh refiles the qualified names the index changed since the table last
// read it, one lookup each: the table then equals one built afresh.
func (t *Table) Refresh(names map[string]bool) {
	if t.idx == nil {
		return
	}
	for fqn := range names {
		t.unfile(fqn)
		if syms := t.idx.RegisteredUnder(fqn); len(syms) > 0 {
			t.file(fqn, simpleName(fqn, syms))
		}
	}
}

// file records fqn under the simple name last, entering last if it is new.
func (t *Table) file(fqn, last string) {
	if _, seen := t.byName[last]; !seen {
		n := len([]rune(last))
		t.byLength[n] = append(t.byLength[n], lowered{name: last, lower: strings.ToLower(last)})
		at := sort.SearchStrings(t.sorted, last)
		t.sorted = append(t.sorted, "")
		copy(t.sorted[at+1:], t.sorted[at:])
		t.sorted[at] = last
	}
	t.byName[last] = append(t.byName[last], fqn)
	t.simple[fqn] = last
}

// unfile forgets fqn, and the simple name it was filed under once nothing else is.
func (t *Table) unfile(fqn string) {
	last, ok := t.simple[fqn]
	if !ok {
		return
	}
	delete(t.simple, fqn)
	fqns := t.byName[last]
	for i, f := range fqns {
		if f == fqn {
			fqns = append(fqns[:i], fqns[i+1:]...)
			break
		}
	}
	if len(fqns) > 0 {
		t.byName[last] = fqns
		return
	}
	delete(t.byName, last)
	n := len([]rune(last))
	for i, c := range t.byLength[n] {
		if c.name == last {
			t.byLength[n] = append(t.byLength[n][:i], t.byLength[n][i+1:]...)
			break
		}
	}
	if len(t.byLength[n]) == 0 {
		delete(t.byLength, n)
	}
	if at := sort.SearchStrings(t.sorted, last); at < len(t.sorted) && t.sorted[at] == last {
		t.sorted = append(t.sorted[:at], t.sorted[at+1:]...)
	}
}

// simpleName is the name fqn registers a declaration under: the declared name,
// which may hold `::` of its own, else the last segment (as a short name is).
func simpleName(fqn string, syms []*symbols.Symbol) string {
	for _, sym := range syms {
		if sym != nil && symbols.HasFQN(sym, fqn) {
			if sym.Name != "" {
				return sym.Name
			}
			break
		}
	}
	return LastSegment(fqn)
}

// Unquoted returns the registered simple names word is the unquoted start of
// (see Unquoted), in name order.
func (t *Table) Unquoted(word string) []string {
	if word == "" {
		return nil
	}
	from := sort.SearchStrings(t.sorted, word)
	to := from
	for to < len(t.sorted) && strings.HasPrefix(t.sorted[to], word) {
		to++
	}
	return Unquoted(word, t.sorted[from:to])
}

// Declared is Qualified over every spelling, including the names that must be
// written quoted.
func (t *Table) Declared(name string) []string {
	return t.qualified(name, func(string) bool { return true })
}

// Qualified returns the qualified names, shortest first, under which the index
// declares the simple name name: `Integer` is declared as `ScalarValues::Integer`.
func (t *Table) Qualified(name string) []string {
	return t.qualified(name, typable)
}

// qualified is Qualified over the spellings admit accepts.
func (t *Table) qualified(name string, admit func(fqn string) bool) []string {
	if t.idx == nil || name == "" {
		return nil
	}
	cands := append([]string(nil), t.byName[name]...)
	sort.Slice(cands, func(i, j int) bool {
		if len(cands[i]) != len(cands[j]) {
			return len(cands[i]) < len(cands[j])
		}
		return cands[i] < cands[j]
	})
	var out []string
	for i, fqn := range cands {
		if i == scanLimit {
			break
		}
		if admit(fqn) && t.idx.Declaring(fqn) != nil {
			out = append(out, fqn)
			if len(out) == Limit {
				break
			}
		}
	}
	return out
}

// Neighbours returns the registered simple names within word's edit-distance
// budget, closest first, at most NeighbourLimit of them: what a caller scores
// against the scope the name was written in.
func (t *Table) Neighbours(word string) []Neighbour {
	n := len([]rune(word))
	budget := Budget(n)
	lower := strings.ToLower(word)
	var hits []Neighbour
	for l := n - budget; l <= n+budget; l++ {
		for _, c := range t.byLength[l] {
			if c.name == word {
				continue
			}
			if EditDistance(lower, c.lower) <= budget {
				hits = append(hits, Neighbour{Name: c.name, Distance: EditDistance(word, c.name)})
			}
		}
	}
	return nearestFirst(hits)
}
