package main

import (
	"fmt"
	"math/bits"
	"sort"
)

// Bucket is the verdict for one production. Every production lands in exactly
// one bucket.
type Bucket string

const (
	// BucketEvidence means one corpus file contains every literal some way
	// through the production requires, so that input plausibly exercised it.
	BucketEvidence Bucket = "evidence"
	// BucketNoEvidence means no single input does: notation nobody has fed us.
	BucketNoEvidence Bucket = "no-evidence"
	// BucketIndistinguishable means literal search cannot decide, because the
	// production can be matched without any literal at all (delegation rules,
	// fragments like Identification) or is a lexer terminal.
	BucketIndistinguishable Bucket = "indistinguishable"
)

// Reasons a production landed in its bucket.
const (
	reasonTerminal    = "lexer terminal: matched over characters, not keywords"
	reasonLiteralFree = "every path through it can be matched without a literal"
	reasonAbsent      = "a required literal appears in no corpus file"
	reasonNotTogether = "the required literals occur in the corpora, but never in one file"
	reasonUndecidable = "only recursive paths through it: literal search cannot derive a requirement"
)

// Row is one production's verdict.
type Row struct {
	Grammar  string `json:"grammar"`
	Name     string `json:"name"`
	Kind     Kind   `json:"kind"`
	Line     int    `json:"line"`
	Override bool   `json:"override,omitempty"`
	Returns  string `json:"returns,omitempty"`
	Bucket   Bucket `json:"bucket"`
	Reason   string `json:"reason,omitempty"`
	// Literals are the literals the production's own body spells out.
	Literals []string `json:"literals,omitempty"`
	// Required are the literals of the cheapest path through the production,
	// after expanding the rules it calls.
	Required []string `json:"requiredLiterals,omitempty"`
	// Missing are required literals no corpus file contains at all.
	Missing []string `json:"missingLiterals,omitempty"`
	// File is the corpus file the evidence comes from, empty when there is none.
	File     string     `json:"file,omitempty"`
	Evidence []Citation `json:"evidence,omitempty"`
	// Branches are the production's literal-bearing forms, each with the input
	// that has its literals, if any.
	Branches []Branch `json:"branches,omitempty"`
}

// UnseenBranches returns the forms of the production no input has the literals
// for.
func (r Row) UnseenBranches() []Branch {
	var out []Branch
	for _, branch := range r.Branches {
		if !branch.Seen() {
			out = append(out, branch)
		}
	}
	return out
}

// classify decides one production's bucket from the corpus literal index.
func classify(p Production, an *analyzer, idx *literalIndex) Row {
	row := Row{
		Grammar:  p.Grammar,
		Name:     p.Name,
		Kind:     p.Kind,
		Line:     p.Line,
		Override: p.Override,
		Returns:  p.Returns,
		Literals: p.Literals(),
		Branches: classifyBranches(p, an, idx),
	}
	if p.Kind == KindTerminal {
		row.Bucket = BucketIndistinguishable
		row.Reason = reasonTerminal
		return row
	}

	// Every minimal set of literals an input must contain to take some path
	// through this production, the called rules expanded.
	paths := an.Paths(p)
	if len(paths) == 0 {
		row.Bucket = BucketIndistinguishable
		row.Reason = reasonUndecidable
		return row
	}
	if paths[0].isZero() {
		row.Bucket = BucketIndistinguishable
		row.Reason = reasonLiteralFree
		return row
	}

	// The first file, in search order, holding a whole path's literals.
	for _, file := range idx.Files() {
		for _, want := range paths {
			if !want.subsetOf(file.mask) {
				continue
			}
			row.Bucket = BucketEvidence
			row.Required = an.lits.names(want)
			row.File = file.path
			for _, lit := range row.Required {
				row.Evidence = append(row.Evidence, Citation{
					Literal: lit, Root: file.root, File: file.path, Line: file.lines[lit],
				})
			}
			return row
		}
	}

	// No single file suffices, so report the closest path: the one fewest of
	// whose literals are missing corpus-wide.
	row.Bucket = BucketNoEvidence
	closest, missing := an.closest(paths, idx.mask)
	row.Required = an.lits.names(closest)
	row.Missing = an.lits.names(missing)
	row.Reason = reasonNotTogether
	if !missing.isZero() {
		row.Reason = reasonAbsent
	}
	for _, lit := range row.Required {
		if citation, ok := idx.Citation(lit); ok {
			row.Evidence = append(row.Evidence, citation)
		}
	}
	sort.Slice(row.Evidence, func(i, j int) bool {
		return row.Evidence[i].Literal < row.Evidence[j].Literal
	})
	return row
}

