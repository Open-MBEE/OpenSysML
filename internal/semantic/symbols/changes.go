package symbols

// Changes is what a run of writes to an index changed, at the granularity a ReadRecorder
// records reads at: names, namespaces and documents. Reads meeting none saw nothing move.
type Changes struct {
	Names      map[string]bool
	Namespaces map[string]bool
	Docs       map[string]bool
	// Spellings reports that the set of names the index registers changed: a
	// name came or went, or is spelled otherwise. A name registered again under
	// the same spelling leaves it unset, whatever symbol it now names.
	Spellings bool
}

// Empty reports whether nothing changed.
func (c Changes) Empty() bool {
	return len(c.Names) == 0 && len(c.Namespaces) == 0 && len(c.Docs) == 0
}

// Registered reports whether the index's own tables changed: every write to
// them records a namespace or a document, so names alone are a caller's.
func (c Changes) Registered() bool {
	return len(c.Namespaces) > 0 || len(c.Docs) > 0
}

func newChanges() *Changes {
	return &Changes{Names: map[string]bool{}, Namespaces: map[string]bool{}, Docs: map[string]bool{}}
}

// TrackChanges starts recording what writes to the index change, for
// TakeChanges to hand out. An index that is never asked records nothing.
func (idx *Index) TrackChanges() {
	if idx.changes == nil {
		idx.changes = newChanges()
	}
}

// TakeChanges returns what changed since the last call and starts over. A
// name registered again exactly as it was (the same symbols, re-exported and
// hidden alike, claimed by the same documents on the same routes) is left
// out: a document replaced by one declaring the same names re-registers every
// re-export its wildcard imports surface, and none of them reads differently.
func (idx *Index) TakeChanges() Changes {
	if idx.changes == nil {
		return Changes{}
	}
	out := *idx.changes
	for fqn := range out.Names {
		before, noted := idx.changesBefore[fqn]
		now := registrationOf(idx, fqn)
		if !noted || before.spelling(fqn) != now.spelling(fqn) {
			out.Spellings = true
		}
		if noted && before.same(now) {
			delete(out.Names, fqn)
		}
	}
	idx.changes = newChanges()
	idx.changesBefore = nil
	return out
}

// noteBefore keeps how fqn was registered before the first write to it since
// the last TakeChanges, for TakeChanges to tell a name that changed from one
// registered again as it was. Callers note before they write.
func (idx *Index) noteBefore(fqn string) {
	if idx.changes == nil {
		return
	}
	if _, noted := idx.changesBefore[fqn]; noted {
		return
	}
	if idx.changesBefore == nil {
		idx.changesBefore = map[string]registration{}
	}
	idx.changesBefore[fqn] = registrationOf(idx, fqn)
}

// registration is everything a lookup of a name reads from the index's
// tables: the symbols registered under it, in order, each with its re-export
// and hidden marks and the claims and routes that surfaced it. It holds the
// symbols themselves, so one dropped since cannot have its address reused by
// a new symbol that would then compare equal to it.
type registration []registeredSymbol

type registeredSymbol struct {
	sym                *Symbol
	reexported, hidden bool
	claims             map[string]reexportClaim
}

func registrationOf(idx *Index, fqn string) registration {
	syms := idx.fqn.at(fqn)
	if len(syms) == 0 {
		return nil
	}
	reexported, _ := idx.reexported.get(fqn)
	hidden, _ := idx.hidden.get(fqn)
	out := make(registration, len(syms))
	for i, sym := range syms {
		out[i] = registeredSymbol{sym: sym, reexported: reexported.has(sym), hidden: hidden.has(sym)}
		if claims := idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym}); len(claims) > 0 {
			out[i].claims = make(map[string]reexportClaim, len(claims))
			for doc, claim := range claims {
				out[i].claims[doc] = reexportClaim{public: claim.public, routes: append([]gateRoute(nil), claim.routes...)}
			}
		}
	}
	return out
}

// spelling is the simple name a registration is filed under, as a suggestion
// table files it (suggest.simpleName): the declared name of the first symbol
// registered under fqn as its own, else fqn's last segment; "" when nothing is
// registered there.
func (r registration) spelling(fqn string) string {
	if len(r) == 0 {
		return ""
	}
	for _, entry := range r {
		if entry.sym != nil && HasFQN(entry.sym, fqn) {
			if entry.sym.Name != "" {
				return entry.sym.Name
			}
			break
		}
	}
	return LastSegment(fqn)
}

// same reports whether two registrations read alike: the same symbols, by
// identity, so a declaration parsed again is a change, with the same marks,
// claims and routes, a route's filters compared by the expression and scope
// they were written with.
func (r registration) same(other registration) bool {
	if len(r) != len(other) {
		return false
	}
	for i, a := range r {
		b := other[i]
		if a.sym != b.sym || a.reexported != b.reexported || a.hidden != b.hidden || len(a.claims) != len(b.claims) {
			return false
		}
		for doc, claim := range a.claims {
			them, ok := b.claims[doc]
			if !ok || claim.public != them.public || len(claim.routes) != len(them.routes) {
				return false
			}
			for k, route := range claim.routes {
				theirs := them.routes[k]
				if route.private != theirs.private || len(route.filters) != len(theirs.filters) {
					return false
				}
				for f, filter := range route.filters {
					if filter.Expr != theirs.filters[f].Expr || filter.Scope != theirs.filters[f].Scope {
						return false
					}
				}
			}
		}
	}
	return true
}

func (idx *Index) changedName(fqn string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Names[fqn] = true
	parent, _ := splitFQN(fqn)
	idx.changes.Namespaces[parent] = true
}

func (idx *Index) changedNamespace(fqn string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Namespaces[fqn] = true
}

func (idx *Index) changedDoc(name string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Docs[name] = true
}

// A ReadRecorder is told what an index read is about — a name looked up, a namespace
// enumerated or its children read, a document's root or kind, the whole table.
type ReadRecorder interface {
	ReadName(fqn string)
	ReadNamespace(fqn string)
	ReadDocument(name string)
	ReadAllNames()
	ReadSegment(name string)
}

// SetReadRecorder installs the recorder the index's reads report to, or none.
func (idx *Index) SetReadRecorder(r ReadRecorder) {
	idx.reads = r
}

// Recording returns a view of the index whose reads report to r: the same
// tables, caches and change log as idx, so a batch worker reading the index
// beside others records what its analysis read without one recorder for all.
// The view is for reading only; a write to it is a write to idx.
func (idx *Index) Recording(r ReadRecorder) *Index {
	view := *idx
	view.reads = r
	return &view
}

func (idx *Index) readName(fqn string) {
	if idx.reads != nil {
		idx.reads.ReadName(fqn)
	}
}

func (idx *Index) readNamespace(fqn string) {
	if idx.reads != nil {
		idx.reads.ReadNamespace(fqn)
	}
}

func (idx *Index) readDocument(name string) {
	if idx.reads != nil {
		idx.reads.ReadDocument(name)
	}
}

func (idx *Index) readAllNames() {
	if idx.reads != nil {
		idx.reads.ReadAllNames()
	}
}

func (idx *Index) readSegment(name string) {
	if idx.reads != nil {
		idx.reads.ReadSegment(name)
	}
}
