// Package suggest offers the spellings a name that did not resolve may have
// meant: the qualified names the index declares it under, or the nearest
// spellings by edit distance. Shared by every surface reporting an unresolved name.
package suggest

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Limit bounds how many candidates a "did you mean" offers.
const Limit = 3

// scanLimit bounds how many same-named registrations are ranked; the library
// re-exports popular names many times.
const scanLimit = 200

// NeighbourLimit bounds how many near spellings are considered for one name,
// so a common short name does not cost a scope search per registered name.
const NeighbourLimit = 25

// slack is how much further than the best candidate another may be and still be
// offered: a name two or more edits worse is not the same guess.
const slack = 1

// Candidate is a spelling a name that did not resolve may have meant, beside
// what makes it plausible: how far it is from the typed name, and how the user
// would reach it.
type Candidate struct {
	Spelling string // what to offer: a name usable as written, else a qualified path
	Distance int    // edit distance from the typed name
	InScope  bool   // the spelling resolves from the reference's scope as written
	Library  bool   // declared by bundled library content, not by the workspace
}

// reach ranks how the user would have to reach a candidate: a spelling that
// resolves where they typed it beats one only a qualified path reaches, and
// their own declaration beats a bundled library name.
func (c Candidate) reach() int {
	r := 0
	if !c.InScope {
		r += 2
	}
	if c.Library {
		r++
	}
	return r
}

// dominates reports whether c rules out other: it is at least as close and at
// least as reachable, and strictly better in one of the two. A candidate a
// better one rules out is not offered, so a distant library name is not listed
// beside the declaration one edit from the typed name.
func (c Candidate) dominates(other Candidate) bool {
	if c.Distance > other.Distance || c.reach() > other.reach() {
		return false
	}
	return c.Distance < other.Distance || c.reach() < other.reach()
}

// Rank offers the candidates no other candidate rules out, closest and most
// reachable first, at most Limit of them. Ordering is total, so the same
// candidate set always reads the same way.
func Rank(cands []Candidate) []string {
	offerable := make([]Candidate, 0, len(cands))
	best := -1
	for _, c := range cands {
		if c.Spelling == "" {
			continue
		}
		offerable = append(offerable, c)
		if best < 0 || c.Distance < best {
			best = c.Distance
		}
	}
	kept := make([]Candidate, 0, len(offerable))
	for _, c := range offerable {
		if c.Distance > best+slack || dominated(c, offerable) {
			continue
		}
		kept = append(kept, c)
	}
	sort.Slice(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		switch {
		case a.Distance != b.Distance:
			return a.Distance < b.Distance
		case a.reach() != b.reach():
			return a.reach() < b.reach()
		case len(a.Spelling) != len(b.Spelling):
			return len(a.Spelling) < len(b.Spelling)
		}
		return a.Spelling < b.Spelling
	})
	seen := map[string]bool{}
	out := make([]string, 0, Limit)
	for _, c := range kept {
		if seen[c.Spelling] {
			continue
		}
		seen[c.Spelling] = true
		out = append(out, c.Spelling)
		if len(out) == Limit {
			break
		}
	}
	return out
}

// dominated reports whether any candidate rules c out.
func dominated(c Candidate, cands []Candidate) bool {
	for _, other := range cands {
		if other.Spelling != c.Spelling && other.dominates(c) {
			return true
		}
	}
	return false
}