// Branch is one literal-bearing alternative or optional group inside a
// production's own body: a form of the notation an input can spell out, taken
// together with the literals every path through the production needs anyway. A
// production lands in the evidence bucket as soon as its cheapest form is
// present, so branches are where the notation nobody has fed us shows up.
type Branch struct {
	// Literals are the literals an input must contain to take this form.
	Literals []string `json:"literals"`
	// File and Line cite the input that has them, empty when none does.
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	// Missing are the form's literals no corpus file contains at all; when it is
	// empty and the form is unseen, the literals just never occur together.
	Missing []string `json:"missingLiterals,omitempty"`
}

// Seen reports whether a corpus input has the branch's literals.
func (b Branch) Seen() bool { return b.File != "" }

// classifyBranches decides, per literal-bearing form of the production, whether
// an input has its literals.
func classifyBranches(p Production, an *analyzer, idx *literalIndex) []Branch {
	if p.Kind == KindTerminal {
		return nil
	}
	var out []Branch
	for _, paths := range an.branches(p, intersect(an.Paths(p))) {
		branch := Branch{Literals: an.lits.names(paths[0])}
		for _, file := range idx.Files() {
			want, ok := firstSubset(paths, file.mask)
			if !ok {
				continue
			}
			branch.Literals = an.lits.names(want)
			branch.File = file.path
			for _, lit := range branch.Literals {
				if line := file.lines[lit]; line > branch.Line {
					branch.Line = line
				}
			}
			break
		}
		if !branch.Seen() {
			branch.Missing = an.lits.names(paths[0].andNot(idx.mask))
		}
		out = append(out, branch)
	}
	return out
}

// firstSubset returns the first path the file's literals satisfy.
func firstSubset(paths []mask, have mask) (mask, bool) {
	for _, want := range paths {
		if want.subsetOf(have) {
			return want, true
		}
	}
	return mask{}, false
}

// branches enumerates the literal-bearing alternatives and optional groups of
// the production's own body, cheapest form first, deduplicated. Each entry is
// that form's own path set.
// core is the literals every path through the production needs, which an input
// taking any of its forms must contain too.
func (an *analyzer) branches(p Production, core mask) [][]mask {
	var out [][]mask
	seen := map[mask]bool{}
	var walk func(e expr)
	walk = func(e expr) {
		switch v := e.(type) {
		case seqExpr:
			for _, item := range v.Items {
				walk(item)
			}
		case altExpr:
			for _, item := range v.Items {
				an.addBranch(p.Grammar, item, core, seen, &out)
				walk(item)
			}
		case optExpr:
			an.addBranch(p.Grammar, v.Item, core, seen, &out)
			walk(v.Item)
		}
	}
	walk(p.Body)
	return out
}

// addBranch records one form's path set, unless it needs no literal of its own or
// a cheaper identical form is already recorded.
func (an *analyzer) addBranch(grammar string, e expr, core mask, seen map[mask]bool, out *[][]mask) {
	paths := an.paths(grammar, e)
	if len(paths) == 0 || paths[0].isZero() {
		return
	}
	with := make([]mask, 0, len(paths))
	for _, path := range paths {
		with = append(with, path.or(core))
	}
	with = prune(with)
	if seen[with[0]] {
		return
	}
	seen[with[0]] = true
	*out = append(*out, with)
}

