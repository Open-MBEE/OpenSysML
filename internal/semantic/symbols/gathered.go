package symbols

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

// Empty reports whether g states no relationship.
func (g *GatheredRelationships) Empty() bool {
	return g == nil || len(g.Derivations)+len(g.Allocations)+len(g.Conformances)+len(g.Satisfactions) == 0
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
