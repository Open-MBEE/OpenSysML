package symbols

import "slices"

// GatheredRelationships are the relationships a document's body states that
// the workspace-wide audits read across documents — what it derives,
// allocates, satisfies and conforms — held by reference. A recorded document
// carries them in place of the usages stating them; the audits resolve and
// judge the ends when they gather, as they do a loaded document's.
type GatheredRelationships struct {
	// Derivations: Sources are the referents of the `#original` ends of a
	// `#derivation` connection, Targets those of its `#derive` ends.
	Derivations []GatheredEnds
	// Allocations: Sources are what an allocation allocates, Targets where to.
	Allocations []GatheredEnds
	// Conformances: Sources are the referents of the `#conformsTo` ends of a
	// `#conformance` connection, Targets those of its `#conformant` ends.
	Conformances  []GatheredEnds
	Satisfactions []GatheredSatisfaction
	// SentSignals are the signal names the body's `send` actions send, sorted.
	SentSignals []string
}

// GatheredEnds are the two sides of one gathered relationship.
type GatheredEnds struct {
	Sources, Targets []ElementRef
}

// GatheredSatisfaction is one `satisfy` that is neither negated nor a verify:
// the requirements it states satisfied — itself when it declares its
// requirement, else those it subsets — and what satisfies them — the subjects
// after `by`, else the feature whose body declares it.
type GatheredSatisfaction struct {
	Requirements []ElementRef
	Satisfiers   []ElementRef
}

// Clone returns a copy sharing no slice with g; nil for nil.
func (g *GatheredRelationships) Clone() *GatheredRelationships {
	if g == nil {
		return nil
	}
	ends := func(in []GatheredEnds) []GatheredEnds {
		out := slices.Clone(in)
		for i := range out {
			out[i].Sources = cloneRefs(out[i].Sources)
			out[i].Targets = cloneRefs(out[i].Targets)
		}
		return out
	}
	out := &GatheredRelationships{
		Derivations:   ends(g.Derivations),
		Allocations:   ends(g.Allocations),
		Conformances:  ends(g.Conformances),
		Satisfactions: slices.Clone(g.Satisfactions),
		SentSignals:   slices.Clone(g.SentSignals),
	}
	for i := range out.Satisfactions {
		out.Satisfactions[i].Requirements = cloneRefs(out.Satisfactions[i].Requirements)
		out.Satisfactions[i].Satisfiers = cloneRefs(out.Satisfactions[i].Satisfiers)
	}
	return out
}

// Empty reports whether g states no relationship.
func (g *GatheredRelationships) Empty() bool {
	return g == nil || len(g.Derivations)+len(g.Allocations)+len(g.Conformances)+len(g.Satisfactions)+len(g.SentSignals) == 0
}

// Gathered returns the relationships doc's record carries, nil for a document
// whose usages state them.
func (idx *Index) Gathered(doc string) *GatheredRelationships {
	idx.readDocument(doc)
	return idx.gathered.at(doc)
}

// Elements restores the referenced elements, dropping any a reference no
// longer reaches.
func (idx *Index) Elements(refs []ElementRef) []*Symbol {
	var out []*Symbol
	for _, ref := range refs {
		if sym := idx.Element(ref); sym != nil {
			out = append(out, sym)
		}
	}
	return out
}