// maxPaths caps how many alternative literal sets are tracked per production.
// Bodies with many independent alternatives would otherwise multiply out; the
// cheapest sets are kept, which is what evidence is decided on.
const maxPaths = 24

// analyzer answers what literals a production requires, expanding the rules it
// calls so that a rule delegating to a keyword rule is credited with that
// keyword.
type analyzer struct {
	lits *litTable
	// byGrammar indexes productions per grammar file, and chain gives each
	// grammar the grammars it inherits rules from, nearest first.
	byGrammar map[string]map[string]Production
	chain     map[string][]string
	memo      map[string][]mask
	active    map[string]bool
	// cuts counts recursive paths dropped so far, so a result that depended on
	// dropping one is recomputed rather than cached under a partial answer.
	cuts int
}

// newAnalyzer indexes the productions for rule resolution. A grammar's own
// rules win over the inherited ones it names with `with`, which is what Xtext's
// @Override rules rely on.
func newAnalyzer(grammars []*Grammar, lits *litTable) *analyzer {
	an := &analyzer{
		lits:      lits,
		byGrammar: map[string]map[string]Production{},
		chain:     map[string][]string{},
		memo:      map[string][]mask{},
		active:    map[string]bool{},
	}
	byDeclared := map[string]*Grammar{}
	for _, g := range grammars {
		byDeclared[g.Declared] = g
	}
	for _, g := range grammars {
		index := map[string]Production{}
		for _, p := range g.Productions {
			index[p.Name] = p
		}
		an.byGrammar[g.Name] = index
		seen := map[string]bool{g.Name: true}
		for at := byDeclared[g.Extends]; at != nil && !seen[at.Name]; at = byDeclared[at.Extends] {
			seen[at.Name] = true
			an.chain[g.Name] = append(an.chain[g.Name], at.Name)
		}
	}
	return an
}

// Paths returns the minimal literal sets, cheapest first, that an input must
// contain to take some path through the production. A zero set means a path
// needs no literal at all.
func (an *analyzer) Paths(p Production) []mask {
	key := p.Grammar + "::" + p.Name
	if cached, ok := an.memo[key]; ok {
		return cached
	}
	if an.active[key] {
		// Recursion: a path that comes back here has not reached a base case, so
		// it is not a path at all.
		an.cuts++
		return nil
	}
	an.active[key] = true
	before := an.cuts
	paths := an.paths(p.Grammar, p.Body)
	delete(an.active, key)
	if an.cuts == before {
		an.memo[key] = paths
	}
	return paths
}

// paths computes the minimal literal sets of one body expression.
func (an *analyzer) paths(grammar string, e expr) []mask {
	switch v := e.(type) {
	case litExpr:
		return []mask{an.lits.mask(v.Value)}
	case refExpr:
		called, ok := an.lookup(grammar, v.Name)
		if !ok {
			// A cross-reference, an action or a terminal: no literal of its own.
			return []mask{{}}
		}
		return an.Paths(called)
	case optExpr:
		// Optional and zero-or-more parts are treated as not taken, keeping the
		// requirement a lower bound on what an input must contain.
		return []mask{{}}
	case seqExpr:
		out := []mask{{}}
		for _, item := range v.Items {
			out = combine(out, an.paths(grammar, item))
		}
		return out
	case altExpr:
		var out []mask
		for _, item := range v.Items {
			out = append(out, an.paths(grammar, item)...)
		}
		return prune(out)
	}
	return []mask{{}}
}

// lookup resolves a rule name in the referring grammar, then in the grammars it
// inherits from.
func (an *analyzer) lookup(grammar, name string) (Production, bool) {
	if p, ok := an.byGrammar[grammar][name]; ok {
		return p, true
	}
	for _, inherited := range an.chain[grammar] {
		if p, ok := an.byGrammar[inherited][name]; ok {
			return p, true
		}
	}
	return Production{}, false
}