// Qualified returns the qualified names, shortest first, under which the index
// declares the simple name name: `Integer` is declared as `ScalarValues::Integer`.
func Qualified(idx *symbols.Index, name string) []string {
	if idx == nil || name == "" {
		return nil
	}
	var out []string
	// A top-level name qualifies nothing, so it is not a suffix of itself.
	if idx.Declaring(name) != nil {
		out = append(out, name)
	}
	for _, fqn := range idx.FQNsEndingIn(name, scanLimit) {
		if typable(fqn) && idx.Declaring(fqn) != nil {
			out = append(out, fqn)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	if len(out) > Limit {
		out = out[:Limit]
	}
	return out
}

// typable reports whether every segment of fqn is an identifier, so the
// candidate can be written as it reads: an operator member such as
// `BaseFunctions::#::index` is no use as a suggestion.
func typable(fqn string) bool {
	for _, seg := range strings.Split(fqn, "::") {
		if !lexer.IsIdentifier(seg) {
			return false
		}
	}
	return true
}

// globalRoot starts a qualified name read from the document root.
const globalRoot = "$::"

// Notation writes a registered qualified name as it is typed back: a segment no
// basic name spells is quoted, one the library writes bare (`when`) is left so.
func Notation(fqn string) string {
	if fqn == "" {
		return fqn
	}
	rest, global := strings.CutPrefix(fqn, globalRoot)
	segments := strings.Split(rest, "::")
	for i, seg := range segments {
		if !lexer.IsIdentifier(seg) {
			segments[i] = "'" + seg + "'"
		}
	}
	if global {
		return globalRoot + strings.Join(segments, "::")
	}
	return strings.Join(segments, "::")
}

// identifierRune reports whether r can appear in a basic name as the lexer
// scans one; a name holding any other character is written quoted.
func identifierRune(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// Unquoted returns the names, in the order given, that the identifier word starts
// and that go on with a character no basic name holds: `SA` starts 'SA-506'.
func Unquoted(word string, names []string) []string {
	if !lexer.IsIdentifier(word) {
		return nil
	}
	var out []string
	for _, name := range names {
		if len(name) <= len(word) || !strings.HasPrefix(name, word) {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(name[len(word):]); !identifierRune(r) {
			out = append(out, name)
		}
	}
	return out
}

// QuotingRule states why the names must be quoted, naming the characters (in the
// order met) no basic name holds; a leading digit is one such character.
func QuotingRule(names []string) string {
	var chars []string
	seen := map[rune]bool{}
	for _, name := range names {
		name, _ = strings.CutPrefix(name, globalRoot)
		for _, seg := range strings.Split(name, "::") {
			for i, r := range seg {
				if seen[r] || identifierRune(r) && !(i == 0 && r >= '0' && r <= '9') {
					continue
				}
				seen[r] = true
				chars = append(chars, "'"+string(r)+"'")
			}
		}
	}
	if len(chars) == 0 {
		return ""
	}
	return "Names containing " + OrList(chars) + " must be quoted."
}

// SimpleNames returns every simple name the index registers, sorted.
func SimpleNames(idx *symbols.Index) []string {
	if idx == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, fqn := range idx.FQNs() {
		if last := LastSegment(fqn); !seen[last] {
			seen[last] = true
			out = append(out, last)
		}
	}
	sort.Strings(out)
	return out
}

// LastSegment returns the simple name a qualified name ends in.
func LastSegment(fqn string) string {
	if cut := strings.LastIndex(fqn, "::"); cut >= 0 {
		return fqn[cut+2:]
	}
	return fqn
}

// Nearest returns the spellings closest to word by edit distance, within the
// budget a typo of that length justifies. It is what a surface with one flat
// set of equally reachable names asks — the REPL's meta commands.
func Nearest(word string, candidates []string) []string {
	return Rank(candidatesOf(Neighbours(word, candidates)))
}

// Neighbours returns the candidates within word's edit-distance budget, closest
// first, at most NeighbourLimit of them.
func Neighbours(word string, candidates []string) []Neighbour {
	budget := Budget(len([]rune(word)))
	lower := strings.ToLower(word)
	var hits []Neighbour
	for _, c := range candidates {
		if c == word {
			continue
		}
		if EditDistance(lower, strings.ToLower(c)) <= budget {
			hits = append(hits, Neighbour{Name: c, Distance: EditDistance(word, c)})
		}
	}
	return nearestFirst(hits)
}

// Neighbour is a registered name beside its distance from the one that did not
// resolve. The budget is spent case-insensitively — a wrong case is a typo —
// but Distance counts case, so a name spelled as typed is the closer guess.
type Neighbour struct {
	Name     string
	Distance int
}

// nearestFirst orders neighbours by distance then name and keeps at most
// NeighbourLimit, so what a caller then scores is bounded and deterministic.
func nearestFirst(hits []Neighbour) []Neighbour {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Distance != hits[j].Distance {
			return hits[i].Distance < hits[j].Distance
		}
		return hits[i].Name < hits[j].Name
	})
	if len(hits) > NeighbourLimit {
		hits = hits[:NeighbourLimit]
	}
	return hits
}

// candidatesOf reads neighbours as candidates reached alike, for a caller that
// has no scope to judge them from.
func candidatesOf(hits []Neighbour) []Candidate {
	out := make([]Candidate, 0, len(hits))
	for _, h := range hits {
		out = append(out, Candidate{Spelling: h.Name, Distance: h.Distance})
	}
	return out
}

// Budget is how many edits a name of n runes justifies as a typo. It scales
// with the name, since one edit of a short name reaches most short names: a
// name of two runes or fewer is too short for a typo to be identifiable.
func Budget(n int) int {
	switch {
	case n <= 2:
		return 0
	case n <= 5:
		return 1
	case n <= 8:
		return 2
	}
	return 3
}

// EditDistance is the Levenshtein distance between a and b, counting a
// transposition as one edit.
func EditDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev2 := make([]int, len(br)+1)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				if t := prev2[j-2] + 1; t < cur[j] {
					cur[j] = t
				}
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// OrList joins candidates as "a, b or c", for a message that offers a choice.
func OrList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// With appends "— did you mean …?" to a message about word, ignoring a
// candidate equal to word.
func With(msg, word string, candidates []string) string {
	return Hint(msg, word, candidates, nil)
}

// Hint is With, offering first the registered names word is the unquoted start of,
// each in Notation, and stating the QuotingRule after the question.
func Hint(msg, word string, candidates, unquoted []string) string {
	seen := map[string]bool{word: true}
	var offer []string
	add := func(spelling string) {
		if !seen[spelling] && len(offer) < Limit {
			seen[spelling] = true
			offer = append(offer, spelling)
		}
	}
	for _, name := range unquoted {
		add(Notation(name))
	}
	for _, c := range candidates {
		add(c)
	}
	if len(offer) == 0 {
		return msg
	}
	msg += " — did you mean " + OrList(offer) + "?"
	if rule := QuotingRule(unquoted); rule != "" {
		msg += " " + rule
	}
	return msg
}
