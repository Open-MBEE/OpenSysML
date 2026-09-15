package symbols

// Changes is what a run of writes to an index changed, at the granularity a ReadRecorder
// records reads at: names, namespaces and documents. Reads meeting none saw nothing move.
type Changes struct {
	Names      map[string]bool
	Namespaces map[string]bool
	Docs       map[string]bool
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

// TakeChanges returns what changed since the last call and starts over.
func (idx *Index) TakeChanges() Changes {
	if idx.changes == nil {
		return Changes{}
	}
	out := *idx.changes
	idx.changes = newChanges()
	return out
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
}

// SetReadRecorder installs the recorder the index's reads report to, or none.
func (idx *Index) SetReadRecorder(r ReadRecorder) {
	idx.reads = r
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