// closest returns the path fewest of whose literals are absent from the corpora,
// with those absent literals.
func (an *analyzer) closest(paths []mask, corpus mask) (chosen, missing mask) {
	best := -1
	for _, want := range paths {
		gap := want.andNot(corpus)
		if n := gap.count(); best < 0 || n < best {
			best, chosen, missing = n, want, gap
		}
	}
	return chosen, missing
}

// combine returns the pairwise unions of two path sets: a sequence requires the
// literals of both of its parts.
func combine(left, right []mask) []mask {
	out := make([]mask, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			out = append(out, l.or(r))
		}
	}
	return prune(out)
}

// prune drops duplicate and redundant path sets, keeping the cheapest. A set
// that contains another is redundant: whenever it is satisfied, so is the
// smaller one it contains.
func prune(in []mask) []mask {
	sort.Slice(in, func(i, j int) bool { return in[i].less(in[j]) })
	out := make([]mask, 0, len(in))
	for _, candidate := range in {
		redundant := false
		for _, kept := range out {
			if kept.subsetOf(candidate) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, candidate)
		}
		if len(out) == maxPaths {
			break
		}
	}
	return out
}

// intersect returns the literals every one of the path sets requires.
func intersect(in []mask) mask {
	if len(in) == 0 {
		return mask{}
	}
	out := in[0]
	for _, m := range in[1:] {
		out = out.and(m)
	}
	return out
}

// maskWords bounds how many distinct literals the grammars may use.
const maskWords = 16

// mask is a set of grammar literals, one bit per literal.
type mask [maskWords]uint64

func (m mask) and(o mask) mask {
	for i := range m {
		m[i] &= o[i]
	}
	return m
}

func (m mask) or(o mask) mask {
	for i := range m {
		m[i] |= o[i]
	}
	return m
}

// andNot returns the literals of m that o lacks.
func (m mask) andNot(o mask) mask {
	for i := range m {
		m[i] &^= o[i]
	}
	return m
}

func (m mask) subsetOf(o mask) bool {
	for i := range m {
		if m[i]&^o[i] != 0 {
			return false
		}
	}
	return true
}

func (m mask) isZero() bool {
	for _, w := range m {
		if w != 0 {
			return false
		}
	}
	return true
}

func (m mask) count() int {
	n := 0
	for _, w := range m {
		n += bits.OnesCount64(w)
	}
	return n
}

// less orders masks by size, then by their bits, so path sets are deterministic.
func (m mask) less(o mask) bool {
	if a, b := m.count(), o.count(); a != b {
		return a < b
	}
	for i := range m {
		if m[i] != o[i] {
			return m[i] < o[i]
		}
	}
	return false
}

// litTable assigns each grammar literal a bit position.
type litTable struct {
	bit   map[string]int
	order []string
}

// newLitTable numbers the literals in sorted order, so bit positions and every
// report derived from them are stable.
func newLitTable(literals []string) (*litTable, error) {
	order := dedupe(literals)
	if len(order) > maskWords*64 {
		return nil, fmt.Errorf("%d grammar literals exceed the %d this tool tracks: raise maskWords",
			len(order), maskWords*64)
	}
	t := &litTable{bit: make(map[string]int, len(order)), order: order}
	for i, lit := range order {
		t.bit[lit] = i
	}
	return t, nil
}

// mask returns the one-literal set, or the empty set for an unknown literal.
func (t *litTable) mask(literal string) mask {
	var m mask
	if bit, ok := t.bit[literal]; ok {
		m[bit/64] |= 1 << (bit % 64)
	}
	return m
}

// maskOf returns the set of those literals the predicate accepts.
func (t *litTable) maskOf(has func(string) bool) mask {
	var m mask
	for _, lit := range t.order {
		if has(lit) {
			m = m.or(t.mask(lit))
		}
	}
	return m
}

// names returns the literals of a set, sorted.
func (t *litTable) names(m mask) []string {
	var out []string
	for _, lit := range t.order {
		if t.mask(lit).subsetOf(m) {
			out = append(out, lit)
		}
	}
	return out
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
