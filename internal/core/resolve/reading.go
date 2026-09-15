package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// readingKey keys a reading of a qualified name in a scope its reader chose,
// which may not be the one the name was written in.
type readingKey struct {
	scope *symbols.Scope
	qn    *ast.QualifiedName
}

// Reading is what a qualified name named when read in a chosen scope, with the
// per-segment records and ambiguity a consumer of ResolveQualified would ask
// the resolver for afterwards: see ReadQualified.
type Reading struct {
	sym       *symbols.Symbol
	ok        bool
	parts     []*symbols.Symbol
	aliases   []*symbols.Symbol
	ambiguity int
}

// Symbol returns the element the name reached, or false when it reached none.
func (rd Reading) Symbol() (*symbols.Symbol, bool) {
	return rd.sym, rd.ok
}

// Part returns the element segment i named, as PartSymbol does for a written name.
func (rd Reading) Part(i int) (*symbols.Symbol, bool) {
	if i < 0 || i >= len(rd.parts) || rd.parts[i] == nil {
		return nil, false
	}
	return rd.parts[i], true
}

// Alias returns the alias membership segment i was written as, as PartAlias does.
func (rd Reading) Alias(i int) (*symbols.Symbol, bool) {
	if i < 0 || i >= len(rd.aliases) || rd.aliases[i] == nil {
		return nil, false
	}
	return rd.aliases[i], true
}

// Ambiguity returns how many elements the name named when it failed for naming
// several, as Ambiguity does for a written name.
func (rd Reading) Ambiguity() (int, bool) {
	return rd.ambiguity, rd.ambiguity > 0
}

// ReadQualified resolves qn as if it were written in scope, the way an
// expression evaluated in a chosen scope reads its names. A written name is
// read in one scope, so ResolveQualified memoizes by node; an expression may be
// evaluated in several, so a reading is memoized by scope and node, and leaves
// the node's own records untouched.
func (r *Resolver) ReadQualified(scope *symbols.Scope, qn *ast.QualifiedName) Reading {
	if qn == nil {
		return Reading{}
	}
	r.EnterDoc(symbols.DocNameOf(scope))
	defer r.LeaveDoc()
	key := readingKey{scope: scope, qn: qn}
	if rd, done := r.readings[key]; done {
		return rd
	}
	if depth := r.resolving[qn]; depth != 0 {
		r.CutShort(depth)
		return Reading{}
	}
	r.resolving[qn] = r.Enter()
	// The walk records its segments where the written name keeps its own, so
	// those are set aside, read off once it is done, and put back.
	saved := r.saveSegments(qn)
	r.clearSegments(qn)
	var res resolution
	r.aside(func() { res = r.walkQualified(scope, qn, (*refFilter)(nil).hiding(qn)) })
	rd := r.segmentReading(qn, res.sym, res.ok)
	r.restoreSegments(qn, saved)
	delete(r.resolving, qn)
	if r.Leave() && r.inCondition == 0 && r.allVisible == 0 {
		journalNew(r, r.readings, key, qn)
		r.readings[key] = rd
	}
	return rd
}

// ProbeReading is ProbeReference with what each segment of ref.QN reached, a
// failed reading's segments included. ref.QN should be a fresh node (see
// Reference.Spelled): its segment records are read off and left cleared.
func (r *Resolver) ProbeReading(ref Reference) Reading {
	if ref.QN == nil {
		return Reading{}
	}
	saved := r.saveSegments(ref.QN)
	r.clearSegments(ref.QN)
	outer := r.trial
	r.trial = ref.QN
	sym, ok := r.ProbeReference(ref)
	r.trial = outer
	rd := r.segmentReading(ref.QN, sym, ok)
	r.restoreSegments(ref.QN, saved)
	return rd
}

// segmentReading is the Reading of qn from the records its walk just left.
func (r *Resolver) segmentReading(qn *ast.QualifiedName, sym *symbols.Symbol, ok bool) Reading {
	return Reading{
		sym: sym, ok: ok, parts: r.parts[qn], aliases: r.aliasNames[qn], ambiguity: r.ambiguities[qn],
	}
}
